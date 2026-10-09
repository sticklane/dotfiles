package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestJJCreationIntentSurvivesInterruption(t *testing.T) {
	for _, afterAdd := range []bool{false, true} {
		t.Run(fmt.Sprint(afterAdd), func(t *testing.T) {
			m, repo, _ := fixture(t)
			if _, e := command(repo, "jj", "git", "init", "--colocate"); e != nil {
				t.Fatal(e)
			}
			realAdd := m.jjAdd
			m.jjAdd = func(dir string, args ...string) (string, error) {
				if afterAdd {
					if _, e := realAdd(dir, args...); e != nil {
						return "", e
					}
				}
				return "", fmt.Errorf("simulated caller interruption")
			}
			if _, e := m.CreateJJ(repo, "interrupted"); e == nil {
				t.Fatal("expected failure")
			}
			s, e := m.ReadState()
			if e != nil {
				t.Fatal(e)
			}
			if len(s.JJCreations) != 1 || len(s.Pending) != 0 || len(s.Workspaces) != 1 || activeReservations(s) != 1 {
				t.Fatalf("lost or double-counted intent: %+v", s)
			}
			var intent *JJCreation
			for _, c := range s.JJCreations {
				intent = c
			}
			if intent.ID == "" || intent.Base == "" || intent.Name != "interrupted" {
				t.Fatal("incomplete intent")
			}
			if _, e := os.Stat(intent.Path); afterAdd != (e == nil) {
				t.Fatalf("unexpected tree presence: %v", e)
			}
			// Model restart: reload from disk and advance beyond ordinary reservation expiry.
			m2 := NewManager(m.cfg)
			m2.verify = m.verify
			m2.space = m.space
			m2.now = func() time.Time { return m.now().Add(24 * time.Hour) }
			if e := m2.Reconcile(); e != nil {
				t.Fatal(e)
			}
			s, _ = m2.ReadState()
			if len(s.JJCreations) != 1 || activeReservations(s) != 1 {
				t.Fatal("intent expired")
			}
			if _, e := m2.CreateJJ(repo, "interrupted"); e == nil || !strings.Contains(e.Error(), "pending") {
				t.Fatalf("duplicate allowed: %v", e)
			}
			m2.cfg.MaxWorkspaces = 2
			if _, e := m2.CreateJJ(repo, "another"); e == nil || !strings.Contains(e.Error(), "limit") {
				t.Fatalf("capacity lost: %v", e)
			}
			if len(m2.Candidates()) != 0 {
				t.Fatal("creation eligible for deletion")
			}
			if afterAdd {
				if _, e := m2.Lease(intent.Path); e == nil {
					t.Fatal("partial tree leased")
				}
				if e := m2.Finish(intent.Path); e == nil {
					t.Fatal("partial tree finished")
				}
			}
		})
	}
}

func TestJJCreationRejectsUnexpectedToken(t *testing.T) {
	m, repo, _ := fixture(t)
	if _, e := command(repo, "jj", "git", "init", "--colocate"); e != nil {
		t.Fatal(e)
	}
	realAdd := m.jjAdd
	m.jjAdd = func(dir string, args ...string) (string, error) {
		out, e := realAdd(dir, args...)
		if e != nil {
			return out, e
		}
		e = os.WriteFile(filepath.Join(args[len(args)-1], ".jj", "dev-workspace-id"), []byte("foreign"), 0600)
		return out, e
	}
	if _, e := m.CreateJJ(repo, "foreign-token"); e == nil {
		t.Fatal("foreign token accepted")
	}
	s, _ := m.ReadState()
	if len(s.JJCreations) != 1 || len(s.Workspaces) != 1 {
		t.Fatal("failed creation lost")
	}
	for _, c := range s.JJCreations {
		b, _ := os.ReadFile(filepath.Join(c.Path, ".jj", "dev-workspace-id"))
		if string(b) != "foreign" {
			t.Fatal("foreign token overwritten")
		}
	}
}

func TestRegistryV2Migration(t *testing.T) {
	m, _, _ := fixture(t)
	s := newState()
	s.Version = 1
	s.JJCreations = nil
	s.Pending["git-reservation"] = m.now()
	s.Workspaces["retained"] = &Workspace{ID: "identity", Pinned: true, Removing: true, RemovalPath: "quarantine", Leases: map[string]Lease{"owner": {PID: 123}}}
	b, _ := json.Marshal(s)
	// A real v1 writer omits the newly introduced field.
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(b, &raw)
	delete(raw, "jj_creations")
	b, _ = json.Marshal(raw)
	if e := atomicWrite(m.statePath(), b); e != nil {
		t.Fatal(e)
	}
	if e := m.update(func(s *State) error { return nil }); e != nil {
		t.Fatal(e)
	}
	migrated, e := m.ReadState()
	if e != nil {
		t.Fatal(e)
	}
	w := migrated.Workspaces["retained"]
	if migrated.Version != 2 || migrated.JJCreations == nil || len(migrated.Pending) != 1 || !w.Pinned || !w.Removing || w.RemovalPath != "quarantine" || w.Leases["owner"].PID != 123 {
		t.Fatal("migration changed existing state")
	}
	// Simulate the previous binary's ReadState version gate.
	b, _ = os.ReadFile(m.statePath())
	var old struct{ Version int }
	_ = json.Unmarshal(b, &old)
	if old.Version == 1 {
		t.Fatal("old writer could erase new fields")
	}
	raw["version"] = json.RawMessage("2")
	b, _ = json.Marshal(raw)
	_ = atomicWrite(m.statePath(), b)
	if _, e := m.ReadState(); e == nil {
		t.Fatal("incomplete v2 accepted")
	}
}

func TestJJCreationRejectsChangedRevision(t *testing.T) {
	m, repo, _ := fixture(t)
	if _, e := command(repo, "jj", "git", "init", "--colocate"); e != nil {
		t.Fatal(e)
	}
	realAdd := m.jjAdd
	m.jjAdd = func(dir string, args ...string) (string, error) {
		out, e := realAdd(dir, args...)
		if e != nil {
			return out, e
		}
		_, e = command(args[len(args)-1], "jj", "new", "@")
		return out, e
	}
	if _, e := m.CreateJJ(repo, "changed-revision"); e == nil {
		t.Fatal("changed parent accepted")
	}
	s, _ := m.ReadState()
	if len(s.JJCreations) != 1 || len(s.Workspaces) != 1 {
		t.Fatal("mismatched creation finalized")
	}
}
