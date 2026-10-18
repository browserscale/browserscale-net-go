package browserscalenet

import (
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// Cookie is a stored cookie in the neutral form the jar keeps. It carries just
// enough RFC 6265 state to decide when the cookie is sent and in what order.
type Cookie struct {
	Name  string
	Value string

	// Domain is the cookie's domain scope without a leading dot. With HostOnly
	// the cookie is sent only to exactly this host; otherwise it is sent to the
	// host and its subdomains.
	Domain   string
	HostOnly bool

	// Path is the cookie's path scope (defaults to "/").
	Path string

	Secure   bool
	HttpOnly bool

	// SameSite is informational ("", "Lax", "Strict", "None"); it is not
	// enforced when choosing cookies to send, since there is no document context
	// to compare against.
	SameSite string

	// Expires is the absolute expiry; the zero value means a session cookie that
	// never expires on its own.
	Expires time.Time
}

// Jar is an opt-in cookie store for continuing a session request-by-request. It
// is deliberately explicit: the engine never reads or writes cookies on its
// own, so you call Cookie to get the header value for a request and
// SetFromResponse to fold in any set-cookie headers afterwards. This keeps the
// cookie header's position and the decision to send it in your hands.
//
// A Jar is safe for concurrent use.
type Jar struct {
	mu      sync.Mutex
	seq     uint64
	entries []jarEntry
}

type jarEntry struct {
	c       Cookie
	created time.Time
	seq     uint64
}

// NewJar returns an empty jar.
func NewJar() *Jar { return &Jar{} }

// Set inserts or replaces cookies by their (name, domain, path) identity. Use
// it to seed the jar, e.g. from a browser session's exported cookies.
func (j *Jar) Set(cookies ...Cookie) {
	j.mu.Lock()
	defer j.mu.Unlock()
	now := time.Now()
	for _, c := range cookies {
		if c.Path == "" {
			c.Path = "/"
		}
		c.Domain = strings.TrimPrefix(c.Domain, ".")
		j.upsert(c, now)
	}
}

// All returns a snapshot of every stored cookie, for export or inspection.
func (j *Jar) All() []Cookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]Cookie, len(j.entries))
	for i, e := range j.entries {
		out[i] = e.c
	}
	return out
}

// Cookie returns the value for a Cookie request header for reqURL: every stored
// cookie that domain-, path-, secure- and expiry-matches, in the order a
// browser sends them (longest path first, then oldest first). It returns ""
// when nothing matches, so callers can skip the header entirely.
func (j *Jar) Cookie(reqURL string) string {
	u, err := url.Parse(reqURL)
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	reqPath := u.Path
	if reqPath == "" {
		reqPath = "/"
	}
	https := u.Scheme == "https"
	now := time.Now()

	j.mu.Lock()
	defer j.mu.Unlock()

	matches := make([]jarEntry, 0, len(j.entries))
	for _, e := range j.entries {
		if !e.c.Expires.IsZero() && !e.c.Expires.After(now) {
			continue
		}
		if e.c.Secure && !https {
			continue
		}
		if !domainMatch(host, e.c.Domain, e.c.HostOnly) {
			continue
		}
		if !pathMatch(reqPath, e.c.Path) {
			continue
		}
		matches = append(matches, e)
	}
	sort.SliceStable(matches, func(a, b int) bool {
		la, lb := len(matches[a].c.Path), len(matches[b].c.Path)
		if la != lb {
			return la > lb
		}
		return matches[a].seq < matches[b].seq
	})

	var sb strings.Builder
	for _, e := range matches {
		if sb.Len() > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(e.c.Name)
		sb.WriteByte('=')
		sb.WriteString(e.c.Value)
	}
	return sb.String()
}

