//go:build goldmark_v1_attribute

package highlighting

import (
	"github.com/yuin/goldmark/v2/ast"
)

func parseAttribute[T any](n ast.Node, name string, source []byte) (T, bool) {
	if attr, ok := n.Attribute(name); ok {
		if v, ok := attr.Any(source).(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}
