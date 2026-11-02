## goldmark-highlighting

goldmark-highlighting is an extension for the [goldmark](http://github.com/yuin/goldmark) 
that adds syntax-highlighting to the fenced code blocks.

goldmark-highlighting uses [chroma](https://github.com/alecthomas/chroma) as a
syntax highlighter.

### Compatibility

- goldmark-highlighting/v3 uses goldmark/v2, chroma/v3, json/v2
  - minimum Go version: 1.27
  - you can build goldmark-highlighting/v3 with `goldmark_v1_attribute` build tag to use goldmark/v1 compatible attribute parsing.
- For goldmark/v1, please use goldmark-highlighting/v2.

### Breaking changes from v2 to v3

- `WithCSSWriter(w io.Writer)` is removed. Use `WithCSSMap(rc renderer.Context, m map[string]string)` instead.
- `WithFormatOptions(opts ...chromahtml.Option)` is renamed to `WithFormatterOptions(opts ...chromahtml.Option)`.
- `WithWrapperRenderer(w WrapperRenderer) Option` is removed. Use goldmark/v2 `NodeRendererDecorator` instead.

### Installation

```
go get github.com/yuin/goldmark-highlighting/v3
```

### Usage

```go
package main

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark/v2"

	chromahtml "github.com/alecthomas/chroma/v3/formatters/html"
	highlighting "github.com/yuin/goldmark-highlighting/v3"
)

func main() {
	mdsrc := `
		Title
		=======
		` + "```go {hl_lines=2}" + `
		func main() {
		    fmt.Println("ok")
		}
		` + "```" + `
	`

	// Simple usage
	p := parser.New(parser.WithExtensions(highlighting.NewParser()))
	r := html.New(html.WithExtensions(highlighting.NewHTMLRenderer()))
	node := p.Parse([]byte(mdsrc))
	var buf bytes.Buffer
	if err := r.Render([]byte(mdsrc), &buf, node); err != nil {
		panic(err)
	}
	title := buf.String()
	fmt.Print(title)

	// Custom configuration
	r := html.New(html.WithExtensions(
		highlighting.NewHTMLRenderer(
			highlighting.WithStyle("monokai"),
			highlighting.WithFormatterOptions(
				chromahtml.WithClasses(true),
				chromahtml.WithLineNumbers(false),
			),
			highlighting.WithCodeBlockFormatterOptions(
				func(n *ast.CodeBlock, source []byte, _ renderer.Context) []chromahtml.Option {
					if language, ok := n.Language(source); ok && language == "go" {
						return []chromahtml.Option{
							chromahtml.WithLineNumbers(true),
						}
					}
					return nil
				}),
		),
	),
	)
	var buf2 bytes.Buffer
	if err := r.Render([]byte(mdsrc), &buf2, node); err != nil {
		panic(err)
	}
	title2 := buf2.String()
	fmt.Print(title2)
}

```

### Use goldmark-highlighting with other code block extensions

This extension is implemented using `NodeRendererDecorator`. You can skip this extension and use other code block extensions by using the `WithExcludeLanguages` or `WithExcludeFunc` options.

```go
r := html.New(html.WithExtensions(highlighting.NewHTMLRenderer(
	highlighting.WithExcludeLanguages("mermaid"), // exclude mermaid code blocks( to render mermaid code as diagrams)
)))
```

### License
MIT

### Author
Yusuke Inuzuka
