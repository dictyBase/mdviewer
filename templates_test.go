package main

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func renderComponent(t *testing.T, component templ.Component) string {
	t.Helper()
	var output strings.Builder
	if err := component.Render(context.Background(), &output); err != nil {
		t.Fatalf("render component: %v", err)
	}
	return output.String()
}

func TestBaseLayoutInitializesMermaidSecurelyAndConditionally(t *testing.T) {
	t.Parallel()

	html := renderComponent(t, BaseLayout("Test", MarkdownContent("fixture.md", `<pre class="mermaid">graph TD</pre>`, nil, "")))
	for _, want := range []string{
		`document.querySelectorAll(".mermaid")`,
		`if (diagrams.length === 0)`,
		`void renderMermaid();`,
		`const diagrams = [...document.querySelectorAll(".mermaid")]`,
		`if (diagrams.length === 0)`,
		`document.createElement("script")`,
		`script.src = "/_mdviewer/assets/mermaid-11.12.2.min.js"`,
		`script.onload = async () =>`,
		`script.onerror = (error)`,
		`if (!globalThis.mermaid)`,
		`startOnLoad: false`,
		`securityLevel: "strict"`,
		`await globalThis.mermaid.run({ nodes: [diagram] })`,
		`restoreMermaidSource(diagram, source)`,
		`console.error("Unable to load Mermaid runtime", error)`,
		`This diagram could not be rendered. Mermaid source is shown above.`,
		`message.setAttribute("role", "status")`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered layout does not contain %q", want)
		}
	}
	if strings.Contains(html, "cdn.jsdelivr.net/npm/mermaid") {
		t.Error("rendered layout still depends on the Mermaid CDN")
	}
}

func TestMarkdownContentDoesNotRenderOutline(t *testing.T) {
	t.Parallel()

	headings := []Heading{
		{Level: 1, ID: "overview", Text: "Overview"},
		{Level: 3, ID: "details", Text: "Details"},
	}
	html := renderComponent(t, BaseLayout("Test", MarkdownContent("fixture.md", "<h1>Overview</h1>", headings, "")))
	for _, wanted := range []string{
		"← Back",
		"github-file-box",
		"github-file-header",
		"github-tab active",
		"Preview",
		"fixture.md",
		"<h1>Overview</h1>",
	} {
		if !strings.Contains(html, wanted) {
			t.Errorf("rendered document is missing preserved content %q", wanted)
		}
	}
	for _, unwanted := range []string{
		"document-outline",
		"On this page",
		"document-layout",
		"outline-level-",
	} {
		if strings.Contains(html, unwanted) {
			t.Errorf("rendered document contains removed outline marker %q", unwanted)
		}
	}
}

func TestBaseLayoutStylesAllMarkdownAlertTypes(t *testing.T) {
	t.Parallel()

	html := renderComponent(t, BaseLayout("Test", MarkdownContent("fixture.md", "", nil, "")))
	for _, alertType := range []string{"note", "tip", "important", "warning", "caution"} {
		if !strings.Contains(html, `.markdown-alert-`+alertType) {
			t.Errorf("layout does not style %s alerts", alertType)
		}
	}
	wants := []string{
		`.markdown-alert {`, `.markdown-alert-title {`, `font-style: normal;`,
		`light-dark(`, `.mermaid-error {`,
	}
	for _, want := range wants {
		if !strings.Contains(html, want) {
			t.Errorf("layout does not contain alert style %q", want)
		}
	}
}

func TestBaseLayoutCopyButtonAccessibilityAndContrast(t *testing.T) {
	t.Parallel()

	var output strings.Builder
	content := MarkdownContent("fixture.md", "<pre><code>example</code></pre>", nil, "")
	if err := BaseLayout("Test", content).Render(context.Background(), &output); err != nil {
		t.Fatalf("render BaseLayout: %v", err)
	}

	html := output.String()
	for _, want := range []string{
		`color: inherit;`,
		`width: 1.75rem;`,
		`height: 1.75rem;`,
		`display: flex;`,
		`align-items: center;`,
		`justify-content: center;`,
		`padding: 0;`,
		`.copy-btn svg`,
		`.copy-btn.copied`,
		`.copy-btn.error`,
		`const COPY_ICON =`,
		`const CHECK_ICON =`,
		`const ERROR_ICON =`,
		`button.setAttribute("aria-label", "Copy code to clipboard")`,
		`button.setAttribute("aria-label", "Copied!")`,
		`button.setAttribute("aria-label", "Error copying code")`,
		`button.setAttribute("aria-live", "polite")`,
		`button.setAttribute("aria-atomic", "true")`,
		`button.innerHTML = COPY_ICON`,
		`button.innerHTML = CHECK_ICON`,
		`button.innerHTML = ERROR_ICON`,
		`button.classList.add("copied")`,
		`button.classList.add("error")`,
		`button.classList.remove("copied", "error")`,
		`}, 1500);`,
		`const text = code.textContent ?? pre.textContent ?? "";`,
		`Ignore visually empty blocks, but preserve original whitespace when copying.`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered layout does not contain %q", want)
		}
	}
	if got := strings.Count(html, `aria-hidden="true"`); got != 3 {
		t.Errorf("rendered layout contains %d aria-hidden SVGs, want 3", got)
	}
	if got := strings.Count(html, `focusable="false"`); got != 3 {
		t.Errorf("rendered layout contains %d non-focusable SVGs, want 3", got)
	}
	if strings.Contains(html, "opacity:") {
		t.Error("copy button styling reduces icon opacity")
	}
}
