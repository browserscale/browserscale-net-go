// Package cloudbrowser bridges a live browserscale cloud browser to a
// request-based [bsnet.Client], so you can start cheap request traffic that
// continues the browser's session: same proxy exit, same cookies, same
// User-Agent and client hints.
//
// It lives in its own module on purpose. The bridge pulls in the browserscale
// Go SDK (and its gRPC dependencies), while the core browserscale-net module
// stays dependency-free — only code that actually wires a cloud browser to the
// request client pays for the SDK.
package cloudbrowser

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	browserscale "github.com/browserscale/browserscale-go"
	bsnet "github.com/browserscale/browserscale-net-go"
)

// Profile is a request client seeded from a cloud browser session, together
// with the state needed to keep requests consistent with it.
type Profile struct {
	// Client is a request client configured with the session's proxy.
	Client *bsnet.Client

	// Jar holds the session's cookies. Use Jar.Cookie(url) to get the cookie
	// header for a request and Jar.SetFromResponse after it.
	Jar *bsnet.Jar

	// Header is an ordered template of the browser's identity headers
	// (sec-ch-ua family, user-agent, accept-language). Merge it into each
	// request's header in the order you want on the wire.
	Header bsnet.Header

	// Identity is the full client identity the profile was built from.
	Identity *browserscale.ClientIdentity
}

// FromCloudBrowser builds a Profile from a running session in a single call: it
// reads the session's identity (User-Agent, client hints, Accept-Language),
// proxy and cookies, and returns a request client wired to the same proxy with
// the cookies loaded. Just hand it the CloudBrowser.
//
// The returned Client owns resources; call Profile.Close when done.
func FromCloudBrowser(ctx context.Context, b *browserscale.CloudBrowser) (*Profile, error) {
	id, err := b.GetClientIdentity(ctx, browserscale.WithCookies())
	if err != nil {
		return nil, err
	}

	jar := bsnet.NewJar()
	jar.Set(toNetCookies(id.Cookies)...)

	header := identityToHeader(id)

	// Wire the jar and identity into the client so bsnet.AUTO resolves on its own:
	// {"cookie", bsnet.AUTO} fills from the jar, {"user-agent", bsnet.AUTO} and the
	// other identity headers fill from Defaults, and responses fold back into the jar.
	client, err := bsnet.New(bsnet.Options{
		Proxy:    proxyURL(id.Proxy),
		Jar:      jar,
		Defaults: header,
	})
	if err != nil {
		return nil, err
	}

	return &Profile{
		Client:   client,
		Jar:      jar,
		Header:   header,
		Identity: id,
	}, nil
}

// Close releases the underlying request client.
func (p *Profile) Close() {
	if p.Client != nil {
		p.Client.Close()
		p.Client = nil
	}
}

// proxyURL turns the session's proxy into a URL for bsnet.Options. MASQUE is
// ignored: browserscale-net is HTTP/1.1 + HTTP/2 only for now. Empty host =>
// direct connection.
func proxyURL(p browserscale.ProxyConfig) string {
	if p.Host == "" {
		return ""
	}
	scheme := p.Scheme
	if scheme == "" {
		scheme = "http"
	}
	u := &url.URL{Scheme: scheme, Host: fmt.Sprintf("%s:%d", p.Host, p.Port)}
	if p.Username != "" || p.Password != "" {
		u.User = url.UserPassword(p.Username, p.Password)
	}
	return u.String()
}

// identityToHeader builds the low-entropy identity headers Chrome sends on every
// request, in Chrome's order. High-entropy hints (sec-ch-ua-arch,
// -full-version-list, …) are sent only when the origin asks via Accept-CH; add
// them yourself from Identity when you need them.
func identityToHeader(id *browserscale.ClientIdentity) bsnet.Header {
	var h bsnet.Header
	if s := secCHUA(id.Brands); s != "" {
		h = h.Add("sec-ch-ua", s)
	}
	h = h.Add("sec-ch-ua-mobile", boolHint(id.Mobile))
	if id.Platform != "" {
		h = h.Add("sec-ch-ua-platform", quoteHint(id.Platform))
	}
	if id.UserAgent != "" {
		h = h.Add("user-agent", id.UserAgent)
	}
	if id.AcceptLanguage != "" {
		h = h.Add("accept-language", id.AcceptLanguage)
	}
	return h
}

func secCHUA(brands []browserscale.UaBrand) string {
	parts := make([]string, 0, len(brands))
	for _, b := range brands {
		parts = append(parts, fmt.Sprintf("%s;v=%s", quoteHint(b.Brand), quoteHint(b.Version)))
	}
	return strings.Join(parts, ", ")
}

func quoteHint(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func boolHint(v bool) string {
	if v {
		return "?1"
	}
	return "?0"
}

func toNetCookies(in []browserscale.CookieParam) []bsnet.Cookie {
	out := make([]bsnet.Cookie, 0, len(in))
	for _, c := range in {
		nc := bsnet.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			HostOnly: c.Domain != "" && !strings.HasPrefix(c.Domain, "."),
			Path:     c.Path,
		}
		if c.Secure != nil {
			nc.Secure = *c.Secure
		}
		if c.HTTPOnly != nil {
			nc.HttpOnly = *c.HTTPOnly
		}
		if c.SameSite != nil {
			nc.SameSite = *c.SameSite
		}
		// CDP expiry is Unix seconds; <= 0 or absent means a session cookie.
		if c.Expires != nil && *c.Expires > 0 {
			sec := int64(*c.Expires)
			nsec := int64((*c.Expires - float64(sec)) * 1e9)
			nc.Expires = time.Unix(sec, nsec)
		}
		out = append(out, nc)
	}
	return out
}
