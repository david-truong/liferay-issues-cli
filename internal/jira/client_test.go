package jira

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	client := &Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		AuthHeader: http.Header{"Authorization": []string{"Basic dGVzdDp0ZXN0"}},
	}
	return client, server
}

func TestGetIssue(t *testing.T) {
	issue := Issue{
		Key: "LPS-12345",
		Fields: Fields{
			Summary: "Test issue",
			Status:  &Status{Name: "Open", ID: "1"},
		},
	}

	client, server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issue/LPS-12345" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("missing authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(issue)
	})
	defer server.Close()

	got, err := client.GetIssue("LPS-12345")
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}
	if got.Key != "LPS-12345" {
		t.Errorf("Key = %q, want LPS-12345", got.Key)
	}
	if got.Fields.Summary != "Test issue" {
		t.Errorf("Summary = %q, want 'Test issue'", got.Fields.Summary)
	}
}

func TestGetIssue_NotFound(t *testing.T) {
	client, server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(ErrorResponse{
			ErrorMessages: []string{"Issue does not exist or you do not have permission to see it."},
		})
	})
	defer server.Close()

	_, err := client.GetIssue("NOPE-999")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestCreateIssue(t *testing.T) {
	client, server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/issue" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req CreateIssueRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decoding request: %v", err)
		}
		if req.Fields["summary"] != "New bug" {
			t.Errorf("summary = %v, want 'New bug'", req.Fields["summary"])
		}

		w.WriteHeader(201)
		json.NewEncoder(w).Encode(CreateIssueResponse{
			ID:  "10001",
			Key: "LPS-99999",
		})
	})
	defer server.Close()

	fields := map[string]interface{}{
		"project":   map[string]string{"key": "LPS"},
		"issuetype": map[string]string{"name": "Bug"},
		"summary":   "New bug",
	}

	resp, err := client.CreateIssue(fields)
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}
	if resp.Key != "LPS-99999" {
		t.Errorf("Key = %q, want LPS-99999", resp.Key)
	}
}

func TestUpdateIssue(t *testing.T) {
	client, server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/issue/LPS-12345" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(204)
	})
	defer server.Close()

	err := client.UpdateIssue("LPS-12345", map[string]interface{}{
		"summary": "Updated summary",
	})
	if err != nil {
		t.Fatalf("UpdateIssue() error = %v", err)
	}
}

func TestGetTransitions(t *testing.T) {
	resp := TransitionsResponse{
		Transitions: []Transition{
			{ID: "11", Name: "Start Progress", To: Status{Name: "In Progress"}},
			{ID: "21", Name: "Resolve", To: Status{Name: "Resolved"}},
		},
	}

	client, server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issue/LPS-12345/transitions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	transitions, err := client.GetTransitions("LPS-12345")
	if err != nil {
		t.Fatalf("GetTransitions() error = %v", err)
	}
	if len(transitions) != 2 {
		t.Fatalf("got %d transitions, want 2", len(transitions))
	}
	if transitions[0].Name != "Start Progress" {
		t.Errorf("first transition name = %q, want 'Start Progress'", transitions[0].Name)
	}
}

func TestDoTransition(t *testing.T) {
	client, server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var req TransitionRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Transition.ID != "11" {
			t.Errorf("transition ID = %q, want '11'", req.Transition.ID)
		}
		w.WriteHeader(204)
	})
	defer server.Close()

	err := client.DoTransition("LPS-12345", "11", "", nil)
	if err != nil {
		t.Fatalf("DoTransition() error = %v", err)
	}
}

func TestDoTransition_WithComment(t *testing.T) {
	client, server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req TransitionRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Update == nil {
			t.Fatal("expected update with comment")
		}
		if len(req.Update.Comment) != 1 {
			t.Fatalf("expected 1 comment, got %d", len(req.Update.Comment))
		}
		w.WriteHeader(204)
	})
	defer server.Close()

	err := client.DoTransition("LPS-12345", "11", "Starting work", nil)
	if err != nil {
		t.Fatalf("DoTransition() error = %v", err)
	}
}

