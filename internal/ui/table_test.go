package ui

import "testing"

func TestExtractText_String(t *testing.T) {
	got := ExtractText("plain text")
	if got != "plain text" {
		t.Errorf("ExtractText(string) = %q, want 'plain text'", got)
	}
}

func TestExtractText_Nil(t *testing.T) {
	got := ExtractText(nil)
	if got != "" {
		t.Errorf("ExtractText(nil) = %q, want ''", got)
	}
}

func TestExtractText_ADF_Paragraph(t *testing.T) {
	adf := map[string]interface{}{
		"version": 1,
		"type":    "doc",
		"content": []interface{}{
			map[string]interface{}{
				"type": "paragraph",
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "Hello world",
					},
				},
			},
		},
	}

	got := ExtractText(adf)
	if got != "Hello world" {
		t.Errorf("ExtractText(ADF paragraph) = %q, want 'Hello world'", got)
	}
}

func TestExtractText_ADF_MultipleParagraphs(t *testing.T) {
	adf := map[string]interface{}{
		"version": 1,
		"type":    "doc",
		"content": []interface{}{
			map[string]interface{}{
				"type": "paragraph",
				"content": []interface{}{
					map[string]interface{}{"type": "text", "text": "First paragraph"},
				},
			},
			map[string]interface{}{
				"type": "paragraph",
				"content": []interface{}{
					map[string]interface{}{"type": "text", "text": "Second paragraph"},
				},
			},
		},
	}

	got := ExtractText(adf)
	want := "First paragraph\nSecond paragraph"
	if got != want {
		t.Errorf("ExtractText(ADF multi-paragraph) = %q, want %q", got, want)
	}
}

func TestExtractText_ADF_BulletList(t *testing.T) {
	adf := map[string]interface{}{
		"version": 1,
		"type":    "doc",
		"content": []interface{}{
			map[string]interface{}{
				"type": "bulletList",
				"content": []interface{}{
					map[string]interface{}{
						"type": "listItem",
						"content": []interface{}{
							map[string]interface{}{
								"type": "paragraph",
								"content": []interface{}{
									map[string]interface{}{"type": "text", "text": "Item one"},
								},
							},
						},
					},
					map[string]interface{}{
						"type": "listItem",
						"content": []interface{}{
							map[string]interface{}{
								"type": "paragraph",
								"content": []interface{}{
									map[string]interface{}{"type": "text", "text": "Item two"},
								},
							},
						},
					},
				},
			},
		},
	}

	got := ExtractText(adf)
	if got == "" {
		t.Error("ExtractText(ADF bulletList) should not be empty")
	}
}

func TestExtractText_ADF_CodeBlock(t *testing.T) {
	adf := map[string]interface{}{
		"version": 1,
		"type":    "doc",
		"content": []interface{}{
			map[string]interface{}{
				"type": "codeBlock",
				"content": []interface{}{
					map[string]interface{}{"type": "text", "text": "fmt.Println(\"hi\")"},
				},
			},
		},
	}

	got := ExtractText(adf)
	want := "```\nfmt.Println(\"hi\")\n```"
	if got != want {
		t.Errorf("ExtractText(ADF codeBlock) = %q, want %q", got, want)
	}
}

func TestExtractText_ADF_InlineCard(t *testing.T) {
	adf := map[string]interface{}{
		"version": 1,
		"type":    "doc",
		"content": []interface{}{
			map[string]interface{}{
				"type": "paragraph",
				"content": []interface{}{
					map[string]interface{}{
						"type": "inlineCard",
						"attrs": map[string]interface{}{
							"url": "https://example.com",
						},
					},
				},
			},
		},
	}

	got := ExtractText(adf)
	if got != "https://example.com" {
		t.Errorf("ExtractText(ADF inlineCard) = %q, want 'https://example.com'", got)
	}
}

func TestExtractText_ADF_Mention(t *testing.T) {
	adf := map[string]interface{}{
		"version": 1,
		"type":    "doc",
		"content": []interface{}{
			map[string]interface{}{
				"type": "paragraph",
				"content": []interface{}{
					map[string]interface{}{
						"type": "mention",
						"attrs": map[string]interface{}{
							"text": "@johndoe",
						},
					},
				},
			},
		},
	}

	got := ExtractText(adf)
	if got != "@johndoe" {
		t.Errorf("ExtractText(ADF mention) = %q, want '@johndoe'", got)
	}
}
