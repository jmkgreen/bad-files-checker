package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bad-files-checker/internal/report"
	"bad-files-checker/internal/scanner"
)

const (
	ExitOK       = 0
	ExitFindings = 1
	ExitFatal    = 2
)

type config struct {
	scanPath          string
	logFile           string
	runtimeSettings   runtimeSettings
	perDirectoryDelay time.Duration
	progressInterval  time.Duration
}

type dependencies struct {
	stat                 func(string) (os.FileInfo, error)
	mkdirAll             func(string, os.FileMode) error
	create               func(string) (io.WriteCloser, error)
	applyRuntimeSettings func(runtimeSettings) error
	scan                 func(string, time.Duration, func(scanner.Progress)) (scanner.Result, error)
}

func Run(args []string, stdout io.Writer, stderr io.Writer) (int, error) {
	return run(args, stdout, stderr, dependencies{
		stat:                 os.Stat,
		mkdirAll:             os.MkdirAll,
		create:               func(path string) (io.WriteCloser, error) { return os.Create(path) },
		applyRuntimeSettings: applyRuntimeSettings,
		scan: func(path string, perDirectoryDelay time.Duration, progress func(scanner.Progress)) (scanner.Result, error) {
			scan := scanner.New(scanner.Options{
				Clock:             time.Now,
				PerDirectoryDelay: perDirectoryDelay,
				Progress:          progress,
			})
			return scan.Scan(path)
		},
	})
}

func run(args []string, stdout io.Writer, stderr io.Writer, deps dependencies) (int, error) {
	cfg, err := parseArgs(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitOK, nil
		}
		return ExitFatal, err
	}

	info, err := deps.stat(cfg.scanPath)
	if err != nil {
		return ExitFatal, fmt.Errorf("scan path is not accessible: %w", err)
	}
	if !info.IsDir() {
		return ExitFatal, fmt.Errorf("scan path must be a directory: %s", cfg.scanPath)
	}

	if err := deps.applyRuntimeSettings(cfg.runtimeSettings); err != nil {
		return ExitFatal, err
	}

	if err := deps.mkdirAll(filepath.Dir(cfg.logFile), 0o755); err != nil {
		return ExitFatal, fmt.Errorf("create log directory: %w", err)
	}

	log, err := deps.create(cfg.logFile)
	if err != nil {
		return ExitFatal, fmt.Errorf("create log file: %w", err)
	}
	timestampedLog := newTimestampedWriter(log, time.Now)
	if _, err := fmt.Fprintf(timestampedLog, "Run begun: scanning %s\n", cfg.scanPath); err != nil {
		log.Close()
		return ExitFatal, fmt.Errorf("write log: %w", err)
	}

	progressSnapshot := newProgressSnapshot()
	stopProgress := startProgressLogging(timestampedLog, progressSnapshot, cfg.progressInterval)

	result, err := deps.scan(cfg.scanPath, cfg.perDirectoryDelay, progressSnapshot.update)
	stopProgress()
	if err != nil {
		log.Close()
		return ExitFatal, err
	}

	if _, err := fmt.Fprintln(timestampedLog, "Run completed: writing scan report"); err != nil {
		log.Close()
		return ExitFatal, fmt.Errorf("write log: %w", err)
	}
	if err := report.WriteText(timestampedLog, result); err != nil {
		log.Close()
		return ExitFatal, fmt.Errorf("write log: %w", err)
	}
	if err := log.Close(); err != nil {
		return ExitFatal, fmt.Errorf("close log file: %w", err)
	}

	if len(result.Findings) > 0 {
		if _, err := fmt.Fprintf(stdout, "scan completed with %d bad folder(s)\n", len(result.Findings)); err != nil {
			return ExitFatal, fmt.Errorf("write status: %w", err)
		}
		return ExitFindings, nil
	}

	if _, err := fmt.Fprintln(stdout, "scan completed with no bad folders"); err != nil {
		return ExitFatal, fmt.Errorf("write status: %w", err)
	}
	return ExitOK, nil
}

