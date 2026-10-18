// Package browserscalenet is an HTTP client that puts Chromium's own network
// stack on the wire: the same TLS ClientHello, the same HTTP/2 framing, the
// same connection handling as the browserscale browser, so a request is
// indistinguishable from one the browser made.
//
// Unlike [net/http], nothing is added behind your back. The method, the header
// fields in the exact order and case you give them, and the body go out as-is.
// Cookies, redirects, content decoding and caching are yours to handle (the
// chrome subpackage has helpers for the common cases). What the library owns is
// everything below the request: TLS, HTTP/2, pooling, proxies and DNS.
//
// The engine is a native library (wrc_net) built from the same Chromium tree as
// the browser. It is loaded at run time; today only windows/amd64 ships a
// binary, and other platforms return [ErrUnsupportedPlatform].
//
// A [Client] is one browser profile: its own connection pool, TLS session
// cache, proxy and auth state. Create one per identity.
//
//	c, err := browserscalenet.New(browserscalenet.Options{
//	    Proxy: "http://user:pass@gw.example:7000",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer c.Close()
//
//	resp, err := c.Do(ctx, &browserscalenet.Request{
//	    Method: "GET",
//	    URL:    "https://example.com/",
//	    Header: browserscalenet.Header{
//	        {"user-agent", ua},
//	        {"accept", "*/*"},
//	        {"accept-encoding", "identity"},
//	    },
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer resp.Body.Close()
package browserscalenet
