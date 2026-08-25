//go:build !goldmark_v1_attribute

package highlighting

import (
	"encoding/json/v2"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/util"
)

func parseAttribute[T any](n ast.Node, name string, source []byte) (T, bool) {
	if attr, ok := n.Attribute(name); ok {
		var dest T
		if err := json.Unmarshal(util.StringToReadOnlyBytes(attr.Value(source)), &dest); err == nil {
			return dest, true
		}
		if v, ok := any(attr.Value(source)).(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}
