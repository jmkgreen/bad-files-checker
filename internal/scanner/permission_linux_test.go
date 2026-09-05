//go:build linux

package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanAttributesUnreadableNestedDirectoryToItself(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read directories after chmod 000")
	}

	root := t.TempDir()
	locked := filepath.Join(root, "set")
	mkdir(t, locked)
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o755)

	result, err := New(Options{Clock: fixedClock}).Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	findings := findingsByBase(result.Findings)
	finding, ok := findings["set"]
	if !ok {
		t.Fatalf("missing finding for unreadable directory: %#v", result.Findings)
	}
	if finding.Path != locked {
		t.Fatalf("finding path = %s, want %s", finding.Path, locked)
	}
	assertIssue(t, findings, "set", IssuePermissionDenied)
}
