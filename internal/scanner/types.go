package scanner

import "time"

type IssueType string

const (
	IssueEmptyFolder        IssueType = "empty-folder"
	IssueArchiveOnly        IssueType = "archive-only"
	IssueZeroSizeFile       IssueType = "zero-size-file"
	IssueReadFailure        IssueType = "read-failure"
	IssueArchiveInvalid     IssueType = "archive-invalid"
	IssueArchiveUnsupported IssueType = "archive-unsupported"
	IssuePermissionDenied   IssueType = "permission-denied"
)

type Result struct {
	ScannedAt time.Time
	Root      string
	Findings  []FolderFinding
}

type FolderFinding struct {
	Path    string
	Issues  []IssueType
	Details []FileFinding
}

type FileFinding struct {
	Path    string
	Issue   IssueType
	Details string
}
