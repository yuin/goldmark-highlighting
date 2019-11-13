// package highlighting is a extension for the goldmark(http://github.com/yuin/goldmark).
//
// This extension adds syntax-highlighting to the fenced code blocks using
// chroma(https://github.com/alecthomas/chroma).
package highlighting

import (
	"bytes"
	"io"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/alecthomas/chroma"
	chromahtml "github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
)

// WrapperRenderer renders wrapper elements like div, pre, etc.
type WrapperRenderer func(w util.BufWriter, language []byte, attrs parser.Attributes, entering bool)

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

	// WrapperRenderer allows you to change wrapper elements.
	WrapperRenderer WrapperRenderer
}

// NewConfig returns a new Config with defaults.
func NewConfig() Config {
	return Config{
		Config:          html.NewConfig(),
		Style:           "github",
		FormatOptions:   []chromahtml.Option{},
		CSSWriter:       nil,
		WrapperRenderer: nil,
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
	case optWrapperRenderer:
		c.WrapperRenderer = value.(WrapperRenderer)
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

var highlightLinesAttrName = []byte("hl_lines")

var styleAttrName = []byte("hl_style")
var nohlAttrName = []byte("nohl")

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

const optWrapperRenderer renderer.OptionName = "HighlightingWrapperRenderer"

type withWrapperRenderer struct {
	value WrapperRenderer
}

func (o *withWrapperRenderer) SetConfig(c *renderer.Config) {
	c.Options[optWrapperRenderer] = o.value
}

func (o *withWrapperRenderer) SetHighlightingOption(c *Config) {
	c.WrapperRenderer = o.value
}

// WithWrapperRenderer is a functional option that sets WrapperRenderer that
// renders wrapper elements like div, pre, etc.
func WithWrapperRenderer(w WrapperRenderer) Option {
	return &withWrapperRenderer{w}
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

func getAttributes(node *ast.FencedCodeBlock, infostr []byte) (parser.Attributes, []byte) {
	if node.Attributes() != nil {
		r := parser.Attributes{}
		for _, a := range node.Attributes() {
			r = append(r, parser.Attribute{Name: a.Name, Value: a.Value})
		}
		return r, infostr
	}

	if infostr != nil {
		attrStartIdx := -1

		for idx, char := range infostr {
			if char == '{' {
				attrStartIdx = idx
				break
			}
		}

		if attrStartIdx > 0 {
			attrStr := infostr[attrStartIdx:]
			if attrs, hasAttr := parser.ParseAttributes(text.NewReader(attrStr)); hasAttr {
				return attrs, infostr[:attrStartIdx]
			}
		}
	}
	return nil, infostr
}

func (r *HTMLRenderer) renderFencedCodeBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.FencedCodeBlock)
	if !entering {
		return ast.WalkContinue, nil
	}
	language := n.Language(source)

	chromaFormatterOptions := r.FormatOptions
	style := styles.Get(r.Style)
	nohl := false

	attrs, language := getAttributes(n, language)
	if attrs != nil {
		if linesAttr, hasLinesAttr := attrs.Find(highlightLinesAttrName); hasLinesAttr {
			if lines, ok := linesAttr.([]interface{}); ok {
				var hlRanges [][2]int
				for _, l := range lines {
					if ln, ok := l.(float64); ok {
						hlRanges = append(hlRanges, [2]int{int(ln), int(ln)})
					}
					if rng, ok := l.([]uint8); ok {
						slices := strings.Split(string([]byte(rng)), "-")
						lhs, err := strconv.Atoi(slices[0])
						if err != nil {
							continue
						}
						rhs, err := strconv.Atoi(slices[1])
						if err != nil {
							continue
						}
						hlRanges = append(hlRanges, [2]int{lhs, rhs})
					}
				}
				chromaFormatterOptions = append(chromaFormatterOptions, chromahtml.HighlightLines(hlRanges))
			}
		}
		if styleAttr, hasStyleAttr := attrs.Find(styleAttrName); hasStyleAttr {
			styleStr := string([]byte(styleAttr.([]uint8)))
			style = styles.Get(styleStr)
		}
		if _, hasNohlAttr := attrs.Find(nohlAttrName); hasNohlAttr {
			nohl = true
		}
	}

	var lexer chroma.Lexer
	if language != nil {
		lexer = lexers.Get(string(language))
	}
	if !nohl && lexer != nil {
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
			if r.WrapperRenderer != nil {
				chromaFormatterOptions = append(chromaFormatterOptions, chromahtml.PreventSurroundingPre())
			}
			formatter := chromahtml.New(chromaFormatterOptions...)
			if r.WrapperRenderer != nil {
				r.WrapperRenderer(w, language, attrs, true)
			}
			_ = formatter.Format(w, style, iterator) == nil
			if r.WrapperRenderer != nil {
				r.WrapperRenderer(w, language, attrs, false)
			}
			if r.CSSWriter != nil {
				_ = formatter.WriteCSS(r.CSSWriter, style)
			}
			return ast.WalkContinue, nil
		}
	}

	if r.WrapperRenderer != nil {
		r.WrapperRenderer(w, language, attrs, true)
	} else {
		_, _ = w.WriteString("<pre><code")
		language := n.Language(source)
		if language != nil {
			_, _ = w.WriteString(" class=\"language-")
			r.Writer.Write(w, language)
			_, _ = w.WriteString("\"")
		}
		_ = w.WriteByte('>')
	}
	l := n.Lines().Len()
	for i := 0; i < l; i++ {
		line := n.Lines().At(i)
		r.Writer.RawWrite(w, line.Value(source))
	}
	if r.WrapperRenderer != nil {
		r.WrapperRenderer(w, language, attrs, false)
	} else {
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
