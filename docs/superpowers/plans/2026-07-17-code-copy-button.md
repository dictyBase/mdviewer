# Code Block Copy Button Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an accessible **Copy** button to every fenced code block rendered in markdown pages served at `http://localhost:8888`.

**Architecture:** Use a pure client-side enhancement in the shared `templates.templ` layout. JavaScript will find rendered `article pre` blocks, wrap each one, add an accessible button, and copy the contained code text with the Clipboard API. Regenerate the derived Go template after changing the source template.

**Tech Stack:** Go, templ, HTML, CSS, browser JavaScript, Pico CSS variables, and the browser Clipboard API.

## Global Constraints

- Do not hand-edit generated `templates_templ.go`; regenerate it from `templates.templ`.
- Scope button injection to fenced code blocks rendered inside `article pre`.
- Do not add buttons to inline code spans or Mermaid diagrams.
- Preserve the existing light/dark theme and responsive behavior.
- Do not implement unrelated application changes.

---

### Task 1: Inspect the existing rendering structure

**Files:**
- Inspect: `markdown.go`
- Inspect: `templates.templ`

**Interfaces:**
- Consumes: Existing markdown rendering and shared page layout.
- Produces: Confirmed DOM assumptions for the implementation tasks.

- [ ] **Step 1: Inspect the template layout and current code-block styles**

  Locate the shared layout, its existing `<style>` block, the `pre` rules, and the position immediately before `</body>` where the script can be added.

- [ ] **Step 2: Inspect Markdown and Mermaid output**

  Confirm that fenced code blocks render as:

  ```html
  <pre><code>...</code></pre>
  ```

  Confirm that Mermaid diagrams render as `.mermaid` elements rather than `<pre>` elements. If Mermaid can produce a `<pre>` fallback, identify the class or marker needed to exclude it.

- [ ] **Step 3: Identify the local verification route**

  Inspect the server routes and determine whether `/README`, `/example`, or another route serves a markdown page containing fenced code blocks.

---

### Task 2: Add copy-button CSS

**Files:**
- Modify: `templates.templ`

**Interfaces:**
- Consumes: Existing layout styles and Pico CSS variables.
- Produces: `.code-block-wrapper`, `.copy-btn`, and `.copy-btn.copied` styles used by the clipboard script.

- [ ] **Step 1: Add wrapper positioning styles**

  Add the following rules to the existing `<style>` block:

  ```css
  .code-block-wrapper {
    position: relative;
  }
  ```

- [ ] **Step 2: Add button styles**

  Add the following rules:

  ```css
  .copy-btn {
    position: absolute;
    top: 0.5rem;
    right: 0.5rem;
    padding: 0.25rem 0.5rem;
    font-size: 0.75rem;
    line-height: 1;
    border-radius: var(--pico-border-radius);
    border: 1px solid var(--pico-card-border-color);
    background: var(--pico-card-background-color);
    color: var(--pico-color);
    cursor: pointer;
    opacity: 0.7;
    transition: opacity 0.2s ease;
  }

  .copy-btn:hover {
    opacity: 1;
  }

  .copy-btn.copied {
    color: var(--pico-primary);
  }
  ```

- [ ] **Step 3: Prevent overlap with horizontally scrolling code**

  Review the existing `pre` styles. If the button can cover long lines, add an appropriate right-side padding such as:

  ```css
  pre {
    padding-right: 2.5rem;
  }
  ```

  Keep the change limited to the existing code-block presentation and verify it does not create a layout regression.

---

### Task 3: Add clipboard behavior

**Files:**
- Modify: `templates.templ`

**Interfaces:**
- Consumes: `.code-block-wrapper` and `.copy-btn` styles from Task 2.
- Produces: One independent copy button for each eligible `article pre` element.

- [ ] **Step 1: Add the client-side script before `</body>`**

  Add a script at the end of the shared layout so the rendered content is already present. The script must select only `article pre` elements and leave inline code untouched.

