goldmark-highlighting
=========================

goldmark-highlighting is an extension for the [goldmark](http://github.com/yuin/goldmark) 
that adds syntax-highlighting to the fenced code blocks.

goldmark-highlighting uses [chroma](https://github.com/alecthomas/chroma) as a
syntax highlighter.

Deprecated
--------------------
This branch(master) uses chroma v1 as a syntax highlighter.

Now goldmark-highlighting uses chroma v2 as a syntax highlighter and defaults to [v2 branch](https://github.com/yuin/goldmark-highlighting/tree/v2).

Installation
--------------------

```
go get github.com/yuin/goldmark-highlighting
```

Usage
--------------------

```go
import (
    "bytes"
    "fmt"
    "github.com/alecthomas/chroma/formatters/html"
    "github.com/yuin/goldmark"
    "github.com/yuin/goldmark/extension"
    "github.com/yuin/goldmark/parser"
    "github.com/yuin/goldmark-highlighting"

)

func main() {
    markdown := goldmark.New(
        goldmark.WithExtensions(
            highlighting.Highlighting,
        ),
    )
    var buf bytes.Buffer
    if err := markdown.Convert([]byte(source), &buf); err != nil {
        panic(err)
    }
    fmt.Print(title)
}
```


```go
    markdown := goldmark.New(
        goldmark.WithExtensions(
            highlighting.NewHighlighting(
               highlighting.WithStyle("monokai"),
               highlighting.WithFormatOptions(
                   html.WithLineNumbers(),
               ),
            ),
        ),
    )
```

License
--------------------
MIT

Author
--------------------
Yusuke Inuzuka
