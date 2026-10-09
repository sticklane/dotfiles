package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func jjTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	s, err := command(dir, "jj", args...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func jjFixture(t *testing.T) (*Manager, string, string) {
	t.Helper()
	m, repo, _ := fixture(t)
	remote := filepath.Join(filepath.Dir(repo), "remote.git")
	gitTest(t, repo, "init", "--bare", remote)
	gitTest(t, repo, "remote", "add", "origin", remote)
	gitTest(t, repo, "push", "origin", "main")
	jjTest(t, repo, "git", "init", "--colocate")
	p, err := m.CreateJJ(repo, "cleanup-test")
	if err != nil {
		t.Fatal(err)
	}
	return m, repo, p
}
func TestJJFinishSafety(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, *Manager, string, string)
		want   string
	}{
		{"remote empty child", func(t *testing.T, m *Manager, r, p string) {}, ""},
		{"untracked", func(t *testing.T, m *Manager, r, p string) {
			os.WriteFile(filepath.Join(p, "notes"), []byte("precious"), 0600)
		}, "untracked or ignored"},
		{"ignored", func(t *testing.T, m *Manager, r, p string) {
			os.WriteFile(filepath.Join(r, ".git", "info", "exclude"), []byte("local.db\n"), 0600)
			os.WriteFile(filepath.Join(p, "local.db"), []byte("precious"), 0600)
		}, "untracked or ignored"},
		{"empty directory", func(t *testing.T, m *Manager, r, p string) { os.Mkdir(filepath.Join(p, "notes"), 0700) }, "untracked or ignored"},
		{"scratch", func(t *testing.T, m *Manager, r, p string) {
			os.MkdirAll(filepath.Join(p, ".dev-build"), 0700)
			os.WriteFile(filepath.Join(p, ".dev-build", "tmp"), []byte("scratch"), 0600)
		}, ""},
		{"anonymous content", func(t *testing.T, m *Manager, r, p string) {
			os.WriteFile(filepath.Join(p, "work"), []byte("unique"), 0600)
			jjTest(t, p, "file", "track", "work")
		}, "remotely preserved"},
		{"described empty", func(t *testing.T, m *Manager, r, p string) { jjTest(t, p, "describe", "-m", "important task notes") }, "remotely preserved"},
		{"local only bookmark", func(t *testing.T, m *Manager, r, p string) {
			jjTest(t, p, "describe", "-m", "unpushed")
			jjTest(t, p, "bookmark", "set", "local-only")
		}, "remotely preserved"},
		{"pin", func(t *testing.T, m *Manager, r, p string) {
			if e := m.Pin(p, true); e != nil {
				t.Fatal(e)
			}
		}, "pinned"},
		{"identity", func(t *testing.T, m *Manager, r, p string) {
			os.WriteFile(filepath.Join(p, ".jj", "dev-workspace-id"), []byte("other"), 0600)
		}, "identity"},
		{"shared root", func(t *testing.T, m *Manager, r, p string) {
			os.Remove(filepath.Join(p, ".jj", "repo"))
			os.Mkdir(filepath.Join(p, ".jj", "repo"), 0700)
		}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, r, p := jjFixture(t)
			tc.mutate(t, m, r, p)
			err := m.Finish(p)
			if tc.name == "shared root" {
				if err == nil {
					t.Fatal("independent repository accepted")
				}
				return
			}
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q got %v", tc.want, err)
			}
		})
	}
}
func TestJJRemoteNonemptyAndChangedAfterFinish(t *testing.T) {
	m, r, p := jjFixture(t)
	os.WriteFile(filepath.Join(p, "work"), []byte("preserved"), 0600)
	jjTest(t, p, "file", "track", "work")
	head := jjTest(t, p, "log", "--no-graph", "-r", "@", "-T", "commit_id")
	// A real push establishes remote preservation without publishing to an external service.
	gitTest(t, r, "push", "origin", head+":refs/heads/task")
	jjTest(t, p, "git", "fetch", "--remote", "origin")
	if err := m.Finish(p); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(p, "work"), []byte("changed later"), 0600)
	if err := m.RemoveJJ(p, true); err == nil {
		t.Fatal("changed working tree removed")
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal("workspace lost")
	}
}
func TestJJCollectionRetentionAndSibling(t *testing.T) {
	m, r, p := jjFixture(t)
	sibling, err := m.CreateJJ(r, "sibling")
	if err != nil {
		t.Fatal(err)
	}
	if err = m.RemoveJJ(p, true); err == nil {
		t.Fatal("unfinished removed")
	}
	if err = m.Finish(p); err != nil {
		t.Fatal(err)
	}
	if len(m.Candidates()) != 0 {
		t.Fatal("retention bypassed")
	}
	m.now = func() time.Time { return time.Now().Add(25 * time.Hour) }
	if len(m.Candidates()) != 1 {
		t.Fatal("finished workspace not eligible")
	}
	if err = m.GC(false); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(p); err != nil {
		t.Fatal("preview removed workspace")
	}
	if err = m.GC(true); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("not removed: %v", err)
	}
	s, _ := m.ReadState()
	if s.Workspaces[p] != nil {
		t.Fatal("registry not reclaimed")
	}
	names := jjTest(t, sibling, "workspace", "list")
	if strings.Contains(names, "cleanup-test") {
		t.Fatal("workspace not forgotten")
	}
	if !strings.Contains(names, "sibling") || !strings.Contains(names, "default") {
		t.Fatal("sibling/primary lost")
	}
	jjTest(t, sibling, "op", "log", "--limit", "2")
	m.cfg.MaxWorkspaces = len(s.Workspaces) + 1
	if _, err = m.CreateJJ(r, "replacement"); err != nil {
		t.Fatal("admission not restored", err)
	}
}
func TestJJLiveLeaseAndOpenProcess(t *testing.T) {
	m, _, p := jjFixture(t)
	if err := m.Finish(p); err != nil {
		t.Fatal(err)
	}
	token, err := m.Lease(p)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := m.ReadState()
	if err = m.checkJJ(s.Workspaces[p]); err == nil || !strings.Contains(err.Error(), "active lease") {
		t.Fatal(err)
	}
	m.Release(p, token)
	// An unmanaged process also protects its workspace, even without a lease.
	c := exec.Command("/bin/sleep", "30")
	c.Dir = p
	if err = c.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { c.Process.Kill(); c.Wait() }()
	if err = m.Finish(p); err == nil || !strings.Contains(err.Error(), "in use") {
		t.Fatal("open cwd accepted", err)
	}
}
func TestJJInterruptedRemovalFailsClosed(t *testing.T) {
	m, _, p := jjFixture(t)
	if err := m.Finish(p); err != nil {
		t.Fatal(err)
	}
	m.removeDir = func(string) error { return errors.New("simulated remove failure") }
	if err := m.RemoveJJ(p, true); err == nil {
		t.Fatal("failure hidden")
	}
	s, _ := m.ReadState()
	if s.Workspaces[p] == nil || !s.Workspaces[p].Removing {
		t.Fatal("recovery record lost")
	}
	if _, err := os.Stat(s.Workspaces[p].RemovalPath); err != nil {
		t.Fatal("recovery directory lost")
	}
	m.removeDir = os.RemoveAll
	if err := m.RemoveJJ(p, true); err == nil {
		t.Fatal("forgotten workspace implicitly adopted")
	}
}

