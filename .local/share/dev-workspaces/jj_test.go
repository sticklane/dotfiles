package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJJWorkspaceLeaseAndRetention(t *testing.T) {
	m, repo, _ := fixture(t)
	if _, e := command(repo, "jj", "git", "init", "--colocate"); e != nil {
		t.Fatal(e)
	}
	p, e := m.CreateJJ(repo, "task-jj")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(p, ".git")); !os.IsNotExist(e) {
		t.Fatal("native workspace must not masquerade as a Git worktree")
	}
	if got, e := command(p, "jj", "root"); e != nil || got != p {
		t.Fatalf("root %q %v", got, e)
	}
	token, e := m.Lease(p)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Release(p, token); e != nil {
		t.Fatal(e)
	}
	if e = m.Run(p, []string{"sh", "-c", `test "$DEV_WORKSPACE_MANAGED" = 1 && test -d "$TMPDIR" && pwd > .dev-build/ran`}); e != nil {
		t.Fatal(e)
	}
	ran, e := os.ReadFile(filepath.Join(p, ".dev-build", "ran"))
	if e != nil || strings.TrimSpace(string(ran)) != p {
		t.Fatalf("managed run used wrong workspace: %s %v", ran, e)
	}
	s, _ := m.ReadState()
	if s.Workspaces[p].Pinned || s.Workspaces[p].Backend != "jj" || !s.Workspaces[p].Finished.IsZero() {
		t.Fatal("jj workspace must remain unfinished")
	}
	if e = m.Finish(p); e == nil || !strings.Contains(e.Error(), "remotely preserved") {
		t.Fatalf("must not pass Git-only deletion guard: %v", e)
	}
	if _, e = m.CreateJJ(repo, "task-jj"); e == nil {
		t.Fatal("duplicate must not be adopted")
	}
	if _, e = m.CreateJJ(repo, "../../escape"); e == nil {
		t.Fatal("unsafe task name accepted")
	}
	if e = os.WriteFile(filepath.Join(p, ".jj", "dev-workspace-id"), []byte("replacement"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Lease(p); e == nil {
		t.Fatal("replaced identity accepted")
	}
}

func TestJJWorkspaceAdmission(t *testing.T) {
	m, repo, _ := fixture(t)
	if _, e := command(repo, "jj", "git", "init", "--colocate"); e != nil {
		t.Fatal(e)
	}
	m.space = func(string) (uint64, error) { return 1 * GiB, nil }
	if _, e := m.CreateJJ(repo, "no-space"); e == nil {
		t.Fatal("insufficient capacity accepted")
	}
}
