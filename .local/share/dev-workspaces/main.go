package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

func loadConfig() (Config, error) {
	h, e := os.UserHomeDir()
	if e != nil {
		return Config{}, e
	}
	p := os.Getenv("DEV_WORKSPACE_CONFIG")
	if p == "" {
		p = filepath.Join(h, ".config/dev-workspaces/config.json")
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return Config{}, e
	}
	var c Config
	if e = json.Unmarshal(b, &c); e != nil {
		return c, e
	}
	if c.Root == "" || c.Root == "/" || c.VolumeUUID == "" || c.BackingUUID == "" || c.StateDir == "" || c.MaxWorkspaces < 1 || c.ReserveGiB < 1 || c.ReserveGiB > 1024 || c.RetentionHours < 1 {
		return c, fmt.Errorf("invalid workspace configuration")
	}
	for _, p := range []string{c.Root, c.BackingPath, c.Image, c.StateDir, c.InternalPath} {
		if !filepath.IsAbs(p) {
			return c, fmt.Errorf("configuration paths must be absolute")
		}
	}
	return c, nil
}
func currentWorkspace() string {
	p, e := git("", "rev-parse", "--show-toplevel")
	if e == nil {
		return p
	}
	p, _ = os.Getwd()
	return p
}
func printJSON(v any) error {
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	return e.Encode(v)
}
func main() {
	if e := entry(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, "dev-workspace:", e)
		os.Exit(1)
	}
}
func entry(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Println(`dev-workspace start <claude|codex|gemini> <branch> [-- agent arguments]
dev-workspace run <command> [arguments]    Run with a lease and managed caches
dev-workspace finish [path]              Mark preserved work for cleanup after 24 hours
dev-workspace pin|unpin [path]           Control explicit retention
dev-workspace status                    Show budgets, registry, and disk use
dev-workspace gc [--apply]               Preview/apply cleanup of completed work only
dev-workspace service                   Mount, reconcile, clean completed work and native caches
dev-workspace mount                     Mount the configured APFS image
dev-workspace compact                   Reclaim sparse-image space when the pool has no workspaces
dev-workspace hook <admit|register|guard|removed>  Worktrunk lifecycle hooks`)
		return nil
	}
	c, e := loadConfig()
	if e != nil {
		return e
	}
	m := NewManager(c)
	path := currentWorkspace()
	if len(args) > 1 {
		path = args[1]
	}
	switch args[0] {
	case "mount":
		return m.Mount()
	case "compact":
		return m.Compact()
	case "start":
		if len(args) < 3 {
			return fmt.Errorf("start requires agent and branch")
		}
		return m.Start(args[1], args[2], trimSeparator(args[3:]))
	case "run":
		if len(args) < 2 {
			return fmt.Errorf("run requires a command")
		}
		return m.Run(currentWorkspace(), args[1:])
	case "finish":
		return m.Finish(path)
	case "pin":
		return m.Pin(path, true)
	case "unpin":
		return m.Pin(path, false)
	case "status":
		return m.Status()
	case "gc":
		if len(args) > 2 || (len(args) == 2 && args[1] != "--apply") {
			return fmt.Errorf("usage: gc [--apply]")
		}
		return m.GC(len(args) == 2)
	case "service":
		return m.Service()
	case "hook":
		if len(args) != 2 {
			return fmt.Errorf("hook name required")
		}
		return m.Hook(args[1])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (m *Manager) Service() error {
	log := filepath.Join(m.cfg.StateDir, "service.log")
	if info, e := os.Stat(log); e == nil && info.Size() > 1<<20 {
		_ = os.Rename(log, log+".1")
	}
	defer m.Status()
	if e := m.Mount(); e != nil {
		return e
	}
	if e := m.GC(true); e != nil {
		return e
	}
	if e := m.PruneCaches(); e != nil {
		return e
	}
	s, e := m.ReadState()
	if e != nil {
		return e
	}
	if s.NeedsCompact && len(s.Workspaces) == 0 && len(s.Pending) == 0 {
		return m.Compact()
	}
	return nil
}

func (m *Manager) Compact() error {
	if e := m.verify(); e != nil {
		return e
	}
	return m.update(func(s *State) error {
		expirePending(s, m.now())
		if len(s.Workspaces) > 0 || len(s.Pending) > 0 {
			return fmt.Errorf("compaction waits until every managed workspace has been released")
		}
		if e := openFiles(m.cfg.Root); e != nil {
			return e
		}
		// All managed admissions and leases hold this same lock. Detach is never forced.
		if _, e := command("", "/usr/bin/hdiutil", "detach", m.cfg.Root); e != nil {
			return e
		}
		_, compactErr := command("", "/usr/bin/hdiutil", "compact", m.cfg.Image)
		// Restore access even if compaction is interrupted or the image format refuses it.
		_, attachErr := command("", "/usr/bin/hdiutil", "attach", "-nobrowse", "-mountpoint", m.cfg.Root, m.cfg.Image)
		if attachErr != nil {
			return attachErr
		}
		if e := m.verify(); e != nil {
			return e
		}
		if compactErr != nil {
			return compactErr
		}
		s.NeedsCompact = false
		return nil
	})
}
func trimSeparator(args []string) []string {
	if len(args) > 0 && args[0] == "--" {
		return args[1:]
	}
	return args
}
func (m *Manager) Hook(name string) error {
	var ctx struct {
		Repo   string `json:"repo_path"`
		Branch string `json:"branch"`
		Path   string `json:"worktree_path"`
	}
	if e := json.NewDecoder(os.Stdin).Decode(&ctx); e != nil {
		return e
	}
	switch name {
	case "admit":
		if ctx.Branch == "" {
			return nil
		}
		return m.Admit(ctx.Repo, ctx.Branch)
	case "register":
		return m.Register(ctx.Path)
	case "guard":
		// Worktrunk may display legacy worktrees, but this controller never adopts them.
		if !inside(filepath.Join(m.cfg.Root, "worktrees"), ctx.Path) {
			return fmt.Errorf("legacy worktree: use its existing explicit removal procedure")
		}
		return m.Guard(ctx.Path)
	case "removed":
		return m.Removed(ctx.Path)
	default:
		return fmt.Errorf("unknown hook")
	}
}
func (m *Manager) Start(agent, branch string, args []string) error {
	switch agent {
	case "claude", "codex", "gemini":
	default:
		return fmt.Errorf("agent must be claude, codex, or gemini")
	}
	if e := m.Mount(); e != nil {
		return e
	}
	repo, e := os.Getwd()
	if e != nil {
		return e
	}
	// Admission also runs through the global Worktrunk hook, including Claude native isolation.
	if e = m.Admit(repo, branch); e != nil {
		return e
	}
	c := exec.Command("/opt/homebrew/bin/wt", "switch", "--create", branch, "--format=json")
	c.Dir = repo
	c.Stderr = os.Stderr
	b, e := c.Output()
	if e != nil {
		return e
	}
	var out struct {
		Path string `json:"path"`
	}
	if e = json.Unmarshal(b, &out); e != nil {
		return e
	}
	if out.Path == "" {
		return fmt.Errorf("Worktrunk did not return a path")
	}
	return m.Run(out.Path, append([]string{agent}, args...))
}
func (m *Manager) environment(path string) ([]string, error) {
	build := filepath.Join(path, ".dev-build")
	cache := filepath.Join(m.cfg.Root, "caches")
	overrides := map[string]string{"TMPDIR": filepath.Join(build, "tmp") + "/", "GOTMPDIR": filepath.Join(build, "tmp"), "CARGO_TARGET_DIR": filepath.Join(build, "cargo"), "UV_CACHE_DIR": filepath.Join(cache, "uv"), "PIP_CACHE_DIR": filepath.Join(cache, "pip"), "npm_config_cache": filepath.Join(cache, "npm"), "GOCACHE": filepath.Join(cache, "go"), "GOMODCACHE": filepath.Join(cache, "go-mod"), "PUPPETEER_CACHE_DIR": filepath.Join(cache, "puppeteer"), "PLAYWRIGHT_BROWSERS_PATH": filepath.Join(cache, "playwright"), "DEV_WORKSPACE_MANAGED": "1"}
	for k, p := range overrides {
		if k != "DEV_WORKSPACE_MANAGED" {
			if e := os.MkdirAll(p, 0700); e != nil {
				return nil, e
			}
		}
	}
	env := []string{}
	for _, v := range os.Environ() {
		k, _, _ := strings.Cut(v, "=")
		if _, ok := overrides[k]; !ok {
			env = append(env, v)
		}
	}
	for k, v := range overrides {
		env = append(env, k+"="+v)
	}
	return env, nil
}
func (m *Manager) Run(path string, args []string) error {
	if e := m.verify(); e != nil {
		return e
	}
	token, e := m.Lease(path)
	if e != nil {
		return e
	}
	defer m.Release(path, token)
	env, e := m.environment(path)
	if e != nil {
		return e
	}
	c := exec.Command(args[0], args[1:]...)
	c.Dir = path
	c.Env = env
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if e = c.Start(); e != nil {
		return e
	}
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sig)
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case e := <-done:
			return e
		case s := <-sig:
			_ = c.Process.Signal(s)
		case <-ticker.C:
			// The process identity is authoritative; heartbeat is useful for diagnostics.
			_ = m.withWorkspace(path, func(w *Workspace) error {
				l, ok := w.Leases[token]
				if !ok {
					return fmt.Errorf("lease disappeared")
				}
				l.Seen = m.now()
				w.Leases[token] = l
				return nil
			})
		}
	}
}
func (m *Manager) GC(apply bool) error {
	if e := m.Reconcile(); e != nil {
		return e
	}
	paths := m.Candidates()
	sort.Strings(paths)
	for _, p := range paths {
		s, e := m.ReadState()
		if e != nil {
			return e
		}
		w := s.Workspaces[p]
		if w == nil {
			continue
		}
		if e = m.check(w); e != nil {
			fmt.Printf("KEEP %s: %v\n", p, e)
			continue
		}
		if !apply {
			fmt.Printf("WOULD REMOVE %s\n", p)
			continue
		}
		if e = m.Guard(p); e != nil {
			fmt.Printf("KEEP %s: %v\n", p, e)
			continue
		}
		// Foreground avoids cross-volume trash and preserves branches. Hooks recheck identity.
		_, e = command("", "/opt/homebrew/bin/wt", "-C", w.Common, "remove", "--foreground", "--no-delete-branch", p)
		if e != nil {
			fmt.Printf("KEEP %s: %v\n", p, e)
			continue
		}
		if e = m.Removed(p); e != nil {
			return e
		}
		fmt.Printf("REMOVED %s\n", p)
	}
	return nil
}
func (m *Manager) Status() error {
	s, e := m.ReadState()
	if e != nil {
		return e
	}
	free := map[string]float64{}
	for _, p := range []string{m.cfg.InternalPath, m.cfg.BackingPath, m.cfg.Root} {
		if n, e := m.space(p); e == nil {
			free[p] = float64(n) / float64(GiB)
		}
	}
	volumeError := ""
	if e = m.verify(); e != nil {
		volumeError = e.Error()
	}
	report := struct {
		Time        time.Time          `json:"time"`
		Free        map[string]float64 `json:"free_gib"`
		VolumeError string             `json:"volume_error,omitempty"`
		Config      Config             `json:"policy"`
		State       *State             `json:"registry"`
	}{m.now(), free, volumeError, m.cfg, s}
	b, e := json.MarshalIndent(report, "", "  ")
	if e != nil {
		return e
	}
	if e = atomicWrite(filepath.Join(m.cfg.StateDir, "status.json"), b); e != nil {
		return e
	}
	return printJSON(report)
}
func (m *Manager) PruneCaches() error {
	if e := m.verify(); e != nil {
		return e
	}
	// Serialize maintenance with admission and lease acquisition. A launcher
	// cannot start a new build between the idle check and native cache eviction.
	return m.update(m.pruneCachesLocked)
}
func (m *Manager) pruneCachesLocked(s *State) error {
	var e error
	if m.now().Sub(s.LastCachePrune) < 24*time.Hour {
		return nil
	}
	// Avoid eviction during managed installs/builds; unmanaged users are checked via open files.
	for _, w := range s.Workspaces {
		for _, l := range w.Leases {
			if live(l) {
				return nil
			}
		}
		if !w.Missing {
			if e := openFiles(w.Path); e != nil {
				return nil
			}
		}
	}
	root := filepath.Join(m.cfg.Root, "caches")
	if _, e = os.Stat(root); errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e = openFiles(root); e != nil {
		return nil
	}
	env := append(os.Environ(), "UV_LOCK_TIMEOUT=1", "GOCACHE="+filepath.Join(root, "go"))
	specs := []struct {
		folder  string
		limit   uint64
		program string
		args    []string
	}{
		{"uv", 2, "uv", []string{"cache", "clean", "--cache-dir", filepath.Join(root, "uv")}},
		{"npm", 2, "npm", []string{"cache", "clean", "--force", "--cache", filepath.Join(root, "npm")}},
		{"go", 2, "go", []string{"clean", "-cache"}},
	}
	if _, e = os.Stat(filepath.Join(root, "uv")); e == nil {
		c := exec.Command("uv", "cache", "prune", "--cache-dir", filepath.Join(root, "uv"))
		c.Env = env
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if e = c.Run(); e != nil {
			return e
		}
	}
	for _, spec := range specs {
		p := filepath.Join(root, spec.folder)
		if _, e = os.Stat(p); errors.Is(e, os.ErrNotExist) {
			continue
		}
		n, e := diskUsage(p)
		if e != nil {
			return e
		}
		if n <= spec.limit*GiB {
			continue
		}
		c := exec.Command(spec.program, spec.args...)
		c.Env = env
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if e = c.Run(); e != nil {
			return e
		}
	}
	s.LastCachePrune = m.now()
	return nil
}
