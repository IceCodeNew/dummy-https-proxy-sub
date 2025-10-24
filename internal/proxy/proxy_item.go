package proxy

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"

	lg "dummy-https-proxy-sub/internal/logger"

	goyaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

// ProxyItem is the exported representation of a single proxy entry parsed
// from the upstream YAML document.
type ProxyItem struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Server   string `yaml:"server"`
	Port     int    `yaml:"port"`
	TLS      bool   `yaml:"tls"`
	Type     string `yaml:"type"`
	Name     string `yaml:"name"`
	SNI      string `yaml:"sni"`
}

// ParseProxiesFromReader parses proxies from r, transforms each proxy using
// transformProxy and returns the resulting https-lines.
func ParseProxiesFromReader(r io.Reader) ([]string, int, error) {
	var (
		err  error
		node ast.Node
		path *goyaml.Path
	)
	if path, err = goyaml.PathString("$.proxies"); err != nil {
		return nil, 0, fmt.Errorf("failed to create go-yaml.Path: %v", err)
	}
	if node, err = path.ReadNode(io.LimitReader(r, maxYAMLBytes)); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, 0, fmt.Errorf("upstream empty")
		}
		return nil, 0, fmt.Errorf("failed to read proxies: %v", err)
	}

	seq, ok := node.(ast.ArrayNode)
	if !ok {
		return nil, 0, fmt.Errorf("proxies must be a sequence, got %T", node)
	}

	result, totalStrLen := make([]string, 0, 64), 0
	for iter := seq.ArrayRange(); iter.Next(); {
		elem := iter.Value()
		mnode, ok := elem.(*ast.MappingNode)
		if !ok {
			return nil, 0, fmt.Errorf("proxy entry not a mapping, got %T", elem)
		}

		line, err := transformProxy(mnode)
		if err != nil {
			return nil, 0, fmt.Errorf("fatal error while parsing proxy item: %v", err)
		}
		if line == "" {
			continue
		}
		result = append(result, line)
		totalStrLen += len(line)
	}
	totalStrLen += len(result)
	return result, totalStrLen, nil
}

// transformProxy converts a parsed ProxyItem into the https://... form
func transformProxy(mnode *ast.MappingNode) (string, error) {
	var it ProxyItem
	for miter := mnode.MapRange(); miter.Next(); {
		k, v := strings.TrimSpace(miter.Key().String()), miter.Value()
		switch k {
		case "username":
			if s, err := nodeToString(v); err == nil {
				it.Username = s
			} else {
				return "", fmt.Errorf("failed while parsing the key %s: %v", k, err)
			}
		case "password":
			if s, err := nodeToString(v); err == nil {
				it.Password = s
			} else {
				return "", fmt.Errorf("failed while parsing the key %s: %v", k, err)
			}
		case "server":
			if s, err := nodeToString(v); err == nil {
				it.Server = s
			} else {
				return "", fmt.Errorf("failed while parsing the key %s: %v", k, err)
			}
		case "port":
			if p, err := nodeToInt(v); err == nil {
				it.Port = p
			} else {
				return "", fmt.Errorf("failed while parsing the key %s: %v", k, err)
			}
		case "tls":
			if b, err := nodeToBool(v); err == nil {
				it.TLS = b
			} else {
				return "", fmt.Errorf("failed while parsing the key %s: %v", k, err)
			}
		case "type":
			if s, err := nodeToString(v); err == nil {
				it.Type = s
			} else {
				return "", fmt.Errorf("failed while parsing the key %s: %v", k, err)
			}
		case "name":
			if s, err := nodeToString(v); err == nil {
				it.Name = s
			} else {
				return "", fmt.Errorf("failed while parsing the key %s: %v", k, err)
			}
		case "sni":
			if s, err := nodeToString(v); err == nil {
				it.SNI = s
			} else {
				return "", fmt.Errorf("failed while parsing the key %s: %v", k, err)
			}
		default:
			// ignore unknown keys
		}
	}

	return craftURL(it), nil
}

func craftURL(it ProxyItem) string {
	if it.Username == "" {
		lg.WarnLogger.Printf("username is empty in proxy item: %v", it)
		return ""
	}
	if it.Password == "" {
		lg.WarnLogger.Printf("password is empty in proxy item: %v", it)
		return ""
	}
	if it.Server == "" {
		lg.WarnLogger.Printf("server addr is empty in proxy item: %v", it)
		return ""
	}
	if it.Port <= 0 || it.Port > 65535 {
		lg.WarnLogger.Printf("invalid port %d in proxy item: %v", it.Port, it)
		return ""
	}
	if !it.TLS {
		lg.WarnLogger.Printf("skipped insecure HTTP proxy: %v", it)
		return ""
	}
	if it.Type != "http" {
		lg.WarnLogger.Printf("unsupported proxy type %s in proxy item: %v", it.Type, it)
		return ""
	}

	u, q := &url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(it.Server, strconv.Itoa(it.Port)),
		User:   url.UserPassword(it.Username, it.Password),
	}, url.Values{}
	if it.Name != "" {
		u.Fragment = it.Name
	}
	if it.SNI != "" {
		q.Set("sni", it.SNI)
		u.RawQuery = q.Encode()
	}
	return u.String()
}
