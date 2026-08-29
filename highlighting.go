// package highlighting is a extension for the goldmark(http://github.com/yuin/goldmark).
//
// This extension adds syntax-highlighting to the fenced code blocks using
// chroma(https://github.com/alecthomas/chroma).
package highlighting

import (
	"bytes"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v3"
	chromahtml "github.com/alecthomas/chroma/v3/formatters/html"
	"github.com/alecthomas/chroma/v3/lexers"
	"github.com/alecthomas/chroma/v3/styles"
	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer"
	"github.com/yuin/goldmark/v2/renderer/html"
	"github.com/yuin/goldmark/v2/text"
	"github.com/yuin/goldmark/v2/util"
)

type codeBlockASTTransformer struct {
}

var defaultCodeBlockASTTransformer parser.ASTTransformer = &codeBlockASTTransformer{}

func (a *codeBlockASTTransformer) Transform(n *ast.Document, reader text.Reader, pc parser.Context) {
	_ = ast.Walk(n, func(n ast.Node, entering bool) (s ast.WalkStatus, err error) {
		s = ast.WalkStatus(ast.WalkContinue)
		if entering {
			if codeblock, ok := n.(*ast.CodeBlock); ok && codeblock.CodeBlockKind == ast.CodeBlockKindFenced {
				info := util.StringToReadOnlyBytes(codeblock.Info.Value(reader.Source()))
				for i := 0; i < len(info); i++ {
					if info[i] == '\\' && i+1 < len(info) && util.IsPunct(info[i+1]) {
						i++
						continue
					}
					if info[i] == '{' {
						attreader := text.NewReader(info[i:], reader.Decoder())
						attributes, ok := parser.ParseAttributes(attreader)
						if ok {
							for _, attr := range attributes {
								codeblock.SetAttribute(attr.Name,
									text.NewMultiLineValueFromString(attr.Value.Str(info[i:]), reader.Decoder()))
							}
						}
					}
				}
			}
		}
		return
	})
}

type parserExtension struct {
}

// NewParser returns a new [parser.Extension] for highlighted fenced code blocks.
func NewParser() parser.Extension {
	return &parserExtension{}
}

func (e *parserExtension) ParserOptions(_ *parser.Config) []parser.Option {
	return []parser.Option{
		parser.WithASTTransformers(
			util.Prioritized(defaultCodeBlockASTTransformer, 200),
		),
	}
}

// Parser is a default [parser.Extension] for highlighted fenced code blocks.
var Parser = NewParser()

// ExcludeFunc is a function type that can be used to exclude certain code blocks from being highlighted.
type ExcludeFunc func(n *ast.CodeBlock, source []byte, rc renderer.Context) bool

type htmlRendererConfig struct {
	style string

	customStyle *chroma.Style

	guessLanguage bool

	excludeFunc ExcludeFunc

	formatterOptions []chromahtml.Option

	codeBlockFormatterOptions func(n *ast.CodeBlock, source []byte, rc renderer.Context) []chromahtml.Option
}

// HTMLRendererOption is a functional option for HTMLRenderer.
type HTMLRendererOption func(*htmlRendererConfig)

// WithStyle is a functional option that changes highlighting style.
//
// Supported styles are defined under https://github.com/alecthomas/chroma/tree/master/formatters.
func WithStyle(style string) HTMLRendererOption {
	return func(c *htmlRendererConfig) {
		c.style = style
	}
}

// WithExcludeFunc is a functional option that sets a function to exclude certain
// code blocks from being highlighted.
func WithExcludeFunc(f ExcludeFunc) HTMLRendererOption {
	return func(c *htmlRendererConfig) {
		c.excludeFunc = f
	}
}

// WithExcludeLanguages is a functional option that sets a list of
// languages to exclude from being highlighted.
func WithExcludeLanguages(langs ...string) HTMLRendererOption {
	return func(c *htmlRendererConfig) {
		c.excludeFunc = func(n *ast.CodeBlock, source []byte, rc renderer.Context) bool {
			language, _ := n.Language(source)
			return slices.Contains(langs, language)
		}
	}
}

