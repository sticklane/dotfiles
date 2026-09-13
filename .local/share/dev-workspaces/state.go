package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const GiB uint64 = 1 << 30

type Config struct {
	Root           string `json:"root"`
	VolumeUUID     string `json:"volume_uuid"`
	BackingPath    string `json:"backing_path"`
	BackingUUID    string `json:"backing_uuid"`
	Image          string `json:"image"`
	StateDir       string `json:"state_dir"`
	InternalPath   string `json:"internal_path"`
	PoolMinGiB     uint64 `json:"pool_min_gib"`
	BackingMinGiB  uint64 `json:"backing_min_gib"`
	InternalMinGiB uint64 `json:"internal_min_gib"`
	ReserveGiB     uint64 `json:"reserve_gib"`
	MaxWorkspaces  int    `json:"max_workspaces"`
	RetentionHours int    `json:"retention_hours"`
}

type Lease struct {
	PID   int       `json:"pid"`
	Start string    `json:"start"`
	Seen  time.Time `json:"seen"`
}
type Workspace struct {
	ID       string           `json:"id"`
	Path     string           `json:"path"`
	GitDir   string           `json:"git_dir"`
	Common   string           `json:"common_dir"`
	Branch   string           `json:"branch"`
	Created  time.Time        `json:"created"`
	Finished time.Time        `json:"finished,omitempty"`
	Head     string           `json:"finished_head,omitempty"`
	Pinned   bool             `json:"pinned"`
	Removing bool             `json:"removing"`
	Missing  bool             `json:"missing"`
	Leases   map[string]Lease `json:"leases"`
}
type State struct {
	Version        int                   `json:"version"`
	Workspaces     map[string]*Workspace `json:"workspaces"`
	Pending        map[string]time.Time  `json:"pending"`
	LastCachePrune time.Time             `json:"last_cache_prune"`
	NeedsCompact   bool                  `json:"needs_compact"`
}
type Manager struct {
	cfg    Config
	now    func() time.Time
	verify func() error
	space  func(string) (uint64, error)
}

func NewManager(c Config) *Manager {
	m := &Manager{cfg: c, now: time.Now, space: freeSpace}
	m.verify = m.verifyVolumes
	return m
}
func newState() *State {
	return &State{Version: 1, Workspaces: map[string]*Workspace{}, Pending: map[string]time.Time{}}
}
func (m *Manager) statePath() string { return filepath.Join(m.cfg.StateDir, "state.json") }
func (m *Manager) ReadState() (*State, error) {
	s := newState()
	b, e := os.ReadFile(m.statePath())
	if errors.Is(e, os.ErrNotExist) {
		return s, nil
	}
	if e != nil {
		return nil, e
	}
	if e = json.Unmarshal(b, s); e != nil {
		return nil, e
	}
	if s.Version != 1 || s.Workspaces == nil || s.Pending == nil {
		return nil, fmt.Errorf("unsupported or incomplete registry")
	}
	return s, nil
}
func atomicWrite(path string, b []byte) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".write-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}
func (m *Manager) update(fn func(*State) error) error {
	if e := os.MkdirAll(m.cfg.StateDir, 0700); e != nil {
		return e
	}
	f, e := os.OpenFile(filepath.Join(m.cfg.StateDir, "state.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); e != nil {
		return e
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	s, e := m.ReadState()
	if e != nil {
		return e
	}
	if e = fn(s); e != nil {
		return e
	}
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	return atomicWrite(m.statePath(), append(b, '\n'))
}
func randomID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
func admissionKey(common, branch string) string {
	s := sha256.Sum256([]byte(common + "\x00" + branch))
	return hex.EncodeToString(s[:])
}
func inside(root, path string) bool {
	rel, e := filepath.Rel(root, path)
	return e == nil && rel != "." && rel != ".." && !filepath.IsAbs(rel) && len(rel) > 0 && rel[:min(3, len(rel))] != "../"
}
func (m *Manager) managedPath(path string) (string, error) {
	p, e := filepath.Abs(path)
	if e != nil {
		return "", e
	}
	p = filepath.Clean(p)
	r, e := filepath.EvalSymlinks(p)
	if e != nil {
		return "", e
	}
	if p != r || !inside(filepath.Join(m.cfg.Root, "worktrees"), p) {
		return "", fmt.Errorf("not a managed workspace: %s", p)
	}
	return p, nil
}
func expirePending(s *State, now time.Time) {
	for k, t := range s.Pending {
		if now.Sub(t) > 10*time.Minute {
			delete(s.Pending, k)
		}
	}
}
