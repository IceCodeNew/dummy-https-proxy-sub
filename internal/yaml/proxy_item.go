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
	"time"

	"dummy-https-proxy-sub/internal/resolver"

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
// transformProxy and returns the resulting https-lines. The caller must
// provide a resolver implementing resolverpkg.Resolver.
func ParseProxiesFromReader(ctx context.Context, r io.Reader, resolver resolver.Resolver) ([]string, error) {
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

		line, err := transformProxy(ctx, it, resolver)
		if err != nil {
			return nil, fmt.Errorf("failed to convert proxy item from %v, getting err: %v", it, err)
		}
		result = append(result, line)
	}
	return result, nil
}

// transformProxy converts a parsed ProxyItem into the https://... form
// resolving the server name to an IP address via the provided resolver.
func transformProxy(ctx context.Context, it ProxyItem, resolver resolver.Resolver) (string, error) {
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

	var err error
	if it.Server, err = resolveProxyHost(ctx, it.Server, resolver); err != nil {
		return "", err
	}
	return buildURLFromParts(it), nil
}

func resolveProxyHost(ctx context.Context, host string, resolver resolver.Resolver) (string, error) {
	// Resolve server: accept IPv4 or IPv6 literals, otherwise resolve it through DNS.
	if addr := net.ParseIP(host); addr != nil {
		// host is an IP literal (v4 or v6)
		_addr_str := addr.String()
		if len(addr) == net.IPv6len {
			_addr_str = "[" + _addr_str + "]"
		}
		return _addr_str, nil
	}

	// Prefer IPv4, when only IPv6 addrs are found, use IPv6.
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	addrs, err := resolver.LookupIPAddr(lookupCtx, host)
	if err != nil || len(addrs) == 0 {
		return "", fmt.Errorf("dns lookup for host %s failed: %v", host, err)
	}

	var v6Record string
	for _, a := range addrs {
		if len(a.IP) == net.IPv6len {
			v6Record = a.String()
		} else {
			return a.String(), nil
		}
	}
	return v6Record, nil
}