// WithCustomStyle is a functional option that changes highlighting style.
//
// Pass in a custom Chroma style. If this is not nil, the Style string will be ignored.
func WithCustomStyle(style *chroma.Style) HTMLRendererOption {
	return func(c *htmlRendererConfig) {
		c.customStyle = style
	}
}

// WithGuessLanguage is a functional option that toggles language guessing if none provided.
//
// If the guessing fails, we will fall back to a text lexer.
// Note that while Chroma's API supports language guessing, the implementation
// is not there yet, so you will currently always get the basic text lexer.
func WithGuessLanguage(b ...bool) HTMLRendererOption {
	return func(c *htmlRendererConfig) {
		if len(b) > 0 {
			c.guessLanguage = b[0]
		} else {
			c.guessLanguage = true
		}
	}
}

// WithFormatterOptions is a functional option that wraps chroma HTML formatter options.
//
// See https://github.com/alecthomas/chroma#the-html-formatter for details.
func WithFormatterOptions(opts ...chromahtml.Option) HTMLRendererOption {
	return func(c *htmlRendererConfig) {
		c.formatterOptions = append(c.formatterOptions, opts...)
	}
}

// WithCodeBlockFormatterOptions is a functional option that allows set Chroma options per code block.
func WithCodeBlockFormatterOptions(f func(*ast.CodeBlock, []byte,
	renderer.Context) []chromahtml.Option) HTMLRendererOption {
	return func(c *htmlRendererConfig) {
		c.codeBlockFormatterOptions = f
	}
}

var cssMapKey = renderer.NewContextKey()

// WithCssMap sets `map[string]string` reference for CSS data in the [renderer.Context].
//
// If `WithClasses()`(FormatterOption) is enabled, renderer will write CSS data corresponds to the style to
// the given map. Keys are style names, and values are CSS data.
func WithCSSMap(rc renderer.Context, m map[string]string) renderer.Context {
	rc.Set(cssMapKey, m)
	return rc
}

type htmlRenderer struct {
	config htmlRendererConfig
}

