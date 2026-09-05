package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	scanPath string
	logFile  string
}

type dependencies struct {
	stat     func(string) (os.FileInfo, error)
	mkdirAll func(string, os.FileMode) error
	create   func(string) (io.WriteCloser, error)
	scan     func(string) (scanner.Result, error)
}

func Run(args []string, stdout io.Writer, stderr io.Writer) (int, error) {
	scan := scanner.New(scanner.Options{
		Clock: time.Now,
	})
	return run(args, stdout, stderr, dependencies{
		stat:     os.Stat,
		mkdirAll: os.MkdirAll,
		create:   func(path string) (io.WriteCloser, error) { return os.Create(path) },
		scan:     scan.Scan,
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

	if err := deps.mkdirAll(filepath.Dir(cfg.logFile), 0o755); err != nil {
		return ExitFatal, fmt.Errorf("create log directory: %w", err)
	}

	log, err := deps.create(cfg.logFile)
	if err != nil {
		return ExitFatal, fmt.Errorf("create log file: %w", err)
	}

	result, err := deps.scan(cfg.scanPath)
	if err != nil {
		log.Close()
		return ExitFatal, err
	}

	if err := report.WriteText(log, result); err != nil {
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

	cfg := config{}
	fs.StringVar(&cfg.scanPath, "scan-path", "", "root directory to scan recursively")
	fs.StringVar(&cfg.logFile, "log-file", "", "path to write scan results")

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

	return cfg, nil
}
