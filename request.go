package browserscalenet

import "io"

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

// Get returns the value of the first field whose name matches (case-sensitive),
// and whether it was found.
func (h Header) Get(name string) (string, bool) {
	for _, f := range h {
		if f[0] == name {
			return f[1], true
		}
	}
	return "", false
}

// Request is a single HTTP request. The engine sends Method, URL and Header
// as-is; it never adds headers of its own, so include everything the origin
// should see (user-agent, accept, content-length for a body, …) in Header, in
// the order you want them on the wire.
type Request struct {
	// Method is the HTTP method, e.g. "GET" or "POST".
	Method string

	// URL is the absolute request URL (https or http).
	URL string

	// Header is the ordered list of request headers.
	Header Header

	// Body is the optional request body. It is read fully before the request is
	// sent; set a matching content-length header yourself if the origin needs
	// one.
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
