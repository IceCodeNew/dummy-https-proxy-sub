package yaml

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"

	goyaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

// ProxyItem is the exported representation of a single proxy entry parsed
// from the upstream YAML document.
type ProxyItem struct {
	Name     string `yaml:"name"`
	Password string `yaml:"password"`
	Port     int    `yaml:"port"`
	Server   string `yaml:"server"`
	SNI      string `yaml:"sni"`
	TLS      bool   `yaml:"tls"`
	Type     string `yaml:"type"`
	Username string `yaml:"username"`
}

// buildURLFromParts builds the https proxy URL from host and proxy item
func buildURLFromParts(it ProxyItem) string {
	u := &url.URL{Scheme: "https", Host: net.JoinHostPort(it.Server, strconv.Itoa(it.Port)), Fragment: it.Name}
	u.User = url.UserPassword(it.Username, it.Password)
	q := url.Values{}
	q.Set("sni", it.SNI)
	u.RawQuery = q.Encode()
	return u.String()
}

// ParseProxiesFromReader parses proxies from r, transforms each proxy using
// transformProxy and returns the resulting https-lines.
func ParseProxiesFromReader(ctx context.Context, r io.Reader) ([]string, error) {
	limitReader := io.LimitReader(r, MaxYAMLBytes)

	var (
		err    error
		node   ast.Node
		path   *goyaml.Path
		result []string
	)
	if path, err = goyaml.PathString("$.proxies"); err != nil {
		return nil, fmt.Errorf("failed to create go-yaml.Path: %v", err)
	}
	if node, err = path.ReadNode(limitReader); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("upstream empty")
		}
		return nil, fmt.Errorf("failed to read proxies: %v", err)
	}

	seq, ok := node.(ast.ArrayNode)
	if !ok {
		return nil, fmt.Errorf("proxies must be a sequence, got %T", node)
	}

	for iter := seq.ArrayRange(); iter.Next(); {
		elem := iter.Value()
		mnode, ok := elem.(*ast.MappingNode)
		if !ok {
			return nil, fmt.Errorf("proxy entry not a mapping, got %T", elem)
		}

		var it ProxyItem
		for miter := mnode.MapRange(); miter.Next(); {
			k := strings.TrimSpace(miter.Key().String())
			v := miter.Value()
			switch k {
			case "name":
				if s, err := nodeToString(v); err == nil {
					it.Name = s
				} else {
					return nil, fmt.Errorf("failed while parsing the key %s: %v", k, err)
				}
			case "password":
				if s, err := nodeToString(v); err == nil {
					it.Password = s
				} else {
					return nil, fmt.Errorf("failed while parsing the key %s: %v", k, err)
				}
			case "port":
				if p, err := nodeToInt(v); err == nil {
					it.Port = p
				} else {
					return nil, fmt.Errorf("failed while parsing the key %s: %v", k, err)
				}
			case "server":
				if s, err := nodeToString(v); err == nil {
					it.Server = s
				} else {
					return nil, fmt.Errorf("failed while parsing the key %s: %v", k, err)
				}
			case "sni":
				if s, err := nodeToString(v); err == nil {
					it.SNI = s
				} else {
					return nil, fmt.Errorf("failed while parsing the key %s: %v", k, err)
				}
			case "tls":
				if b, err := nodeToBool(v); err == nil {
					it.TLS = b
				} else {
					return nil, fmt.Errorf("failed while parsing the key %s: %v", k, err)
				}
			case "type":
				if s, err := nodeToString(v); err == nil {
					it.Type = s
				} else {
					return nil, fmt.Errorf("failed while parsing the key %s: %v", k, err)
				}
			case "username":
				if s, err := nodeToString(v); err == nil {
					it.Username = s
				} else {
					return nil, fmt.Errorf("failed while parsing the key %s: %v", k, err)
				}
			default:
				// ignore unknown keys
			}
		}

		line, err := transformProxy(ctx, it)
		if err != nil {
			return nil, fmt.Errorf("failed to convert proxy item from %v, getting err: %v", it, err)
		}
		result = append(result, line)
	}
	return result, nil
}

// transformProxy converts a parsed ProxyItem into the https://... form
func transformProxy(ctx context.Context, it ProxyItem) (string, error) {
	if it.Username == "" {
		return "", fmt.Errorf("username is empty")
	}
	if it.Password == "" {
		return "", fmt.Errorf("password is empty")
	}
	if it.Server == "" {
		return "", fmt.Errorf("server addr is empty")
	}
	if it.Port <= 0 || it.Port > 65535 {
		return "", fmt.Errorf("invalid port %d", it.Port)
	}
	if it.SNI == "" {
		return "", fmt.Errorf("SNI is empty")
	}
	// it is OK if it.Name == ""

	return buildURLFromParts(it), nil
}
