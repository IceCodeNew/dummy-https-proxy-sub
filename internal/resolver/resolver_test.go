package resolver

import (
	"net"
	"os"
	"testing"
)

func TestNewNetResolverDefaultsToGoogleDNS(t *testing.T) {
	unsetEnv(t, dnsResolverEnvVar)

	resolver := NewNetResolver()
	if resolver.resolver == nil {
		t.Fatalf("resolver instance not initialized")
	}
	if resolver.resolver == net.DefaultResolver {
		t.Fatalf("expected resolver: 8.8.8.8:53, got: %v", resolver.resolver)
	}
	if !resolver.resolver.PreferGo {
		t.Fatalf("expected PreferGo to be true")
	}
	if resolver.resolver.Dial == nil {
		t.Fatalf("expected custom dialer")
	}
}

func TestNewNetResolverUsesDefaultWhenEnvEmpty(t *testing.T) {
	t.Setenv(dnsResolverEnvVar, "")

	resolver := NewNetResolver()
	if resolver.resolver != net.DefaultResolver {
		t.Fatalf("expected default resolver when env empty, got: %v", resolver.resolver)
	}
}

func TestNewNetResolverUsesEnvOverride(t *testing.T) {
	t.Setenv(dnsResolverEnvVar, "1.1.1.1")

	resolver := NewNetResolver()
	if resolver.resolver == nil {
		t.Fatalf("resolver instance not initialized")
	}
	if resolver.resolver == net.DefaultResolver {
		t.Fatalf("expected custom resolver: 1.1.1.1:53, got: %v", resolver.resolver)
	}
	if resolver.resolver.Dial == nil {
		t.Fatalf("expected custom dialer")
	}
}

func TestNormalizeDNSServer(t *testing.T) {
	tests := []struct {
		input    string
		want     string
		wantOkay bool
	}{
		{input: "", wantOkay: false},
		{input: "   ", wantOkay: false},
		{input: "8.8.8.8", want: "8.8.8.8:53", wantOkay: true},
		{input: "1.1.1.1:5353", want: "1.1.1.1:5353", wantOkay: true},
		{input: "[2001:4860:4860::8888]", want: "[2001:4860:4860::8888]:53", wantOkay: true},
		{input: "[2001:4860:4860::8888]:5300", want: "[2001:4860:4860::8888]:5300", wantOkay: true},
		{input: ":5353", wantOkay: false},
	}

	for _, tt := range tests {
		got, ok := normalizeDNSServer(tt.input)
		if ok != tt.wantOkay {
			t.Fatalf("normalizeDNSServer(%q) ok: want %v got %v", tt.input, tt.wantOkay, ok)
		}
		if ok && got != tt.want {
			t.Fatalf("normalizeDNSServer(%q) address: want %q got %q", tt.input, tt.want, got)
		}
	}
}

func TestSanitizeDNSNetwork(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "udp", want: "udp"},
		{input: "udp4", want: "udp4"},
		{input: "tcp6", want: "tcp6"},
		{input: "invalid", want: "udp"},
	}

	for _, tt := range tests {
		got := sanitizeDNSNetwork(tt.input)
		if got != tt.want {
			t.Fatalf("sanitizeDNSNetwork(%q): want %q got %q", tt.input, tt.want, got)
		}
	}
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	original, present := os.LookupEnv(key)
	if present {
		t.Cleanup(func() {
			os.Setenv(key, original)
		})
	} else {
		t.Cleanup(func() {
			os.Unsetenv(key)
		})
	}
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("failed to unset env %s: %v", key, err)
	}
}
