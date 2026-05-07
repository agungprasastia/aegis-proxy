package rtk

import (
	"fmt"
	"strings"
	"testing"
)

func TestUnknownPayloadPassesThrough(t *testing.T) {
	engine := NewEngine(true)
	input := "plain prose that should not be changed"
	out, result, err := engine.CompressToolResult(input)
	if err != nil {
		t.Fatalf("CompressToolResult error: %v", err)
	}
	if out != input || result.Applied || result.ContentType != ContentTypeUnknown {
		t.Fatalf("unknown payload mutated: out=%q result=%+v", out, result)
	}
}

func TestDisabledEnginePassesThrough(t *testing.T) {
	engine := NewEngine(false)
	input := largeGrep(140)
	out, result, err := engine.CompressToolResult(input)
	if err != nil {
		t.Fatalf("CompressToolResult error: %v", err)
	}
	if out != input || result.Applied {
		t.Fatalf("disabled engine mutated payload")
	}
}

func TestGitDiffCompression(t *testing.T) {
	input := "diff --git a/a.go b/a.go\n@@ -1,14 +1,14 @@\n line1\n line2\n line3\n line4\n line5\n line6\n line7\n line8\n line9\n-old\n+new\n line10\n line11\n line12\n"
	assertCompressed(t, input, ContentTypeGitDiff)
}

func TestGrepCompression(t *testing.T) {
	assertCompressed(t, largeGrep(160), ContentTypeGrep)
}

func TestLSCompression(t *testing.T) {
	lines := []string{"total 240"}
	for i := 0; i < 100; i++ {
		lines = append(lines, fmt.Sprintf("-rw-r--r-- 1 user group %d May 07 file-%d.go", i, i))
	}
	assertCompressed(t, strings.Join(lines, "\n"), ContentTypeLS)
}

func TestTreeCompression(t *testing.T) {
	lines := []string{"."}
	for i := 0; i < 180; i++ {
		lines = append(lines, fmt.Sprintf("├── file-%d.go", i))
	}
	assertCompressed(t, strings.Join(lines, "\n"), ContentTypeTree)
}

func TestFindCompression(t *testing.T) {
	lines := []string{}
	for i := 0; i < 230; i++ {
		lines = append(lines, fmt.Sprintf("./src/file-%d.go", i))
	}
	assertCompressed(t, strings.Join(lines, "\n"), ContentTypeFind)
}

func TestLogCompression(t *testing.T) {
	lines := []string{}
	for i := 0; i < 60; i++ {
		lines = append(lines, fmt.Sprintf("commit abcdef%d", i), "Author: Test <t@example.com>", "Date: Thu May 07 2026", "", "    verbose body line")
	}
	assertCompressed(t, strings.Join(lines, "\n"), ContentTypeLog)
}

func assertCompressed(t *testing.T, input string, contentType ContentType) {
	t.Helper()
	engine := NewEngine(true)
	out, result, err := engine.CompressToolResult(input)
	if err != nil {
		t.Fatalf("CompressToolResult error: %v", err)
	}
	if !result.Applied {
		t.Fatalf("compression not applied: %+v", result)
	}
	if result.ContentType != contentType {
		t.Fatalf("content type = %q, want %q", result.ContentType, contentType)
	}
	if len(out) >= len(input) || result.Ratio >= 1 {
		t.Fatalf("output did not shrink: original=%d compressed=%d ratio=%f", len(input), len(out), result.Ratio)
	}
}

func largeGrep(count int) string {
	lines := make([]string, 0, count)
	for i := 0; i < count; i++ {
		lines = append(lines, fmt.Sprintf("src/file.go:%d: matched content %d", i+1, i))
	}
	return strings.Join(lines, "\n")
}
