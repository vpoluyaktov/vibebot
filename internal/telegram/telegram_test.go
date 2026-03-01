package telegram

import (
	"strings"
	"testing"
)

func TestMarkdownTableToHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Simple table",
			input: `| Name | Age | City |
|------|-----|------|
| John | 30  | NYC  |
| Jane | 25  | LA   |`,
			expected: "<pre>",
		},
		{
			name: "Table with text before and after",
			input: `Here is a table:

| Column1 | Column2 |
|---------|---------|
| Value1  | Value2  |

End of table.`,
			expected: "<pre>",
		},
		{
			name: "Table with alignment markers",
			input: `| Left | Center | Right |
|:-----|:------:|------:|
| A    | B      | C     |`,
			expected: "<pre>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := markdownToTelegramHTML(tt.input)
			if !strings.Contains(result, tt.expected) {
				t.Errorf("Expected result to contain:\n%s\n\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestConvertMarkdownTables(t *testing.T) {
	input := `| Name | Age |
|------|-----|
| John | 30  |
| Jane | 25  |`

	var htmlTables []string
	result := convertMarkdownTables(input, &htmlTables)

	// Should have one table extracted
	if len(htmlTables) != 1 {
		t.Errorf("Expected 1 table, got %d", len(htmlTables))
	}

	// Result should contain placeholder
	if !strings.Contains(result, "@@TABLE_0@@") {
		t.Errorf("Expected placeholder in result, got: %s", result)
	}

	// Formatted table should be properly aligned
	expectedFormatted := "Name │ Age\n─────┼────\nJohn │ 30 \nJane │ 25 "
	if htmlTables[0] != expectedFormatted {
		t.Errorf("Expected formatted table:\n%s\n\nGot:\n%s", expectedFormatted, htmlTables[0])
	}
}

func TestMarkdownToTelegramHTMLWithMixedContent(t *testing.T) {
	input := `**Bold text** and *italic*

| Feature | Status |
|---------|--------|
| Tables  | ✓      |
| Code    | ✓      |

Here is some ` + "`inline code`" + ` and a code block:

` + "```go\nfunc main() {\n}\n```" + `

End of message.`

	result := markdownToTelegramHTML(input)

	// Check that table is converted to pre-formatted text
	if !strings.Contains(result, "<pre>") {
		t.Error("Expected <pre> formatted table in result")
	}

	// Should contain table separator characters
	if !strings.Contains(result, "│") || !strings.Contains(result, "─") {
		t.Error("Expected table formatting characters in result")
	}

	// Check that bold is converted
	if !strings.Contains(result, "<b>Bold text</b>") {
		t.Error("Expected bold HTML in result")
	}

	// Check that code is converted
	if !strings.Contains(result, "<code>inline code</code>") {
		t.Error("Expected inline code HTML in result")
	}

	if !strings.Contains(result, "<pre><code>") {
		t.Error("Expected code block HTML in result")
	}
}
