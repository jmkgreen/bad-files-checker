//go:build linux

package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTarXzValidationWithRealTarTool(t *testing.T) {
	requireTool(t, "tar")
	requireTool(t, "xz")

	dir := t.TempDir()
	source := filepath.Join(dir, "image.txt")
	writeFile(t, source, "ok")

	valid := filepath.Join(dir, "valid.tar.xz")
	runTool(t, "tar", "-cJf", valid, "-C", dir, "image.txt")

	validator := newArchiveValidator()
	if err := validator.validate(valid); err != nil {
		t.Fatalf("validate valid tar.xz returned error: %v", err)
	}

	corrupt := filepath.Join(dir, "corrupt.tar.xz")
	writeFile(t, corrupt, "not a tar.xz archive")
	if err := validator.validate(corrupt); err == nil {
		t.Fatal("validate corrupt tar.xz returned nil error")
	}
}

func TestTxzValidationWithRealTarTool(t *testing.T) {
	requireTool(t, "tar")
	requireTool(t, "xz")

	dir := t.TempDir()
	source := filepath.Join(dir, "image.txt")
	writeFile(t, source, "ok")

	valid := filepath.Join(dir, "valid.txz")
	runTool(t, "tar", "-cJf", valid, "-C", dir, "image.txt")

	validator := newArchiveValidator()
	if err := validator.validate(valid); err != nil {
		t.Fatalf("validate valid txz returned error: %v", err)
	}
}

func TestSevenZipValidationWithRealTool(t *testing.T) {
	tool := firstTool("7z", "7zz", "7za")
	if tool == "" {
		t.Skip("7z-compatible tool is not installed")
	}

	dir := t.TempDir()
	source := filepath.Join(dir, "image.txt")
	writeFile(t, source, "ok")

	valid := filepath.Join(dir, "valid.7z")
	runTool(t, tool, "a", valid, source)

	validator := newArchiveValidator()
	if err := validator.validate(valid); err != nil {
		t.Fatalf("validate valid 7z returned error: %v", err)
	}

	corrupt := filepath.Join(dir, "corrupt.7z")
	writeFile(t, corrupt, "not a 7z archive")
	if err := validator.validate(corrupt); err == nil {
		t.Fatal("validate corrupt 7z returned nil error")
	}
}

func TestRarValidationWithRealToolRejectsCorruptArchive(t *testing.T) {
	if firstTool("unrar", "rar") == "" {
		t.Skip("RAR validation tool is not installed")
	}

	path := filepath.Join(t.TempDir(), "corrupt.rar")
	writeFile(t, path, "not a rar archive")

	validator := newArchiveValidator()
	if err := validator.validate(path); err == nil {
		t.Fatal("validate corrupt rar returned nil error")
	}
}

func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s is not installed", name)
	}
}

func firstTool(names ...string) string {
	for _, name := range names {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}

func runTool(t *testing.T, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "LANG=C")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, string(output))
	}
}
