package jira

import (
	"encoding/json"
	"testing"
)

func TestMakeADFBody_Markdown(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain text regression",
			in:   "Hello world",
			want: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Hello world"}]}]}`,
		},
		{
			name: "plain text with punctuation stays one text node",
			in:   "Fix: the bug (again), it's #123 & done.",
			want: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Fix: the bug (again), it's #123 & done."}]}]}`,
		},
		{
			name: "blank line splits paragraphs and newline is a hard break",
			in:   "one\ntwo\n\nthree",
			want: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"one"},{"type":"hardBreak"},{"type":"text","text":"two"}]},{"type":"paragraph","content":[{"type":"text","text":"three"}]}]}`,
		},
		{
			name: "empty input",
			in:   "",
			want: `{"type":"doc","version":1,"content":[{"type":"paragraph"}]}`,
		},
		{
			name: "heading",
			in:   "## Title",
			want: `{"type":"doc","version":1,"content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Title"}]}]}`,
		},
		{
			name: "bold and italic",
			in:   "a **b** *c* d",
			want: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"a "},{"type":"text","text":"b","marks":[{"type":"strong"}]},{"type":"text","text":" "},{"type":"text","text":"c","marks":[{"type":"em"}]},{"type":"text","text":" d"}]}]}`,
		},
		{
			name: "nested bold italic",
			in:   "**a *b***",
			want: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"a ","marks":[{"type":"strong"}]},{"type":"text","text":"b","marks":[{"type":"strong"},{"type":"em"}]}]}]}`,
		},
		{
			name: "strikethrough",
			in:   "~~gone~~",
			want: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"gone","marks":[{"type":"strike"}]}]}]}`,
		},
		{
			name: "inline code",
			in:   "run `go test` now",
			want: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"run "},{"type":"text","text":"go test","marks":[{"type":"code"}]},{"type":"text","text":" now"}]}]}`,
		},
		{
			name: "fenced code block with language",
			in:   "```go\nfmt.Println(1)\nx := 2\n```",
			want: `{"type":"doc","version":1,"content":[{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"fmt.Println(1)\nx := 2"}]}]}`,
		},
		{
			name: "fenced code block without language is not parsed as markdown",
			in:   "```\n**not bold**\n```",
			want: `{"type":"doc","version":1,"content":[{"type":"codeBlock","content":[{"type":"text","text":"**not bold**"}]}]}`,
		},
		{
			name: "bullet list",
			in:   "- a\n- b",
			want: `{"type":"doc","version":1,"content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]}]},{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"b"}]}]}]}]}`,
		},
		{
			name: "ordered list keeps start",
			in:   "3. a\n4. b",
			want: `{"type":"doc","version":1,"content":[{"type":"orderedList","attrs":{"order":3},"content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]}]},{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"b"}]}]}]}]}`,
		},
		{
			name: "nested list",
			in:   "- a\n  - b",
			want: `{"type":"doc","version":1,"content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]},{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"b"}]}]}]}]}]}]}`,
		},
		{
			name: "link",
			in:   "see [docs](https://example.com/a?b=1)",
			want: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"see "},{"type":"text","text":"docs","marks":[{"type":"link","attrs":{"href":"https://example.com/a?b=1"}}]}]}]}`,
		},
		{
			name: "autolink",
			in:   "<https://example.com>",
			want: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"https://example.com","marks":[{"type":"link","attrs":{"href":"https://example.com"}}]}]}]}`,
		},
		{
			name: "image becomes link",
			in:   "![mockup](https://example.com/a.png)",
			want: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"mockup","marks":[{"type":"link","attrs":{"href":"https://example.com/a.png"}}]}]}]}`,
		},
		{
			name: "blockquote",
			in:   "> quoted",
			want: `{"type":"doc","version":1,"content":[{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"quoted"}]}]}]}`,
		},
		{
			name: "thematic break",
			in:   "a\n\n---\n\nb",
			want: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"a"}]},{"type":"rule"},{"type":"paragraph","content":[{"type":"text","text":"b"}]}]}`,
		},
		{
			name: "raw html stays literal text",
			in:   "a <b>x</b>",
			want: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"a <b>x</b>"}]}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(MakeADFBody(tt.in))
			if err != nil {
				t.Fatal(err)
			}

			var gotV, wantV interface{}
			if err := json.Unmarshal(got, &gotV); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tt.want), &wantV); err != nil {
				t.Fatal(err)
			}
			g, _ := json.Marshal(gotV)
			w, _ := json.Marshal(wantV)
			if string(g) != string(w) {
				t.Errorf("MakeADFBody(%q)\n got: %s\nwant: %s", tt.in, g, w)
			}
		})
	}
}
