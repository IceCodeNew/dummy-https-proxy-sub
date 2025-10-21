package resolver

import (
	"context"
	"net"
	"os"
	"strings"
)

const (
	dnsResolverEnvVar = "DNS_SERVER"
	defaultDNSServer  = "8.8.8.8:53"
)

// Resolver is the minimal DNS resolver interface used by other packages.
// It is implemented by *NetResolver and can be satisfied by test fakes.
type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// NetResolver wraps net.Resolver to satisfy the Resolver interface.
type NetResolver struct {
	resolver *net.Resolver
}

// NewNetResolver constructs a Resolver honoring environment overrides and sensible defaults.
func NewNetResolver() *NetResolver {
	if server, ok := os.LookupEnv(dnsResolverEnvVar); ok {
		if strings.TrimSpace(server) == "" {
			return &NetResolver{resolver: net.DefaultResolver}
		}
		return &NetResolver{resolver: strToResolver(server)}
	}
	return &NetResolver{resolver: strToResolver(defaultDNSServer)}
}

// LookupIPAddr delegates to the underlying net.Resolver.
func (n *NetResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return n.resolver.LookupIPAddr(ctx, host)
}

func strToResolver(server string) *net.Resolver {
	address, ok := normalizeDNSServer(server)
	if !ok {
		return net.DefaultResolver
	}

	dialer := &net.Dialer{}
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			network = sanitizeDNSNetwork(network)
			return dialer.DialContext(ctx, network, address)
		},
	}
}

func normalizeDNSServer(server string) (string, bool) {
	host, port, err := net.SplitHostPort(server)
	if err != nil {
		host = server
		port = "53"
	}

	if host = strings.TrimSpace(host); host == "" {
		return "", false
	}
	if parsed := net.ParseIP(strings.Trim(host, "[]")); parsed != nil {
		host = parsed.String()
	} else {
		return "", false
	}

	if port = strings.TrimSpace(port); port == "" {
		port = "53"
	}
	return net.JoinHostPort(host, port), true
}

func sanitizeDNSNetwork(network string) string {
	switch network {
	case "udp", "udp4", "udp6", "tcp", "tcp4", "tcp6":
		return network
	default:
		return "udp"
	}
}
