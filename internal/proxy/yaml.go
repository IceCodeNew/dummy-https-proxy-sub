package proxy

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml/ast"
)

// maxYAMLBytes is the maximum number of bytes we will read from the
// upstream YAML document. Tests may adjust this value.
var maxYAMLBytes int64 = 1 << 20 // 1 MiB

// nodeToString tries to extract a stable string representation from the
// AST node. For scalar nodes it prefers the typed value when present and
// falls back to the node.String() text.
func nodeToString(n ast.Node) (string, error) {
	if n == nil {
		return "", fmt.Errorf("nil node")
	}
	var _str string
	if s, ok := n.(ast.ScalarNode); ok {
		if v := s.GetValue(); v != nil {
			switch vv := v.(type) {
			case string:
				_str = vv
			case []byte:
				_str = string(vv)
			default:
				_str = fmt.Sprintf("%v", vv)
			}
		} else {
			_str = s.String()
		}
	}
	return strings.TrimSpace(_str), nil
}

func nodeToInt(n ast.Node) (int, error) {
	if n == nil {
		return 0, fmt.Errorf("nil node")
	}
	if s, ok := n.(ast.ScalarNode); ok {
		if v := s.GetValue(); v != nil {
			switch vv := v.(type) {
			case int:
				return vv, nil
			}
		}
	}
	return strconv.Atoi(strings.TrimSpace(n.String()))
}

func nodeToBool(n ast.Node) (bool, error) {
	if n == nil {
		return false, fmt.Errorf("nil node")
	}
	if s, ok := n.(ast.ScalarNode); ok {
		if v := s.GetValue(); v != nil {
			switch vv := v.(type) {
			case bool:
				return vv, nil
			}
		}
	}
	return strconv.ParseBool(strings.TrimSpace(n.String()))
}
