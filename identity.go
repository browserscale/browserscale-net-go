package browserscalenet

import (
	"encoding/json"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/browserscale/browserscale-net-go/internal/native"
)

// engineIdentity is what Google Chrome of the engine's version on Windows says
// about itself in its request headers. The engine is the Windows browser on
// every platform it is built for, so the identity is the Windows one wherever
// the client runs. AUTO falls back to it for a header that Defaults has no
// value for, so a client without a cloud browser behind it still looks like
// the Chrome its TLS and HTTP/2 come from.
type engineIdentity struct {
	fixed map[string]string // by lower-cased header name
}

// newEngineIdentity builds the identity for a Chromium version such as
// "157.0.8103.39". It returns nil for a version it cannot read.
func newEngineIdentity(chromium string) *engineIdentity {
	majorStr, _, _ := strings.Cut(chromium, ".")
	major, err := strconv.Atoi(majorStr)
	if err != nil || major <= 0 {
		return nil
	}
	// Chrome reports a fixed platform and only its major version in the
	// User-Agent; the rest is the same on every Windows machine.
	return &engineIdentity{fixed: map[string]string{
		"user-agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" +
			majorStr + ".0.0.0 Safari/537.36",
		"sec-ch-ua":          chromeBrands(major),
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"accept-language":    "en-US,en;q=0.9",
	}}
}

// value is the header value Chrome sends for name on a request to reqURL, or
// "" for a header that is not part of its identity. hasRange says whether the
// request carries a range header.
func (e *engineIdentity) value(name, reqURL string, hasRange bool) string {
	if e == nil {
		return ""
	}
	switch name {
	case "accept-encoding":
		return chromeAcceptEncoding(reqURL, hasRange)
	case "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform":
		// Chrome keeps its client hints to itself on plain HTTP.
		if !secureOrLocal(reqURL) {
			return ""
		}
	}
	return e.fixed[name]
}

// chromeBrands is the sec-ch-ua value of Google Chrome with the given major
// version: the browser's two brands and a made-up third one, whose name,
// version and position Chrome derives from the major version so that they
// change with every release.
func chromeBrands(major int) string {
	greaseChars := []string{" ", "(", ":", "-", ".", "/", ")", ";", "=", "?", "_"}
	greaseVersions := []string{"8", "99", "24"}
	orders := [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}

	version := strconv.Itoa(major)
	brands := [3][2]string{
		{"Not" + greaseChars[major%len(greaseChars)] + "A" + greaseChars[(major+1)%len(greaseChars)] + "Brand",
			greaseVersions[major%len(greaseVersions)]},
		{"Chromium", version},
		{"Google Chrome", version},
	}
	var placed [3]string
	for i, at := range orders[major%len(orders)] {
		placed[at] = quoteHint(brands[i][0]) + ";v=" + quoteHint(brands[i][1])
	}
	return strings.Join(placed[:], ", ")
}

// chromeAcceptEncoding is the accept-encoding Chrome sends on its own: Brotli
// and zstd only where a proxy cannot read the body, that is over TLS or to the
// local machine, and no encoding at all next to a range header.
func chromeAcceptEncoding(reqURL string, hasRange bool) string {
	if hasRange {
		return "identity"
	}
	if secureOrLocal(reqURL) {
		return "gzip, deflate, br, zstd"
	}
	return "gzip, deflate"
}

// secureOrLocal reports whether a request to reqURL travels over TLS or stays
// on the local machine.
func secureOrLocal(reqURL string) bool {
	u, err := url.Parse(reqURL)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "https", "wss":
		return true
	}
	return isLocalhost(u.Hostname())
}

func isLocalhost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

var (
	loadedIdentityOnce sync.Once
	loadedIdentity     *engineIdentity
)

// identityOfLoadedEngine is the identity of the engine in this process. Call it
// after the engine has been loaded.
func identityOfLoadedEngine() *engineIdentity {
	loadedIdentityOnce.Do(func() {
		var v struct {
			Chromium string `json:"chromium"`
		}
		if json.Unmarshal([]byte(native.Version()), &v) == nil {
			loadedIdentity = newEngineIdentity(v.Chromium)
		}
	})
	return loadedIdentity
}
