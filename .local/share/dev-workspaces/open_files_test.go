package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestOpenFileSnapshotParsing(t *testing.T) {
	path := "/pool/work space\nline"
	data := []byte("p12\x00\nfcwd\x00tREG\x00n" + path + "\x00\nf3\x00tREG\x00n" + path + "/nested/file\x00\np13\x00\nftxt\x00tREG\x00n" + path + "-other/keep\x00\np14\x00\nf4\x00tREG\x00n" + path + "/gone (deleted)\x00\n")
	got, e := openFilePIDs(data, path)
	if e != nil || !reflect.DeepEqual(got, []int{12, 14}) {
		t.Fatalf("%v %v", got, e)
	}
	for _, bad := range []string{"", "p12", "pno\x00", "n/path\x00", "p12\x00n/path\x00", "p12\x00zunknown\x00", "p12\x00f3\x00tREG\x00n\x00"} {
		if _, e := openFilePIDs([]byte(bad), path); e == nil {
			t.Fatalf("accepted malformed snapshot %q", bad)
		}
	}
	b := boundedOutput{limit: 3}
	if _, e := b.Write([]byte("1234")); e == nil || b.Len() != 0 {
		t.Fatal("unbounded snapshot accepted")
	}
}
func TestOpenFilesFindsNestedHandleWithoutCWD(t *testing.T) {
	path, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	nested := filepath.Join(path, "sub", "nested")
	if e := os.MkdirAll(filepath.Dir(nested), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(nested, []byte("data"), 0600); e != nil {
		t.Fatal(e)
	}
	c := exec.Command("sh", "-c", `exec 3< "$1"; echo ready; exec sleep 30`, "check", nested)
	pipe, e := c.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	if e = c.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { c.Process.Kill(); c.Wait() }()
	var ready [6]byte
	if _, e = pipe.Read(ready[:]); e != nil {
		t.Fatal(e)
	}
	if e = openFiles(path); e == nil || !strings.Contains(e.Error(), "workspace in use by pid "+strconv.Itoa(c.Process.Pid)) {
		t.Fatalf("nested descriptor not protected by its owner: %v", e)
	}
}

func TestOpenFileSnapshotUnnamedNonFilesystemDescriptors(t *testing.T) {
	for _, typ := range []string{"PIPE", "NPOLICY", "NEXUS"} {
		pids, e := openFilePIDs([]byte("p12\x00f3\x00t"+typ+"\x00n\x00\n"), "/pool/task")
		if e != nil || len(pids) != 0 {
			t.Fatalf("%s: %v %v", typ, pids, e)
		}
	}
}
