// package highlighting is a extension for the goldmark(http://github.com/yuin/goldmark).
//
// This extension adds syntax-highlighting to the fenced code blocks using
// chroma(https://github.com/alecthomas/chroma).
package highlighting

import (
	"bytes"
	"io"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"

	"strconv"

	"github.com/alecthomas/chroma"
	chromahtml "github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
)

// Config struct holds options for the extension.
type Config struct {
	html.Config

	// Style is a highlighting style.
	// Supported styles are defined under https://github.com/alecthomas/chroma/tree/master/formatters.
	Style string

	// FormatOptions is a option related to output formats.
	// See https://github.com/alecthomas/chroma#the-html-formatter for details.
	FormatOptions []chromahtml.Option

	// CSSWriter is an io.Writer that will be used as CSS data output buffer.
	// If WithClasses() is enabled, you can get CSS data corresponds to the style.
	CSSWriter io.Writer
}

// NewConfig returns a new Config with defaults.
func NewConfig() Config {
	return Config{
		Config:        html.NewConfig(),
		Style:         "github",
		FormatOptions: []chromahtml.Option{},
		CSSWriter:     nil,
	}
}

// SetOption implements renderer.SetOptioner.
func (c *Config) SetOption(name renderer.OptionName, value interface{}) {
	switch name {
	case optStyle:
		c.Style = value.(string)
	case optFormatOptions:
		if value != nil {
			c.FormatOptions = value.([]chromahtml.Option)
		}
	case optCSSWriter:
		c.CSSWriter = value.(io.Writer)
	default:
		c.Config.SetOption(name, value)
	}
}

// Option interface is a functional option interface for the extension.
type Option interface {
	renderer.Option
	// SetHighlightingOption sets given option to the extension.
	SetHighlightingOption(*Config)
}

type withHTMLOptions struct {
	value []html.Option
}

func (o *withHTMLOptions) SetConfig(c *renderer.Config) {
	if o.value != nil {
		for _, v := range o.value {
			v.(renderer.Option).SetConfig(c)
		}
	}
}

func (o *withHTMLOptions) SetHighlightingOption(c *Config) {
	if o.value != nil {
		for _, v := range o.value {
			v.SetHTMLOption(&c.Config)
		}
	}
}

// WithHTMLOptions is functional option that wraps goldmark HTMLRenderer options.
func WithHTMLOptions(opts ...html.Option) Option {
	return &withHTMLOptions{opts}
}

const optStyle renderer.OptionName = "HighlightingStyle"

type withStyle struct {
	value string
}

func (o *withStyle) SetConfig(c *renderer.Config) {
	c.Options[optStyle] = o.value
}

func (o *withStyle) SetHighlightingOption(c *Config) {
	c.Style = o.value
}

// WithStyle is a functional option that changes highlighting style.
func WithStyle(style string) Option {
	return &withStyle{style}
}

const optCSSWriter renderer.OptionName = "HighlightingCSSWriter"

type withCSSWriter struct {
	value io.Writer
}

func (o *withCSSWriter) SetConfig(c *renderer.Config) {
	c.Options[optCSSWriter] = o.value
}

func (o *withCSSWriter) SetHighlightingOption(c *Config) {
	c.CSSWriter = o.value
}

// WithCSSWriter is a functional option that sets io.Writer for CSS data.
func WithCSSWriter(w io.Writer) Option {
	return &withCSSWriter{w}
}

const optFormatOptions renderer.OptionName = "HighlightingFormatOptions"

type withFormatOptions struct {
	value []chromahtml.Option
}

func (o *withFormatOptions) SetConfig(c *renderer.Config) {
	if _, ok := c.Options[optFormatOptions]; !ok {
		c.Options[optFormatOptions] = []chromahtml.Option{}
	}
	c.Options[optStyle] = append(c.Options[optFormatOptions].([]chromahtml.Option), o.value...)
}

func (o *withFormatOptions) SetHighlightingOption(c *Config) {
	c.FormatOptions = append(c.FormatOptions, o.value...)
}

// WithFormatOptions is a functional option that wraps chroma HTML formatter options.
func WithFormatOptions(opts ...chromahtml.Option) Option {
	return &withFormatOptions{opts}
}

// HTMLRenderer struct is a renderer.NodeRenderer implementation for the extension.
type HTMLRenderer struct {
	Config
}

// NewHTMLRenderer builds a new HTMLRenderer with given options and returns it.
func NewHTMLRenderer(opts ...Option) renderer.NodeRenderer {
	r := &HTMLRenderer{
		Config: NewConfig(),
	}
	for _, opt := range opts {
		opt.SetHighlightingOption(&r.Config)
	}
	return r
}

// RegisterFuncs implements NodeRenderer.RegisterFuncs.
func (r *HTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.renderFencedCodeBlock)
}

func (r *HTMLRenderer) renderFencedCodeBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.FencedCodeBlock)
	if !entering {
		return ast.WalkContinue, nil
	}
	language := n.Language(source)

	doHlLines := false
	hlRanges := [][2]int{}
	chromaFormatterOptions := r.FormatOptions
	if language != nil {
		highlightLinesIdx := -1

		for idx, char := range language {
			if char == '{' {
				highlightLinesIdx = idx
				break
			}
		}

		var linesStr []string
		if highlightLinesIdx > 0 && language[len(language)-1] == '}' {
			doHlLines = true
			rangesStr := string(language[highlightLinesIdx+1 : len(language)-1])
			linesStr = strings.Split(rangesStr, ",")
		}
		for _, l := range linesStr {
			num, err := strconv.Atoi(l)
			if err != nil {
				doHlLines = false
				break
			}
			hlRanges = append(hlRanges, [2]int{num, num})
		}

		if doHlLines {
			language = language[:highlightLinesIdx]
			chromaFormatterOptions = append(chromaFormatterOptions, chromahtml.HighlightLines(hlRanges))
		}
	}

	var lexer chroma.Lexer
	if language != nil {
		lexer = lexers.Get(string(language))
	}
	rendered := false
	if lexer != nil {
		style := styles.Get(r.Style)
		if style == nil {
			style = styles.Fallback
		}
		var buffer bytes.Buffer
		l := n.Lines().Len()
		for i := 0; i < l; i++ {
			line := n.Lines().At(i)
			buffer.Write(line.Value(source))
		}
		iterator, err := lexer.Tokenise(nil, buffer.String())
		if err == nil {
			formatter := chromahtml.New(chromaFormatterOptions...)
			rendered = formatter.Format(w, style, iterator) == nil
			if rendered && r.CSSWriter != nil {
				_ = formatter.WriteCSS(r.CSSWriter, style)
			}
		}
	}

	if !rendered {
		_, _ = w.WriteString("<pre><code")
		_, _ = w.WriteString(" class=\"language-")
		r.Writer.Write(w, language)
		_, _ = w.WriteString("\"")
		_ = w.WriteByte('>')
		l := n.Lines().Len()
		for i := 0; i < l; i++ {
			line := n.Lines().At(i)
			r.Writer.RawWrite(w, line.Value(source))
		}
		_, _ = w.WriteString("</code></pre>\n")
	}
	return ast.WalkContinue, nil
}

type highlighting struct {
	options []Option
}

// Highlighting is a goldmark.Extender implementation.
var Highlighting = &highlighting{
	options: []Option{},
}

// NewHighlighting returns a new extension with given options.
func NewHighlighting(opts ...Option) goldmark.Extender {
	return &highlighting{
		options: opts,
	}
}

// Extend implements goldmark.Extender.
func (e *highlighting) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(NewHTMLRenderer(e.options...), 200),
	))
}
