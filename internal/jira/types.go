package jira

import "time"

// Issue represents a Jira issue.
type Issue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Self   string `json:"self"`
	Fields Fields `json:"fields"`
}

// Fields contains the standard Jira issue fields.
type Fields struct {
	Summary     string      `json:"summary"`
	Description interface{} `json:"description,omitempty"` // Can be string or ADF
	Status      *Status     `json:"status,omitempty"`
	Priority    *Priority   `json:"priority,omitempty"`
	Assignee    *User       `json:"assignee,omitempty"`
	Reporter    *User       `json:"reporter,omitempty"`
	IssueType   *IssueType  `json:"issuetype,omitempty"`
	Project     *Project    `json:"project,omitempty"`
	Labels      []string    `json:"labels,omitempty"`
	Components  []Component `json:"components,omitempty"`
	Versions    []Version   `json:"versions,omitempty"`
	FixVersions []Version   `json:"fixVersions,omitempty"`
	Created     string      `json:"created,omitempty"`
	Updated     string      `json:"updated,omitempty"`
	Comment     *Comments   `json:"comment,omitempty"`
	Parent      *Issue      `json:"parent,omitempty"`
}

type Status struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type Priority struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type User struct {
	DisplayName string `json:"displayName"`
	AccountID   string `json:"accountId"`
	EmailAddress string `json:"emailAddress,omitempty"`
}

type IssueType struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type Project struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	ID   string `json:"id"`
}

type Component struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// Version represents a Jira project version (used for affects versions and fix versions).
type Version struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	ReleaseDate string `json:"releaseDate,omitempty"`
	Released    bool   `json:"released,omitempty"`
}

type Comments struct {
	Total    int       `json:"total"`
	Comments []Comment `json:"comments"`
}

type Comment struct {
	ID      string      `json:"id"`
	Author  *User       `json:"author"`
	Body    interface{} `json:"body"` // Can be string or ADF
	Created string      `json:"created"`
	Updated string      `json:"updated"`
}

type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   Status `json:"to"`
}

type TransitionsResponse struct {
	Transitions []Transition `json:"transitions"`
}

type SearchResult struct {
	Total      int     `json:"total"`
	MaxResults int     `json:"maxResults"`
	StartAt    int     `json:"startAt"`
	Issues     []Issue `json:"issues"`
}

type CreateIssueRequest struct {
	Fields map[string]interface{} `json:"fields"`
}

type CreateIssueResponse struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

type TransitionRequest struct {
	Transition TransitionID               `json:"transition"`
	Update     *TransitionUpdate          `json:"update,omitempty"`
	Fields     map[string]interface{}     `json:"fields,omitempty"`
}

type TransitionID struct {
	ID string `json:"id"`
}

type TransitionUpdate struct {
	Comment []TransitionComment `json:"comment,omitempty"`
}

type TransitionComment struct {
	Add TransitionCommentBody `json:"add"`
}

type TransitionCommentBody struct {
	Body interface{} `json:"body"`
}

type ErrorResponse struct {
	ErrorMessages []string          `json:"errorMessages"`
	Errors        map[string]string `json:"errors"`
}

func (e *ErrorResponse) String() string {
	var msgs []string
	msgs = append(msgs, e.ErrorMessages...)
	for field, msg := range e.Errors {
		msgs = append(msgs, field+": "+msg)
	}
	if len(msgs) == 0 {
		return "unknown error"
	}
	result := msgs[0]
	for _, m := range msgs[1:] {
		result += "\n" + m
	}
	return result
}

type IssueTypeMetadata struct {
	IssueTypes []IssueType `json:"issueTypes"`
}

// Sprint represents a Jira sprint from the Agile API.
type Sprint struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	State         string `json:"state"`       // active, closed, future
	StartDate     string `json:"startDate"`
	EndDate       string `json:"endDate"`
	CompleteDate  string `json:"completeDate,omitempty"`
	Goal          string `json:"goal,omitempty"`
	OriginBoardID int    `json:"originBoardId,omitempty"`
}

// Board represents a Jira Agile board.
type Board struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // scrum, kanban
}

type BoardsResponse struct {
	MaxResults int     `json:"maxResults"`
	StartAt    int     `json:"startAt"`
	Total      int     `json:"total"`
	IsLast     bool    `json:"isLast"`
	Values     []Board `json:"values"`
}

type SprintsResponse struct {
	MaxResults int      `json:"maxResults"`
	StartAt    int      `json:"startAt"`
	IsLast     bool     `json:"isLast"`
	Values     []Sprint `json:"values"`
}

// ParseJiraTime parses a Jira timestamp.
func ParseJiraTime(s string) (time.Time, error) {
	// Jira uses ISO 8601 format
	return time.Parse("2006-01-02T15:04:05.000-0700", s)
}
