package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	base, e := command(repo, "jj", "--ignore-working-copy", "log", "--no-graph", "-r", "@-", "-T", "commit_id")
	if e != nil || !regexp.MustCompile(`^[0-9a-f]{40,64}$`).MatchString(base) {
		return "", fmt.Errorf("cannot resolve exact jj creation base: %s: %v", base, e)
	}
	key := admissionKey(common, "jj:"+name)
	path := filepath.Join(m.cfg.Root, "worktrees", "jj-"+key[:16])
	intent := &JJCreation{ID: randomID(), Path: path, Common: common, Name: name, Base: base, Created: m.now()}
	e = m.update(func(s *State) error {
		expirePending(s, m.now())
		if _, ok := s.JJCreations[key]; ok {
			return fmt.Errorf("jj workspace creation already pending")
		}
		if _, ok := s.Pending[key]; ok {
			return fmt.Errorf("legacy jj workspace creation already pending; inspect its owner")
		}
		if _, e := os.Lstat(path); !os.IsNotExist(e) {
			return fmt.Errorf("workspace path exists or cannot be inspected: %s", path)
		}
		count := len(s.Pending) + len(s.JJCreations)
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
		s.JJCreations[key] = intent
		return nil
	})
	if e != nil {
		return "", e
	}
	// On every failure retain intent, capacity reservation, and any partial tree.
	// Recovery is manual and must not adopt, delete, or forget unknown work.
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return "", e
	}
	parent, e := canon(filepath.Dir(path))
	if e != nil || parent != filepath.Dir(path) {
		return "", fmt.Errorf("jj workspace parent is not the configured physical pool path")
	}
	if _, e = m.jjAdd(repo, "workspace", "add", "--no-colocate", "--name", name, "-r", base, path); e != nil {
		return "", e
	}
	e = m.update(func(s *State) error {
		current := s.JJCreations[key]
		if current == nil || current.ID != intent.ID || current.Path != intent.Path || current.Common != intent.Common || current.Name != intent.Name || current.Base != intent.Base || !current.Created.Equal(intent.Created) {
			return fmt.Errorf("jj creation intent changed; manual recovery required")
		}
		w := &Workspace{Backend: "jj", ID: intent.ID, Path: path, GitDir: filepath.Join(path, ".jj"), Common: common, Branch: name, Created: intent.Created, Leases: map[string]Lease{}}
		if _, exists := s.Workspaces[path]; exists {
			return fmt.Errorf("workspace already registered")
		}
		if e := m.validateJJCreation(intent); e != nil {
			return e
		}
		tokenPath := filepath.Join(path, ".jj", "dev-workspace-id")
		// Never replace even a matching token left by another execution.
		f, e := os.OpenFile(tokenPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return e
		}
		_, e = f.WriteString(intent.ID)
		if e == nil {
			e = f.Sync()
		}
		closeErr := f.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
		// Recheck after the token write before completing the transaction.
		if e = m.validateJJCreation(intent); e != nil {
			return e
		}
		if e = m.jjIdentity(w); e != nil {
			return e
		}
		s.Workspaces[path] = w
		delete(s.JJCreations, key)
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

// Validate only after a successful checkout command. Recovery from an interrupted
// command additionally requires a manual content inventory; metadata alone cannot
// establish that checkout completed.
func (m *Manager) validateJJCreation(c *JJCreation) error {
	if _, e := m.managedPath(c.Path); e != nil {
		return e
	}
	if _, e := os.Lstat(filepath.Join(c.Path, ".git")); !os.IsNotExist(e) {
		return fmt.Errorf("native jj workspace contains unexpected Git metadata")
	}
	w := &Workspace{Path: c.Path}
	if e := jjSharedRepo(w); e != nil {
		return e
	}
	root, e := command(c.Path, "jj", "--ignore-working-copy", "root")
	if e != nil || root != c.Path {
		return fmt.Errorf("jj creation root mismatch: %v", e)
	}
	common, e := command(c.Path, "jj", "--ignore-working-copy", "git", "root")
	if e != nil {
		return e
	}
	common, e = canon(common)
	if e != nil || common != c.Common {
		return fmt.Errorf("jj creation repository mismatch")
	}
	head, e := command(c.Path, "jj", "--ignore-working-copy", "log", "--no-graph", "-r", "@", "-T", `commit_id ++ "\t" ++ (empty && !conflict && description == "" && parents.len() == 1) ++ "\t" ++ parents.map(|p| p.commit_id()).join(",")`)
	if e != nil {
		return e
	}
	fields := strings.Split(head, "\t")
	if len(fields) != 3 || fields[1] != "true" || fields[2] != c.Base {
		return fmt.Errorf("jj creation revision differs from intended empty child")
	}
	names, e := command(c.Path, "jj", "--ignore-working-copy", "workspace", "list", "-T", `name ++ "\t" ++ target.commit_id() ++ "\n"`)
	if e != nil {
		return e
	}
	for _, line := range strings.Split(names, "\n") {
		if line == c.Name+"\t"+fields[0] {
			return nil
		}
	}
	return fmt.Errorf("jj creation registration mismatch")
}
