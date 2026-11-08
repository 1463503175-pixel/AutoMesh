package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyWorkspaceVerifiedKeepsSource(t *testing.T) {
	p := t.TempDir()
	src := filepath.Join(p, "old")
	dst := filepath.Join(p, "new")
	if e := os.MkdirAll(filepath.Join(src, "nested"), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(src, "nested", "quality_report.html"), []byte("old data"), 0600); e != nil {
		t.Fatal(e)
	}
	n, e := copyWorkspaceVerified(src, dst)
	if e != nil || n != 1 {
		t.Fatalf("%d %v", n, e)
	}
	for _, dir := range []string{src, dst} {
		b, e := os.ReadFile(filepath.Join(dir, "nested", "quality_report.html"))
		if e != nil || string(b) != "old data" {
			t.Fatalf("data changed %s %v", dir, e)
		}
	}
}
func TestCopyWorkspaceVerifiedRejectsOverwriteAndNestedPaths(t *testing.T) {
	p := t.TempDir()
	src := filepath.Join(p, "src")
	dst := filepath.Join(p, "new")
	os.MkdirAll(src, 0755)
	os.MkdirAll(dst, 0755)
	if _, e := copyWorkspaceVerified(src, dst); e == nil {
		t.Fatal("overwrote existing destination")
	}
	if _, e := copyWorkspaceVerified(src, filepath.Join(src, "nested")); e == nil {
		t.Fatal("accepted nested destination")
	}
}
func TestCopyWorkspaceVerifiedRejectsSymlink(t *testing.T) {
	p := t.TempDir()
	src := filepath.Join(p, "src")
	os.Mkdir(src, 0755)
	outside := filepath.Join(p, "outside")
	os.WriteFile(outside, []byte("personal"), 0600)
	if e := os.Symlink(outside, filepath.Join(src, "link")); e != nil {
		t.Skip(e)
	}
	if _, e := copyWorkspaceVerified(src, filepath.Join(p, "dst")); e == nil {
		t.Fatal("followed symlink")
	}
}
func TestDataDirectoryRejectsRelative(t *testing.T) {
	if e := validateDataDirectory("./C:/name"); e == nil {
		t.Fatal("accepted relative path")
	}
	if e := validateDataDirectory("/tmp/中文"); e == nil {
		t.Fatal("accepted non ASCII path")
	}
	if e := validateDataDirectory(strings.Repeat("a", 3)); e == nil {
		t.Fatal("accepted non-absolute")
	}
}
func TestUnrecognizedWorkerNotOverwritten(t *testing.T) {
	old := []byte("manually edited worker")
	if e := checkGeneratedAssetReplacement("meshing_worker.py", old, []byte("new worker")); e == nil {
		t.Fatal("unknown local worker accepted")
	}
	if e := checkGeneratedAssetReplacement("meshing_worker.py", old, old); e != nil {
		t.Fatal("matching file refused", e)
	}
}
