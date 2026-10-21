package jira

import (
	"bytes"
	"regexp"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

type adfNode = map[string]interface{}

var markdownParser = goldmark.New(goldmark.WithExtensions(extension.Strikethrough)).Parser()

// mentionPattern matches Jira's wiki markup mention, [~accountid:ID]. Account
// IDs may contain a colon (557058:<uuid>).
var mentionPattern = regexp.MustCompile(`\[~accountid:([^\]]+)\]`)

// MakeADFBody converts Markdown to an Atlassian Document Format document.
// Jira Cloud requires ADF for descriptions and comments. Text with no
// Markdown syntax becomes plain paragraphs: blank lines split paragraphs and
// single newlines become hard breaks.
func MakeADFBody(markdown string) interface{} {
	source := []byte(markdown)
	root := markdownParser.Parse(text.NewReader(source))

	content := adfBlocks(root, source)
	if len(content) == 0 {
		content = []interface{}{adfNode{"type": "paragraph"}}
	}

	return adfNode{
		"version": 1,
		"type":    "doc",
		"content": content,
	}
}

func adfBlocks(parent ast.Node, source []byte) []interface{} {
	var blocks []interface{}
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		if block := adfBlock(n, source); block != nil {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

func adfBlock(n ast.Node, source []byte) adfNode {
	switch n := n.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		return adfWithContent(adfNode{"type": "paragraph"}, adfInlines(n, source, nil))
	case *ast.Heading:
		return adfWithContent(adfNode{
			"type":  "heading",
			"attrs": adfNode{"level": n.Level},
		}, adfInlines(n, source, nil))
	case *ast.FencedCodeBlock:
		block := adfCodeBlock(n.Lines(), source)
		if language := n.Language(source); len(language) > 0 {
			block["attrs"] = adfNode{"language": string(language)}
		}
		return block
	case *ast.CodeBlock:
		return adfCodeBlock(n.Lines(), source)
	case *ast.HTMLBlock:
		lines := n.Lines()
		var raw []byte
		for i := 0; i < lines.Len(); i++ {
			segment := lines.At(i)
			raw = append(raw, segment.Value(source)...)
		}
		return adfWithContent(adfNode{"type": "paragraph"}, adfTextNodes(string(bytes.TrimRight(raw, "\n")), nil))
	case *ast.Blockquote:
		return adfWithContent(adfNode{"type": "blockquote"}, adfBlocks(n, source))
	case *ast.List:
		list := adfNode{"type": "bulletList"}
		if n.IsOrdered() {
			list["type"] = "orderedList"
			if n.Start != 1 {
				list["attrs"] = adfNode{"order": n.Start}
			}
		}
		return adfWithContent(list, adfListItems(n, source))
	case *ast.ThematicBreak:
		return adfNode{"type": "rule"}
	}
	return nil
}

func adfListItems(list ast.Node, source []byte) []interface{} {
	var items []interface{}
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		items = append(items, adfWithContent(adfNode{"type": "listItem"}, adfBlocks(item, source)))
	}
	return items
}

func adfCodeBlock(lines *text.Segments, source []byte) adfNode {
	var code []byte
	for i := 0; i < lines.Len(); i++ {
		segment := lines.At(i)
		code = append(code, segment.Value(source)...)
	}

	block := adfNode{"type": "codeBlock"}
	if code = bytes.TrimRight(code, "\n"); len(code) > 0 {
		block["content"] = []interface{}{adfNode{"type": "text", "text": string(code)}}
	}
	return block
}

// adfInlines flattens the inline children of n into ADF inline nodes,
// applying marks inherited from enclosing emphasis and links.
func adfInlines(n ast.Node, source []byte, marks []interface{}) []interface{} {
	var nodes []interface{}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c := c.(type) {
		case *ast.Text:
			nodes = append(nodes, adfTextNodes(string(c.Segment.Value(source)), marks)...)
			if c.HardLineBreak() || c.SoftLineBreak() {
				nodes = append(nodes, adfNode{"type": "hardBreak"})
			}
		case *ast.String:
			nodes = append(nodes, adfTextNodes(string(c.Value), marks)...)
		case *ast.CodeSpan:
			nodes = append(nodes, adfTextNodes(string(c.Text(source)), adfMarks(marks, adfNode{"type": "code"}))...)
		case *ast.Emphasis:
			mark := "em"
			if c.Level == 2 {
				mark = "strong"
			}
			nodes = append(nodes, adfInlines(c, source, adfMarks(marks, adfNode{"type": mark}))...)
		case *extast.Strikethrough:
			nodes = append(nodes, adfInlines(c, source, adfMarks(marks, adfNode{"type": "strike"}))...)
		case *ast.Link:
			nodes = append(nodes, adfInlines(c, source, adfMarks(marks, adfLinkMark(c.Destination)))...)
		case *ast.Image:
			// ADF media nodes need an uploaded media id, so images become links.
			nodes = append(nodes, adfInlines(c, source, adfMarks(marks, adfLinkMark(c.Destination)))...)
		case *ast.AutoLink:
			url := c.URL(source)
			nodes = append(nodes, adfTextNodes(string(c.Label(source)), adfMarks(marks, adfLinkMark(url)))...)
		case *ast.RawHTML:
			segments := c.Segments
			for i := 0; i < segments.Len(); i++ {
				segment := segments.At(i)
				nodes = append(nodes, adfTextNodes(string(segment.Value(source)), marks)...)
			}
		}
	}
	return adfMentions(adfMergeText(nodes))
}

