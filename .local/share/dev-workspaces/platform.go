package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func command(dir, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, name, args...)
	c.Dir = dir
	var errbuf bytes.Buffer
	c.Stderr = &errbuf
	b, e := c.Output()
	if e != nil {
		return "", fmt.Errorf("%s: %w: %s", name, e, strings.TrimSpace(errbuf.String()))
	}
	return strings.TrimSpace(string(b)), nil
}
func git(dir string, args ...string) (string, error) { return command(dir, "git", args...) }
func freeSpace(path string) (uint64, error) {
	var s syscall.Statfs_t
	e := syscall.Statfs(path, &s)
	return uint64(s.Bavail) * uint64(s.Bsize), e
}
func diskInfo(path string) (map[string]string, error) {
	b, e := command("", "/usr/sbin/diskutil", "info", "-plist", path)
	if e != nil {
		return nil, e
	}
	d := xml.NewDecoder(strings.NewReader(b))
	out := map[string]string{}
	for {
		t, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		s, ok := t.(xml.StartElement)
		if !ok || s.Name.Local != "key" {
			continue
		}
		var k string
		if e = d.DecodeElement(&k, &s); e != nil {
			return nil, e
		}
		for {
			t, e = d.Token()
			if e != nil {
				return nil, e
			}
			v, ok := t.(xml.StartElement)
			if !ok {
				continue
			}
			if v.Name.Local == "string" || v.Name.Local == "integer" {
				var value string
				if e = d.DecodeElement(&value, &v); e != nil {
					return nil, e
				}
				out[k] = value
			} else if v.Name.Local == "true" || v.Name.Local == "false" {
				out[k] = v.Name.Local
			} else {
				if e = d.Skip(); e != nil {
					return nil, e
				}
			}
			break
		}
	}
	return out, nil
}
func verifyDisk(path, uuid, fs string) error {
	d, e := diskInfo(path)
	if e != nil {
		return e
	}
	if d["MountPoint"] != path || !strings.EqualFold(d["VolumeUUID"], uuid) || d["WritableVolume"] != "true" {
		return fmt.Errorf("wrong, missing, or read-only volume at %s", path)
	}
	if fs != "" && d["FilesystemType"] != fs {
		return fmt.Errorf("%s requires %s", path, fs)
	}
	return nil
}
func (m *Manager) verifyVolumes() error {
	if e := verifyDisk(m.cfg.BackingPath, m.cfg.BackingUUID, ""); e != nil {
		return e
	}
	if e := verifyDisk(m.cfg.Root, m.cfg.VolumeUUID, "apfs"); e != nil {
		return e
	}
	return nil
}
func (m *Manager) Mount() error {
	if e := verifyDisk(m.cfg.BackingPath, m.cfg.BackingUUID, ""); e != nil {
		return e
	}
	if m.verify() == nil {
		return nil
	}
	if _, e := os.Stat(m.cfg.Root); e == nil {
		return fmt.Errorf("mount point already exists with unexpected volume: %s", m.cfg.Root)
	}
	if !inside(m.cfg.BackingPath, m.cfg.Image) {
		return fmt.Errorf("image is outside backing volume")
	}
	if _, e := command("", "/usr/bin/hdiutil", "attach", "-nobrowse", "-mountpoint", m.cfg.Root, m.cfg.Image); e != nil {
		return e
	}
	return m.verify()
}
func processStart(pid int) string {
	v, e := command("", "/bin/ps", "-p", strconv.Itoa(pid), "-o", "lstart=")
	if e != nil {
		return ""
	}
	return v
}
func live(l Lease) bool { return l.Start != "" && processStart(l.PID) == l.Start }
func currentAncestors() map[int]bool {
	result := map[int]bool{}
	pid := os.Getpid()
	for pid > 1 && !result[pid] {
		result[pid] = true
		s, e := command("", "/bin/ps", "-p", strconv.Itoa(pid), "-o", "ppid=")
		if e != nil {
			break
		}
		pid, _ = strconv.Atoi(strings.TrimSpace(s))
	}
	return result
}
func openFiles(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "/usr/sbin/lsof", "-t", "+D", path)
	var stderr bytes.Buffer
	c.Stderr = &stderr
	b, e := c.Output()
	if ctx.Err() != nil {
		return fmt.Errorf("could not complete active-file check")
	}
	if e != nil {
		x, ok := e.(*exec.ExitError)
		if !ok || x.ExitCode() != 1 || stderr.Len() != 0 {
			return fmt.Errorf("cannot verify open files: %v %s", e, stderr.String())
		}
	}
	own := currentAncestors()
	for _, line := range strings.Fields(string(b)) {
		p, e := strconv.Atoi(line)
		if e != nil {
			return e
		}
		// macOS lsof may fork a filesystem helper. Completed helpers (and any
		// other process that has since exited) no longer hold the workspace.
		if !own[p] && p != c.Process.Pid && !errors.Is(syscall.Kill(p, 0), syscall.ESRCH) {
			return fmt.Errorf("workspace in use by pid %d", p)
		}
	}
	return nil
}
func diskUsage(path string) (uint64, error) {
	s, e := command("", "/usr/bin/du", "-sk", path)
	if e != nil {
		return 0, e
	}
	f := strings.Fields(s)
	if len(f) == 0 {
		return 0, fmt.Errorf("empty du output")
	}
	n, e := strconv.ParseUint(f[0], 10, 64)
	return n * 1024, e
}
func canon(path string) (string, error) {
	p, e := filepath.Abs(path)
	if e != nil {
		return "", e
	}
	return filepath.EvalSymlinks(p)
}
