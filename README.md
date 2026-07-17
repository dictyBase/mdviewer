# MDViewer

A simple markdown viewer web application built with Go, using:
- **templ** for HTML templating
- **goldmark** for markdown processing with syntax highlighting
- **Pico CSS** for beautiful, semantic styling
- **urfave/cli/v2** for command-line interface

## Features

- 📁 **Case-insensitive file matching** - Access files without worrying about exact case
- 🎨 **Beautiful styling** with Pico CSS framework
- 🌓 **Dark/light mode** support automatically
- 🔍 **Syntax highlighting** for code blocks
- 📋 **GitHub Flavored Markdown** support including:
  - Tables
  - Task lists
  - Strikethrough
  - Auto-linking
  - Footnotes
  - Definition lists
- 📊 **Mermaid diagrams** with client-side rendering support:
  - Flowcharts
  - Sequence diagrams
  - Pie charts
  - Class diagrams
  - And all other Mermaid diagram types
- 📱 **Responsive design** works on mobile and desktop
- 🚀 **Fast and lightweight** - pure Go with minimal dependencies

## Installation

1. Clone or download this application
2. Install dependencies:
   ```bash
   go mod download
   ```
3. Generate templates:
   ```bash
   templ generate
   ```
4. Build the application:
   ```bash
   go build -o mdviewer
   ```

## Usage

### Basic usage
```bash
./mdviewer
```
This starts the server on port 8888 serving markdown files from the current directory.

### Custom directory, host, and port
```bash
./mdviewer --dir /path/to/markdown/files --host 127.0.0.1 --port 3000
```

### Command line options
- `--dir, -d`: Directory containing markdown files (default: current directory)
- `--host`: Host interface to serve on (default: `127.0.0.1`; use `0.0.0.0` only on a trusted network)
- `--port, -p`: Port to serve on (default: 8888)
- `--help, -h`: Show help

## Supported File Extensions

The application recognizes these markdown file extensions:
- `.md`
- `.markdown`
- `.mdown`
- `.mkd`
- `.mkdn`
- `.mdwn`
- `.mdtxt`
- `.mdtext`

## File Access and Trust Model

MDViewer is intended for trusted local documentation trees. It renders Markdown as HTML, including raw HTML supported by Goldmark, so do not point it at untrusted repositories. The server binds to `127.0.0.1` by default; use `--host 0.0.0.0` only when you deliberately want network access.

Non-Markdown files are served only when they have an allowed documentation-asset extension (images, PDF, common audio/video, CSS, CSV, and plain text). Files such as `.env`, `.html`, `.js`, binaries, and extensionless files are not served. Markdown URLs always render the document; raw Markdown download is not currently supported.

Files can be accessed via URLs without the markdown extension. For example:
- `README.md` → `http://localhost:8888/README`
- `docs/guide.md` → `http://localhost:8888/docs/guide`

The matching is case-insensitive, so `readme`, `README`, or `ReAdMe` will all match `README.md`.

## Mermaid Diagrams

MDViewer supports Mermaid diagrams with client-side rendering. Simply use fenced code blocks with the `mermaid` language:

### Flowchart Example
````markdown
```mermaid
graph TD
    A[Start] --> B{Decision}
    B -->|Yes| C[Success]
    B -->|No| D[Try Again]
    D --> A
```
````

### Sequence Diagram Example
````markdown
```mermaid
sequenceDiagram
    participant User
    participant App
    participant Server
    
    User->>App: Request
    App->>Server: Process
    Server->>App: Response
    App->>User: Display
```
````

Diagrams are rendered client-side using the pinned MermaidJS runtime bundled with MDViewer, so no external CDN or server-side processing is required. If a browser cannot render a diagram, the readable Mermaid source remains visible and the page shows a fallback message. All diagram types supported by Mermaid are available.

## Example

1. Create some markdown files in a directory
2. Run the server: `./mdviewer --dir ./my-docs`
3. Open your browser to `http://localhost:8888`
4. Browse and view your markdown files with beautiful formatting!

## Development

To run in development mode with live reloading:
```bash
templ generate --watch --proxy="http://localhost:8888" --cmd="go run ."
```

This will:
- Watch for changes to `.templ` files and regenerate Go code
- Restart the server when Go files change
- Provide live browser reloading
