// docker-cleanup is user-scoped Colima housekeeping, independent of any repository runtime.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const retention = 7 * 24 * time.Hour
const dockerPath = "/usr/local/bin/docker"
const endpoint = "unix:///Users/sjaconette/.colima/default/docker.sock"

var containerID = regexp.MustCompile(`^[a-f0-9]{64}$`)

type container struct {
	ID, Name, Status, FinishedAt, Restart string
	Running, Paused, Restarting           bool
	Labels                                map[string]string
}

// Empty reason means eligible. Pins use full IDs or exact names without a leading slash.
func eligible(c container, now time.Time, pins map[string]bool) string {
	if !containerID.MatchString(c.ID) {
		return "invalid container identity"
	}
	if c.Running || c.Paused || c.Restarting || (c.Status != "exited" && c.Status != "dead") {
		return "not stopped"
	}
	if pins[c.ID] || pins[strings.TrimPrefix(c.Name, "/")] {
		return "keep list"
	}
	if c.Restart != "no" {
		return "restart policy"
	}
	for key := range c.Labels {
		if strings.HasPrefix(key, "com.docker.compose.") {
			return "Compose managed"
		}
		if key == "keep" || key == "local.cleanup.keep" {
			return "keep label"
		}
	}
	finished, err := time.Parse(time.RFC3339Nano, c.FinishedAt)
	if err != nil || finished.IsZero() || finished.After(now) {
		return "invalid stop time"
	}
	if now.Sub(finished) < retention {
		return "stopped less than seven days"
	}
	return ""
}

type dockerClient interface {
	list(context.Context) ([]string, error)
	inspect(context.Context, string) (container, error)
	remove(context.Context, string) error
}

type dockerCLI struct{}

func (dockerCLI) call(parent context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, dockerPath, append([]string{"--host", endpoint}, args...)...)
	// Ignore an interactive shell's remote context or TLS overrides.
	cmd.Env = make([]string, 0)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "DOCKER_") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker %s: %w: %.512s", args[0], err, output)
	}
	return output, nil
}

func (d dockerCLI) list(ctx context.Context) ([]string, error) {
	output, err := d.call(ctx, "container", "ls", "--all", "--quiet", "--no-trunc", "--filter", "status=exited", "--filter", "status=dead")
	return strings.Fields(string(output)), err
}

func (d dockerCLI) inspect(ctx context.Context, id string) (container, error) {
	if !containerID.MatchString(id) {
		return container{}, errors.New("invalid listed container ID")
	}
	// Request only cleanup metadata, never environment variables, commands or container logs.
	const format = `{"ID":{{json .Id}},"Name":{{json .Name}},"Status":{{json .State.Status}},"Running":{{json .State.Running}},"Paused":{{json .State.Paused}},"Restarting":{{json .State.Restarting}},"FinishedAt":{{json .State.FinishedAt}},"Restart":{{json .HostConfig.RestartPolicy.Name}},"Labels":{{json .Config.Labels}}}`
	output, err := d.call(ctx, "container", "inspect", "--format", format, id)
	if err != nil {
		return container{}, err
	}
	var c container
	if err := json.Unmarshal(output, &c); err != nil {
		return c, err
	}
	if c.ID != id {
		return c, errors.New("inspected identity mismatch")
	}
	return c, nil
}

func (d dockerCLI) remove(ctx context.Context, id string) error {
	if !containerID.MatchString(id) {
		return errors.New("invalid removal identity")
	}
	// No --force: the daemon rejects a currently running container. No -v: preserve volumes.
	_, err := d.call(ctx, "container", "rm", id)
	return err
}

func sweep(ctx context.Context, docker dockerClient, apply bool, now time.Time, pins map[string]bool, logger *log.Logger) error {
	ids, err := docker.list(ctx)
	if err != nil {
		return err
	}
	var problems []error
	removed, candidates := 0, 0
	for _, id := range ids {
		c, err := docker.inspect(ctx, id)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		if reason := eligible(c, now, pins); reason != "" {
			logger.Printf("keep id=%s name=%q reason=%q", c.ID, c.Name, reason)
			continue
		}
		candidates++
		if !apply {
			logger.Printf("would-remove id=%s name=%q stopped=%s", c.ID, c.Name, c.FinishedAt)
			continue
		}
		// Re-inspect by immutable ID immediately before deletion; fresh restarts retain their grace period.
		current, err := docker.inspect(ctx, id)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		if reason := eligible(current, now, pins); reason != "" || current.FinishedAt != c.FinishedAt {
			logger.Printf("keep id=%s reason=%q", id, "changed before removal")
			continue
		}
		if err := docker.remove(ctx, id); err != nil {
			problems = append(problems, err)
			continue
		}
		removed++
		logger.Printf("removed id=%s name=%q stopped=%s", current.ID, current.Name, current.FinishedAt)
	}
	logger.Printf("complete apply=%t inspected=%d eligible=%d removed=%d errors=%d", apply, len(ids), candidates, removed, len(problems))
	return errors.Join(problems...)
}

func readPins(path string) (map[string]bool, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	pins := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		value := strings.TrimSpace(scanner.Text())
		if value != "" && !strings.HasPrefix(value, "#") {
			pins[value] = true
		}
	}
	return pins, scanner.Err()
}

func run(apply bool) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	stateDir := filepath.Join(homeDir, ".local/state/docker-cleanup")
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(stateDir, "run.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil
		}
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	logPath := filepath.Join(stateDir, "service.log")
	if info, err := os.Stat(logPath); err == nil && info.Size() >= 1024*1024 {
		if err := os.Rename(logPath, logPath+".1"); err != nil {
			return err
		}
	}
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	logger := log.New(io.MultiWriter(os.Stderr, file), "", log.LstdFlags|log.LUTC)
	pins, err := readPins(filepath.Join(homeDir, ".config/docker-cleanup/keep.txt"))
	if err != nil {
		logger.Printf("error reading keep list: %v", err)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	docker := dockerCLI{}
	if _, err := docker.call(ctx, "version", "--format", "{{.Server.Version}}"); err != nil {
		logger.Printf("skip: Colima Docker unavailable: %v", err)
		return nil
	}
	if err := sweep(ctx, docker, apply, time.Now(), pins, logger); err != nil {
		logger.Printf("cleanup errors: %v", err)
		return err
	}
	return nil
}

func main() {
	apply := flag.Bool("apply", false, "remove eligible containers; default only previews")
	flag.Parse()
	if flag.NArg() != 0 {
		log.Fatal("usage: docker-cleanup [--apply]")
	}
	if err := run(*apply); err != nil {
		log.Fatal(err)
	}
}