// SetFromResponse folds the set-cookie headers of resp into the jar, scoped to
// reqURL (the URL the response came from). Cookies with an elapsed expiry or a
// non-positive max-age are removed. Call it after each request to keep the jar
// current.
func (j *Jar) SetFromResponse(reqURL string, resp *Response) {
	if resp == nil {
		return
	}
	u, err := url.Parse(reqURL)
	if err != nil || u.Host == "" {
		return
	}
	var setCookies []string
	for _, f := range resp.Header {
		if strings.EqualFold(f[0], "set-cookie") {
			setCookies = append(setCookies, f[1])
		}
	}
	if len(setCookies) == 0 {
		return
	}
	parsed := (&http.Response{Header: http.Header{"Set-Cookie": setCookies}}).Cookies()

	host := strings.ToLower(u.Hostname())
	defPath := defaultPath(u.Path)
	now := time.Now()

	j.mu.Lock()
	defer j.mu.Unlock()
	for _, hc := range parsed {
		c := Cookie{
			Name:     hc.Name,
			Value:    hc.Value,
			Secure:   hc.Secure,
			HttpOnly: hc.HttpOnly,
			SameSite: sameSiteName(hc.SameSite),
		}
		if hc.Domain != "" {
			dom := strings.TrimPrefix(strings.ToLower(hc.Domain), ".")
			// A cookie may only set a domain that covers the responding host.
			if host != dom && !strings.HasSuffix(host, "."+dom) {
				continue
			}
			c.Domain = dom
			c.HostOnly = false
		} else {
			c.Domain = host
			c.HostOnly = true
		}
		if strings.HasPrefix(hc.Path, "/") {
			c.Path = hc.Path
		} else {
			c.Path = defPath
		}

		deleted := false
		switch {
		case hc.MaxAge < 0:
			deleted = true
		case hc.MaxAge > 0:
			c.Expires = now.Add(time.Duration(hc.MaxAge) * time.Second)
		case !hc.Expires.IsZero():
			c.Expires = hc.Expires
			if !c.Expires.After(now) {
				deleted = true
			}
		}

		if deleted {
			j.remove(c.Name, c.Domain, c.Path)
			continue
		}
		j.upsert(c, now)
	}
}

// upsert / remove assume j.mu is held.

func (j *Jar) upsert(c Cookie, now time.Time) {
	for i := range j.entries {
		e := &j.entries[i]
		if e.c.Name == c.Name && e.c.Domain == c.Domain && e.c.Path == c.Path {
			created := e.created // preserve creation time for ordering
			e.c = c
			e.created = created
			return
		}
	}
	j.seq++
	j.entries = append(j.entries, jarEntry{c: c, created: now, seq: j.seq})
}

func (j *Jar) remove(name, domain, path string) {
	for i := range j.entries {
		e := j.entries[i]
		if e.c.Name == name && e.c.Domain == domain && e.c.Path == path {
			j.entries = append(j.entries[:i], j.entries[i+1:]...)
			return
		}
	}
}

func domainMatch(host, cookieDomain string, hostOnly bool) bool {
	if cookieDomain == "" {
		return false
	}
	if host == cookieDomain {
		return true
	}
	if hostOnly {
		return false
	}
	return strings.HasSuffix(host, "."+cookieDomain)
}

func pathMatch(reqPath, cookiePath string) bool {
	if cookiePath == reqPath {
		return true
	}
	if !strings.HasPrefix(reqPath, cookiePath) {
		return false
	}
	if strings.HasSuffix(cookiePath, "/") {
		return true
	}
	return len(reqPath) > len(cookiePath) && reqPath[len(cookiePath)] == '/'
}

func defaultPath(reqPath string) string {
	if reqPath == "" || !strings.HasPrefix(reqPath, "/") {
		return "/"
	}
	i := strings.LastIndex(reqPath, "/")
	if i <= 0 {
		return "/"
	}
	return reqPath[:i]
}

func sameSiteName(s http.SameSite) string {
	switch s {
	case http.SameSiteLaxMode:
		return "Lax"
	case http.SameSiteStrictMode:
		return "Strict"
	case http.SameSiteNoneMode:
		return "None"
	default:
		return ""
	}
}
