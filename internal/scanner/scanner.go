package scanner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Options struct {
	Clock             func() time.Time
	OpenFile          func(string) (io.ReadCloser, error)
	PerDirectoryDelay time.Duration
}

type Scanner struct {
	clock             func() time.Time
	archiveValidation archiveValidator
	openFile          func(string) (io.ReadCloser, error)
	perDirectoryDelay time.Duration
}

func New(options Options) Scanner {
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	openFile := options.OpenFile
	if openFile == nil {
		openFile = func(path string) (io.ReadCloser, error) { return os.Open(path) }
	}

	return Scanner{
		clock:             clock,
		archiveValidation: newArchiveValidator(),
		openFile:          openFile,
		perDirectoryDelay: options.PerDirectoryDelay,
	}
}

func (s Scanner) Scan(root string) (Result, error) {
	info, err := os.Stat(root)
	if err != nil {
		return Result{}, fmt.Errorf("scan root is not accessible: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("scan root must be a directory: %s", root)
	}

	result := Result{
		ScannedAt: s.clock(),
		Root:      root,
	}

	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			result.addFileIssue(walkErrorFolder(path, entry), path, IssuePermissionDenied, walkErr.Error())
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !entry.IsDir() {
			return nil
		}

		if entry.Type()&os.ModeSymlink != 0 {
			return filepath.SkipDir
		}

		if s.perDirectoryDelay > 0 {
			time.Sleep(s.perDirectoryDelay)
		}

		finding := s.scanDirectory(path)
		if len(finding.Issues) > 0 {
			result.Findings = append(result.Findings, finding)
		}
		return nil
	})
	if err != nil {
		return Result{}, fmt.Errorf("walk scan path: %w", err)
	}

	sort.Slice(result.Findings, func(i, j int) bool {
		return result.Findings[i].Path < result.Findings[j].Path
	})

	return result, nil
}

func (s Scanner) scanDirectory(path string) FolderFinding {
	finding := FolderFinding{Path: path}

	entries, err := os.ReadDir(path)
	if err != nil {
		finding.add(IssuePermissionDenied, path, err.Error())
		return finding
	}

	regularFiles := 0
	archiveFiles := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}

		fullPath := filepath.Join(path, entry.Name())
		info, err := entry.Info()
		if err != nil {
			finding.add(IssueReadFailure, fullPath, err.Error())
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}

		regularFiles++
		if info.Size() == 0 {
			finding.add(IssueZeroSizeFile, fullPath, "file is zero bytes")
		}

		if isArchive(fullPath) {
			archiveFiles++
			if err := s.archiveValidation.validate(fullPath); err != nil {
				issue := IssueArchiveInvalid
				if errors.Is(err, errUnsupportedArchive) {
					issue = IssueArchiveUnsupported
				}
				finding.add(issue, fullPath, err.Error())
			}
			continue
		}

		if err := s.readFully(fullPath); err != nil {
			finding.add(IssueReadFailure, fullPath, err.Error())
		}
	}

	if len(entries) == 0 {
		finding.add(IssueEmptyFolder, path, "folder contains no entries")
	}
	if regularFiles > 0 && regularFiles == archiveFiles {
		finding.add(IssueArchiveOnly, path, "folder contains one or more archive files and no regular non-archive files")
	}

	return finding
}

func (s Scanner) readFully(path string) error {
	file, err := s.openFile(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(io.Discard, file)
	return err
}

func walkErrorFolder(path string, entry os.DirEntry) string {
	if entry != nil && entry.IsDir() {
		return path
	}
	return filepath.Dir(path)
}

func (r *Result) addFileIssue(folder string, path string, issue IssueType, details string) {
	for i := range r.Findings {
		if r.Findings[i].Path == folder {
			r.Findings[i].add(issue, path, details)
			return
		}
	}

	finding := FolderFinding{Path: folder}
	finding.add(issue, path, details)
	r.Findings = append(r.Findings, finding)
}

func (f *FolderFinding) add(issue IssueType, path string, details string) {
	if !containsIssue(f.Issues, issue) {
		f.Issues = append(f.Issues, issue)
	}
	f.Details = append(f.Details, FileFinding{
		Path:    path,
		Issue:   issue,
		Details: details,
	})
}

func containsIssue(issues []IssueType, target IssueType) bool {
	for _, issue := range issues {
		if issue == target {
			return true
		}
	}
	return false
}
