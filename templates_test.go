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
