package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.test", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.test")
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}

func fixture(t *testing.T) (*Manager, string, string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(base, "repo")
	root := filepath.Join(base, "pool")
	for _, p := range []string{repo, root} {
		if e := os.MkdirAll(p, 0700); e != nil {
			t.Fatal(e)
		}
	}
	gitTest(t, repo, "init", "-b", "main")
	gitTest(t, repo, "commit", "--allow-empty", "-m", "initial")
	path := filepath.Join(root, "worktrees", "repo", "task")
	gitTest(t, repo, "worktree", "add", "-b", "task", path)
	m := NewManager(Config{Root: root, StateDir: filepath.Join(base, "state"), ReserveGiB: 4, MaxWorkspaces: 12, RetentionHours: 24, PoolMinGiB: 4, BackingMinGiB: 12, InternalMinGiB: 2})
	m.verify = func() error { return nil }
	m.space = func(string) (uint64, error) { return 100 * GiB, nil }
	if e := m.Register(path); e != nil {
		t.Fatal(e)
	}
	return m, repo, path
}

func TestFinishRequiresPreservedCleanState(t *testing.T) {
	t.Run("merged", func(t *testing.T) {
		m, _, p := fixture(t)
		if e := m.Finish(p); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("uncommitted", func(t *testing.T) {
		m, _, p := fixture(t)
		os.WriteFile(filepath.Join(p, "notes"), []byte("unique"), 0600)
		if e := m.Finish(p); e == nil {
			t.Fatal("accepted untracked work")
		}
	})
	t.Run("unpushed", func(t *testing.T) {
		m, _, p := fixture(t)
		gitTest(t, p, "commit", "--allow-empty", "-m", "unique")
		if e := m.Finish(p); e == nil {
			t.Fatal("accepted unpreserved commit")
		}
	})
	t.Run("ignored data", func(t *testing.T) {
		m, repo, p := fixture(t)
		os.WriteFile(filepath.Join(repo, ".git", "info", "exclude"), []byte("local.db\n"), 0600)
		os.WriteFile(filepath.Join(p, "local.db"), []byte("valuable"), 0600)
		if e := m.Finish(p); e == nil {
			t.Fatal("accepted ignored database")
		}
	})
	t.Run("declared scratch", func(t *testing.T) {
		m, repo, p := fixture(t)
		os.WriteFile(filepath.Join(repo, ".git", "info", "exclude"), []byte(".dev-build/\n"), 0600)
		os.MkdirAll(filepath.Join(p, ".dev-build"), 0700)
		os.WriteFile(filepath.Join(p, ".dev-build", "tmp"), []byte("rebuildable"), 0600)
		if e := m.Finish(p); e != nil {
			t.Fatal(e)
		}
	})
}

func TestRemovalRequiresIdentityPinAndLeaseChecks(t *testing.T) {
	t.Run("identity", func(t *testing.T) {
		m, _, p := fixture(t)
		gd := gitTest(t, p, "rev-parse", "--absolute-git-dir")
		os.WriteFile(filepath.Join(gd, "dev-workspace-id"), []byte("replacement"), 0600)
		if e := m.Finish(p); e == nil {
			t.Fatal("accepted replaced worktree")
		}
	})
	t.Run("pin", func(t *testing.T) {
		m, _, p := fixture(t)
		if e := m.Pin(p, true); e != nil {
			t.Fatal(e)
		}
		if e := m.Guard(p); e == nil {
			t.Fatal("accepted pinned worktree")
		}
	})
	t.Run("live lease", func(t *testing.T) {
		m, _, p := fixture(t)
		token, e := m.Lease(p)
		if e != nil {
			t.Fatal(e)
		}
		defer m.Release(p, token)
		if e = m.Guard(p); e == nil {
			t.Fatal("accepted active lease")
		}
	})
	t.Run("legacy", func(t *testing.T) {
		m, repo, _ := fixture(t)
		if e := m.Register(repo); e == nil {
			t.Fatal("adopted legacy path")
		}
	})
	t.Run("symlink", func(t *testing.T) {
		m, repo, _ := fixture(t)
		p := filepath.Join(m.cfg.Root, "worktrees", "alias")
		os.Symlink(repo, p)
		if e := m.Register(p); e == nil {
			t.Fatal("adopted symlink outside pool")
		}
	})
}

func TestBudgetIncludesConcurrentReservations(t *testing.T) {
	m, repo, path := fixture(t)
	if _, e := m.Lease(path); e != nil {
		t.Fatal(e)
	}
	m.space = func(string) (uint64, error) { return 10 * GiB, nil }
	m.cfg.BackingMinGiB = 0
	m.cfg.InternalMinGiB = 0
	// Four GiB already reserved, another four plus the four GiB pool floor cannot fit.
	if e := m.Admit(repo, "another"); e == nil {
		t.Fatal("overcommitted pool")
	}
	m.space = func(string) (uint64, error) { return 100 * GiB, nil }
	if e := m.Admit(repo, "another"); e != nil {
		t.Fatal(e)
	}
	m.cfg.MaxWorkspaces = 2
	if e := m.Admit(repo, "third"); e == nil {
		t.Fatal("ignored pending creation")
	}
}

func TestGCCandidatesRequireExplicitCompletionAndUnchangedHead(t *testing.T) {
	m, _, p := fixture(t)
	if got := m.Candidates(); len(got) != 0 {
		t.Fatal("unfinished workspace eligible")
	}
	if e := m.Finish(p); e != nil {
		t.Fatal(e)
	}
	if got := m.Candidates(); len(got) != 0 {
		t.Fatal("ignored retention")
	}
	m.now = func() time.Time { return time.Now().Add(25 * time.Hour) }
	if got := m.Candidates(); len(got) != 1 {
		t.Fatalf("expected candidate: %v", got)
	}
	gitTest(t, p, "commit", "--allow-empty", "-m", "new work")
	if e := m.Guard(p); e == nil {
		t.Fatal("accepted head changed after completion")
	}
}

func TestInterruptedCreationExpiresWithoutDeletingFiles(t *testing.T) {
	m, repo, p := fixture(t)
	if e := m.Admit(repo, "interrupted"); e != nil {
		t.Fatal(e)
	}
	m.now = func() time.Time { return time.Now().Add(time.Hour) }
	if e := m.Reconcile(); e != nil {
		t.Fatal(e)
	}
	s, e := m.ReadState()
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Pending) != 0 {
		t.Fatal("reservation leaked")
	}
	if _, e = os.Stat(p); e != nil {
		t.Fatal("removed unfinished worktree")
	}
}

func TestOfflinePoolDoesNotChangeRegistry(t *testing.T) {
	m, _, p := fixture(t)
	before, _ := os.ReadFile(m.statePath())
	m.verify = func() error { return os.ErrNotExist }
	if e := m.Reconcile(); e == nil {
		t.Fatal("accepted offline volume")
	}
	after, _ := os.ReadFile(m.statePath())
	if string(before) != string(after) {
		t.Fatal("changed state while offline")
	}
	if _, e := os.Stat(p); e != nil {
		t.Fatal(e)
	}
}

func TestInterruptedRemovalRecoveryDoesNotLeakReservation(t *testing.T) {
	m, repo, p := fixture(t)
	if e := m.Guard(p); e != nil {
		t.Fatal(e)
	}
	gitTest(t, repo, "worktree", "remove", p)
	if e := m.Reconcile(); e != nil {
		t.Fatal(e)
	}
	s, e := m.ReadState()
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Workspaces) != 0 {
		t.Fatal("completed removal leaked a registry entry")
	}
}

