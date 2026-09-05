package report

import (
	"fmt"
	"io"
	"strings"

	"bad-files-checker/internal/scanner"
)

func WriteText(writer io.Writer, result scanner.Result) error {
	if _, err := fmt.Fprintf(writer, "Bad Files Checker Scan\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Timestamp: %s\n", result.ScannedAt.Format("2006-01-02T15:04:05Z07:00")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Root: %s\n", result.Root); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Bad folders: %d\n\n", len(result.Findings)); err != nil {
		return err
	}

	if len(result.Findings) == 0 {
		_, err := fmt.Fprintln(writer, "No bad folders found.")
		return err
	}

	for _, finding := range result.Findings {
		if _, err := fmt.Fprintf(writer, "Folder: %s\n", finding.Path); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(writer, "Issues: %s\n", joinIssues(finding.Issues)); err != nil {
			return err
		}
		for _, detail := range finding.Details {
			if _, err := fmt.Fprintf(writer, "- [%s] %s", detail.Issue, detail.Path); err != nil {
				return err
			}
			if detail.Details != "" {
				if _, err := fmt.Fprintf(writer, " - %s", detail.Details); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(writer); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(writer); err != nil {
			return err
		}
	}
	return nil
}

func joinIssues(issues []scanner.IssueType) string {
	values := make([]string, 0, len(issues))
	for _, issue := range issues {
		values = append(values, string(issue))
	}
	return strings.Join(values, ", ")
}
