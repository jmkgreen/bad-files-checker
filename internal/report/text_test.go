package report

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"bad-files-checker/internal/scanner"
)

func TestWriteTextIncludesScanSummaryAndFindings(t *testing.T) {
	result := scanner.Result{
		ScannedAt: time.Date(2026, 9, 4, 9, 30, 0, 0, time.UTC),
		Root:      "/data/images",
		Findings: []scanner.FolderFinding{{
			Path:   "/data/images/set",
			Issues: []scanner.IssueType{scanner.IssueZeroSizeFile},
			Details: []scanner.FileFinding{{
				Path:    "/data/images/set/bad.jpg",
				Issue:   scanner.IssueZeroSizeFile,
				Details: "file is zero bytes",
			}},
		}},
	}

	var output bytes.Buffer
	if err := WriteText(&output, result); err != nil {
		t.Fatalf("WriteText returned error: %v", err)
	}

	text := output.String()
	for _, want := range []string{
		"Timestamp: 2026-09-04T09:30:00Z",
		"Root: /data/images",
		"Bad folders: 1",
		"Folder: /data/images/set",
		"[zero-size-file] /data/images/set/bad.jpg - file is zero bytes",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}

func TestWriteTextReportsNoBadFolders(t *testing.T) {
	var output bytes.Buffer
	err := WriteText(&output, scanner.Result{
		ScannedAt: time.Date(2026, 9, 4, 9, 30, 0, 0, time.UTC),
		Root:      "/data/images",
	})
	if err != nil {
		t.Fatalf("WriteText returned error: %v", err)
	}

	if !strings.Contains(output.String(), "No bad folders found.") {
		t.Fatalf("output missing no findings message:\n%s", output.String())
	}
}

func TestWriteTextReturnsWriterErrors(t *testing.T) {
	err := WriteText(failingWriter{}, scanner.Result{})
	if err == nil {
		t.Fatal("WriteText returned nil error")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errWriterFailed
}

var errWriterFailed = errors.New("writer failed")
