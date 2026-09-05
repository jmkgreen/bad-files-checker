package scanner

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type fakeRunner struct {
	available map[string]bool
	calls     [][]string
	err       error
}

func (f *fakeRunner) Exists(name string) bool {
	return f.available[name]
}

func (f *fakeRunner) Run(name string, args ...string) error {
	f.calls = append(f.calls, append([]string{name}, args...))
	return f.err
}

func TestArchiveKindDetectsConfiguredFormats(t *testing.T) {
	tests := map[string]string{
		"images.zip":      "zip",
		"images.7z":       "7z",
		"images.rar":      "rar",
		"images.tar":      "tar",
		"images.tar.gz":   "tar.gz",
		"images.tgz":      "tar.gz",
		"images.tar.bz2":  "tar.bz2",
		"images.tbz2":     "tar.bz2",
		"images.tar.xz":   "tar.xz",
		"images.txz":      "tar.xz",
		"images.not-arch": "",
	}

	for name, want := range tests {
		if got := archiveKind(name); got != want {
			t.Fatalf("archiveKind(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestValidateZipReadsEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ok.zip")
	writeZip(t, path, map[string]string{"image.txt": "ok"})

	if err := validateZip(path); err != nil {
		t.Fatalf("validateZip returned error: %v", err)
	}
}

func TestValidateZipRejectsInvalidArchive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.zip")
	if err := os.WriteFile(path, []byte("not zip"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := validateZip(path); err == nil {
		t.Fatal("validateZip returned nil for invalid archive")
	}
}

func TestValidateTarGzReadsEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ok.tar.gz")
	writeTarGz(t, path, map[string]string{"image.txt": "ok"})

	validator := archiveValidator{runner: &fakeRunner{}}
	if err := validator.validate(path); err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
}

func TestValidatePlainTarReadsEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ok.tar")
	writeTar(t, path, map[string]string{"image.txt": "ok"})

	validator := archiveValidator{runner: &fakeRunner{}}
	if err := validator.validate(path); err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
}

func TestValidateRejectsUnknownArchiveKind(t *testing.T) {
	validator := archiveValidator{runner: &fakeRunner{}}
	err := validator.validate("set.unknown")
	if !errors.Is(err, errUnsupportedArchive) {
		t.Fatalf("err = %v, want errUnsupportedArchive", err)
	}
}

func TestExternalArchiveValidationUsesFirstAvailableTool(t *testing.T) {
	runner := &fakeRunner{available: map[string]bool{"7zz": true}}
	validator := archiveValidator{runner: runner}

	if err := validator.validate("set.7z"); err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	want := [][]string{{"7zz", "t", "set.7z"}}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestTarXzValidationRequiresTarTool(t *testing.T) {
	runner := &fakeRunner{available: map[string]bool{"xz": true}}
	validator := archiveValidator{runner: runner}

	err := validator.validate("set.tar.xz")
	if !errors.Is(err, errUnsupportedArchive) {
		t.Fatalf("err = %v, want errUnsupportedArchive", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v, want no xz-only fallback", runner.calls)
	}
}

func TestExternalArchiveValidationReportsUnsupportedWhenToolMissing(t *testing.T) {
	validator := archiveValidator{runner: &fakeRunner{available: map[string]bool{}}}

	err := validator.validate("set.rar")
	if !errors.Is(err, errUnsupportedArchive) {
		t.Fatalf("err = %v, want errUnsupportedArchive", err)
	}
}

func TestExternalArchiveValidationReturnsToolFailure(t *testing.T) {
	runner := &fakeRunner{
		available: map[string]bool{"unrar": true},
		err:       errors.New("crc failed"),
	}
	validator := archiveValidator{runner: runner}

	err := validator.validate("set.rar")
	if err == nil || err.Error() != "crc failed" {
		t.Fatalf("err = %v, want crc failed", err)
	}
}
