package jira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
)

// AddAttachments uploads files to an issue in a single multipart request.
// Every file is read before anything is sent, so a bad path uploads nothing.
func (c *Client) AddAttachments(key string, paths []string) ([]Attachment, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading attachment: %w", err)
		}
		part, err := writer.CreateFormFile("file", filepath.Base(path))
		if err != nil {
			return nil, fmt.Errorf("building attachment request: %w", err)
		}
		if _, err := part.Write(data); err != nil {
			return nil, fmt.Errorf("building attachment request: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("building attachment request: %w", err)
	}

	respBody, err := c.send("POST", c.BaseURL+"/issue/"+url.PathEscape(key)+"/attachments",
		writer.FormDataContentType(), &body, nil, map[string]string{"X-Atlassian-Token": "no-check"})
	if err != nil {
		return nil, err
	}

	var attachments []Attachment
	if err := json.Unmarshal(respBody, &attachments); err != nil {
		return nil, fmt.Errorf("parsing attachments: %w", err)
	}
	return attachments, nil
}
