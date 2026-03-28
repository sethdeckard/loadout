package fsx

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func mustSetup(t *testing.T, fn func() error) {
	t.Helper()
	if err := fn(); err != nil {
		t.Fatalf("setup: %v", err)
	}
}

func TestCopyDir(t *testing.T) {
	src := t.TempDir()
	// Create a nested structure
	mustSetup(t, func() error { return os.MkdirAll(filepath.Join(src, "sub"), 0o755) })
	mustSetup(t, func() error { return os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello"), 0o644) })
	mustSetup(t, func() error { return os.WriteFile(filepath.Join(src, "sub", "nested.txt"), []byte("world"), 0o644) })

	dst := filepath.Join(t.TempDir(), "dest")
	if err := CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir() error = %v", err)
	}

	// Verify files copied
	got, err := os.ReadFile(filepath.Join(dst, "file.txt"))
	if err != nil {
		t.Fatalf("read file.txt: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("file.txt = %q, want %q", got, "hello")
	}

	got, err = os.ReadFile(filepath.Join(dst, "sub", "nested.txt"))
	if err != nil {
		t.Fatalf("read nested.txt: %v", err)
	}
	if string(got) != "world" {
		t.Errorf("nested.txt = %q, want %q", got, "world")
	}
}

func TestCopyDir_NotADir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	mustSetup(t, func() error { return os.WriteFile(f, []byte("x"), 0o644) })
	err := CopyDir(f, filepath.Join(t.TempDir(), "dst"))
	if err == nil {
		t.Fatal("expected error for non-directory source")
	}
}

func TestCopyDir_MissingSrc(t *testing.T) {
	err := CopyDir("/nonexistent", filepath.Join(t.TempDir(), "dst"))
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestWriteJSONAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "data.json")

	data := map[string]string{"key": "value"}
	if err := WriteJSONAtomic(path, data); err != nil {
		t.Fatalf("WriteJSONAtomic() error = %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["key"] != "value" {
		t.Errorf("got %v, want key=value", got)
	}
}

func TestDirExists(t *testing.T) {
	dir := t.TempDir()
	if !DirExists(dir) {
		t.Error("DirExists should return true for existing dir")
	}
	if DirExists(filepath.Join(dir, "nope")) {
		t.Error("DirExists should return false for nonexistent")
	}
	f := filepath.Join(dir, "file")
	mustSetup(t, func() error { return os.WriteFile(f, []byte("x"), 0o644) })
	if DirExists(f) {
		t.Error("DirExists should return false for file")
	}
}

func TestListDirs(t *testing.T) {
	dir := t.TempDir()
	mustSetup(t, func() error { return os.MkdirAll(filepath.Join(dir, "alpha"), 0o755) })
	mustSetup(t, func() error { return os.MkdirAll(filepath.Join(dir, "beta"), 0o755) })
	mustSetup(t, func() error { return os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755) })
	mustSetup(t, func() error { return os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644) })

	dirs, err := ListDirs(dir)
	if err != nil {
		t.Fatalf("ListDirs() error = %v", err)
	}
	if len(dirs) != 2 {
		t.Fatalf("ListDirs() returned %d dirs, want 2: %v", len(dirs), dirs)
	}
}

func TestEnsureDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "c")
	if err := EnsureDir(path); err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}
	if !DirExists(path) {
		t.Error("EnsureDir did not create directory")
	}
}

// --- copyFile ---

// TestCopyFile_Success exercises the happy path of the unexported copyFile.
func TestCopyFile_Success(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")

	content := []byte("copy me")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("dst content = %q, want %q", got, content)
	}
}

// TestCopyFile_MissingSrc covers the os.Open error path in copyFile.
func TestCopyFile_MissingSrc(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "dst.txt")
	err := copyFile("/nonexistent/src.txt", dst)
	if err == nil {
		t.Fatal("expected error when source does not exist")
	}
}

// TestCopyFile_UnwritableDst covers the os.OpenFile error path in copyFile
// when the destination directory is not writable.
func TestCopyFile_UnwritableDst(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission model differs on Windows")
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Read-only destination directory so creating a file inside fails.
	roDir := filepath.Join(dir, "ro")
	if err := os.MkdirAll(roDir, 0o555); err != nil {
		t.Fatalf("setup ro dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })

	err := copyFile(src, filepath.Join(roDir, "dst.txt"))
	if err == nil {
		t.Fatal("expected error writing into read-only directory")
	}
}

// TestCopyFile_PreservesPermissions verifies that copyFile carries over the
// source file's permission bits.
func TestCopyFile_PreservesPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not meaningful on Windows")
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "exec.sh")
	dst := filepath.Join(dir, "exec_copy.sh")

	if err := os.WriteFile(src, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("dst perm = %o, want %o", info.Mode().Perm(), 0o755)
	}
}