func TestJJInterruptedIntentReconcile(t *testing.T) {
	m, _, p := jjFixture(t)
	if err := m.Finish(p); err != nil {
		t.Fatal(err)
	}
	if err := m.update(func(s *State) error { s.Workspaces[p].Removing = true; return nil }); err != nil {
		t.Fatal(err)
	}
	m.now = func() time.Time { return time.Now().Add(25 * time.Hour) }
	if len(m.Candidates()) != 0 {
		t.Fatal("interrupted JJ removal auto-retried")
	}
	if err := m.RemoveJJ(p, true); err == nil {
		t.Fatal("interrupted removal auto-retried")
	}
	// Simulate a lost directory with a still-registered shared workspace. Keep
	// its recovery record; Git's post-remove recovery must not be applied to jj.
	if err := os.Rename(p, p+"-recovery"); err != nil {
		t.Fatal(err)
	}
	if err := m.Reconcile(); err != nil {
		t.Fatal(err)
	}
	s, _ := m.ReadState()
	if s.Workspaces[p] == nil || !s.Workspaces[p].Missing {
		t.Fatal("JJ recovery record discarded")
	}
}

func TestJJRecoverIntentAndQuarantine(t *testing.T) {
	for _, quarantine := range []bool{false, true} {
		t.Run(fmt.Sprint(quarantine), func(t *testing.T) {
			m, _, p := jjFixture(t)
			if e := m.Finish(p); e != nil {
				t.Fatal(e)
			}
			if e := m.update(func(s *State) error {
				w := s.Workspaces[p]
				w.Removing = true
				// Simulate an older in-memory manager dropping the optional JSON field.
				w.RemovalPath = ""
				if quarantine {
					return os.Rename(p, jjRemovalPath(w))
				}
				return nil
			}); e != nil {
				t.Fatal(e)
			}
			if e := m.RecoverJJ(p, false); e != nil {
				t.Fatal(e)
			}
			s, _ := m.ReadState()
			if !s.Workspaces[p].Removing {
				t.Fatal("preview mutated intent")
			}
			if e := m.RecoverJJ(p, true); e != nil {
				t.Fatal(e)
			}
			s, _ = m.ReadState()
			w := s.Workspaces[p]
			if w.Removing || !w.Pinned || !w.Finished.IsZero() || w.Head != "" {
				t.Fatal("not safely reset")
			}
			if _, e := os.Stat(p); e != nil {
				t.Fatal("tree not restored")
			}
		})
	}
}
func TestJJRecoveryRefusesForgottenRegistration(t *testing.T) {
	m, _, p := jjFixture(t)
	if e := m.Finish(p); e != nil {
		t.Fatal(e)
	}
	m.removeDir = func(string) error { return errors.New("interrupted") }
	if e := m.RemoveJJ(p, true); e == nil {
		t.Fatal("failure hidden")
	}
	if e := m.RecoverJJ(p, true); e == nil {
		t.Fatal("forgotten workspace re-adopted")
	}
}

func TestJJRemovalDoesNotExemptCurrentOwner(t *testing.T) {
	m, _, p := jjFixture(t)
	old, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chdir(p); e != nil {
		t.Fatal(e)
	}
	defer os.Chdir(old)
	// Completion may be recorded by its owner, but immediate deletion may not.
	if e = m.Finish(p); e != nil {
		t.Fatal(e)
	}
	if e = m.RemoveJJ(p, true); e == nil || !strings.Contains(e.Error(), "workspace in use") {
		t.Fatal("active current owner accepted", e)
	}
	s, _ := m.ReadState()
	if s.Workspaces[p].Removing {
		t.Fatal("intent recorded before owner guard")
	}
}
