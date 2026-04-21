package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
)

func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestNavigateJSON(t *testing.T) {
	data := map[string]interface{}{
		"fields": map[string]interface{}{
			"summary": "Test issue",
			"status": map[string]interface{}{
				"name": "Open",
			},
			"labels": []interface{}{"bug", "critical"},
			"issuelinks": []interface{}{
				map[string]interface{}{
					"outwardIssue": map[string]interface{}{"key": "LPS-1"},
				},
				map[string]interface{}{
					"outwardIssue": map[string]interface{}{"key": "LPS-2"},
				},
			},
		},
		"key": "LPS-123",
	}

	tests := []struct {
		name string
		path string
		want interface{}
	}{
		{name: "top-level field", path: ".key", want: "LPS-123"},
		{name: "nested field", path: ".fields.summary", want: "Test issue"},
		{name: "deeply nested field", path: ".fields.status.name", want: "Open"},
		{name: "without leading dot", path: "fields.summary", want: "Test issue"},
		{name: "nonexistent field", path: ".fields.nonexistent", want: nil},
		{name: "empty path returns whole object", path: "", want: data},
		{name: "array iterate returns all elements", path: ".fields.labels[]", want: []interface{}{"bug", "critical"}},
		{name: "array index first element", path: ".fields.labels[0]", want: "bug"},
		{name: "array index second element", path: ".fields.labels[1]", want: "critical"},
		{name: "array index out of range returns nil", path: ".fields.labels[5]", want: nil},
		{name: "iterate and pluck nested key", path: ".fields.issuelinks[].outwardIssue.key", want: []interface{}{"LPS-1", "LPS-2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := navigateJSON(data, tt.path)
			if tt.want == nil {
				if got != nil {
					t.Errorf("navigateJSON(%q) = %v, want nil", tt.path, got)
				}
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("navigateJSON(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestPrintField(t *testing.T) {
	data := map[string]interface{}{
		"fields": map[string]interface{}{
			"summary": "Test issue",
			"status":  map[string]interface{}{"name": "Open"},
			"labels":  []interface{}{"bug", "critical"},
		},
		"key": "LPS-123",
	}

	t.Run("single field preserves existing output format", func(t *testing.T) {
		got := captureStdout(func() {
			printField(navigateJSON(data, ".fields.summary"), true)
		})
		if got != "Test issue\n" {
			t.Errorf("got %q, want %q", got, "Test issue\n")
		}
	})

	t.Run("multi-field prints labeled lines", func(t *testing.T) {
		fields := []string{".fields.summary", ".fields.status.name"}
		got := captureStdout(func() {
			for _, f := range fields {
				fmt.Printf("%s: ", f)
				printField(navigateJSON(data, f), false)
			}
		})
		want := ".fields.summary: Test issue\n.fields.status.name: Open\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("array field prints compact JSON", func(t *testing.T) {
		got := captureStdout(func() {
			fmt.Printf(".fields.labels: ")
			printField(navigateJSON(data, ".fields.labels"), false)
		})
		want := ".fields.labels: [\"bug\",\"critical\"]\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestPrintADFResult(t *testing.T) {
	adfParagraph := func(text string) map[string]interface{} {
		return map[string]interface{}{
			"type": "doc",
			"content": []interface{}{
				map[string]interface{}{
					"type": "paragraph",
					"content": []interface{}{
						map[string]interface{}{"type": "text", "text": text},
					},
				},
			},
		}
	}

	t.Run("plain string passes through unchanged", func(t *testing.T) {
		got := captureStdout(func() { printADFResult("hello") })
		if got != "hello\n" {
			t.Errorf("got %q, want %q", got, "hello\n")
		}
	})

	t.Run("ADF document renders to plain text", func(t *testing.T) {
		got := captureStdout(func() { printADFResult(adfParagraph("Fix the bug")) })
		if got != "Fix the bug\n" {
			t.Errorf("got %q, want %q", got, "Fix the bug\n")
		}
	})

	t.Run("array of ADF documents prints blocks separated by blank lines", func(t *testing.T) {
		arr := []interface{}{adfParagraph("first"), adfParagraph("second")}
		got := captureStdout(func() { printADFResult(arr) })
		want := "first\n\nsecond\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"fields.summary", []string{"fields", "summary"}},
		{"key", []string{"key"}},
		{"a.b.c.d", []string{"a", "b", "c", "d"}},
		{"", nil},
		{"fields.labels[]", []string{"fields", "labels", "[]"}},
		{"fields.labels[0]", []string{"fields", "labels", "[0]"}},
		{"fields.issuelinks[].outwardIssue.key", []string{"fields", "issuelinks", "[]", "outwardIssue", "key"}},
	}

	for _, tt := range tests {
		got := splitPath(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitPath(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitPath(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}
