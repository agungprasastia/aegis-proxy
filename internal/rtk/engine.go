package rtk

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ErrUnsafeCompression = errors.New("compression would lose semantic information")

type ContentType string

const (
	ContentTypeGitDiff ContentType = "git_diff"
	ContentTypeGrep    ContentType = "grep"
	ContentTypeLS      ContentType = "ls"
	ContentTypeTree    ContentType = "tree"
	ContentTypeFind    ContentType = "find"
	ContentTypeLog     ContentType = "log"
	ContentTypeUnknown ContentType = "unknown"
)

type Engine struct {
	Enabled bool
	Filters map[ContentType]Filter
}

type Filter interface {
	Compress(input string) (string, error)
	Detect(input string) bool
}

type CompressionResult struct {
	Original    int         `json:"original"`
	Compressed  int         `json:"compressed"`
	Ratio       float64     `json:"ratio"`
	Applied     bool        `json:"applied"`
	ContentType ContentType `json:"content_type"`
}

func NewEngine(enabled bool) *Engine {
	return &Engine{
		Enabled: enabled,
		Filters: map[ContentType]Filter{
			ContentTypeGitDiff: &GitDiffFilter{},
			ContentTypeGrep:    &GrepFilter{},
			ContentTypeLS:      &LSFilter{},
			ContentTypeTree:    &TreeFilter{},
			ContentTypeFind:    &FindFilter{},
			ContentTypeLog:     &LogFilter{},
		},
	}
}

func (e *Engine) CompressToolResult(content string) (string, *CompressionResult, error) {
	result := &CompressionResult{Original: len(content), Compressed: len(content), Ratio: 1, ContentType: ContentTypeUnknown}
	if e == nil || !e.Enabled || strings.TrimSpace(content) == "" {
		return content, result, nil
	}

	contentType := e.DetectContentType(content)
	result.ContentType = contentType
	if contentType == ContentTypeUnknown {
		return content, result, nil
	}

	filter, ok := e.Filters[contentType]
	if !ok {
		return content, result, nil
	}

	compressed, err := filter.Compress(content)
	if err != nil {
		if errors.Is(err, ErrUnsafeCompression) {
			return content, result, nil
		}
		return content, result, err
	}
	if len(compressed) >= len(content) {
		return content, result, nil
	}

	result.Compressed = len(compressed)
	result.Ratio = float64(len(compressed)) / float64(len(content))
	result.Applied = true
	return compressed, result, nil
}

func (e *Engine) DetectContentType(content string) ContentType {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ContentTypeUnknown
	}
	for contentType, filter := range e.Filters {
		if filter.Detect(trimmed) {
			return contentType
		}
	}
	return ContentTypeUnknown
}

type GitDiffFilter struct{}

func (f *GitDiffFilter) Detect(input string) bool {
	return strings.HasPrefix(input, "diff --git") || strings.HasPrefix(input, "--- ") || strings.Contains(input, "\n@@ ")
}

func (f *GitDiffFilter) Compress(input string) (string, error) {
	lines := strings.Split(input, "\n")
	out := make([]string, 0, len(lines))
	contextRun := 0
	for _, line := range lines {
		if strings.HasPrefix(line, " ") {
			contextRun++
			if contextRun <= 2 {
				out = append(out, line)
			} else if contextRun == 3 {
				out = append(out, " ... context omitted")
			}
			continue
		}
		contextRun = 0
		out = append(out, line)
	}
	return strings.Join(out, "\n"), nil
}

type GrepFilter struct{}

var grepLine = regexp.MustCompile(`^[^:\n]+:\d+:`)

func (f *GrepFilter) Detect(input string) bool {
	for _, line := range strings.Split(input, "\n") {
		if grepLine.MatchString(line) {
			return true
		}
	}
	return false
}

func (f *GrepFilter) Compress(input string) (string, error) {
	lines := strings.Split(input, "\n")
	if len(lines) <= 120 {
		return input, nil
	}
	return strings.Join(lines[:120], "\n") + fmt.Sprintf("\n... %d more grep matches", len(lines)-120), nil
}

type LSFilter struct{}

func (f *LSFilter) Detect(input string) bool {
	first := strings.Split(strings.TrimSpace(input), "\n")[0]
	return strings.HasPrefix(first, "total ") || strings.Contains(input, "drwx") || strings.Contains(input, "-rw-")
}

func (f *LSFilter) Compress(input string) (string, error) {
	lines := strings.Split(input, "\n")
	if len(lines) <= 80 {
		return input, nil
	}
	return strings.Join(lines[:80], "\n") + fmt.Sprintf("\n... %d more entries", len(lines)-80), nil
}

type TreeFilter struct{}

func (f *TreeFilter) Detect(input string) bool {
	return strings.Contains(input, "├──") || strings.Contains(input, "└──") || strings.Contains(input, "|--") || strings.Contains(input, "`--")
}

func (f *TreeFilter) Compress(input string) (string, error) {
	lines := strings.Split(input, "\n")
	if len(lines) <= 140 {
		return input, nil
	}
	return strings.Join(lines[:140], "\n") + fmt.Sprintf("\n... %d more tree items", len(lines)-140), nil
}

type FindFilter struct{}

func (f *FindFilter) Detect(input string) bool {
	lines := strings.Split(strings.TrimSpace(input), "\n")
	if len(lines) < 3 {
		return false
	}
	matches := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "./") || strings.HasPrefix(trimmed, "/") {
			matches++
		}
	}
	return matches >= 3
}

func (f *FindFilter) Compress(input string) (string, error) {
	lines := strings.Split(input, "\n")
	if len(lines) <= 200 {
		return input, nil
	}
	return strings.Join(lines[:200], "\n") + fmt.Sprintf("\n... %d more paths", len(lines)-200), nil
}

type LogFilter struct{}

var shortLogLine = regexp.MustCompile(`^[a-f0-9]{7,40} `)

func (f *LogFilter) Detect(input string) bool {
	trimmed := strings.TrimSpace(input)
	return strings.HasPrefix(trimmed, "commit ") || shortLogLine.MatchString(trimmed)
}

func (f *LogFilter) Compress(input string) (string, error) {
	lines := strings.Split(input, "\n")
	out := make([]string, 0, len(lines))
	keptCommits := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "commit ") || shortLogLine.MatchString(trimmed) {
			keptCommits++
			if keptCommits > 40 {
				break
			}
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(trimmed, "Author:") || strings.HasPrefix(trimmed, "Date:") || (keptCommits > 0 && trimmed != "" && !strings.HasPrefix(trimmed, "Merge:")) {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return input, nil
	}
	return strings.Join(out, "\n"), nil
}

func (e *Engine) ToJSON() map[string]any {
	return map[string]any{"enabled": e.Enabled, "filters": []string{"git_diff", "grep", "ls", "tree", "find", "log"}}
}
