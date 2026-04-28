package jira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client is an HTTP client for the Jira REST API.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	AuthHeader http.Header
	Debug      bool
}

// NewClient creates a new Jira API client.
func NewClient(instance string, authHeader http.Header) *Client {
	return &Client{
		BaseURL: "https://" + instance + "/rest/api/3",
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		AuthHeader: authHeader,
	}
}

func (c *Client) BrowseURL(instance, key string) string {
	return "https://" + instance + "/browse/" + key
}

func (c *Client) do(method, path string, body interface{}) ([]byte, error) {
	return c.doWithBase(c.BaseURL, method, path, body)
}

func (c *Client) doWithBase(baseURL, method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	var reqData []byte
	if body != nil {
		var err error
		reqData, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request: %w", err)
		}
		bodyReader = bytes.NewReader(reqData)
	}

	reqURL := baseURL + path
	req, err := http.NewRequest(method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	for k, v := range c.AuthHeader {
		req.Header[k] = v
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if c.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] %s %s\n", method, reqURL)
		if reqData != nil {
			fmt.Fprintf(os.Stderr, "[DEBUG] Request body: %s\n", string(reqData))
		}
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if c.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Response status: %d\n", resp.StatusCode)
		if resp.StatusCode >= 400 {
			fmt.Fprintf(os.Stderr, "[DEBUG] Response body: %s\n", string(respBody))
		}
	}

	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.String() != "unknown error" {
			return nil, fmt.Errorf("Jira API error (%d): %s", resp.StatusCode, errResp.String())
		}
		return nil, fmt.Errorf("Jira API error (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// GetIssue fetches a single issue by key.
func (c *Client) GetIssue(key string) (*Issue, error) {
	data, err := c.do("GET", "/issue/"+url.PathEscape(key), nil)
	if err != nil {
		return nil, err
	}

	var issue Issue
	if err := json.Unmarshal(data, &issue); err != nil {
		return nil, fmt.Errorf("parsing issue: %w", err)
	}
	return &issue, nil
}

// GetIssueRaw fetches the raw JSON for an issue.
func (c *Client) GetIssueRaw(key string) (json.RawMessage, error) {
	data, err := c.do("GET", "/issue/"+url.PathEscape(key), nil)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// SearchUsers searches for Jira users by display name or email.
func (c *Client) SearchUsers(query string) ([]User, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("maxResults", "10")
	data, err := c.do("GET", "/user/search?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var users []User
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, fmt.Errorf("parsing user search response: %w", err)
	}
	return users, nil
}

// CreateIssue creates a new issue.
func (c *Client) CreateIssue(fields map[string]interface{}) (*CreateIssueResponse, error) {
	req := CreateIssueRequest{Fields: fields}
	data, err := c.do("POST", "/issue", req)
	if err != nil {
		return nil, err
	}

	var resp CreateIssueResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing create response: %w", err)
	}
	return &resp, nil
}

// UpdateIssue updates fields on an existing issue.
func (c *Client) UpdateIssue(key string, fields map[string]interface{}) error {
	req := CreateIssueRequest{Fields: fields}
	_, err := c.do("PUT", "/issue/"+url.PathEscape(key), req)
	return err
}

// GetTransitions returns available transitions for an issue.
func (c *Client) GetTransitions(key string) ([]Transition, error) {
	data, err := c.do("GET", "/issue/"+url.PathEscape(key)+"/transitions", nil)
	if err != nil {
		return nil, err
	}

	var resp TransitionsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing transitions: %w", err)
	}
	return resp.Transitions, nil
}

// DoTransition performs a transition on an issue, optionally setting fields and adding a comment.
func (c *Client) DoTransition(key string, transitionID string, comment string, fields map[string]interface{}) error {
	req := TransitionRequest{
		Transition: TransitionID{ID: transitionID},
	}

	if comment != "" {
		req.Update = &TransitionUpdate{
			Comment: []TransitionComment{
				{Add: TransitionCommentBody{Body: MakeADFBody(comment)}},
			},
		}
	}

	if len(fields) > 0 {
		req.Fields = fields
	}

	_, err := c.do("POST", "/issue/"+url.PathEscape(key)+"/transitions", req)
	return err
}

