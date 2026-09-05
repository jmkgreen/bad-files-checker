package app

import (
	"bad-files-checker/internal/scanner"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReturnsOneAndWritesLogWhenFindingsExist(t *testing.T) {
	root := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "logs", "bad-files.log")
	if err := os.Mkdir(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	code, err := Run([]string{"--scan-path", root, "--log-file", logFile}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if code != ExitFindings {
		t.Fatalf("code = %d, want %d", code, ExitFindings)
	}
	if !strings.Contains(stdout.String(), "scan completed with 1 bad folder") {
		t.Fatalf("stdout = %q", stdout.String())
	}

	log, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "empty-folder") {
		t.Fatalf("log missing finding:\n%s", string(log))
	}
}

func TestRunReturnsZeroWhenNoFindingsExist(t *testing.T) {
	root := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "bad-files.log")
	if err := os.WriteFile(filepath.Join(root, "image.jpg"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	code, err := Run([]string{"--scan-path", root, "--log-file", logFile}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if code != ExitOK {
		t.Fatalf("code = %d, want %d", code, ExitOK)
	}
}

func TestRunReturnsFatalForMissingRequiredArgs(t *testing.T) {
	code, err := Run([]string{"--scan-path", t.TempDir()}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run returned nil error")
	}
	if code != ExitFatal {
		t.Fatalf("code = %d, want %d", code, ExitFatal)
	}
}

func TestRunReturnsZeroForHelp(t *testing.T) {
	var stderr bytes.Buffer

	code, err := Run([]string{"--help"}, &bytes.Buffer{}, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if code != ExitOK {
		t.Fatalf("code = %d, want %d", code, ExitOK)
	}
	if !strings.Contains(stderr.String(), "-scan-path") {
		t.Fatalf("help output missing flags:\n%s", stderr.String())
	}
}

func TestRunReturnsFatalForUnknownFlag(t *testing.T) {
	code, err := Run([]string{"--nope"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run returned nil error")
	}
	if code != ExitFatal {
		t.Fatalf("code = %d, want %d", code, ExitFatal)
	}
}

func TestRunReturnsFatalWhenScanPathIsFile(t *testing.T) {
	root := t.TempDir()
	scanPath := filepath.Join(root, "image.jpg")
	if err := os.WriteFile(scanPath, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, err := Run([]string{"--scan-path", scanPath, "--log-file", filepath.Join(root, "log.txt")}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run returned nil error")
	}
	if code != ExitFatal {
		t.Fatalf("code = %d, want %d", code, ExitFatal)
	}
}

func TestRunCreatesMissingLogParents(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "image.jpg"), "ok")
	logFile := filepath.Join(t.TempDir(), "one", "two", "bad-files.log")

	code, err := Run([]string{"--scan-path", root, "--log-file", logFile}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if code != ExitOK {
		t.Fatalf("code = %d, want %d", code, ExitOK)
	}
	if _, err := os.Stat(logFile); err != nil {
		t.Fatalf("log file was not created: %v", err)
	}
}

func TestRunReturnsFatalForLogCloseError(t *testing.T) {
	root := t.TempDir()
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}

	code, err := run([]string{"--scan-path", root, "--log-file", "bad-files.log"}, &bytes.Buffer{}, &bytes.Buffer{}, dependencies{
		stat:     func(string) (os.FileInfo, error) { return info, nil },
		mkdirAll: func(string, os.FileMode) error { return nil },
		create:   func(string) (io.WriteCloser, error) { return closeFailWriter{}, nil },
		scan: func(string) (scanner.Result, error) {
			return scanner.Result{Root: root}, nil
		},
	})
	if err == nil {
		t.Fatal("run returned nil error")
	}
	if code != ExitFatal {
		t.Fatalf("code = %d, want %d", code, ExitFatal)
	}
	if !strings.Contains(err.Error(), "close log file") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunReturnsFatalForStatusWriteError(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "image.jpg"), "ok")

	code, err := Run([]string{
		"--scan-path", root,
		"--log-file", filepath.Join(t.TempDir(), "bad-files.log"),
	}, failingStatusWriter{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run returned nil error")
	}
	if code != ExitFatal {
		t.Fatalf("code = %d, want %d", code, ExitFatal)
	}
	if !strings.Contains(err.Error(), "write status") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseArgsRejectsUnexpectedPositionalArgs(t *testing.T) {
	_, err := parseArgs([]string{
		"--scan-path", t.TempDir(),
		"--log-file", filepath.Join(t.TempDir(), "log.txt"),
		"extra",
	}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("parseArgs returned nil error")
	}
}

func TestParseArgsAcceptsRequiredArgs(t *testing.T) {
	cfg, err := parseArgs([]string{
		"--scan-path", "/data/images",
		"--log-file", "/logs/bad-files.log",
	}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if cfg.scanPath != "/data/images" || cfg.logFile != "/logs/bad-files.log" {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func writeAppFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

type closeFailWriter struct{}

func (closeFailWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (closeFailWriter) Close() error {
	return errors.New("close failed")
}

type failingStatusWriter struct{}

func (failingStatusWriter) Write([]byte) (int, error) {
	return 0, errors.New("stdout failed")
}
