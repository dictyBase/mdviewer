package main

import (
	"context"
	"strings"
	"testing"
)

func TestBaseLayoutCopyButtonAccessibilityAndContrast(t *testing.T) {
	t.Parallel()

	var output strings.Builder
	content := MarkdownContent("fixture.md", "<pre><code>example</code></pre>")
	if err := BaseLayout("Test", content).Render(context.Background(), &output); err != nil {
		t.Fatalf("render BaseLayout: %v", err)
	}

	html := output.String()
	for _, want := range []string{
		`color: inherit;`,
		`button.setAttribute("aria-live", "polite")`,
		`button.setAttribute("aria-atomic", "true")`,
		`Ignore visually empty blocks, but preserve original whitespace when copying.`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered layout does not contain %q", want)
		}
	}
	if strings.Contains(html, "opacity: 0.7") {
		t.Error("copy button resting style reduces the opacity of its text")
	}
}
