package scanner

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestScanReportsEmptyZeroSizeArchiveOnlyAndInvalidArchiveFolders(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "empty"))
	mkdir(t, filepath.Join(root, "zero"))
	mkdir(t, filepath.Join(root, "archive-only"))
	mkdir(t, filepath.Join(root, "bad-archive"))
	mkdir(t, filepath.Join(root, "good"))

	writeFile(t, filepath.Join(root, "zero", "image.jpg"), "")
	writeZip(t, filepath.Join(root, "archive-only", "set.zip"), map[string]string{"image.txt": "ok"})
	writeFile(t, filepath.Join(root, "bad-archive", "set.zip"), "not a zip")
	writeFile(t, filepath.Join(root, "good", "image.jpg"), "ok")

	result, err := New(Options{Clock: fixedClock}).Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	findings := findingsByBase(result.Findings)
	assertIssue(t, findings, "empty", IssueEmptyFolder)
	assertIssue(t, findings, "zero", IssueZeroSizeFile)
	assertIssue(t, findings, "archive-only", IssueArchiveOnly)
	assertIssue(t, findings, "bad-archive", IssueArchiveInvalid)

	if _, ok := findings["good"]; ok {
		t.Fatal("good folder was reported")
	}
	if got := result.ScannedAt; !got.Equal(fixedClock()) {
		t.Fatalf("ScannedAt = %s, want fixed clock", got)
	}
}

func TestScanTreatsMissingExternalArchiveToolAsUnsupportedFinding(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "rar"))
	writeFile(t, filepath.Join(root, "rar", "set.rar"), "rar-ish")

	scan := Scanner{
		clock:             fixedClock,
		archiveValidation: archiveValidator{runner: &fakeRunner{available: map[string]bool{}}},
		openFile:          func(path string) (io.ReadCloser, error) { return os.Open(path) },
	}
	result, err := scan.Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	findings := findingsByBase(result.Findings)
	assertIssue(t, findings, "rar", IssueArchiveUnsupported)
	assertIssue(t, findings, "rar", IssueArchiveOnly)
}

func TestScanReportsNestedFindings(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "parent", "child")
	mkdir(t, nested)

	result, err := New(Options{Clock: fixedClock}).Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	findings := findingsByBase(result.Findings)
	assertIssue(t, findings, "child", IssueEmptyFolder)
}

func TestScanReportsProgress(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "empty"))
	writeFile(t, filepath.Join(root, "image.jpg"), "ok")

	var updates []Progress
	result, err := New(Options{
		Clock: fixedClock,
		Progress: func(progress Progress) {
			updates = append(updates, progress)
		},
	}).Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	if len(updates) == 0 {
		t.Fatal("Progress was not called")
	}
	last := updates[len(updates)-1]
	if last.DirectoriesScanned != 2 {
		t.Fatalf("DirectoriesScanned = %d, want 2", last.DirectoriesScanned)
	}
	if last.BadFoldersFound != len(result.Findings) {
		t.Fatalf("BadFoldersFound = %d, want %d", last.BadFoldersFound, len(result.Findings))
	}
	if last.IssuesFound != 1 {
		t.Fatalf("IssuesFound = %d, want 1", last.IssuesFound)
	}
}

func TestScanReportsNonArchiveReadFailure(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "set")
	mkdir(t, folder)
	path := filepath.Join(folder, "image.jpg")
	writeFile(t, path, "content")

	scan := New(Options{
		Clock: fixedClock,
		OpenFile: func(got string) (io.ReadCloser, error) {
			if got != path {
				t.Fatalf("OpenFile path = %s, want %s", got, path)
			}
			return nil, errors.New("read failed")
		},
	})
	result, err := scan.Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	findings := findingsByBase(result.Findings)
	assertIssue(t, findings, "set", IssueReadFailure)
}

func TestScanReportsEmptyRoot(t *testing.T) {
	root := t.TempDir()

	result, err := New(Options{Clock: fixedClock}).Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	findings := findingsByBase(result.Findings)
	assertIssue(t, findings, filepath.Base(root), IssueEmptyFolder)
}

