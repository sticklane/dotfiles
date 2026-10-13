package main

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func forgottenJJFixture(t *testing.T) (*Manager, string, string, string) {
	t.Helper()
	m, repo, p := jjFixture(t)
	if err := os.WriteFile(filepath.Join(p, "kept"), []byte("preserved\n"), 0644); err != nil {
		t.Fatal(err)
	}
	jjTest(t, p, "file", "track", "kept")
	if err := os.WriteFile(filepath.Join(p, "executable"), []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("kept", filepath.Join(p, "link")); err != nil {
		t.Fatal(err)
	}
	jjTest(t, p, "file", "track", "executable", "link")
	jjTest(t, p, "commit", "-m", "preserved content")
	head := jjTest(t, p, "--ignore-working-copy", "log", "--no-graph", "-r", "@-", "-T", "commit_id")
	gitTest(t, repo, "push", "origin", head+":refs/heads/preserved")
	jjTest(t, p, "git", "fetch", "--remote", "origin")
	if err := m.Finish(p); err != nil {
		t.Fatal(err)
	}
	m.removeDir = func(string) error { return errors.New("injected interrupted deletion") }
	if err := m.RemoveJJ(p, true); err == nil {
		t.Fatal("expected interrupted deletion")
	}
	s, err := m.ReadState()
	if err != nil {
		t.Fatal(err)
	}
	q := jjRemovalPath(s.Workspaces[p])
	m.removeDir = os.RemoveAll
	return m, repo, p, q
}

func TestResumeForgottenJJ(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(map[bool]string{false: "intact", true: "partial"}[partial], func(t *testing.T) {
			m, repo, p, q := forgottenJJFixture(t)
			if partial {
				if err := os.Remove(filepath.Join(q, "kept")); err != nil {
					t.Fatal(err)
				}
			}
			before := jjTest(t, repo, "--ignore-working-copy", "workspace", "list")
			if err := m.ResumeJJRemoval(p, false); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(q); err != nil {
				t.Fatal("preview changed quarantine", err)
			}
			if err := m.ResumeJJRemoval(p, true); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(q); !os.IsNotExist(err) {
				t.Fatal("quarantine not removed", err)
			}
			s, err := m.ReadState()
			if err != nil || s.Workspaces[p] != nil {
				t.Fatal("registry not completed", err)
			}
			if got := jjTest(t, repo, "--ignore-working-copy", "workspace", "list"); got != before {
				t.Fatal("shared registrations changed")
			}
			if gitTest(t, repo, "show", "refs/remotes/origin/preserved:kept") != "preserved" {
				t.Fatal("preserved history changed")
			}
		})
	}
}

func TestResumeForgottenJJRefusesUnsafeState(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*testing.T, *Manager, string, string, string)
	}{
		{"modified", func(t *testing.T, m *Manager, r, p, q string) {
			os.WriteFile(filepath.Join(q, "kept"), []byte("new work"), 0644)
		}},
		{"mode", func(t *testing.T, m *Manager, r, p, q string) { os.Chmod(filepath.Join(q, "kept"), 0755) }},
		{"unknown file", func(t *testing.T, m *Manager, r, p, q string) {
			os.WriteFile(filepath.Join(q, "notes"), []byte("valuable"), 0600)
		}},
		{"unknown directory", func(t *testing.T, m *Manager, r, p, q string) { os.Mkdir(filepath.Join(q, "notes"), 0700) }},
		{"symlink substitution", func(t *testing.T, m *Manager, r, p, q string) {
			os.Remove(filepath.Join(q, "kept"))
			os.Symlink(filepath.Join(r, "kept"), filepath.Join(q, "kept"))
		}},
		{"original recreated", func(t *testing.T, m *Manager, r, p, q string) { os.Mkdir(p, 0700) }},
		{"foreign token", func(t *testing.T, m *Manager, r, p, q string) {
			os.WriteFile(filepath.Join(q, ".jj", "dev-workspace-id"), []byte(strings.Repeat("0", 32)), 0600)
		}},
		{"lost remote ref", func(t *testing.T, m *Manager, r, p, q string) { gitTest(t, r, "push", "origin", ":preserved") }},
		{"reused name", func(t *testing.T, m *Manager, r, p, q string) {
			jjTest(t, r, "workspace", "add", "--name", "cleanup-test", filepath.Join(filepath.Dir(r), "replacement"))
		}},
		{"pin", func(t *testing.T, m *Manager, r, p, q string) {
			m.update(func(s *State) error { s.Workspaces[p].Pinned = true; return nil })
		}},
		{"lease", func(t *testing.T, m *Manager, r, p, q string) {
			m.update(func(s *State) error {
				s.Workspaces[p].Leases["test"] = Lease{PID: os.Getpid(), Start: processStart(os.Getpid())}
				return nil
			})
		}},
		{"volume", func(t *testing.T, m *Manager, r, p, q string) {
			m.verify = func() error { return errors.New("wrong volume") }
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, r, p, q := forgottenJJFixture(t)
			tc.change(t, m, r, p, q)
			removed := false
			m.removeDir = func(string) error { removed = true; return nil }
			if err := m.ResumeJJRemoval(p, true); err == nil {
				t.Fatal("unsafe state admitted")
			}
			if removed {
				t.Fatal("unsafe deletion attempted")
			}
			s, err := m.ReadState()
			if err != nil || s.Workspaces[p] == nil {
				t.Fatal("recovery record lost", err)
			}
		})
	}
}

