package browserscalenet

import (
	"context"
	"encoding/json"

	"github.com/browserscale/browserscale-net-go/internal/native"
)

// Response is the result of a request. Header is in the exact order the origin
// sent it. Body streams the raw response bytes — no automatic gzip/brotli/zstd
// decoding is done, so if you sent accept-encoding you must decode yourself.
type Response struct {
	// Status is the HTTP status code.
	Status int

	// Proto is the negotiated protocol as reported by the engine: "h2",
	// "http/1.1", or "h3".
	Proto string

	// Header is the ordered list of response headers.
	Header Header

	// Conn describes the transport that served the response.
	Conn ConnInfo

	// Body is the response body. Always close it to release the connection back
	// to the pool (or to cancel an in-flight request).
	Body *Body
}

// ConnInfo is the transport-level detail for a response: what was negotiated on
// the connection that served it. The numeric fields carry raw IANA/Chromium
// identifiers; use the helper methods for readable forms.
type ConnInfo struct {
	// Protocol is Chromium's connection-info string, e.g. "h2" or "http/1.1".
	Protocol string `json:"protocol"`
	// ALPN is the ALPN-negotiated protocol.
	ALPN string `json:"alpn"`
	// Remote is the peer address that served the response. For HTTPS over a
	// proxy this is the proxy's address, since TLS is tunnelled end to end.
	Remote string `json:"remote"`
	// TLSVersion is Chromium's SSLVersion enum (6 = TLS 1.3, 5 = TLS 1.2, …).
	TLSVersion int `json:"tls_version"`
	// CipherSuite is the IANA TLS cipher suite id (e.g. 4865 = TLS_AES_128_GCM_SHA256).
	CipherSuite int `json:"cipher"`
	// Group is the IANA named group of the key exchange (e.g. 4588 = X25519MLKEM768).
	Group int `json:"group"`
	// SignatureAlgorithm is the IANA signature scheme the peer used.
	SignatureAlgorithm int `json:"sigalg"`
	// ECH reports whether Encrypted ClientHello was accepted.
	ECH bool `json:"ech"`
	// Resumed reports whether the TLS session was resumed.
	Resumed bool `json:"resumed"`
}

// TLS version enum values as reported by Chromium (net::SSLVersion).
const (
	tlsVersionTLS1   = 3
	tlsVersionTLS1_1 = 4
	tlsVersionTLS1_2 = 5
	tlsVersionTLS1_3 = 6
	tlsVersionQUIC   = 7
)

// TLSVersionName returns a human-readable TLS version, e.g. "TLS 1.3".
func (ci ConnInfo) TLSVersionName() string {
	switch ci.TLSVersion {
	case tlsVersionTLS1:
		return "TLS 1.0"
	case tlsVersionTLS1_1:
		return "TLS 1.1"
	case tlsVersionTLS1_2:
		return "TLS 1.2"
	case tlsVersionTLS1_3:
		return "TLS 1.3"
	case tlsVersionQUIC:
		return "QUIC"
	default:
		return "unknown"
	}
}

// Body is the streaming response body. It reads raw bytes from the engine and,
// on Close, cancels any in-flight transfer and frees the underlying request.
type Body struct {
	ctx context.Context
	req *native.Req
}

// Read implements io.Reader. It blocks until data is available, EOF, the
// request context is cancelled, or the transfer fails.
func (b *Body) Read(p []byte) (int, error) {
	return b.req.Read(b.ctx, p)
}

// Close cancels the transfer if still running and releases engine resources.
// It is safe to call more than once.
func (b *Body) Close() error {
	if b.req != nil {
		b.req.Cancel()
		b.req.Free()
		b.req = nil
	}
	return nil
}

func parseConn(connJSON string) ConnInfo {
	var ci ConnInfo
	if connJSON != "" {
		_ = json.Unmarshal([]byte(connJSON), &ci)
	}
	return ci
}