// Search performs a JQL search.
func (c *Client) Search(jql string, maxResults int, startAt int) (*SearchResult, error) {
	params := url.Values{}
	params.Set("jql", jql)
	params.Set("maxResults", fmt.Sprintf("%d", maxResults))
	params.Set("startAt", fmt.Sprintf("%d", startAt))
	params.Set("fields", "summary,status,issuetype,priority,assignee,reporter,labels,components,versions,fixVersions,created,updated,comment,parent,project,description")

	data, err := c.do("GET", "/search/jql?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var result SearchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing search results: %w", err)
	}
	return &result, nil
}

// AddComment adds a comment to an issue.
// Jira Cloud requires ADF format for comment bodies via the v3 API.
func (c *Client) AddComment(key string, body string) (*Comment, error) {
	reqBody := map[string]interface{}{
		"body": MakeADFBody(body),
	}

	path := "/issue/" + url.PathEscape(key) + "/comment"
	data, err := c.do("POST", path, reqBody)
	if err != nil {
		return nil, err
	}

	var comment Comment
	if err := json.Unmarshal(data, &comment); err != nil {
		return nil, fmt.Errorf("parsing comment: %w", err)
	}
	return &comment, nil
}

// GetComments fetches comments for an issue.
func (c *Client) GetComments(key string) ([]Comment, error) {
	data, err := c.do("GET", "/issue/"+url.PathEscape(key)+"/comment", nil)
	if err != nil {
		return nil, err
	}

	var resp Comments
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing comments: %w", err)
	}
	return resp.Comments, nil
}

// GetIssueTypes fetches available issue types for a project.
func (c *Client) GetIssueTypes(projectKey string) ([]IssueType, error) {
	data, err := c.do("GET", "/project/"+url.PathEscape(projectKey), nil)
	if err != nil {
		return nil, err
	}

	var project struct {
		IssueTypes []IssueType `json:"issueTypes"`
	}
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, fmt.Errorf("parsing project: %w", err)
	}
	return project.IssueTypes, nil
}

// GetProjectComponents fetches all components for a project.
func (c *Client) GetProjectComponents(projectKey string) ([]Component, error) {
	data, err := c.do("GET", "/project/"+url.PathEscape(projectKey)+"/components", nil)
	if err != nil {
		return nil, err
	}

	var components []Component
	if err := json.Unmarshal(data, &components); err != nil {
		return nil, fmt.Errorf("parsing components: %w", err)
	}
	return components, nil
}

// GetProjectVersions fetches all versions for a project.
func (c *Client) GetProjectVersions(projectKey string) ([]Version, error) {
	data, err := c.do("GET", "/project/"+url.PathEscape(projectKey)+"/versions", nil)
	if err != nil {
		return nil, err
	}

	var versions []Version
	if err := json.Unmarshal(data, &versions); err != nil {
		return nil, fmt.Errorf("parsing versions: %w", err)
	}
	return versions, nil
}

// agileBase returns the Agile REST API base URL.
func (c *Client) agileBase() string {
	return strings.Replace(c.BaseURL, "/rest/api/3", "/rest/agile/1.0", 1)
}

// doAgile sends a request using the Jira Agile REST API.
func (c *Client) doAgile(method, path string, body interface{}) ([]byte, error) {
	return c.doWithBase(c.agileBase(), method, path, body)
}

// GetBoards returns boards visible to the current user.
// projectKeyOrID filters boards by project (optional).
func (c *Client) GetBoards(boardType string, name string, projectKeyOrID string) ([]Board, error) {
	params := url.Values{}
	params.Set("maxResults", "50")
	if boardType != "" {
		params.Set("type", boardType)
	}
	if name != "" {
		params.Set("name", name)
	}
	if projectKeyOrID != "" {
		params.Set("projectKeyOrId", projectKeyOrID)
	}

	data, err := c.doAgile("GET", "/board?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var resp BoardsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing boards: %w", err)
	}
	return resp.Values, nil
}