func TestResumeForgottenJJDeletionFailureRetainsRecord(t *testing.T) {
	m, _, p, q := forgottenJJFixture(t)
	m.removeDir = func(string) error { return errors.New("injected") }
	if err := m.ResumeJJRemoval(p, true); err == nil {
		t.Fatal("deletion failure hidden")
	}
	s, err := m.ReadState()
	if err != nil || s.Workspaces[p] == nil || !s.Workspaces[p].Removing {
		t.Fatal("recovery record lost", err)
	}
	if _, err := os.Stat(q); err != nil {
		t.Fatal("quarantine lost", err)
	}
}

func TestResumeForgottenJJRemoteChangeDuringInventory(t *testing.T) {
	m, repo, p, _ := forgottenJJFixture(t)
	calls := 0
	m.verify = func() error {
		calls++
		if calls == 2 {
			gitTest(t, repo, "push", "origin", ":preserved")
		}
		return nil
	}
	removed := false
	m.removeDir = func(string) error { removed = true; return nil }
	if err := m.ResumeJJRemoval(p, true); err == nil || removed {
		t.Fatal("lost live preservation accepted", err)
	}
}

func TestResumeForgottenJJRechecksAfterRemoteProof(t *testing.T) {
	m, _, p, q := forgottenJJFixture(t)
	calls := 0
	m.verify = func() error {
		calls++
		if calls == 3 {
			if err := os.Rename(q, q+"-retained"); err != nil {
				return err
			}
			return os.Mkdir(q, 0700)
		}
		return nil
	}
	removed := false
	m.removeDir = func(string) error { removed = true; return nil }
	if err := m.ResumeJJRemoval(p, true); err == nil || removed {
		t.Fatal("late replacement admitted", err)
	}
	if calls != 3 {
		t.Fatalf("late proof boundary not exercised: %d", calls)
	}
}

func TestResumeForgottenJJIgnoresReplacementProof(t *testing.T) {
	m, repo, p, q := forgottenJJFixture(t)
	if err := os.WriteFile(filepath.Join(repo, "kept"), []byte("forged"), 0644); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repo, "read-tree", "refs/remotes/origin/preserved")
	gitTest(t, repo, "add", "kept")
	tree := gitTest(t, repo, "write-tree")
	forged := gitTest(t, repo, "commit-tree", tree, "-m", "forged tree")
	original := gitTest(t, repo, "rev-parse", "refs/remotes/origin/preserved")
	gitTest(t, repo, "replace", original, forged)
	if err := os.WriteFile(filepath.Join(q, "kept"), []byte("forged"), 0644); err != nil {
		t.Fatal(err)
	}
	removed := false
	m.removeDir = func(string) error { removed = true; return nil }
	if err := m.ResumeJJRemoval(p, true); err == nil || removed {
		t.Fatal("replacement fabricated preservation", err)
	}
}

func TestResumeForgottenJJRegistrySaveFailure(t *testing.T) {
	m, _, p, q := forgottenJJFixture(t)
	stateDir := m.cfg.StateDir
	m.removeDir = func(path string) error {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
		blocker := filepath.Join(stateDir, "not-a-directory")
		if err := os.WriteFile(blocker, []byte("block save"), 0600); err != nil {
			return err
		}
		m.cfg.StateDir = blocker
		return nil
	}
	err := m.ResumeJJRemoval(p, true)
	m.cfg.StateDir = stateDir
	if err == nil {
		t.Fatal("failed registry save reported success")
	}
	if _, err := os.Stat(q); !os.IsNotExist(err) {
		t.Fatal("expected physical deletion", err)
	}
	s, err := m.ReadState()
	if err != nil || s.Workspaces[p] == nil || !s.Workspaces[p].Removing {
		t.Fatal("durable unresolved record lost", err)
	}
	if err := m.ResumeJJRemoval(p, true); err == nil {
		t.Fatal("missing quarantine implicitly finalized")
	}
}

