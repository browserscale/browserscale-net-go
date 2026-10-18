// Package native is the thin binding to the wrc_net C library. It hides the
// DLL loading, the C ABI structs, and the single poll loop behind a small Go
// surface. Everything here is internal; the public package wraps it.
package native

import "errors"

// ErrUnsupportedPlatform is returned by [Init] on any platform for which no
// wrc_net binary is embedded.
var ErrUnsupportedPlatform = errors.New("browserscale-net: no native engine for this OS/arch (windows/amd64 only for now)")

// HeaderField is one request or response header, kept as raw strings so order
// and case survive the round trip.
type HeaderField struct {
	Name  string
	Value string
}

// StartOpts describes a single request at the C ABI boundary.
type StartOpts struct {
	Method      string
	URL         string
	Fields      []HeaderField
	Body        []byte
	Stream      bool
	Priority    int
	PrioritySet bool
	Incremental bool
	Partition   string
	Credentials bool
}

// NetError is a Chromium net error surfaced from the engine.
type NetError struct {
	Code int
	Name string
}

func (e *NetError) Error() string {
	if e.Name != "" {
		return "browserscale-net: " + e.Name
	}
	return "browserscale-net: net error " + itoa(e.Code)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