// GetBoard returns a single board by ID.
func (c *Client) GetBoard(boardID int) (*Board, error) {
	data, err := c.doAgile("GET", fmt.Sprintf("/board/%d", boardID), nil)
	if err != nil {
		return nil, err
	}

	var board Board
	if err := json.Unmarshal(data, &board); err != nil {
		return nil, fmt.Errorf("parsing board: %w", err)
	}
	return &board, nil
}

// GetBoardIssues returns issues on a board, optionally filtered by JQL.
func (c *Client) GetBoardIssues(boardID int, jql string, maxResults int) ([]Issue, int, error) {
	params := url.Values{}
	params.Set("maxResults", fmt.Sprintf("%d", maxResults))
	params.Set("fields", "summary,status,issuetype,priority,assignee,reporter,labels,components,created,updated,parent,project")
	if jql != "" {
		params.Set("jql", jql)
	}

	data, err := c.doAgile("GET", fmt.Sprintf("/board/%d/issue?%s", boardID, params.Encode()), nil)
	if err != nil {
		return nil, 0, err
	}

	var result SearchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, 0, fmt.Errorf("parsing board issues: %w", err)
	}
	return result.Issues, result.Total, nil
}

// GetSprints returns sprints for a board, optionally filtered by state.
func (c *Client) GetSprints(boardID int, state string) ([]Sprint, error) {
	params := url.Values{}
	params.Set("maxResults", "50")
	if state != "" {
		params.Set("state", state)
	}

	data, err := c.doAgile("GET", fmt.Sprintf("/board/%d/sprint?%s", boardID, params.Encode()), nil)
	if err != nil {
		return nil, err
	}

	var resp SprintsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing sprints: %w", err)
	}
	return resp.Values, nil
}

// GetSprint returns a single sprint by ID.
func (c *Client) GetSprint(sprintID int) (*Sprint, error) {
	data, err := c.doAgile("GET", fmt.Sprintf("/sprint/%d", sprintID), nil)
	if err != nil {
		return nil, err
	}

	var sprint Sprint
	if err := json.Unmarshal(data, &sprint); err != nil {
		return nil, fmt.Errorf("parsing sprint: %w", err)
	}
	return &sprint, nil
}

// GetSprintIssues returns issues in a sprint.
func (c *Client) GetSprintIssues(sprintID int, maxResults int) ([]Issue, error) {
	params := url.Values{}
	params.Set("maxResults", fmt.Sprintf("%d", maxResults))

	data, err := c.doAgile("GET", fmt.Sprintf("/sprint/%d/issue?%s", sprintID, params.Encode()), nil)
	if err != nil {
		return nil, err
	}

	var result SearchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing sprint issues: %w", err)
	}
	return result.Issues, nil
}

// MoveToSprint moves issues to a sprint.
func (c *Client) MoveToSprint(sprintID int, issueKeys []string) error {
	body := map[string]interface{}{
		"issues": issueKeys,
	}
	_, err := c.doAgile("POST", fmt.Sprintf("/sprint/%d/issue", sprintID), body)
	return err
}

// RemoveFromSprint moves issues to the backlog (removing from any sprint).
func (c *Client) RemoveFromSprint(issueKeys []string) error {
	body := map[string]interface{}{
		"issues": issueKeys,
	}
	_, err := c.doAgile("POST", "/backlog/issue", body)
	return err
}

// MakeADFBody creates a simple ADF document from plain text.
// Jira Cloud requires Atlassian Document Format for descriptions and comments.
func MakeADFBody(text string) interface{} {
	return map[string]interface{}{
		"version": 1,
		"type":    "doc",
		"content": []interface{}{
			map[string]interface{}{
				"type": "paragraph",
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": text,
					},
				},
			},
		},
	}
}
