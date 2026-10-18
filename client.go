package browserscalenet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/browserscale/browserscale-net-go/internal/native"
)

// Options configures a Client. A Client is one browser-like profile: its own
// connection pool, TLS session cache, proxy, and auth state.
type Options struct {
	// Proxy routes all traffic through an HTTP or SOCKS proxy, e.g.
	// "http://user:pass@gw.example:7000" or "socks5://host:1080". Credentials in
	// the URL are sent preemptively. Empty means a direct connection.
	Proxy string

	// ConnectUserAgent sets the User-Agent header on HTTP CONNECT tunnels to an
	// HTTPS proxy. It does not affect request headers.
	ConnectUserAgent string

	// InsecureSkipVerify disables TLS certificate verification. Off by default,
	// so the engine verifies like the browser does.
	InsecureSkipVerify bool

	// DisableECH turns off Encrypted ClientHello. By default ECH is on, matching
	// the browser (it sends an ECH GREASE extension even when the origin does
	// not advertise ECH). Turning it off changes the TLS fingerprint, so leave
	// it at the default unless you have a reason not to.
	DisableECH bool

	// H2 optionally overrides HTTP/2 settings. Leave nil for the browser's
	// defaults (recommended; changing these alters the fingerprint).
	H2 map[string]any
}

// InitConfig is one-time, process-wide engine configuration. It takes effect on
// the first Client created in the process; later values are ignored.
type InitConfig struct {
	// NetLogPath, if set, writes a Chromium NetLog JSON file for debugging.
	NetLogPath string `json:"netlog,omitempty"`
}

// Client issues requests through the native engine. It is safe for concurrent
// use by multiple goroutines.
type Client struct {
	h uintptr
}

type clientConfig struct {
	Proxy            string         `json:"proxy,omitempty"`
	ConnectUserAgent string         `json:"connect_user_agent,omitempty"`
	TLS              tlsConfig      `json:"tls"`
	H2               map[string]any `json:"h2,omitempty"`
}

type tlsConfig struct {
	Verify bool `json:"verify"`
	ECH    bool `json:"ech"`
}

// New creates a Client. It loads the native engine on first use and returns
// ErrUnsupportedPlatform if none is available for this OS/arch.
func New(opts Options) (*Client, error) {
	return NewWithInit(opts, InitConfig{})
}

// NewWithInit is like New but also carries one-time engine configuration that
// applies to the first Client created in the process.
func NewWithInit(opts Options, init InitConfig) (*Client, error) {
	initJSON, err := json.Marshal(init)
	if err != nil {
		return nil, err
	}
	if err := native.Init(string(initJSON)); err != nil {
		return nil, err
	}

	cfg := clientConfig{
		ConnectUserAgent: opts.ConnectUserAgent,
		TLS:              tlsConfig{Verify: !opts.InsecureSkipVerify, ECH: !opts.DisableECH},
		H2:               opts.H2,
	}

	var proxyUser, proxyPass string
	var hasProxyAuth bool
	if opts.Proxy != "" {
		u, err := url.Parse(opts.Proxy)
		if err != nil {
			return nil, fmt.Errorf("browserscale-net: invalid proxy URL: %w", err)
		}
		if u.User != nil {
			proxyUser = u.User.Username()
			proxyPass, _ = u.User.Password()
			hasProxyAuth = proxyUser != "" || proxyPass != ""
			u.User = nil
		}
		cfg.Proxy = u.String()
	}

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	h, err := native.NewClient(string(cfgJSON))
	if err != nil {
		return nil, err
	}
	if hasProxyAuth {
		if err := native.SetProxyAuth(h, proxyUser, proxyPass); err != nil {
			native.CloseClient(h)
			return nil, err
		}
	}
	return &Client{h: h}, nil
}

// Close releases the client's connections and resources. The client must not be
// used afterwards.
func (c *Client) Close() {
	if c.h != 0 {
		native.CloseClient(c.h)
		c.h = 0
	}
}

// Do sends req and returns once the response headers have arrived. The response
// body is streamed; always close resp.Body. The context governs the whole
// exchange, including body reads.
func (c *Client) Do(ctx context.Context, req *Request) (*Response, error) {
	if c.h == 0 {
		return nil, fmt.Errorf("browserscale-net: client is closed")
	}
	method := req.Method
	if method == "" {
		method = "GET"
	}

	var body []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("browserscale-net: reading request body: %w", err)
		}
		body = b
	}

	opts := native.StartOpts{
		Method:      method,
		URL:         req.URL,
		Fields:      toNativeFields(req.Header),
		Body:        body,
		Priority:    req.Priority,
		PrioritySet: req.PrioritySet,
		Incremental: req.Incremental,
		Partition:   req.Partition,
		Credentials: req.Credentials,
	}

	r, err := native.Start(c.h, opts)
	if err != nil {
		return nil, err
	}

	status, proto, fields, connJSON, err := r.Headers(ctx)
	if err != nil {
		r.Free()
		return nil, err
	}

	return &Response{
		Status: status,
		Proto:  proto,
		Header: fromNativeFields(fields),
		Conn:   parseConn(connJSON),
		Body:   &Body{ctx: ctx, req: r},
	}, nil
}

func toNativeFields(h Header) []native.HeaderField {
	if len(h) == 0 {
		return nil
	}
	out := make([]native.HeaderField, len(h))
	for i, f := range h {
		out[i] = native.HeaderField{Name: f[0], Value: f[1]}
	}
	return out
}

func fromNativeFields(f []native.HeaderField) Header {
	if len(f) == 0 {
		return nil
	}
	out := make(Header, len(f))
	for i, nf := range f {
		out[i] = Field{nf.Name, nf.Value}
	}
	return out
}
