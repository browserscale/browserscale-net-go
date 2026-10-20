package browserscalenet

import (
	"io"
	"strings"
)

// Field is a single header as a {name, value} pair. It is an array rather than
// a struct so that ordered literals like Header{{"user-agent", ua}} stay
// concise and pass `go vet` without keyed fields.
type Field [2]string

// Name returns the field name.
func (f Field) Name() string { return f[0] }

// Value returns the field value.
func (f Field) Value() string { return f[1] }

// Header is an ordered list of header fields. Order and case are preserved
// exactly as given; nothing is sorted, merged, lower-cased or auto-added. This
// is deliberately a slice, not a map, because on the wire order is part of the
// fingerprint.
type Header []Field

// Add appends a field, returning the extended header for chaining.
func (h Header) Add(name, value string) Header {
	return append(h, Field{name, value})
}

// Get returns the value of the first field whose name matches, and whether it
// was found. Names compare case-insensitively, as HTTP defines them: HTTP/2
// delivers response headers lower-cased, so Get("Content-Type") finds
// "content-type".
func (h Header) Get(name string) (string, bool) {
	for _, f := range h {
		if strings.EqualFold(f[0], name) {
			return f[1], true
		}
	}
	return "", false
}

// Values returns the values of every field whose name matches
// case-insensitively, in order — e.g. all set-cookie lines of a response.
func (h Header) Values(name string) []string {
	var out []string
	for _, f := range h {
		if strings.EqualFold(f[0], name) {
			out = append(out, f[1])
		}
	}
	return out
}

// Request is a single HTTP request. The engine sends Method, URL and Header
// as-is, so include everything the origin should see (user-agent, accept, …)
// in Header, in the order you want them on the wire. The only headers added are
// the transport ones the browser's stack always sets: Host (HTTP/2 uses
// :authority from the URL instead), Connection and Content-Length or
// Transfer-Encoding for a body, placed first as the browser places them. A
// field of the same name in Header replaces them, at your position.
type Request struct {
	// Method is the HTTP method, e.g. "GET" or "POST".
	Method string

	// URL is the absolute request URL (https or http).
	URL string

	// Header is the ordered list of request headers.
	Header Header

	// Body is the optional request body. It is read fully before the request is
	// sent, and Content-Length is set from its size.
	Body io.Reader

	// Partition isolates the connection pool and TLS session cache under a key
	// (typically the top-level site). Requests with different partitions never
	// share a connection. Empty means the default partition.
	Partition string

	// Credentials, when true, enables credentialed connection reuse for this
	// request's partition. The default (false) mirrors a privacy-mode fetch.
	Credentials bool

	// Priority sets the HTTP/2 stream priority (0=lowest … 4=highest). Only
	// applied when PrioritySet is true.
	Priority    int
	PrioritySet bool

	// Incremental marks the HTTP/2 stream as incrementally deliverable.
	Incremental bool
}
