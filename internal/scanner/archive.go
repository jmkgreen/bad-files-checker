package scanner

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var errUnsupportedArchive = errors.New("archive validation tool is unavailable")

type archiveValidator struct {
	runner commandRunner
}

type commandRunner interface {
	Run(name string, args ...string) error
	Exists(name string) bool
}

type osCommandRunner struct{}

func (osCommandRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v failed: %w: %s", name, args, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (osCommandRunner) Exists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func newArchiveValidator() archiveValidator {
	return archiveValidator{runner: osCommandRunner{}}
}

func isArchive(path string) bool {
	return archiveKind(path) != ""
}

func archiveKind(path string) string {
	name := strings.ToLower(filepath.Base(path))
	switch {
	case strings.HasSuffix(name, ".tar.gz"):
		return "tar.gz"
	case strings.HasSuffix(name, ".tgz"):
		return "tar.gz"
	case strings.HasSuffix(name, ".tar.bz2"):
		return "tar.bz2"
	case strings.HasSuffix(name, ".tbz2"):
		return "tar.bz2"
	case strings.HasSuffix(name, ".tar.xz"):
		return "tar.xz"
	case strings.HasSuffix(name, ".txz"):
		return "tar.xz"
	}

	switch filepath.Ext(name) {
	case ".zip":
		return "zip"
	case ".7z":
		return "7z"
	case ".rar":
		return "rar"
	case ".tar":
		return "tar"
	default:
		return ""
	}
}

func (v archiveValidator) validate(path string) error {
	switch archiveKind(path) {
	case "zip":
		return validateZip(path)
	case "tar":
		return validateTarFile(path, func(r io.Reader) (io.Reader, error) { return r, nil })
	case "tar.gz":
		return validateTarFile(path, func(r io.Reader) (io.Reader, error) { return gzip.NewReader(r) })
	case "tar.bz2":
		return validateTarFile(path, func(r io.Reader) (io.Reader, error) { return bzip2.NewReader(r), nil })
	case "tar.xz":
		return v.runFirstAvailable(path, [][]string{{"tar", "-tf", path}})
	case "7z":
		return v.runFirstAvailable(path, [][]string{{"7z", "t", path}, {"7zz", "t", path}, {"7za", "t", path}})
	case "rar":
		return v.runFirstAvailable(path, [][]string{{"unrar", "t", "-idq", path}, {"rar", "t", "-idq", path}})
	default:
		return errUnsupportedArchive
	}
}

func validateZip(path string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("%s: %w", file.Name, err)
		}
		_, copyErr := io.Copy(io.Discard, rc)
		closeErr := rc.Close()
		if copyErr != nil {
			return fmt.Errorf("%s: %w", file.Name, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("%s: %w", file.Name, closeErr)
		}
	}
	return nil
}

func validateTarFile(path string, wrap func(io.Reader) (io.Reader, error)) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	reader, err := wrap(file)
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(reader)
	for {
		_, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := io.Copy(io.Discard, tarReader); err != nil {
			return err
		}
	}
}

func (v archiveValidator) runFirstAvailable(path string, candidates [][]string) error {
	for _, candidate := range candidates {
		name := candidate[0]
		if !v.runner.Exists(name) {
			continue
		}
		return v.runner.Run(name, candidate[1:]...)
	}
	return fmt.Errorf("%w for %s", errUnsupportedArchive, archiveKind(path))
}
