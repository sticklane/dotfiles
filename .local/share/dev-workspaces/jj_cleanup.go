package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func workspaceHead(w *Workspace) (string, error) {
	if w.Backend == "jj" {
		return command(w.Path, "jj", "--ignore-working-copy", "log", "--no-graph", "-r", "@", "-T", "commit_id")
	}
	return git(w.Path, "rev-parse", "HEAD")
}

// The shared repository must live outside the directory being collected. Never
// collect the primary workspace or an independently initialized repository.
func jjSharedRepo(w *Workspace) error {
	p := filepath.Join(w.Path, ".jj", "repo")
	info, err := os.Lstat(p)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("jj shared repository pointer required")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	target := string(b)
	if !filepath.IsAbs(target) {
		target = filepath.Join(w.Path, ".jj", target)
	}
	target, err = canon(target)
	if err != nil || target == w.Path || inside(w.Path, target) {
		return fmt.Errorf("jj shared repository must be outside the workspace")
	}
	return nil
}

func (m *Manager) checkJJ(w *Workspace) error {
	if err := m.jjIdentity(w); err != nil {
		return err
	}
	if err := jjSharedRepo(w); err != nil {
		return err
	}
	if w.Pinned {
		return fmt.Errorf("workspace is pinned; explicit unpin required")
	}
	for _, lease := range w.Leases {
		if live(lease) {
			return fmt.Errorf("active lease held by pid %d", lease.PID)
		}
	}
	if err := openFiles(w.Path); err != nil {
		return err
	}
	// Snapshot tracked files, including deletions, but never implicitly track new
	// files. Stale workspaces fail closed: cleanup never runs update-stale.
	// Status snapshots the working copy and therefore mutates jj state. Do not
	// put it under the read-only command deadline, even during a preview.
	if _, err := commandMutation(w.Path, "jj", "--config", `snapshot.auto-track="none()"`, "status"); err != nil {
		return fmt.Errorf("snapshot jj working copy: %w", err)
	}
	head, err := workspaceHead(w)
	if err != nil {
		return err
	}
	if w.Head != "" && head != w.Head {
		return fmt.Errorf("jj revision changed after completion")
	}
	names, err := command(w.Path, "jj", "--ignore-working-copy", "workspace", "list", "-T", `name ++ "\t" ++ target.commit_id() ++ "\n"`)
	if err != nil {
		return err
	}
	found := false
	for _, line := range strings.Split(names, "\n") {
		if line == w.Branch+"\t"+head {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("registered jj workspace name/revision changed or was forgotten; manual recovery required")
	}
	// A remote ref must preserve the exact revision, or the parent of a single,
	// undescribed empty working change. Local main alone is not a remote backup.
	preserved, err := command(w.Path, "jj", "--ignore-working-copy", "log", "--no-graph", "-r", `@ & ancestors(remote_bookmarks())`, "-T", "commit_id")
	if err != nil {
		return err
	}
	if preserved != head {
		disposableChange, e := command(w.Path, "jj", "--ignore-working-copy", "log", "--no-graph", "-r", "@", "-T", `empty && !conflict && description == "" && parents.len() == 1`)
		if e != nil {
			return e
		}
		parent, e := command(w.Path, "jj", "--ignore-working-copy", "log", "--no-graph", "-r", `parents(@) & ancestors(remote_bookmarks())`, "-T", "commit_id")
		if e != nil {
			return e
		}
		if disposableChange != "true" || parent == "" {
			return fmt.Errorf("jj change must be remotely preserved (or an undescribed empty child of a preserved parent)")
		}
	}
	tracked, err := command(w.Path, "jj", "--ignore-working-copy", "file", "list", "-T", `path ++ "\0"`)
	if err != nil {
		return err
	}
	paths := map[string]bool{}
	dirs := map[string]bool{}
	for _, p := range strings.Split(tracked, "\x00") {
		if p == "" {
			continue
		}
		paths[p] = true
		for d := filepath.Dir(p); d != "."; d = filepath.Dir(d) {
			dirs[d] = true
		}
	}
	// jj status omits ignored files. Walk without following symlinks, and keep
	// everything not tracked or explicitly declared disposable, even empty dirs.
	err = filepath.WalkDir(w.Path, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(w.Path, p)
		if e != nil {
			return e
		}
		if rel == "." {
			return nil
		}
		if rel == ".jj" {
			if !d.IsDir() || d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("invalid jj metadata directory")
			}
			return filepath.SkipDir
		}
		if disposable(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if paths[rel] {
			return nil
		}
		if d.IsDir() && dirs[rel] {
			return nil
		}
		return fmt.Errorf("preserve untracked or ignored data before removal: %s", rel)
	})
	if err != nil {
		return err
	}
	if err = m.jjIdentity(w); err != nil {
		return err
	}
	return openFiles(w.Path)
}