// adfMentions splits [~accountid:ID] mentions out of text nodes into ADF
// mention nodes. Mentions take no marks, and code text is left as written.
func adfMentions(nodes []interface{}) []interface{} {
	var result []interface{}
	for _, n := range nodes {
		node := n.(adfNode)
		value, _ := node["text"].(string)
		marks, _ := node["marks"].([]interface{})
		if node["type"] != "text" || hasCodeMark(marks) || !mentionPattern.MatchString(value) {
			result = append(result, node)
			continue
		}

		last := 0
		for _, m := range mentionPattern.FindAllStringSubmatchIndex(value, -1) {
			result = append(result, adfTextNodes(value[last:m[0]], marks)...)
			result = append(result, adfNode{"type": "mention", "attrs": adfNode{"id": value[m[2]:m[3]]}})
			last = m[1]
		}
		result = append(result, adfTextNodes(value[last:], marks)...)
	}
	return result
}

func hasCodeMark(marks []interface{}) bool {
	for _, mark := range marks {
		if mark.(adfNode)["type"] == "code" {
			return true
		}
	}
	return false
}

func adfLinkMark(destination []byte) adfNode {
	return adfNode{"type": "link", "attrs": adfNode{"href": string(destination)}}
}

// adfMarks returns marks plus mark without mutating the shared slice.
func adfMarks(marks []interface{}, mark adfNode) []interface{} {
	return append(append([]interface{}{}, marks...), mark)
}

func adfTextNodes(value string, marks []interface{}) []interface{} {
	if value == "" {
		return nil
	}
	node := adfNode{"type": "text", "text": value}
	if len(marks) > 0 {
		node["marks"] = marks
	}
	return []interface{}{node}
}

func adfWithContent(node adfNode, content []interface{}) adfNode {
	if len(content) > 0 {
		node["content"] = content
	}
	return node
}

// adfMergeText joins adjacent text nodes with identical marks, since the
// parser splits text at punctuation and escapes.
func adfMergeText(nodes []interface{}) []interface{} {
	var merged []interface{}
	for _, n := range nodes {
		node := n.(adfNode)
		if len(merged) > 0 && node["type"] == "text" {
			prev := merged[len(merged)-1].(adfNode)
			if prev["type"] == "text" && marksEqual(prev["marks"], node["marks"]) {
				prev["text"] = prev["text"].(string) + node["text"].(string)
				continue
			}
		}
		merged = append(merged, node)
	}
	return merged
}

func marksEqual(a, b interface{}) bool {
	am, _ := a.([]interface{})
	bm, _ := b.([]interface{})
	if len(am) != len(bm) {
		return false
	}
	for i := range am {
		if !adfMarkEqual(am[i].(adfNode), bm[i].(adfNode)) {
			return false
		}
	}
	return true
}

func adfMarkEqual(a, b adfNode) bool {
	if a["type"] != b["type"] {
		return false
	}
	aa, _ := a["attrs"].(adfNode)
	ba, _ := b["attrs"].(adfNode)
	return aa["href"] == ba["href"]
}
