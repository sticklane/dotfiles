package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// Native jj workspaces share the repository but never use Git's worktree index.
// Unfinished workspaces are retained; explicit finish opts into guarded collection.
func (m *Manager) CreateJJ(repo, name string) (string, error) {
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`).MatchString(name) {
		return "", fmt.Errorf("invalid jj task name")
	}
	if filepath.Clean(name) != name || filepath.IsAbs(name) || name == "." || name == ".." {
		return "", fmt.Errorf("invalid jj task name")
	}
	common, e := command(repo, "jj", "--ignore-working-copy", "git", "root")
	if e != nil {
		return "", e
	}
	common, e = canon(common)
	if e != nil {
		return "", e
	}
	if e = m.verify(); e != nil {
		return "", e
	}
	key := admissionKey(common, "jj:"+name)
	path := filepath.Join(m.cfg.Root, "worktrees", "jj-"+key[:16])
	e = m.update(func(s *State) error {
		expirePending(s, m.now())
		if _, ok := s.Pending[key]; ok {
			return fmt.Errorf("jj workspace creation already pending")
		}
		if _, e := os.Lstat(path); !os.IsNotExist(e) {
			return fmt.Errorf("workspace path exists or cannot be inspected: %s", path)
		}
		count := len(s.Pending)
		for _, w := range s.Workspaces {
			if !w.Missing {
				count++
			}
		}
		if count >= m.cfg.MaxWorkspaces {
			return fmt.Errorf("workspace limit %d reached", m.cfg.MaxWorkspaces)
		}
		if e := m.checkCapacity(activeReservations(s) + 1); e != nil {
			return e
		}
		s.Pending[key] = m.now()
		return nil
	})
	if e != nil {
		return "", e
	}
	// On failure, retain any partial directory for explicit recovery; never delete it.
	defer m.update(func(s *State) error { delete(s.Pending, key); return nil })
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return "", e
	}
	parent, e := canon(filepath.Dir(path))
	if e != nil || parent != filepath.Dir(path) {
		return "", fmt.Errorf("jj workspace parent is not the configured physical pool path")
	}
	if _, e = command(repo, "jj", "workspace", "add", "--no-colocate", "--name", name, "-r", "@-", path); e != nil {
		return "", e
	}
	id := randomID()
	if e = atomicWrite(filepath.Join(path, ".jj", "dev-workspace-id"), []byte(id)); e != nil {
		return "", e
	}
	e = m.update(func(s *State) error {
		s.Workspaces[path] = &Workspace{Backend: "jj", ID: id, Path: path, GitDir: filepath.Join(path, ".jj"), Common: common, Branch: name, Created: m.now(), Leases: map[string]Lease{}}
		delete(s.Pending, key)
		return nil
	})
	if e != nil {
		return "", e
	}
	return path, nil
}

func (m *Manager) jjIdentity(w *Workspace) error {
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(w.ID) {
		return fmt.Errorf("invalid jj workspace identity token")
	}
	if _, e := m.managedPath(w.Path); e != nil {
		return e
	}
	id, e := os.ReadFile(filepath.Join(w.Path, ".jj", "dev-workspace-id"))
	if e != nil || string(id) != w.ID {
		return fmt.Errorf("jj workspace identity changed")
	}
	root, e := command(w.Path, "jj", "--ignore-working-copy", "root")
	if e != nil || root != w.Path {
		return fmt.Errorf("jj workspace root changed")
	}
	common, e := command(w.Path, "jj", "--ignore-working-copy", "git", "root")
	if e != nil {
		return e
	}
	common, e = canon(common)
	if e != nil || common != w.Common {
		return fmt.Errorf("jj backing repository changed")
	}
	return nil
}
