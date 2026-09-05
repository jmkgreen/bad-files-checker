package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bad-files-checker/internal/app"
)

func TestRunReturnsCodeAndPrintsErrors(t *testing.T) {
	var stderr bytes.Buffer

	code := run(nil, &bytes.Buffer{}, &stderr)

	if code != app.ExitFatal {
		t.Fatalf("code = %d, want %d", code, app.ExitFatal)
	}
	if !strings.Contains(stderr.String(), "missing required --scan-path") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunReturnsSuccessCode(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "image.jpg"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	code := run([]string{
		"--scan-path", root,
		"--log-file", filepath.Join(t.TempDir(), "bad-files.log"),
	}, &bytes.Buffer{}, &bytes.Buffer{})

	if code != app.ExitOK {
		t.Fatalf("code = %d, want %d", code, app.ExitOK)
	}
}
