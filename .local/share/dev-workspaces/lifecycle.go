package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func commonDir(repo string) (string, error) {
	p, e := git(repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if e != nil {
		return "", e
	}
	return canon(p)
}
func (m *Manager) Admit(repo, branch string) error {
	if branch == "" {
		return fmt.Errorf("branch required")
	}
	common, e := commonDir(repo)
	if e != nil {
		return e
	}
	// Existing checkouts do not allocate another workspace, including legacy checkouts.
	raw, e := git(repo, "worktree", "list", "--porcelain")
	if e != nil {
		return e
	}
	if strings.Contains(raw+"\n", "\nbranch refs/heads/"+branch+"\n") {
		return nil
	}
	if e = m.verify(); e != nil {
		return e
	}
	return m.update(func(s *State) error {
		expirePending(s, m.now())
		key := admissionKey(common, branch)
		if _, ok := s.Pending[key]; ok {
			return nil
		}
		count := len(s.Pending)
		for _, w := range s.Workspaces {
			if !w.Missing {
				count++
			}
		}
		if count >= m.cfg.MaxWorkspaces {
			return fmt.Errorf("workspace limit %d reached; finish or park existing work", m.cfg.MaxWorkspaces)
		}
		reserved := uint64(count+1) * m.cfg.ReserveGiB * GiB
		for _, check := range []struct {
			path    string
			floor   uint64
			reserve bool
		}{{m.cfg.Root, m.cfg.PoolMinGiB, true}, {m.cfg.BackingPath, m.cfg.BackingMinGiB, true}, {m.cfg.InternalPath, m.cfg.InternalMinGiB, false}} {
			if check.path == "" {
				continue
			}
			free, e := m.space(check.path)
			if e != nil {
				return e
			}
			need := check.floor * GiB
			if check.reserve {
				need += reserved
			}
			if free < need {
				return fmt.Errorf("storage admission refused: %s has %.1f GiB free; %.1f GiB required including reservations", check.path, float64(free)/float64(GiB), float64(need)/float64(GiB))
			}
		}
		s.Pending[key] = m.now()
		return nil
	})
}
func (m *Manager) Register(path string) error {
	if e := m.verify(); e != nil {
		return e
	}
	p, e := m.managedPath(path)
	if e != nil {
		return e
	}
	gd, e := git(p, "rev-parse", "--absolute-git-dir")
	if e != nil {
		return e
	}
	gd, e = canon(gd)
	if e != nil {
		return e
	}
	common, e := commonDir(p)
	if e != nil {
		return e
	}
	branch, e := git(p, "symbolic-ref", "--short", "HEAD")
	if e != nil {
		return e
	}
	if gd == common {
		return fmt.Errorf("primary checkouts cannot be registered")
	}
	return m.update(func(s *State) error {
		if old, ok := s.Workspaces[p]; ok {
			return m.identity(old)
		}
		id := randomID()
		if e := atomicWrite(filepath.Join(gd, "dev-workspace-id"), []byte(id)); e != nil {
			return e
		}
		s.Workspaces[p] = &Workspace{ID: id, Path: p, GitDir: gd, Common: common, Branch: branch, Created: m.now(), Leases: map[string]Lease{}}
		delete(s.Pending, admissionKey(common, branch))
		return nil
	})
}
func (m *Manager) identity(w *Workspace) error {
	if _, e := m.managedPath(w.Path); e != nil {
		return e
	}
	gd, e := git(w.Path, "rev-parse", "--absolute-git-dir")
	if e != nil {
		return e
	}
	gd, e = canon(gd)
	if e != nil {
		return e
	}
	common, e := commonDir(w.Path)
	if e != nil {
		return e
	}
	if gd != w.GitDir || common != w.Common {
		return fmt.Errorf("worktree registration changed")
	}
	id, e := os.ReadFile(filepath.Join(gd, "dev-workspace-id"))
	if e != nil || string(id) != w.ID {
		return fmt.Errorf("worktree identity changed")
	}
	branch, e := git(w.Path, "symbolic-ref", "--short", "HEAD")
	if e != nil || branch != w.Branch {
		return fmt.Errorf("branch changed since registration")
	}
	return nil
}
func disposable(name string) bool {
	for _, part := range strings.Split(strings.TrimSuffix(name, "/"), "/") {
		switch part {
		case ".dev-build", "node_modules", ".venv", "__pycache__", ".pytest_cache", ".mypy_cache", ".ruff_cache":
			return true
		}
	}
	return false
}
func (m *Manager) check(w *Workspace) error {
	if e := m.identity(w); e != nil {
		return e
	}
	if w.Pinned {
		return fmt.Errorf("workspace is pinned")
	}
	if _, e := os.Stat(filepath.Join(w.GitDir, "locked")); e == nil {
		return fmt.Errorf("Git worktree is locked")
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	for _, l := range w.Leases {
		if live(l) {
			return fmt.Errorf("active lease held by pid %d", l.PID)
		}
	}
	status, e := git(w.Path, "status", "--porcelain", "--untracked-files=all", "--ignore-submodules=none")
	if e != nil {
		return e
	}
	if status != "" {
		return fmt.Errorf("workspace has uncommitted or untracked changes")
	}
	ignored, e := git(w.Path, "ls-files", "--others", "--ignored", "--exclude-standard", "--directory", "-z")
	if e != nil {
		return e
	}
	for _, p := range strings.Split(ignored, "\x00") {
		if p != "" && !disposable(p) {
			return fmt.Errorf("preserve ignored data before removal: %s", p)
		}
	}
	head, e := git(w.Path, "rev-parse", "HEAD")
	if e != nil {
		return e
	}
	if w.Head != "" && w.Head != head {
		return fmt.Errorf("HEAD changed after completion")
	}
	refs, e := git(w.Path, "for-each-ref", "--contains="+head, "--format=%(refname)", "refs/remotes/", "refs/heads/main", "refs/heads/master")
	if e != nil {
		return e
	}
	if refs == "" {
		return fmt.Errorf("HEAD must be preserved on a remote-tracking ref or merged into main/master")
	}
	return openFiles(w.Path)
}
func (m *Manager) withWorkspace(path string, fn func(*Workspace) error) error {
	if e := m.verify(); e != nil {
		return e
	}
	p, e := canon(path)
	if e != nil {
		return e
	}
	return m.update(func(s *State) error {
		w, ok := s.Workspaces[p]
		if !ok {
			return fmt.Errorf("workspace is not registered")
		}
		return fn(w)
	})
}
func (m *Manager) Finish(path string) error {
	return m.withWorkspace(path, func(w *Workspace) error {
		// An owning agent may declare completion while its launcher is still
		// alive. Keep those leases in the registry: removal still waits for exit.
		candidate := *w
		candidate.Leases = map[string]Lease{}
		own := currentAncestors()
		for key, lease := range w.Leases {
			if !own[lease.PID] {
				candidate.Leases[key] = lease
			}
		}
		if e := m.check(&candidate); e != nil {
			return e
		}
		h, e := git(w.Path, "rev-parse", "HEAD")
		if e != nil {
			return e
		}
		w.Head = h
		w.Finished = m.now()
		return nil
	})
}
func (m *Manager) Guard(path string) error {
	return m.withWorkspace(path, func(w *Workspace) error {
		if e := m.check(w); e != nil {
			return e
		}
		h, e := git(w.Path, "rev-parse", "HEAD")
		if e != nil {
			return e
		}
		w.Head = h
		if w.Finished.IsZero() {
			w.Finished = m.now()
		}
		w.Removing = true
		return nil
	})
}
func (m *Manager) Pin(path string, pin bool) error {
	return m.withWorkspace(path, func(w *Workspace) error {
		if e := m.identity(w); e != nil {
			return e
		}
		w.Pinned = pin
		return nil
	})
}
func (m *Manager) Lease(path string) (string, error) {
	token := randomID()
	e := m.withWorkspace(path, func(w *Workspace) error {
		if e := m.identity(w); e != nil {
			return e
		}
		if w.Removing {
			return fmt.Errorf("workspace is being removed")
		}
		if w.Leases == nil {
			w.Leases = map[string]Lease{}
		}
		start := processStart(os.Getpid())
		if start == "" {
			return fmt.Errorf("cannot identify lease owner")
		}
		w.Leases[token] = Lease{PID: os.Getpid(), Start: start, Seen: m.now()}
		w.Finished = time.Time{}
		w.Head = ""
		return nil
	})
	return token, e
}
func (m *Manager) Release(path, token string) error {
	return m.withWorkspace(path, func(w *Workspace) error { delete(w.Leases, token); return nil })
}
func (m *Manager) Candidates() []string {
	s, e := m.ReadState()
	if e != nil {
		return nil
	}
	out := []string{}
	for p, w := range s.Workspaces {
		if !w.Missing && !w.Pinned && !w.Finished.IsZero() && (w.Removing || m.now().Sub(w.Finished) >= time.Duration(m.cfg.RetentionHours)*time.Hour) {
			out = append(out, p)
		}
	}
	return out
}
func (m *Manager) Reconcile() error {
	if e := m.verify(); e != nil {
		return e
	}
	return m.update(func(s *State) error {
		expirePending(s, m.now())
		for p, w := range s.Workspaces {
			if _, e := os.Lstat(p); errors.Is(e, os.ErrNotExist) {
				if w.Removing {
					// Worktrunk finished but its background post-remove hook was interrupted.
					// Only the registry entry is reclaimed; Git metadata is left to Git.
					delete(s.Workspaces, p)
					s.NeedsCompact = true
					continue
				}
				w.Missing = true
				continue
			} else if e != nil {
				return e
			}
			if e := m.identity(w); e != nil {
				continue
			}
			for token, l := range w.Leases {
				if !live(l) {
					delete(w.Leases, token)
				}
			}
		}
		return nil
	})
}
func (m *Manager) Removed(path string) error {
	if e := m.verify(); e != nil {
		return e
	}
	return m.update(func(s *State) error {
		w, ok := s.Workspaces[path]
		if !ok {
			return nil
		}
		if !w.Removing {
			return fmt.Errorf("no removal in progress")
		}
		if _, e := os.Lstat(path); !errors.Is(e, os.ErrNotExist) {
			return fmt.Errorf("path still exists or is unreadable")
		}
		delete(s.Workspaces, path)
		s.NeedsCompact = true
		return nil
	})
}
