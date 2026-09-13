package main

import (
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"testing"
	"time"
)

func oldContainer(now time.Time) container {
	return container{ID: strings.Repeat("a", 64), Name: "/finished-build", Status: "exited",
		FinishedAt: now.Add(-8 * 24 * time.Hour).Format(time.RFC3339Nano), Restart: "no",
		Labels: map[string]string{}}
}

func TestRetentionProtectsRecentWorkAndServices(t *testing.T) {
	now := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		change   func(*container)
		eligible bool
	}{
		{"old stopped task", func(c *container) {}, true},
		{"just stopped", func(c *container) { c.FinishedAt = now.Add(-time.Hour).Format(time.RFC3339Nano) }, false},
		{"running race", func(c *container) { c.Running = true }, false},
		{"paused", func(c *container) { c.Paused = true }, false},
		{"restarting", func(c *container) { c.Restarting = true }, false},
		{"never started", func(c *container) { c.Status = "created" }, false},
		{"missing stop time", func(c *container) { c.FinishedAt = "" }, false},
		{"zero stop time", func(c *container) { c.FinishedAt = "0001-01-01T00:00:00Z" }, false},
		{"future stop time", func(c *container) { c.FinishedAt = now.Add(time.Hour).Format(time.RFC3339Nano) }, false},
		{"restartable service", func(c *container) { c.Restart = "unless-stopped" }, false},
		{"unknown restart policy", func(c *container) { c.Restart = "" }, false},
		{"compose service", func(c *container) { c.Labels["com.docker.compose.service"] = "db" }, false},
		{"keep label", func(c *container) { c.Labels["local.cleanup.keep"] = "true" }, false},
		{"ambiguous keep label", func(c *container) { c.Labels["keep"] = "" }, false},
		{"invalid ID", func(c *container) { c.ID = "--force" }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := oldContainer(now)
			tt.change(&c)
			if got := eligible(c, now, nil) == ""; got != tt.eligible {
				t.Fatalf("eligible=%v, want %v", got, tt.eligible)
			}
		})
	}
	c := oldContainer(now)
	for _, pin := range []string{c.ID, strings.TrimPrefix(c.Name, "/")} {
		if eligible(c, now, map[string]bool{pin: true}) == "" {
			t.Fatal("explicit pin ignored")
		}
	}
}

type fakeDocker struct {
	views        []container
	inspectError error
	removed      []string
}

func (f *fakeDocker) list(context.Context) ([]string, error) {
	return []string{strings.Repeat("a", 64)}, nil
}
func (f *fakeDocker) inspect(context.Context, string) (container, error) {
	if f.inspectError != nil {
		return container{}, f.inspectError
	}
	c := f.views[0]
	if len(f.views) > 1 {
		f.views = f.views[1:]
	}
	return c, nil
}
func (f *fakeDocker) remove(_ context.Context, id string) error {
	f.removed = append(f.removed, id)
	return nil
}

func TestSweepRechecksBeforeRemovalAndDefaultsToPreview(t *testing.T) {
	now := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	old := oldContainer(now)
	fresh := old
	fresh.FinishedAt = now.Format(time.RFC3339Nano)
	for _, tt := range []struct {
		name  string
		apply bool
		views []container
		want  int
	}{
		{"preview", false, []container{old}, 0},
		{"stale removal", true, []container{old, old}, 1},
		{"restarted and stopped since enumeration", true, []container{old, fresh}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeDocker{views: tt.views}
			if err := sweep(context.Background(), f, tt.apply, now, nil, log.New(io.Discard, "", 0)); err != nil {
				t.Fatal(err)
			}
			if len(f.removed) != tt.want {
				t.Fatalf("removed %v", f.removed)
			}
		})
	}
	f := &fakeDocker{inspectError: errors.New("inspection failed")}
	if err := sweep(context.Background(), f, true, now, nil, log.New(io.Discard, "", 0)); err == nil || len(f.removed) != 0 {
		t.Fatal("inspection failure must preserve container and report error")
	}
}
