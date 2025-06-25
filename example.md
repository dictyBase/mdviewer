# Example Markdown Document

This is an example markdown file to demonstrate the capabilities of the MDViewer application.

## Features Showcase

### Basic Formatting

**Bold text** and *italic text* work perfectly. You can also use ~~strikethrough~~ text.

### Code Blocks

Here's some Go code with syntax highlighting:

```go
package main

import (
    "fmt"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello, World!")
    })
    
    fmt.Println("Server starting on :8080")
    http.ListenAndServe(":8080", nil)
}
```

Inline code: `fmt.Println("Hello, World!")`

### Tables

| Feature | Status | Notes |
|---------|--------|-------|
| Markdown Parsing | ✅ | Using goldmark |
| Syntax Highlighting | ✅ | Multiple languages |
| Responsive Design | ✅ | Pico CSS framework |
| Dark Mode | ✅ | Automatic detection |

### Task Lists

- [x] Set up project structure
- [x] Implement markdown parsing
- [x] Add syntax highlighting
- [x] Create beautiful templates
- [ ] Add search functionality
- [ ] Add file editing capability

### Blockquotes

> This is a blockquote. It's great for highlighting important information or quotes from other sources.
>
> You can have multiple paragraphs in a blockquote too.

### Lists

#### Unordered List
- First item
- Second item
  - Nested item
  - Another nested item
- Third item

#### Ordered List
1. First step
2. Second step
3. Third step
   1. Sub-step A
   2. Sub-step B

### Links and Images

[Visit the Go website](https://golang.org) for more information about Go programming.

Auto-linking works too: https://github.com/yuin/goldmark

### Horizontal Rule

---

### Definition Lists

Go
: A programming language developed by Google

Markdown
: A lightweight markup language for creating formatted text

templ
: A language for writing HTML user interfaces in Go

### Footnotes

This is a sentence with a footnote[^1].

Here's another footnote[^note].

[^1]: This is the first footnote.
[^note]: This is a named footnote.

## Mathematical Expressions

While not enabled by default, you could add mathematical expressions:

- Inline math: E = mc²
- Block math expressions could be added with additional plugins

## Emoji Support 🎉

The application supports emoji in markdown! 

- 📚 Documentation
- 🚀 Performance  
- 🎨 Beautiful design
- 🔧 Easy to use

## Conclusion

This example demonstrates the rich formatting capabilities available in the MDViewer application. The combination of goldmark's powerful parsing with Pico CSS's beautiful styling creates an excellent markdown viewing experience.

Try editing this file and refreshing to see the changes!