// NewHTMLRenderer returns a new [html.Extension] .
func NewHTMLRenderer(opts ...HTMLRendererOption) html.Extension {
	cfg := htmlRendererConfig{
		style: "github",
		codeBlockFormatterOptions: func(_ *ast.CodeBlock, _ []byte, _ renderer.Context) []chromahtml.Option {
			return []chromahtml.Option{}
		},
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &htmlRenderer{config: cfg}
}

// HTMLRenderer is a default [html.Extension] for highlighted fenced code blocks.
var HTMLRenderer = NewHTMLRenderer()

func (r *htmlRenderer) RendererOptions(_ *html.Config) []html.Option {
	decorator := func(next html.NodeRenderer) html.NodeRenderer {
		return html.NodeRendererFunc(func(w io.Writer, source []byte, node ast.Node,
			entering bool, rc renderer.Context) (ast.WalkStatus, error) {
			n := node.(*ast.CodeBlock)
			if n.CodeBlockKind != ast.CodeBlockKindFenced {
				return next.Render(w, source, n, entering, rc)
			}
			return r.render(w, source, n, entering, next, rc)
		})
	}

	return []html.Option{
		html.WithNodeRendererDecorator(ast.KindCodeBlock, decorator),
	}
}

func (r *htmlRenderer) render(
	writer io.Writer, source []byte, node ast.Node, entering bool, next html.NodeRenderer, rc renderer.Context) (ast.WalkStatus, error) {
	n := node.(*ast.CodeBlock)
	if r.config.excludeFunc != nil {
		if r.config.excludeFunc(n, source, rc) {
			return next.Render(writer, source, node, entering, rc)
		}
	}

	if !entering {
		return ast.WalkContinue, nil
	}

	w := writer.(util.BufWriter)

	language, lok := n.Language(source)
	formatterOptions := make([]chromahtml.Option, len(r.config.formatterOptions))
	copy(formatterOptions, r.config.formatterOptions)
	style := r.config.customStyle
	if style == nil {
		style = styles.Get(r.config.style)
	}
	nohl := false
	var linenostart int
	linenostart, ok := parseAttribute[int](n, "linenostart", source)
	if ok {
		formatterOptions = append(formatterOptions, chromahtml.BaseLineNumber(linenostart))
	} else {
		linenostart = 1
	}
	var hlRanges [][2]int
	if hllinesv, ok := parseAttribute[any](n, "hl_lines", source); ok {
		var hllines []any
		if v, ok := hllinesv.(float64); ok {
			hllines = append(hllines, v)
		}
		if v, ok := hllinesv.([]any); ok {
			hllines = append(hllines, v...)
		}
		if v, ok := hllinesv.(string); ok {
			hllines = append(hllines, v)
		}
		if v, ok := hllinesv.([]byte); ok { // goldmark_v_attribute compatibilities
			hllines = append(hllines, string(v))
		}

		for _, l := range hllines {
			if ln, ok := l.(float64); ok {
				hlRanges = append(hlRanges, [2]int{int(ln) + linenostart - 1, int(ln) + linenostart - 1})
			}
			if rngb, ok := l.([]byte); ok { // goldmark_v_attribute compatibilities
				l = string(rngb)
			}
			if rng, ok := l.(string); ok {
				slices := strings.Split(rng, "-")
				lhs, err := strconv.Atoi(slices[0])
				if err != nil {
					continue
				}
				rhs := lhs
				if len(slices) > 1 {
					rhs, err = strconv.Atoi(slices[1])
					if err != nil {
						continue
					}
				}
				hlRanges = append(hlRanges, [2]int{lhs + linenostart - 1, rhs + linenostart - 1})
			}
		}
	}
	if len(hlRanges) > 0 {
		formatterOptions = append(formatterOptions, chromahtml.HighlightLines(hlRanges))
	}

	if hlstyle, ok := parseAttribute[string](n, "hl_style", source); ok {
		style = styles.Get(hlstyle)
	}
	if _, ok := n.Attribute("nohl"); ok {
		nohl = true
	}
	if linenos, ok := parseAttribute[any](n, "linenos", source); ok {
		var sv string
		switch v := linenos.(type) {
		case bool:
			formatterOptions = append(formatterOptions, chromahtml.WithLineNumbers(v))
		case []byte: // goldmark_v_attribute compatibilities
			sv = string(v)
		case string:
			sv = v
		}

		if sv != "" {
			formatterOptions = append(formatterOptions, chromahtml.WithLineNumbers(true))
			if sv == "table" {
				formatterOptions = append(formatterOptions, chromahtml.LineNumbersInTable(true))
			}
			if sv == "inline" {
				formatterOptions = append(formatterOptions, chromahtml.LineNumbersInTable(false))
			}
		}
	}

	var lexer chroma.Lexer
	if lok {
		lexer = lexers.Get(language)
	}
	if !nohl && (lexer != nil || r.config.guessLanguage) {
		if style == nil {
			style = styles.Fallback
		}
		var buffer bytes.Buffer
		_, _ = n.Value.WriteTo(&buffer, source)

		if lexer == nil {
			lexer = lexers.Analyse(buffer.String())
			if lexer == nil {
				lexer = lexers.Fallback
			}
			// language = strings.ToLower(lexer.Config().Name)
		}
		lexer = chroma.Coalesce(lexer)

		iterator, err := lexer.Tokenise(nil, buffer.String())
		if err == nil {
			fopts := append(formatterOptions, r.config.codeBlockFormatterOptions(n, source, rc)...)
			formatter := chromahtml.New(fopts...)
			_ = formatter.Format(w, style, iterator) == nil
			if m, ok := rc.Get(cssMapKey).(map[string]string); ok {
				if _, ok := m[style.Name]; !ok {
					var cssbuf bytes.Buffer
					_ = formatter.WriteCSS(&cssbuf, style)
					m[style.Name] = cssbuf.String()
				}
			}
			return ast.WalkContinue, nil
		}
	}

	_, _ = next.Render(writer, source, node, true, rc)
	return next.Render(writer, source, node, false, rc)
}