func TestExistingBranchSwitchDoesNotReserveAgain(t *testing.T) {
	m, repo, _ := fixture(t)
	m.cfg.MaxWorkspaces = 1
	if e := m.Admit(repo, "task"); e != nil {
		t.Fatalf("existing checkout blocked: %v", e)
	}
}

func TestAnotherProcessKeepsWorkspace(t *testing.T) {
	m, _, p := fixture(t)
	c := exec.Command("sleep", "30")
	c.Dir = p
	if e := c.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { c.Process.Kill(); c.Wait() }()
	if e := m.Guard(p); e == nil {
		t.Fatal("accepted workspace with live external process")
	}
}

func TestRemovalDoesNotTreatArbitraryNestedFilesAsBuildOutputs(t *testing.T) {
	if disposable("valuable/node_modules-backup/notes") {
		t.Fatal("matched partial component")
	}
	if disposable(".env") || disposable("data/local.db") {
		t.Fatal("classified durable data as disposable")
	}
}

func TestGuardWorksFromTheWorktreeItChecks(t *testing.T) {
	m, _, p := fixture(t)
	old, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chdir(p); e != nil {
		t.Fatal(e)
	}
	defer os.Chdir(old)
	if e = m.Guard(p); e != nil {
		t.Fatalf("checker counted its own file handles: %v", e)
	}
}

func TestOwnerCanFinishButLeaseStillBlocksRemoval(t *testing.T) {
	m, _, p := fixture(t)
	token, e := m.Lease(p)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Release(p, token)
	if e = m.Finish(p); e != nil {
		t.Fatalf("owner could not mark completion: %v", e)
	}
	if e = m.Guard(p); e == nil {
		t.Fatal("completion bypassed active lease")
	}
}

func TestIdleWorkspaceDoesNotReserveGrowthButResumeDoes(t *testing.T) {
	m, repo, path := fixture(t)
	m.space = func(string) (uint64, error) { return 8 * GiB, nil }
	if e := m.Admit(repo, "new"); e != nil {
		t.Fatal(e)
	}
	// Pending creation already reserves four GiB. Resuming another workspace
	// would exceed the remaining capacity and must not acquire a lease.
	if _, e := m.Lease(path); e == nil {
		t.Fatal("resume overcommitted pool")
	}
	s, e := m.ReadState()
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Workspaces[path].Leases) != 0 {
		t.Fatal("failed resume left lease")
	}
}
func TestNestedLeaseCountsWorkspaceOnce(t *testing.T) {
	m, _, path := fixture(t)
	m.space = func(string) (uint64, error) { return 8 * GiB, nil }
	a, e := m.Lease(path)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Release(path, a)
	b, e := m.Lease(path)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Release(path, b)
}
