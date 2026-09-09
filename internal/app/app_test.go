package app

import (
	"bad-files-checker/internal/scanner"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
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

	log, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`(?m)^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`).Match(log) {
		t.Fatalf("log lines are not timestamped:\n%s", string(log))
	}
	for _, want := range []string{
		"Run begun: scanning",
		"Run completed: writing scan report",
		"No bad folders found.",
	} {
		if !strings.Contains(string(log), want) {
			t.Fatalf("log missing %q:\n%s", want, string(log))
		}
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
		stat:                 func(string) (os.FileInfo, error) { return info, nil },
		mkdirAll:             func(string, os.FileMode) error { return nil },
		create:               func(string) (io.WriteCloser, error) { return closeFailWriter{}, nil },
		applyRuntimeSettings: func(runtimeSettings) error { return nil },
		scan: func(string, time.Duration, func(scanner.Progress)) (scanner.Result, error) {
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
	if cfg.runtimeSettings.Nice != defaultNice {
		t.Fatalf("Nice = %d, want %d", cfg.runtimeSettings.Nice, defaultNice)
	}
	if cfg.runtimeSettings.IoniceClass != defaultIoniceClass {
		t.Fatalf("IoniceClass = %s, want %s", cfg.runtimeSettings.IoniceClass, defaultIoniceClass)
	}
	if cfg.runtimeSettings.IoniceLevel != defaultIoniceLevel {
		t.Fatalf("IoniceLevel = %d, want %d", cfg.runtimeSettings.IoniceLevel, defaultIoniceLevel)
	}
}

func TestParseArgsAcceptsLowImpactOverrides(t *testing.T) {
	cfg, err := parseArgs([]string{
		"--scan-path", "/data/images",
		"--log-file", "/logs/bad-files.log",
		"--nice", "19",
		"--ionice-class", "idle",
		"--scan-delay", "10ms",
	}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if cfg.runtimeSettings.Nice != 19 {
		t.Fatalf("Nice = %d, want 19", cfg.runtimeSettings.Nice)
	}
	if cfg.runtimeSettings.IoniceClass != "idle" {
		t.Fatalf("IoniceClass = %s, want idle", cfg.runtimeSettings.IoniceClass)
	}
	if cfg.runtimeSettings.IoniceLevel != 0 {
		t.Fatalf("IoniceLevel = %d, want 0", cfg.runtimeSettings.IoniceLevel)
	}
	if cfg.perDirectoryDelay != 10*time.Millisecond {
		t.Fatalf("perDirectoryDelay = %s, want 10ms", cfg.perDirectoryDelay)
	}
}

func TestParseArgsRejectsInvalidPrioritySettings(t *testing.T) {
	tests := [][]string{
		{"--nice", "-1"},
		{"--ionice-class", "realtime"},
		{"--ionice-class", "best-effort", "--ionice-level", "8"},
		{"--scan-delay", "-1ms"},
	}

	for _, testArgs := range tests {
		args := append([]string{
			"--scan-path", "/data/images",
			"--log-file", "/logs/bad-files.log",
		}, testArgs...)
		if _, err := parseArgs(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("parseArgs(%v) returned nil error", args)
		}
	}
}

func TestRunAppliesRuntimeSettingsAndScanDelay(t *testing.T) {
	root := t.TempDir()
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}

	var gotSettings runtimeSettings
	var gotDelay time.Duration

	code, err := run([]string{
		"--scan-path", root,
		"--log-file", "bad-files.log",
		"--nice", "12",
		"--ionice-class", "best-effort",
		"--ionice-level", "6",
		"--scan-delay", "5ms",
	}, &bytes.Buffer{}, &bytes.Buffer{}, dependencies{
		stat:     func(string) (os.FileInfo, error) { return info, nil },
		mkdirAll: func(string, os.FileMode) error { return nil },
		create:   func(string) (io.WriteCloser, error) { return closeOKWriter{}, nil },
		applyRuntimeSettings: func(settings runtimeSettings) error {
			gotSettings = settings
			return nil
		},
		scan: func(_ string, delay time.Duration, progress func(scanner.Progress)) (scanner.Result, error) {
			gotDelay = delay
			progress(scanner.Progress{
				DirectoriesScanned: 3,
				BadFoldersFound:    1,
				IssuesFound:        2,
				CurrentPath:        filepath.Join(root, "set"),
			})
			return scanner.Result{Root: root}, nil
		},
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if code != ExitOK {
		t.Fatalf("code = %d, want %d", code, ExitOK)
	}
	if gotSettings != (runtimeSettings{Nice: 12, IoniceClass: "best-effort", IoniceLevel: 6}) {
		t.Fatalf("settings = %#v", gotSettings)
	}
	if gotDelay != 5*time.Millisecond {
		t.Fatalf("delay = %s, want 5ms", gotDelay)
	}
}

func TestTimestampedWriterPrefixesEveryLine(t *testing.T) {
	var output bytes.Buffer
	writer := newTimestampedWriter(&output, fixedAppClock)

	if _, err := writer.Write([]byte("first\nsecond\n")); err != nil {
		t.Fatal(err)
	}

	want := "2026-09-04T09:30:00Z first\n2026-09-04T09:30:00Z second\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func TestProgressLoggingWritesLatestCounts(t *testing.T) {
	var output bytes.Buffer
	snapshot := newProgressSnapshot()
	snapshot.update(scanner.Progress{
		DirectoriesScanned: 7,
		BadFoldersFound:    2,
		IssuesFound:        4,
		CurrentPath:        "/data/images/set",
	})

	stop := startProgressLogging(&output, snapshot, time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	stop()

	for _, want := range []string{
		"Progress:",
		"directories scanned=7",
		"bad folders found=2",
		"bad file issues found=4",
		"current=/data/images/set",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("progress log missing %q:\n%s", want, output.String())
		}
	}
}

func writeAppFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixedAppClock() time.Time {
	return time.Date(2026, 9, 4, 9, 30, 0, 0, time.UTC)
}

type closeFailWriter struct{}

func (closeFailWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (closeFailWriter) Close() error {
	return errors.New("close failed")
}

type closeOKWriter struct{}

func (closeOKWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (closeOKWriter) Close() error {
	return nil
}

type failingStatusWriter struct{}

func (failingStatusWriter) Write([]byte) (int, error) {
	return 0, errors.New("stdout failed")
}
