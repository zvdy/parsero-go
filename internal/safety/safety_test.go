package safety

import (
	"context"
	"net"
	"testing"
)

func TestNormalizeTarget(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"example.com", "example.com", false},
		{"http://example.com/robots.txt", "example.com", false},
		{"https://Example.com:8443/path?q=1", "example.com", false},
		{"user:pass@example.com/x", "example.com", false},
		{"  example.com  ", "example.com", false},
		{"[2606:4700:4700::1111]:443", "2606:4700:4700::1111", false},
		{"", "", true},
		{"localhost", "", true},
		{"foo.local", "", true},
		{"metadata.google.internal", "", true},
		{"127.0.0.1", "", true},
		{"10.1.2.3", "", true},
		{"169.254.169.254", "", true},
	}
	for _, c := range cases {
		got, err := NormalizeTarget(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("NormalizeTarget(%q) expected error, got %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizeTarget(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("NormalizeTarget(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCheckIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "10.0.0.1", "172.16.5.4", "192.168.1.1",
		"169.254.169.254", "0.0.0.0", "::1", "fe80::1", "fc00::1",
	}
	for _, s := range blocked {
		if err := checkIP(net.ParseIP(s)); err == nil {
			t.Errorf("checkIP(%s) = nil, want blocked", s)
		}
	}
	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:4700:4700::1111"}
	for _, s := range allowed {
		if err := checkIP(net.ParseIP(s)); err != nil {
			t.Errorf("checkIP(%s) = %v, want allowed", s, err)
		}
	}
}

func TestResolveAndCheckLiteralIP(t *testing.T) {
	if err := ResolveAndCheck(context.Background(), "10.0.0.1"); err == nil {
		t.Error("expected private literal IP to be rejected")
	}
	if err := ResolveAndCheck(context.Background(), "8.8.8.8"); err != nil {
		t.Errorf("expected public literal IP to pass, got %v", err)
	}
}

func TestGuardedClientBuilds(t *testing.T) {
	if c := GuardedClient(0); c == nil || c.Transport == nil {
		t.Fatal("GuardedClient returned incomplete client")
	}
}