// RemoveJJ is an explicit immediate-removal operation. Scheduled GC reaches it
// only after retention. Finish and unpin remain separate deliberate actions.
// The registry lock blocks new managed leases throughout final checks/removal.
func (m *Manager) RemoveJJ(path string, apply bool) error {
	if err := m.verify(); err != nil {
		return err
	}
	p, err := canon(path)
	if err != nil {
		return err
	}
	return m.update(func(s *State) error {
		w := s.Workspaces[p]
		if w == nil || w.Backend != "jj" || w.Finished.IsZero() {
			return fmt.Errorf("explicitly finished native jj workspace required")
		}
		if w.Removing {
			return fmt.Errorf("interrupted jj removal requires recovery: %s", jjRemovalPath(w))
		}
		if err := m.checkJJ(w); err != nil {
			return err
		}
		if err := openFilesForRemoval(p); err != nil {
			return err
		}
		if !apply {
			fmt.Printf("WOULD REMOVE %s (%s)\n", p, w.Head)
			return nil
		}
		w.Removing = true
		w.RemovalPath = jjRemovalPath(w)
		if err := m.writeStateLocked(s); err != nil {
			return err
		}
		if _, err := os.Lstat(w.RemovalPath); !os.IsNotExist(err) {
			return fmt.Errorf("removal quarantine already exists or cannot be inspected")
		}
		// Moving to a token-bound sibling prevents a late opener of the original
		// path from writing into the tree being deleted. Recheck the moved tree;
		// writers with pre-existing handles are caught or fail closed for recovery.
		if err := os.Rename(p, w.RemovalPath); err != nil {
			return err
		}
		quarantined := *w
		quarantined.Path = w.RemovalPath
		quarantined.GitDir = filepath.Join(w.RemovalPath, ".jj")
		if err := m.checkJJ(&quarantined); err != nil {
			return fmt.Errorf("quarantined workspace requires manual recovery: %w", err)
		}
		if _, err := commandMutation(w.RemovalPath, "jj", "--ignore-working-copy", "workspace", "forget", w.Branch); err != nil {
			return fmt.Errorf("forget quarantined jj workspace: %w", err)
		}
		// Forget changes no files and does not remove commits or shared operation history.
		if err := m.jjIdentity(&quarantined); err != nil {
			return err
		}
		if err := openFilesForRemoval(w.RemovalPath); err != nil {
			return err
		}
		if err := m.removeDir(w.RemovalPath); err != nil {
			return fmt.Errorf("jj workspace forgotten; retained registry requires manual recovery: %w", err)
		}
		delete(s.Workspaces, p)
		s.NeedsCompact = true
		fmt.Printf("REMOVED %s\n", p)
		return nil
	})
}

// Derive this from existing immutable fields so old in-memory registry writers
// cannot erase the recovery location by dropping the newer optional JSON field.
func jjRemovalPath(w *Workspace) string {
	return filepath.Join(filepath.Dir(w.Path), ".removing-"+w.ID)
}

// RecoverJJ only cancels an interrupted removal whose jj registration and full
// preservation checks still pass. It never re-adopts forgotten work or deletes data.
func (m *Manager) RecoverJJ(path string, apply bool) error {
	if err := m.verify(); err != nil {
		return err
	}
	p, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	p = filepath.Clean(p)
	return m.update(func(s *State) error {
		w := s.Workspaces[p]
		if w == nil || w.Backend != "jj" || !w.Removing || w.Finished.IsZero() || w.Head == "" {
			return fmt.Errorf("interrupted finished jj removal required")
		}
		q := jjRemovalPath(w)
		if w.RemovalPath != "" && w.RemovalPath != q {
			return fmt.Errorf("quarantine identity changed")
		}
		_, pe := os.Lstat(p)
		_, qe := os.Lstat(q)
		original := pe == nil
		quarantine := qe == nil
		if (pe != nil && !os.IsNotExist(pe)) || (qe != nil && !os.IsNotExist(qe)) || original == quarantine {
			return fmt.Errorf("exactly one original or quarantine tree must exist")
		}
		candidate := *w
		if quarantine {
			candidate.Path = q
			candidate.GitDir = filepath.Join(q, ".jj")
		}
		if err := m.checkJJ(&candidate); err != nil {
			return err
		}
		if !apply {
			fmt.Printf("WOULD RECOVER %s from %s (%s); pin and reset completion\n", p, candidate.Path, w.Head)
			return nil
		}
		if quarantine {
			if err := openFilesForRemoval(q); err != nil {
				return err
			}
			if err := os.Rename(q, p); err != nil {
				return err
			}
		}
		w.Removing = false
		w.RemovalPath = ""
		w.Missing = false
		w.Pinned = true
		w.Head = ""
		w.Finished = time.Time{}
		fmt.Printf("RECOVERED %s (pinned, unfinished)\n", p)
		return nil
	})
}
