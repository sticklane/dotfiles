package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type boundedOutput struct {
	bytes.Buffer
	limit int
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, fmt.Errorf("active-file snapshot exceeds output limit")
	}
	return b.Buffer.Write(p)
}

// lsof +D recursively stats the entire checkout, including rebuildable caches.
// A kernel snapshot instead finds cwd and nested open files without a tree walk.
func openFiles(path string) error           { return checkOpenFiles(path, true) }
func openFilesForRemoval(path string) error { return checkOpenFiles(path, false) }
func checkOpenFiles(path string, allowOwner bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "/usr/sbin/lsof", "-nP", "-F0pftn")
	out := boundedOutput{limit: 64 << 20}
	stderr := boundedOutput{limit: 16 << 10}
	c.Stdout = &out
	c.Stderr = &stderr
	err := c.Run()
	if ctx.Err() != nil {
		return fmt.Errorf("could not complete active-file check")
	}
	if err != nil || stderr.Len() != 0 {
		return fmt.Errorf("cannot verify open files: %v %s", err, stderr.String())
	}
	pids, err := openFilePIDs(out.Bytes(), path)
	if err != nil {
		return err
	}
	own := map[int]bool{}
	if allowOwner {
		own = currentAncestors()
	}
	for _, p := range pids {
		if !own[p] && p != c.Process.Pid && !errors.Is(syscall.Kill(p, 0), syscall.ESRCH) {
			return fmt.Errorf("workspace in use by pid %d", p)
		}
	}
	return nil
}

func openFilePIDs(data []byte, path string) ([]int, error) {
	if len(data) == 0 || !bytes.HasSuffix(data, []byte{0, '\n'}) && data[len(data)-1] != 0 {
		return nil, fmt.Errorf("incomplete active-file snapshot")
	}
	pid := 0
	file := false
	fileType := ""
	found := map[int]bool{}
	out := []int{}
	for _, raw := range bytes.Split(data, []byte{0}) {
		field := bytes.TrimPrefix(raw, []byte{'\n'})
		if len(field) == 0 {
			continue
		}
		switch field[0] {
		case 'p':
			n, e := strconv.Atoi(string(field[1:]))
			if e != nil || n <= 0 {
				return nil, fmt.Errorf("invalid active-file process field")
			}
			pid = n
			file = false
			fileType = ""
		case 'f':
			if pid == 0 || len(field) < 2 {
				return nil, fmt.Errorf("invalid active-file descriptor field")
			}
			file = true
			fileType = ""
		case 't':
			if pid == 0 || !file || len(field) < 2 {
				return nil, fmt.Errorf("invalid active-file type field")
			}
			fileType = string(field[1:])
		case 'n':
			if pid == 0 || !file || fileType == "" {
				return nil, fmt.Errorf("invalid active-file name field")
			}
			if len(field) == 1 {
				switch fileType {
				case "NPOLICY", "NEXUS", "PIPE":
					continue
				}
				return nil, fmt.Errorf("unnamed filesystem or unknown descriptor in active-file snapshot")
			}
			name := string(field[1:])
			if (name == path || strings.HasPrefix(name, path+string(os.PathSeparator))) && !found[pid] {
				found[pid] = true
				out = append(out, pid)
			}
		default:
			return nil, fmt.Errorf("unexpected active-file field %q", field[0])
		}
	}
	if pid == 0 {
		return nil, fmt.Errorf("active-file snapshot contains no processes")
	}
	return out, nil
}
