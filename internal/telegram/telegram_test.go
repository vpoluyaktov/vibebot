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
			expected: "<table><thead><tr><th>Name</th><th>Age</th><th>City</th></tr></thead><tbody><tr><td>John</td><td>30</td><td>NYC</td></tr><tr><td>Jane</td><td>25</td><td>LA</td></tr></tbody></table>",
		},
		{
			name: "Table with text before and after",
			input: `Here is a table:

| Column1 | Column2 |
|---------|---------|
| Value1  | Value2  |

End of table.`,
			expected: "<table><thead><tr><th>Column1</th><th>Column2</th></tr></thead><tbody><tr><td>Value1</td><td>Value2</td></tr></tbody></table>",
		},
		{
			name: "Table with alignment markers",
			input: `| Left | Center | Right |
|:-----|:------:|------:|
| A    | B      | C     |`,
			expected: "<table><thead><tr><th>Left</th><th>Center</th><th>Right</th></tr></thead><tbody><tr><td>A</td><td>B</td><td>C</td></tr></tbody></table>",
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

	// HTML table should be properly formatted
	expectedHTML := "<table><thead><tr><th>Name</th><th>Age</th></tr></thead><tbody><tr><td>John</td><td>30</td></tr><tr><td>Jane</td><td>25</td></tr></tbody></table>"
	if htmlTables[0] != expectedHTML {
		t.Errorf("Expected HTML:\n%s\n\nGot:\n%s", expectedHTML, htmlTables[0])
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

	// Check that table is converted to HTML
	if !strings.Contains(result, "<table>") {
		t.Error("Expected HTML table in result")
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
