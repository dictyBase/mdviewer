package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestMarkdownDocumentCollectsRenderedHeadings(t *testing.T) {
	t.Parallel()

	document, err := convertMarkdown([]byte("# Hello *world*\n\n### Repeat\n\n## Repeat\n"))
	if err != nil {
		t.Fatalf("convert markdown: %v", err)
	}

	want := []Heading{
		{Level: 1, ID: "hello-world", Text: "Hello world"},
		{Level: 3, ID: "repeat", Text: "Repeat"},
		{Level: 2, ID: "repeat-1", Text: "Repeat"},
	}
	if !reflect.DeepEqual(document.Headings, want) {
		t.Errorf("headings = %#v, want %#v", document.Headings, want)
	}
	for _, heading := range document.Headings {
		needle := fmt.Sprintf(`<h%d id="%s">`, heading.Level, heading.ID)
		if !strings.Contains(document.HTML, needle) {
			t.Errorf("HTML does not contain rendered heading ID %q:\n%s", heading.ID, document.HTML)
		}
	}
}

func TestMarkdownDocumentCollectsPlainTextFromRichHeadings(t *testing.T) {
	t.Parallel()

	document, err := convertMarkdown([]byte("# API `fmt.Println` &amp; ![Gopher](gopher.png)\n"))
	if err != nil {
		t.Fatalf("convert markdown: %v", err)
	}

	want := []Heading{{
		Level: 1,
		ID:    "api-fmtprintln-amp-gophergopherpng",
		Text:  "API fmt.Println & Gopher",
	}}
	if !reflect.DeepEqual(document.Headings, want) {
		t.Errorf("headings = %#v, want %#v", document.Headings, want)
	}
}

func TestMarkdownAlerts(t *testing.T) {
	t.Parallel()

	for _, alertType := range []string{"NOTE", "TIP", "IMPORTANT", "WARNING", "CAUTION", "note", "WaRnInG"} {
		t.Run(alertType, func(t *testing.T) {
			t.Parallel()

			source := "> [!" + alertType + "]\n> First **important** paragraph.\n>\n> Second paragraph.\n>\n> - one\n> - two\n"
			html, err := convertMarkdownToHTML([]byte(source))
			if err != nil {
				t.Fatalf("convert markdown: %v", err)
			}

			upper := strings.ToUpper(alertType)
			lower := strings.ToLower(upper)
			title := strings.ToUpper(upper[:1]) + strings.ToLower(upper[1:])
			for _, wanted := range []string{
				`<div class="markdown-alert markdown-alert-` + lower + `" role="note" aria-label="` + title + `">`,
				`<p class="markdown-alert-title">` + title + `</p>`,
				`<p>First <strong>important</strong> paragraph.</p>`,
				`<p>Second paragraph.</p>`,
				`<li>one</li>`,
			} {
				if !strings.Contains(html, wanted) {
					t.Errorf("missing %q in:\n%s", wanted, html)
				}
			}
			if strings.Contains(html, "[!"+alertType+"]") || strings.Contains(html, "<blockquote>") {
				t.Errorf("alert marker or blockquote remains in:\n%s", html)
			}
		})
	}
}

func TestNonAlertBlockquotesRemainBlockquotes(t *testing.T) {
	t.Parallel()

	for _, source := range []string{
		"> Ordinary quote.\n",
		"> [!UNKNOWN]\n> Still quoted.\n",
		"> prefix [!NOTE]\n> Still quoted.\n",
		"> [!NOTE] trailing\n> Still quoted.\n",
		"> **[!NOTE]**\n> Still quoted.\n",
	} {
		html, err := convertMarkdownToHTML([]byte(source))
		if err != nil {
			t.Fatalf("convert markdown: %v", err)
		}
		if !strings.Contains(html, "<blockquote>") {
			t.Errorf("ordinary blockquote changed for %q:\n%s", source, html)
		}
		if strings.Contains(html, "markdown-alert") {
			t.Errorf("ordinary blockquote became alert for %q:\n%s", source, html)
		}
	}
}

func TestMermaidRendersClientMarkupWithoutRuntime(t *testing.T) {
	t.Parallel()

	html, err := convertMarkdownToHTML([]byte("```mermaid\ngraph TD\n  A --> B\n```\n"))
	if err != nil {
		t.Fatalf("convert markdown: %v", err)
	}

	if !strings.Contains(html, `<pre class="mermaid">`) {
		t.Errorf("missing Mermaid client markup in:\n%s", html)
	}
	if !strings.Contains(html, "graph TD") || !strings.Contains(html, "A --&gt; B") {
		t.Errorf("missing readable Mermaid source in:\n%s", html)
	}
	for _, unwanted := range []string{"<script", "cdn.jsdelivr.net/npm/mermaid", "mermaid.initialize"} {
		if strings.Contains(html, unwanted) {
			t.Errorf("output contains Mermaid runtime %q:\n%s", unwanted, html)
		}
	}
}
