package main

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/mermaid"
)

func convertMarkdownToHTML(source []byte) (string, error) {
	markdown := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,            // GitHub Flavored Markdown
			extension.Table,          // Tables
			extension.Strikethrough,  // Strikethrough
			extension.Linkify,        // Auto-link URLs
			extension.TaskList,       // Task lists
			extension.DefinitionList, // Definition lists
			extension.Footnote,       // Footnotes
			highlighting.NewHighlighting( // Syntax highlighting
				highlighting.WithStyle("github"),
				highlighting.WithFormatOptions(
				// Add line numbers and other formatting options
				),
			),
			meta.Meta, // YAML front matter
			&mermaid.Extender{
				RenderMode: mermaid.RenderModeClient, // Client-side rendering
			},
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // Auto-generate heading IDs
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(), // Convert line breaks to <br>
			html.WithXHTML(),     // Generate XHTML-compliant output
			html.WithUnsafe(),    // Allow raw HTML (be careful with this)
		),
	)

	var buf bytes.Buffer
	if err := markdown.Convert(source, &buf); err != nil {
		return "", fmt.Errorf("failed to convert markdown to HTML: %w", err)
	}

	return buf.String(), nil
}
