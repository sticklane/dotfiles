package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const resumeTreeLimit = 16 << 20
const resumeFileLimit = 64 << 20
const resumeByteLimit = 512 << 20
const resumeEntryLimit = 100000

var preservedObjectID = regexp.MustCompile(`^[0-9a-f]{40}$`)

type remainingJJProof struct {
	Files, Entries int
	Bytes          int64
	Revision       string
}
type preservedEntry struct{ mode, oid string }

// Binary Git output must not pass through command(), which trims whitespace.
func boundedGit(common string, limit int, args ...string) ([]byte, error) {
	if _, err := os.Lstat(filepath.Join(common, "info", "grafts")); !os.IsNotExist(err) {
		return nil, fmt.Errorf("legacy Git grafts prevent immutable preservation proof")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "git", append([]string{"--no-replace-objects", "--git-dir=" + common}, args...)...)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "GIT_") {
			c.Env = append(c.Env, value)
		}
	}
	c.Env = append(c.Env, "GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE=/dev/null", "GIT_TERMINAL_PROMPT=0")
	out, stderr := boundedOutput{limit: limit}, boundedOutput{limit: 16 << 10}
	c.Stdout = &out
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		return nil, fmt.Errorf("cannot read bounded Git preservation evidence: %w", err)
	}
	return out.Bytes(), nil
}

// Require a live origin branch, not just a potentially stale remote-tracking ref.
// Changed remote tips absent from the local object store fail closed; this path
// never fetches, updates shared history, or assumes that a local bookmark is remote.
func liveOriginPreserves(common, revision string) error {
	raw, err := boundedGit(common, 2<<20, "ls-remote", "--refs", "origin", "refs/heads/*")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 2 || !preservedObjectID.MatchString(fields[0]) || !strings.HasPrefix(fields[1], "refs/heads/") {
			return fmt.Errorf("invalid origin preservation evidence")
		}
		if _, err := boundedGit(common, 1024, "merge-base", "--is-ancestor", revision, fields[0]); err == nil {
			return nil
		}
	}
	return fmt.Errorf("finished revision is not preserved by a live origin branch")
}

func preservedRemovalRevision(w *Workspace, q string) (string, error) {
	if !preservedObjectID.MatchString(w.Head) {
		return "", fmt.Errorf("explicit SHA-1 finished revision required")
	}
	if err := liveOriginPreserves(w.Common, w.Head); err == nil {
		return w.Head, nil
	}
	// The sole exception is the same undescribed empty child permitted by Finish.
	eligible, err := command(q, "jj", "--ignore-working-copy", "log", "--no-graph", "-r", w.Head, "-T", `empty && !conflict && description == "" && parents.len() == 1`)
	if err != nil || eligible != "true" {
		return "", fmt.Errorf("finished change is not remotely preserved or an eligible empty child")
	}
	parent, err := command(q, "jj", "--ignore-working-copy", "log", "--no-graph", "-r", "parents("+w.Head+")", "-T", "commit_id")
	if err != nil || !preservedObjectID.MatchString(parent) {
		return "", fmt.Errorf("cannot resolve preserved parent")
	}
	parents, err := boundedGit(w.Common, 1024, "rev-list", "--parents", "--max-count=1", w.Head)
	if err != nil || strings.TrimSpace(string(parents)) != w.Head+" "+parent {
		return "", fmt.Errorf("raw finished-parent identity differs")
	}
	trees, err := boundedGit(w.Common, 1024, "rev-parse", w.Head+"^{tree}", parent+"^{tree}")
	treeIDs := strings.Fields(string(trees))
	if err != nil || len(treeIDs) != 2 || treeIDs[0] != treeIDs[1] {
		return "", fmt.Errorf("raw finished change is not empty")
	}
	if err := liveOriginPreserves(w.Common, parent); err != nil {
		return "", err
	}
	return parent, nil
}