- [ ] **Step 2: Wrap each code block and add its button**

  For each selected `<pre>`:

  1. Find its `<code>` child.
  2. Skip the block if it has no meaningful code text.
  3. Create a `<div class="code-block-wrapper">`.
  4. Insert the wrapper where the `<pre>` was and move the `<pre>` into it.
  5. Create this button:

  ```html
  <button
    class="copy-btn"
    type="button"
    aria-label="Copy code to clipboard"
  >Copy</button>
  ```

  6. Append the button as a sibling of `<pre>` inside the wrapper.

- [ ] **Step 3: Implement successful copying**

  Read the code from `code.textContent` (falling back to `pre.textContent` if necessary), then call:

  ```js
  navigator.clipboard.writeText(text)
  ```

  On success, change the button text to `Copied!`, add the `copied` class, and restore `Copy` after approximately 1.5 seconds.

- [ ] **Step 4: Implement failure handling**

  Catch Clipboard API failures, including unavailable clipboard access in an insecure context. Temporarily change the button text to `Error`, then restore `Copy`. The failure must not create an uncaught browser error or break other code-block buttons.

- [ ] **Step 5: Exclude Mermaid if required by the inspected DOM**

  If Task 1 confirms Mermaid can render within a `<pre>` element, add a narrowly scoped guard for its class or marker. Otherwise, rely on the `article pre` selector and record that Mermaid is naturally excluded.

---

### Task 4: Regenerate the templ output

**Files:**
- Modify: `templates.templ`
- Regenerate: `templates_templ.go`

**Interfaces:**
- Consumes: Completed template and script changes from Tasks 2–3.
- Produces: Go-generated template code matching `templates.templ`.

- [ ] **Step 1: Run the project’s templ generator**

  Use the repository’s established command, such as:

  ```bash
  templ generate
  ```

  If the repository defines a wrapper command, use that command instead.

- [ ] **Step 2: Confirm the generated file changed appropriately**

  Verify that `templates_templ.go` contains the generated CSS and JavaScript, and that no unrelated generated changes were introduced.

- [ ] **Step 3: Do not hand-edit generated output**

  If generated output is incorrect, fix `templates.templ` and rerun generation rather than editing `templates_templ.go` directly.

---

### Task 5: Build and browser verification

**Files:**
- Test: generated application and rendered markdown page

**Interfaces:**
- Consumes: Regenerated templates from Task 4.
- Produces: Evidence that the feature compiles and behaves correctly in the browser.

- [ ] **Step 1: Build the application**

  Run:

  ```bash
  go build ./...
  ```

  Expected result: successful compilation with exit code 0.

- [ ] **Step 2: Start the local server**

  Run the application using the project’s normal command, for example:

  ```bash
  go run .
  ```

  Use the repository’s configured directory or route arguments if required.

- [ ] **Step 3: Open a markdown page**

  Browse:

  ```text
  http://localhost:8888/README
  ```

  If that route is not available, use the markdown route identified in Task 1.

- [ ] **Step 4: Verify button rendering**

  Confirm that:

  - Every fenced code block has one Copy button.
  - Multiple code blocks have independent buttons.
  - Inline code spans do not receive buttons.
  - Mermaid diagrams do not receive buttons and still render normally.

- [ ] **Step 5: Verify clipboard behavior**

  Click a button and paste the result elsewhere. Confirm that the clipboard contains the code text without HTML tags, syntax-highlighting markup, or unintended wrapper whitespace. Confirm that the button changes to `Copied!` and then returns to `Copy`.

- [ ] **Step 6: Verify failure behavior**

  If clipboard access is unavailable, confirm that the button briefly displays `Error` and that the page remains usable.

- [ ] **Step 7: Verify themes and responsive layout**

  Check both light and dark themes. Resize to a narrow viewport and confirm that the button remains readable and does not interfere with horizontally scrolling code.

## Risks and Assumptions

- Clipboard access should work on `localhost`, but may fail on non-HTTPS remote addresses.
- The implementation assumes fenced blocks use the standard `<pre><code>` structure.
- `textContent` removes syntax-highlighting markup while preserving the displayed code text.
- The page is server-rendered, so duplicate wrapping should not occur during normal page loads. If client-side navigation is introduced later, an idempotency guard may be needed.
- CSS `padding-right` changes should be checked against existing code blocks to avoid a visual regression.
