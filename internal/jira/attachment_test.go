package jira

import (
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestAddAttachments(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "mockup.png")
	second := filepath.Join(dir, "trace.log")
	os.WriteFile(first, []byte("png-bytes"), 0o644)
	os.WriteFile(second, []byte("log-bytes"), 0o644)

	var gotFiles = map[string]string{}
	client, server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/issue/LPD-1/attachments" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Atlassian-Token"); got != "no-check" {
			t.Errorf("X-Atlassian-Token = %q, want no-check", got)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("missing authorization header")
		}

		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/form-data" {
			t.Fatalf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			data, _ := io.ReadAll(part)
			if part.FormName() != "file" {
				t.Errorf("field name = %q, want file", part.FormName())
			}
			gotFiles[part.FileName()] = string(data)
		}

		json.NewEncoder(w).Encode([]Attachment{
			{ID: "10", Filename: "mockup.png", Size: 9},
			{ID: "11", Filename: "trace.log", Size: 9},
		})
	})
	defer server.Close()

	got, err := client.AddAttachments("LPD-1", []string{first, second})
	if err != nil {
		t.Fatalf("AddAttachments() error = %v", err)
	}

	if len(got) != 2 || got[0].ID != "10" || got[1].Filename != "trace.log" {
		t.Errorf("attachments = %+v", got)
	}
	if gotFiles["mockup.png"] != "png-bytes" || gotFiles["trace.log"] != "log-bytes" {
		t.Errorf("uploaded files = %v", gotFiles)
	}
}

func TestAddAttachments_Errors(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		status  int
		wantHit bool
	}{
		{name: "missing file sends nothing", path: "does-not-exist.png", status: 200, wantHit: false},
		{name: "api error is returned", path: "", status: 403, wantHit: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.path
			if path == "" {
				path = filepath.Join(t.TempDir(), "a.txt")
				os.WriteFile(path, []byte("x"), 0o644)
			}

			hit := false
			client, server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
				hit = true
				w.WriteHeader(tt.status)
				json.NewEncoder(w).Encode(ErrorResponse{ErrorMessages: []string{"nope"}})
			})
			defer server.Close()

			if _, err := client.AddAttachments("LPD-1", []string{path}); err == nil {
				t.Fatal("expected error")
			}
			if hit != tt.wantHit {
				t.Errorf("server hit = %v, want %v", hit, tt.wantHit)
			}
		})
	}
}
