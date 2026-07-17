package main

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"strings"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/mermaid"
)

type Heading struct {
	Level int
	ID    string
	Text  string
}

type MarkdownDocument struct {
	HTML     string
	Headings []Heading
}

var alertTitles = map[string]string{
	"NOTE":      "Note",
	"TIP":       "Tip",
	"IMPORTANT": "Important",
	"WARNING":   "Warning",
	"CAUTION":   "Caution",
}

var kindMarkdownAlert = ast.NewNodeKind("MarkdownAlert")

type markdownAlert struct {
	ast.BaseBlock
	alertType string
	title     string
}

func (n *markdownAlert) Kind() ast.NodeKind { return kindMarkdownAlert }
func (n *markdownAlert) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"Type": n.alertType}, nil)
}

type alertExtension struct{}

func (alertExtension) Extend(markdown goldmark.Markdown) {
	markdown.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(alertTransformer{}, 100),
	))
	markdown.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(alertRenderer{}, 500),
	))
}

type alertTransformer struct{}

func (alertTransformer) Transform(document *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()
	var blockquotes []*ast.Blockquote
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if blockquote, isBlockquote := node.(*ast.Blockquote); isBlockquote {
				blockquotes = append(blockquotes, blockquote)
			}
		}
		return ast.WalkContinue, nil
	})

	for _, blockquote := range blockquotes {
		transformAlertBlockquote(blockquote, source)
	}
}

func transformAlertBlockquote(blockquote *ast.Blockquote, source []byte) {
	paragraph, isParagraph := blockquote.FirstChild().(*ast.Paragraph)
	if !isParagraph {
		return
	}
	markerNodes, marker := alertMarker(paragraph, source)
	if len(marker) < 4 || !strings.HasPrefix(marker, "[!") || !strings.HasSuffix(marker, "]") {
		return
	}
	alertType := strings.ToUpper(marker[2 : len(marker)-1])
	title, isAlert := alertTitles[alertType]
	if !isAlert {
		return
	}

	for _, markerNode := range markerNodes {
		paragraph.RemoveChild(paragraph, markerNode)
	}
	if paragraph.ChildCount() == 0 {
		blockquote.RemoveChild(blockquote, paragraph)
	}
	alert := &markdownAlert{alertType: strings.ToLower(alertType), title: title}
	for child := blockquote.FirstChild(); child != nil; {
		next := child.NextSibling()
		blockquote.RemoveChild(blockquote, child)
		alert.AppendChild(alert, child)
		child = next
	}
	parent := blockquote.Parent()
	parent.ReplaceChild(parent, blockquote, alert)
}

func alertMarker(paragraph *ast.Paragraph, source []byte) ([]*ast.Text, string) {
	var markerNodes []*ast.Text
	var markerText strings.Builder
	for child := paragraph.FirstChild(); child != nil; child = child.NextSibling() {
		textNode, isText := child.(*ast.Text)
		if !isText {
			break
		}
		markerNodes = append(markerNodes, textNode)
		markerText.Write(textNode.Segment.Value(source))
		if textNode.SoftLineBreak() || textNode.HardLineBreak() {
			break
		}
	}
	return markerNodes, markerText.String()
}

type alertRenderer struct{}

func (alertRenderer) RegisterFuncs(registerer renderer.NodeRendererFuncRegisterer) {
	registerer.Register(kindMarkdownAlert, renderMarkdownAlert)
}

func renderMarkdownAlert(
	writer util.BufWriter,
	_ []byte,
	node ast.Node,
	entering bool,
) (ast.WalkStatus, error) {
	alert := node.(*markdownAlert)
	if entering {
		_, _ = fmt.Fprintf(
			writer,
			"<div class=\"markdown-alert markdown-alert-%s\" role=\"note\" aria-label=\"%s\">\n"+
				"<p class=\"markdown-alert-title\">%s</p>\n",
			alert.alertType,
			alert.title,
			alert.title,
		)
	} else {
		_, _ = writer.WriteString("</div>\n")
	}
	return ast.WalkContinue, nil
}

func newMarkdown() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			alertExtension{},
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
				NoScript:   true,                     // The shared page template owns the runtime.
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
}

func convertMarkdown(source []byte) (MarkdownDocument, error) {
	markdown := newMarkdown()
	document := markdown.Parser().Parse(text.NewReader(source))

	var headings []Heading
	if err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		heading, isHeading := node.(*ast.Heading)
		if !isHeading {
			return ast.WalkContinue, nil
		}
		idValue, hasID := heading.AttributeString("id")
		if !hasID {
			return ast.WalkContinue, nil
		}
		headingID, validID := idValue.([]byte)
		if !validID {
			return ast.WalkContinue, nil
		}
		headings = append(headings, Heading{
			Level: heading.Level,
			ID:    string(headingID),
			Text:  headingPlainText(heading, source),
		})
		return ast.WalkContinue, nil
	}); err != nil {
		return MarkdownDocument{}, fmt.Errorf("failed to collect Markdown headings: %w", err)
	}

	var buf bytes.Buffer
	if err := markdown.Renderer().Render(&buf, source, document); err != nil {
		return MarkdownDocument{}, fmt.Errorf("failed to convert markdown to HTML: %w", err)
	}
	return MarkdownDocument{HTML: buf.String(), Headings: headings}, nil
}

func headingPlainText(heading *ast.Heading, source []byte) string {
	var textContent strings.Builder
	_ = ast.Walk(heading, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if textNode, isText := node.(*ast.Text); isText {
				textContent.Write(textNode.Segment.Value(source))
			}
		}
		return ast.WalkContinue, nil
	})
	return stdhtml.UnescapeString(textContent.String())
}

func convertMarkdownToHTML(source []byte) (string, error) {
	document, err := convertMarkdown(source)
	return document.HTML, err
}