func TestRemainingJJTreeRejectsMalformedAndOversizedOutput(t *testing.T) {
	for _, raw := range []string{
		"truncated",
		"160000 commit " + strings.Repeat("a", 40) + "\tsubmodule\x00",
		"100644 blob " + strings.Repeat("a", 40) + "\t../outside\x00",
		"100644 blob " + strings.Repeat("a", 40) + "\tkept\x00" + "100644 blob " + strings.Repeat("a", 40) + "\tkept\x00",
	} {
		t.Run(strings.ReplaceAll(raw[:min(len(raw), 16)], " ", "-"), func(t *testing.T) {
			common, bin, q := t.TempDir(), t.TempDir(), t.TempDir()
			response := filepath.Join(t.TempDir(), "response")
			if err := os.WriteFile(response, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\nexec cat \"$TEST_GIT_RESPONSE\"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("TEST_GIT_RESPONSE", response)
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			if _, err := remainingJJTree(common, strings.Repeat("a", 40), q); err == nil {
				t.Fatal("malformed tree accepted")
			}
			if _, err := boundedGit(common, 1, "ls-tree"); err == nil {
				t.Fatal("output limit ignored")
			}
		})
	}
}

func TestResumeForgottenJJAdditionalGuards(t *testing.T) {
	for _, name := range []string{"changed symlink", "missing identity", "missing quarantine", "conflicting quarantine", "legacy graft", "file bound", "false deletion success", "caller owner", "unmanaged owner", "foreign repository", "symlink metadata"} {
		t.Run(name, func(t *testing.T) {
			m, repo, p, q := forgottenJJFixture(t)
			s, _ := m.ReadState()
			w := s.Workspaces[p]
			switch name {
			case "changed symlink":
				os.Remove(filepath.Join(q, "link"))
				os.Symlink("another-target", filepath.Join(q, "link"))
			case "missing identity":
				os.Remove(filepath.Join(q, ".jj", "dev-workspace-id"))
			case "missing quarantine":
				os.Rename(q, q+"-retained")
			case "conflicting quarantine":
				m.update(func(s *State) error { s.Workspaces[p].RemovalPath = q + "-other"; return nil })
			case "legacy graft":
				os.WriteFile(filepath.Join(w.Common, "info", "grafts"), []byte("\n"), 0600)
			case "file bound":
				os.Truncate(filepath.Join(q, "kept"), resumeFileLimit+1)
			case "false deletion success":
				m.removeDir = func(string) error { return nil }
			case "caller owner":
				cwd, _ := os.Getwd()
				if err := os.Chdir(q); err != nil {
					t.Fatal(err)
				}
				defer os.Chdir(cwd)
			case "unmanaged owner":
				cmd := exec.Command("sh", "-c", `cd "$1" && printf 'ready\n' && exec sleep 60`, "owner", q)
				stdout, err := cmd.StdoutPipe()
				if err != nil {
					t.Fatal(err)
				}
				if err = cmd.Start(); err != nil {
					t.Fatal(err)
				}
				defer func() { cmd.Process.Kill(); cmd.Wait() }()
				if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || line != "ready\n" {
					t.Fatal("owner did not start", err)
				}
			case "foreign repository":
				other := filepath.Join(filepath.Dir(repo), "other-repo")
				os.Mkdir(other, 0700)
				gitTest(t, other, "init", "-b", "main")
				gitTest(t, other, "commit", "--allow-empty", "-m", "other")
				jjTest(t, other, "git", "init", "--colocate")
				os.WriteFile(filepath.Join(q, ".jj", "repo"), []byte(filepath.Join(other, ".jj", "repo")), 0600)
			case "symlink metadata":
				metadata := filepath.Join(filepath.Dir(repo), "identity")
				os.Rename(filepath.Join(q, ".jj"), metadata)
				os.Symlink(metadata, filepath.Join(q, ".jj"))
			}
			if err := m.ResumeJJRemoval(p, true); err == nil {
				t.Fatal("unsafe state accepted")
			}
			s, err := m.ReadState()
			if err != nil || s.Workspaces[p] == nil {
				t.Fatal("failed cleanup lost record", err)
			}
		})
	}
}