func remainingJJTree(common, revision, q string) (remainingJJProof, error) {
	proof := remainingJJProof{Revision: revision}
	raw, err := boundedGit(common, resumeTreeLimit, "ls-tree", "-rz", "--full-tree", revision)
	if err != nil {
		return proof, err
	}
	entries := map[string]preservedEntry{}
	dirs := map[string]bool{}
	if len(raw) > 0 && raw[len(raw)-1] != 0 {
		return proof, fmt.Errorf("incomplete immutable tree")
	}
	for _, record := range bytes.Split(raw, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		parts := bytes.SplitN(record, []byte{'\t'}, 2)
		if len(parts) != 2 {
			return proof, fmt.Errorf("invalid immutable tree entry")
		}
		fields := strings.Fields(string(parts[0]))
		name := string(parts[1])
		if len(fields) != 3 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755" && fields[0] != "120000") || !preservedObjectID.MatchString(fields[2]) || filepath.IsAbs(name) || filepath.Clean(name) != name || name == "." || name == ".." || strings.HasPrefix(name, "../") || name == ".jj" || strings.HasPrefix(name, ".jj/") {
			return proof, fmt.Errorf("unsupported immutable tree entry")
		}
		if _, exists := entries[name]; exists {
			return proof, fmt.Errorf("duplicate immutable tree path")
		}
		entries[name] = preservedEntry{fields[0], fields[2]}
		if len(entries) > resumeEntryLimit {
			return proof, fmt.Errorf("immutable tree exceeds entry limit")
		}
		for parent := filepath.Dir(name); parent != "."; parent = filepath.Dir(parent) {
			dirs[parent] = true
		}
	}
	deadline := time.Now().Add(5 * time.Minute)
	err = filepath.WalkDir(q, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		proof.Entries++
		if proof.Entries > resumeEntryLimit || time.Now().After(deadline) {
			return fmt.Errorf("remaining tree inventory limit exceeded")
		}
		rel, err := filepath.Rel(q, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if rel == ".jj" {
			if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("invalid jj metadata")
			}
			return filepath.SkipDir
		}
		if disposable(rel) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if !dirs[rel] {
				return fmt.Errorf("unpreserved directory: %s", rel)
			}
			return nil
		}
		want, ok := entries[rel]
		if !ok {
			return fmt.Errorf("unpreserved file: %s", rel)
		}
		before, err := os.Lstat(path)
		if err != nil {
			return err
		}
		var digest string
		if want.mode == "120000" {
			if before.Mode()&os.ModeSymlink == 0 {
				return fmt.Errorf("preserved symlink type changed: %s", rel)
			}
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if len(target) > resumeFileLimit {
				return fmt.Errorf("symlink exceeds bound")
			}
			h := sha1.New()
			fmt.Fprintf(h, "blob %d\x00", len(target))
			h.Write([]byte(target))
			digest = hex.EncodeToString(h.Sum(nil))
			proof.Bytes += int64(len(target))
		} else {
			if !before.Mode().IsRegular() || before.Size() > resumeFileLimit || before.Size() < 0 || (before.Mode()&0111 != 0) != (want.mode == "100755") {
				return fmt.Errorf("preserved file type/mode/size changed: %s", rel)
			}
			if proof.Bytes+before.Size() > resumeByteLimit {
				return fmt.Errorf("remaining tree byte limit exceeded")
			}
			f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
			if err != nil {
				return err
			}
			opened, statErr := f.Stat()
			if statErr != nil || !os.SameFile(before, opened) {
				f.Close()
				return fmt.Errorf("remaining file identity changed: %s", rel)
			}
			h := sha1.New()
			fmt.Fprintf(h, "blob %d\x00", before.Size())
			n, readErr := io.Copy(h, io.LimitReader(f, before.Size()+1))
			closeErr := f.Close()
			if readErr != nil || closeErr != nil || n != before.Size() {
				return fmt.Errorf("remaining file read changed: %s", rel)
			}
			digest = hex.EncodeToString(h.Sum(nil))
			proof.Bytes += n
		}
		if proof.Bytes > resumeByteLimit {
			return fmt.Errorf("remaining tree byte limit exceeded")
		}
		after, err := os.Lstat(path)
		if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) || digest != want.oid {
			return fmt.Errorf("unpreserved or changing contents: %s", rel)
		}
		proof.Files++
		return nil
	})
	return proof, err
}