func TestArchiveOnlyIgnoresSubdirectoriesByDesign(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "archive-with-subdir")
	mkdir(t, filepath.Join(folder, "images"))
	writeZip(t, filepath.Join(folder, "set.zip"), map[string]string{"image.txt": "ok"})

	result, err := New(Options{Clock: fixedClock}).Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	findings := findingsByBase(result.Findings)
	assertIssue(t, findings, "archive-with-subdir", IssueArchiveOnly)
}

func TestScanSkipsDirectorySymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating directory symlinks requires elevated privileges on many Windows setups")
	}

	root := t.TempDir()
	target := filepath.Join(root, "target")
	mkdir(t, target)
	mkdir(t, filepath.Join(root, "linked-empty"))
	if err := os.Symlink(filepath.Join(root, "linked-empty"), filepath.Join(target, "link")); err != nil {
		t.Fatal(err)
	}

	result, err := New(Options{Clock: fixedClock}).Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	for _, finding := range result.Findings {
		if filepath.Base(finding.Path) == "link" {
			t.Fatal("directory symlink was scanned")
		}
	}
}

func TestWalkErrorFolderAttributesDirectoryErrorsToDirectory(t *testing.T) {
	got := walkErrorFolder(filepath.Join("root", "bad-dir"), fakeDirEntry{dir: true})
	want := filepath.Join("root", "bad-dir")
	if got != want {
		t.Fatalf("walkErrorFolder = %s, want %s", got, want)
	}
}

func TestWalkErrorFolderAttributesFileErrorsToParent(t *testing.T) {
	got := walkErrorFolder(filepath.Join("root", "set", "bad.jpg"), fakeDirEntry{})
	want := filepath.Join("root", "set")
	if got != want {
		t.Fatalf("walkErrorFolder = %s, want %s", got, want)
	}
}

func TestScanReturnsFatalForMissingRoot(t *testing.T) {
	_, err := New(Options{}).Scan(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("Scan returned nil for missing root")
	}
}

type fakeDirEntry struct {
	dir bool
}

func (f fakeDirEntry) Name() string               { return "entry" }
func (f fakeDirEntry) IsDir() bool                { return f.dir }
func (f fakeDirEntry) Type() os.FileMode          { return 0 }
func (f fakeDirEntry) Info() (os.FileInfo, error) { return nil, nil }

func TestScanReturnsFatalWhenRootIsFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "image.jpg")
	writeFile(t, path, "ok")

	_, err := New(Options{}).Scan(path)
	if err == nil {
		t.Fatal("Scan returned nil for file root")
	}
}

func TestResultAddFileIssueMergesIssuesForSameFolder(t *testing.T) {
	var result Result

	result.addFileIssue("/root/set", "/root/set/a.jpg", IssueReadFailure, "first")
	result.addFileIssue("/root/set", "/root/set/b.jpg", IssueZeroSizeFile, "second")

	if len(result.Findings) != 1 {
		t.Fatalf("Findings length = %d, want 1", len(result.Findings))
	}
	assertIssue(t, map[string]FolderFinding{"set": result.Findings[0]}, "set", IssueReadFailure)
	assertIssue(t, map[string]FolderFinding{"set": result.Findings[0]}, "set", IssueZeroSizeFile)
}

func fixedClock() time.Time {
	return time.Date(2026, 9, 4, 9, 30, 0, 0, time.UTC)
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func findingsByBase(findings []FolderFinding) map[string]FolderFinding {
	values := map[string]FolderFinding{}
	for _, finding := range findings {
		values[filepath.Base(finding.Path)] = finding
	}
	return values
}

func assertIssue(t *testing.T, findings map[string]FolderFinding, folder string, issue IssueType) {
	t.Helper()
	finding, ok := findings[folder]
	if !ok {
		t.Fatalf("missing finding for %s", folder)
	}
	for _, got := range finding.Issues {
		if got == issue {
			return
		}
	}
	t.Fatalf("finding %s issues = %v, want %s", folder, finding.Issues, issue)
}