// --- CopyDir ---

// TestCopyDir_NestedAndPermissions verifies that CopyDir preserves file
// permissions across multiple levels of nesting.
func TestCopyDir_NestedAndPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not meaningful on Windows")
	}

	src := t.TempDir()

	subDir := filepath.Join(src, "level1", "level2")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("setup MkdirAll: %v", err)
	}

	// 0o600 restricted file at the top level.
	topFile := filepath.Join(src, "top.txt")
	if err := os.WriteFile(topFile, []byte("top"), 0o600); err != nil {
		t.Fatalf("setup top file: %v", err)
	}

	// 0o755 executable at a nested level.
	deepFile := filepath.Join(subDir, "deep.sh")
	if err := os.WriteFile(deepFile, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("setup deep file: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "dest")
	if err := CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir() error = %v", err)
	}

	// Check top-level file.
	dstTop := filepath.Join(dst, "top.txt")
	gotBytes, err := os.ReadFile(dstTop)
	if err != nil {
		t.Fatalf("read top.txt: %v", err)
	}
	if string(gotBytes) != "top" {
		t.Errorf("top.txt content = %q, want %q", gotBytes, "top")
	}
	info, err := os.Stat(dstTop)
	if err != nil {
		t.Fatalf("stat top.txt: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("top.txt perm = %o, want %o", info.Mode().Perm(), 0o600)
	}

	// Check deeply nested file.
	dstDeep := filepath.Join(dst, "level1", "level2", "deep.sh")
	gotBytes, err = os.ReadFile(dstDeep)
	if err != nil {
		t.Fatalf("read deep.sh: %v", err)
	}
	if string(gotBytes) != "#!/bin/sh\n" {
		t.Errorf("deep.sh content = %q, want %q", gotBytes, "#!/bin/sh\n")
	}
	info, err = os.Stat(dstDeep)
	if err != nil {
		t.Fatalf("stat deep.sh: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("deep.sh perm = %o, want %o", info.Mode().Perm(), 0o755)
	}
}

// --- WriteJSONAtomic edge cases ---

// TestWriteJSONAtomic_UnmarshalableValue covers the json.MarshalIndent error
// path by passing a value that cannot be marshalled to JSON.
func TestWriteJSONAtomic_UnmarshalableValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	// A channel cannot be marshalled to JSON.
	err := WriteJSONAtomic(path, make(chan int))
	if err == nil {
		t.Fatal("expected error marshalling channel")
	}
}

// TestWriteJSONAtomic_NewlineAppended confirms the written file ends with a
// trailing newline (the byte appended after json.MarshalIndent).
func TestWriteJSONAtomic_NewlineAppended(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nl.json")
	if err := WriteJSONAtomic(path, map[string]int{"a": 1}); err != nil {
		t.Fatalf("WriteJSONAtomic() error = %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Errorf("file does not end with newline; content = %q", raw)
	}
}

// TestWriteJSONAtomic_Overwrites verifies that calling WriteJSONAtomic twice on
// the same path replaces the previous contents (atomic rename semantics).
func TestWriteJSONAtomic_Overwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")

	if err := WriteJSONAtomic(path, map[string]string{"v": "one"}); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteJSONAtomic(path, map[string]string{"v": "two"}); err != nil {
		t.Fatalf("second write: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["v"] != "two" {
		t.Errorf("got v=%q, want %q", got["v"], "two")
	}
}

// TestWriteJSONAtomic_UnwritableDir covers the os.MkdirAll / os.CreateTemp
// error path by targeting a path whose parent cannot be created.
func TestWriteJSONAtomic_UnwritableDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission model differs on Windows")
	}
	// /nonexistent-readonly-dir does not exist and cannot be created under /.
	err := WriteJSONAtomic("/nonexistent-readonly-dir/data.json", map[string]string{"k": "v"})
	if err == nil {
		t.Fatal("expected error writing to uncreateable directory")
	}
}

// --- ListDirs error path ---

// TestListDirs_NonExistent covers the os.ReadDir error path in ListDirs.
func TestListDirs_NonExistent(t *testing.T) {
	_, err := ListDirs("/nonexistent-dir-xyz-abc")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}
