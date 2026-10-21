package browserscalenet

import (
	"errors"

	"github.com/browserscale/browserscale-net-go/internal/native"
)

// ErrUnsupportedPlatform is returned when no native engine is available for the
// current OS/arch. windows/amd64 and darwin/arm64 ship a binary today.
var ErrUnsupportedPlatform = native.ErrUnsupportedPlatform

// NetError is a Chromium net error (negative code plus its symbolic name, e.g.
// ERR_CONNECTION_RESET). Compare against the exported code constants or use the
// classification helpers below.
type NetError = native.NetError

// A small, stable subset of Chromium net error codes, enough to tell a proxy
// failure apart from an origin failure. The full list lives in Chromium's
// net_error_list.h; values here match it.
const (
	ErrConnectionReset        = -101
	ErrConnectionRefused      = -102
	ErrConnectionAborted      = -103
	ErrConnectionFailed       = -104
	ErrNameNotResolved        = -105
	ErrConnectionTimedOut     = -118
	ErrTunnelConnectionFailed = -303
	ErrProxyConnectionFailed  = -130
	ErrProxyAuthRequested     = -115
	ErrProxyAuthUnsupported   = -341
	ErrTimedOut               = -7
)

// IsProxyError reports whether err is a net error that points at the proxy
// rather than the origin. Useful for marking a proxy bad and retrying on
// another.
func IsProxyError(err error) bool {
	var ne *NetError
	if !errors.As(err, &ne) {
		return false
	}
	switch ne.Code {
	case ErrProxyConnectionFailed,
		ErrTunnelConnectionFailed,
		ErrProxyAuthRequested,
		ErrProxyAuthUnsupported:
		return true
	default:
		return false
	}
}

// Code returns the Chromium net error code if err is a NetError, or 0.
func Code(err error) int {
	var ne *NetError
	if errors.As(err, &ne) {
		return ne.Code
	}
	return 0
}
