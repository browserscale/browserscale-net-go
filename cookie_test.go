package browserscalenet

import (
	"testing"
	"time"
)

func TestJarCookieMatching(t *testing.T) {
	j := NewJar()
	j.Set(
		Cookie{Name: "host", Value: "1", Domain: "example.com", HostOnly: true, Path: "/"},
		Cookie{Name: "dom", Value: "2", Domain: "example.com", Path: "/"},
		Cookie{Name: "secure", Value: "3", Domain: "example.com", Path: "/", Secure: true},
		Cookie{Name: "deep", Value: "4", Domain: "example.com", Path: "/a/b"},
		Cookie{Name: "other", Value: "5", Domain: "other.com", Path: "/"},
	)

	tests := []struct {
		name string
		url  string
		want string
	}{
		{"https root", "https://example.com/", "host=1; dom=2; secure=3"},
		{"http root drops secure", "http://example.com/", "host=1; dom=2"},
		{"subdomain gets domain cookie only", "https://www.example.com/", "dom=2; secure=3"},
		{"deeper path, longest first", "https://example.com/a/b/c", "deep=4; host=1; dom=2; secure=3"},
		{"unrelated host", "https://nope.com/", ""},
		{"other host", "https://other.com/", "other=5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := j.Cookie(tt.url); got != tt.want {
				t.Errorf("Cookie(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestJarExpiry(t *testing.T) {
	j := NewJar()
	j.Set(
		Cookie{Name: "live", Value: "1", Domain: "example.com", HostOnly: true, Path: "/", Expires: time.Now().Add(time.Hour)},
		Cookie{Name: "dead", Value: "2", Domain: "example.com", HostOnly: true, Path: "/", Expires: time.Now().Add(-time.Hour)},
		Cookie{Name: "session", Value: "3", Domain: "example.com", HostOnly: true, Path: "/"},
	)
	if got, want := j.Cookie("https://example.com/"), "live=1; session=3"; got != want {
		t.Errorf("Cookie = %q, want %q", got, want)
	}
}

func TestJarSetFromResponse(t *testing.T) {
	j := NewJar()
	resp := &Response{Header: Header{
		{"set-cookie", "a=1; Path=/"},
		{"set-cookie", "b=2; Domain=example.com; Path=/sub; Secure"},
		{"Set-Cookie", "c=3; Domain=evil.com"}, // must be rejected (foreign domain)
	}}
	j.SetFromResponse("https://www.example.com/page", resp)

	if got, want := j.Cookie("https://www.example.com/"), "a=1"; got != want {
		t.Errorf("root = %q, want %q", got, want)
	}
	if got, want := j.Cookie("https://www.example.com/sub/x"), "b=2; a=1"; got != want {
		t.Errorf("sub = %q, want %q", got, want)
	}
	if got := j.Cookie("https://evil.com/"); got != "" {
		t.Errorf("evil.com should have no cookies, got %q", got)
	}

	// Deleting via Max-Age<=0 removes it.
	j.SetFromResponse("https://www.example.com/", &Response{Header: Header{
		{"set-cookie", "a=1; Path=/; Max-Age=0"},
	}})
	if got := j.Cookie("https://www.example.com/"); got != "" {
		t.Errorf("a should be deleted, got %q", got)
	}
}

func TestJarHostOnlyDefaultPath(t *testing.T) {
	j := NewJar()
	// No Domain attribute -> host-only; no Path -> default path from request dir.
	j.SetFromResponse("https://example.com/a/b/c", &Response{Header: Header{
		{"set-cookie", "x=1"},
	}})
	if got := j.Cookie("https://sub.example.com/a/b/c"); got != "" {
		t.Errorf("host-only must not leak to subdomain, got %q", got)
	}
	if got, want := j.Cookie("https://example.com/a/b/c"), "x=1"; got != want {
		t.Errorf("same host default path = %q, want %q", got, want)
	}
	if got := j.Cookie("https://example.com/a"); got != "" {
		t.Errorf("default path /a/b should not match /a, got %q", got)
	}
}