func (m *Manager) forgottenJJIdentity(w *Workspace, q string) error {
	if _, err := os.Lstat(w.Path); !os.IsNotExist(err) {
		return fmt.Errorf("original path exists or cannot be inspected")
	}
	if w.RemovalPath != "" && w.RemovalPath != q {
		return fmt.Errorf("quarantine identity changed")
	}
	for _, name := range []string{"repo", "dev-workspace-id"} {
		info, err := os.Lstat(filepath.Join(q, ".jj", name))
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("intact regular jj identity metadata required")
		}
	}
	candidate := *w
	candidate.Path = q
	candidate.GitDir = filepath.Join(q, ".jj")
	if err := m.jjIdentity(&candidate); err != nil {
		return err
	}
	if err := jjSharedRepo(&candidate); err != nil {
		return err
	}
	names, err := command(q, "jj", "--ignore-working-copy", "workspace", "list", "-T", `name ++ "\n"`)
	if err != nil {
		return err
	}
	for _, name := range strings.Split(names, "\n") {
		if name == w.Branch {
			return fmt.Errorf("jj workspace name remains registered or was reused")
		}
	}
	for _, lease := range w.Leases {
		if live(lease) {
			return fmt.Errorf("active lease held by pid %d", lease.PID)
		}
	}
	return openFilesForRemoval(q)
}

// ResumeJJRemoval is explicit completion of a previously authorized forget.
// It does not adopt a workspace, snapshot files, forget a name or alter history.
// Missing identity metadata or a missing quarantine still requires manual review.
func (m *Manager) ResumeJJRemoval(path string, apply bool) error {
	if err := m.verify(); err != nil {
		return err
	}
	p, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	p = filepath.Clean(p)
	err = m.update(func(s *State) error {
		w := s.Workspaces[p]
		if w == nil || w.Path != p || w.Backend != "jj" || !w.Removing || w.Finished.IsZero() || w.Pinned || w.Head == "" || w.Branch == "" {
			return fmt.Errorf("finished unpinned interrupted jj removal required")
		}
		if filepath.Dir(p) != filepath.Join(m.cfg.Root, "worktrees") {
			return fmt.Errorf("original path is not a direct managed workspace")
		}
		q := jjRemovalPath(w)
		if err := m.forgottenJJIdentity(w, q); err != nil {
			return err
		}
		revision, err := preservedRemovalRevision(w, q)
		if err != nil {
			return err
		}
		proof, err := remainingJJTree(w.Common, revision, q)
		if err != nil {
			return err
		}
		// The registry lock stays held. Repeat identity and ownership checks after
		// hashing; no failure here may be treated as evidence of completed deletion.
		if err := m.verify(); err != nil {
			return err
		}
		if err := m.forgottenJJIdentity(w, q); err != nil {
			return err
		}
		if !apply {
			fmt.Printf("WOULD RESUME REMOVAL %s token=%s quarantine=%s finished=%s preserved=%s files=%d bytes=%d\n", p, w.ID, q, w.Head, proof.Revision, proof.Files, proof.Bytes)
			return nil
		}
		if err := liveOriginPreserves(w.Common, proof.Revision); err != nil {
			return err
		}
		// A remote proof can wait on the network. Keep the local deletion guard
		// after that wait so a replaced quarantine or newly active owner is caught.
		if err := m.verify(); err != nil {
			return err
		}
		if err := m.forgottenJJIdentity(w, q); err != nil {
			return err
		}
		if err := m.removeDir(q); err != nil {
			return fmt.Errorf("interrupted deletion retained for review: %w", err)
		}
		if _, err := os.Lstat(q); !os.IsNotExist(err) {
			return fmt.Errorf("quarantine removal did not establish absence")
		}
		delete(s.Workspaces, p)
		s.NeedsCompact = true
		return nil
	})
	if err == nil && apply {
		fmt.Printf("COMPLETED FORGOTTEN JJ REMOVAL %s\n", p)
	}
	return err
}