func TestSearch(t *testing.T) {
	searchResp := SearchResult{
		Total:      2,
		MaxResults: 20,
		Issues: []Issue{
			{Key: "LPS-1", Fields: Fields{Summary: "First"}},
			{Key: "LPS-2", Fields: Fields{Summary: "Second"}},
		},
	}

	client, server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/jql" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		jql := r.URL.Query().Get("jql")
		if jql != "project = LPS" {
			t.Errorf("jql = %q, want 'project = LPS'", jql)
		}
		json.NewEncoder(w).Encode(searchResp)
	})
	defer server.Close()

	result, err := client.Search("project = LPS", 20, 0)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(result.Issues) != 2 {
		t.Fatalf("got %d issues, want 2", len(result.Issues))
	}
}

func TestAddComment(t *testing.T) {
	client, server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/issue/LPS-12345/comment" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.WriteHeader(201)
		json.NewEncoder(w).Encode(Comment{
			ID:   "100",
			Body: "Test comment",
		})
	})
	defer server.Close()

	comment, err := client.AddComment("LPS-12345", "Test comment")
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if comment.ID != "100" {
		t.Errorf("comment ID = %q, want '100'", comment.ID)
	}
}

func TestGetComments(t *testing.T) {
	resp := Comments{
		Total: 1,
		Comments: []Comment{
			{ID: "1", Body: "A comment", Author: &User{DisplayName: "Test User"}},
		},
	}

	client, server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issue/LPS-12345/comment" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	comments, err := client.GetComments("LPS-12345")
	if err != nil {
		t.Fatalf("GetComments() error = %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(comments))
	}
	if comments[0].Author.DisplayName != "Test User" {
		t.Errorf("author = %q, want 'Test User'", comments[0].Author.DisplayName)
	}
}

func TestGetIssueTypes(t *testing.T) {
	client, server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/project/LPS" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"issueTypes": []IssueType{
				{Name: "Bug", ID: "1"},
				{Name: "Task", ID: "2"},
				{Name: "Story", ID: "3"},
			},
		})
	})
	defer server.Close()

	types, err := client.GetIssueTypes("LPS")
	if err != nil {
		t.Fatalf("GetIssueTypes() error = %v", err)
	}
	if len(types) != 3 {
		t.Fatalf("got %d types, want 3", len(types))
	}
}

func TestClient_ErrorHandling(t *testing.T) {
	client, server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(ErrorResponse{
			ErrorMessages: []string{"Field 'summary' is required"},
		})
	})
	defer server.Close()

	_, err := client.CreateIssue(map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if got := err.Error(); got == "" {
		t.Error("error message should not be empty")
	}
}

func TestClient_ServerError(t *testing.T) {
	client, server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("Internal Server Error"))
	})
	defer server.Close()

	_, err := client.GetIssue("LPS-1")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestGetIssueLinkTypes(t *testing.T) {
	client, server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/issueLinkType" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(IssueLinkTypesResponse{
			IssueLinkTypes: []IssueLinkType{
				{ID: "10000", Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
			},
		})
	})
	defer server.Close()

	types, err := client.GetIssueLinkTypes()
	if err != nil {
		t.Fatalf("GetIssueLinkTypes() error = %v", err)
	}
	if len(types) != 1 || types[0].Name != "Blocks" || types[0].Inward != "is blocked by" {
		t.Errorf("GetIssueLinkTypes() = %+v", types)
	}
}

func TestLinkIssues(t *testing.T) {
	client, server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/issueLink" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var body struct {
			Type         struct{ Name string } `json:"type"`
			InwardIssue  struct{ Key string }  `json:"inwardIssue"`
			OutwardIssue struct{ Key string }  `json:"outwardIssue"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request: %v", err)
		}
		if body.Type.Name != "Blocks" {
			t.Errorf("type = %q, want Blocks", body.Type.Name)
		}
		if body.InwardIssue.Key != "LPD-1" {
			t.Errorf("inwardIssue = %q, want LPD-1", body.InwardIssue.Key)
		}
		if body.OutwardIssue.Key != "LPD-2" {
			t.Errorf("outwardIssue = %q, want LPD-2", body.OutwardIssue.Key)
		}

		w.WriteHeader(201)
	})
	defer server.Close()

	if err := client.LinkIssues("Blocks", "LPD-1", "LPD-2"); err != nil {
		t.Fatalf("LinkIssues() error = %v", err)
	}
}