func parseArgs(args []string, stderr io.Writer) (config, error) {
	fs := flag.NewFlagSet("bad-files-checker", flag.ContinueOnError)
	fs.SetOutput(stderr)

	cfg := config{
		runtimeSettings: runtimeSettings{
			Nice:        defaultNice,
			IoniceClass: defaultIoniceClass,
			IoniceLevel: defaultIoniceLevel,
		},
	}
	fs.StringVar(&cfg.scanPath, "scan-path", "", "root directory to scan recursively")
	fs.StringVar(&cfg.logFile, "log-file", "", "path to write scan results")
	fs.IntVar(&cfg.runtimeSettings.Nice, "nice", defaultNice, "process niceness to request before scanning, from 0 to 19")
	fs.StringVar(&cfg.runtimeSettings.IoniceClass, "ionice-class", defaultIoniceClass, "Linux I/O priority class: none, idle, or best-effort")
	fs.IntVar(&cfg.runtimeSettings.IoniceLevel, "ionice-level", defaultIoniceLevel, "Linux best-effort I/O priority level, from 0 highest to 7 lowest")
	fs.DurationVar(&cfg.perDirectoryDelay, "scan-delay", 0, "optional pause before each directory scan, e.g. 10ms")
	fs.DurationVar(&cfg.progressInterval, "progress-interval", 30*time.Second, "how often to write in-progress scan counts to the log; set 0 to disable")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if cfg.scanPath == "" {
		return cfg, fmt.Errorf("missing required --scan-path")
	}
	if cfg.logFile == "" {
		return cfg, fmt.Errorf("missing required --log-file")
	}
	if fs.NArg() > 0 {
		return cfg, fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}
	settings, err := parseRuntimeSettings(cfg.runtimeSettings.Nice, cfg.runtimeSettings.IoniceClass, cfg.runtimeSettings.IoniceLevel)
	if err != nil {
		return cfg, err
	}
	cfg.runtimeSettings = settings
	if cfg.perDirectoryDelay < 0 {
		return cfg, fmt.Errorf("scan delay cannot be negative")
	}
	if cfg.progressInterval < 0 {
		return cfg, fmt.Errorf("progress interval cannot be negative")
	}

	return cfg, nil
}

type timestampedWriter struct {
	writer      io.Writer
	clock       func() time.Time
	mu          sync.Mutex
	atLineStart bool
}

func newTimestampedWriter(writer io.Writer, clock func() time.Time) *timestampedWriter {
	return &timestampedWriter{writer: writer, clock: clock, atLineStart: true}
}

func (w *timestampedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	remaining := string(p)
	written := len(p)
	for len(remaining) > 0 {
		if w.atLineStart {
			if _, err := fmt.Fprintf(w.writer, "%s ", w.clock().Format("2006-01-02T15:04:05Z07:00")); err != nil {
				return 0, err
			}
			w.atLineStart = false
		}

		index := strings.IndexByte(remaining, '\n')
		if index == -1 {
			if _, err := io.WriteString(w.writer, remaining); err != nil {
				return 0, err
			}
			break
		}

		if _, err := io.WriteString(w.writer, remaining[:index+1]); err != nil {
			return 0, err
		}
		w.atLineStart = true
		remaining = remaining[index+1:]
	}
	return written, nil
}

type progressSnapshot struct {
	mu       sync.Mutex
	progress scanner.Progress
}

func newProgressSnapshot() *progressSnapshot {
	return &progressSnapshot{}
}

func (s *progressSnapshot) update(progress scanner.Progress) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress = progress
}

func (s *progressSnapshot) current() scanner.Progress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progress
}

func startProgressLogging(writer io.Writer, snapshot *progressSnapshot, interval time.Duration) func() {
	if interval == 0 {
		return func() {}
	}

	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				progress := snapshot.current()
				fmt.Fprintf(
					writer,
					"Progress: directories scanned=%d, bad folders found=%d, bad file issues found=%d, current=%s\n",
					progress.DirectoriesScanned,
					progress.BadFoldersFound,
					progress.IssuesFound,
					progress.CurrentPath,
				)
			case <-done:
				return
			}
		}
	}()

	return func() {
		close(done)
		<-stopped
	}
}
