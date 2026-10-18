//go:build !(windows && amd64)

package native

import "context"

// Req is a placeholder on unsupported platforms; it is never constructed.
type Req struct{}

func Init(configJSON string) error { return ErrUnsupportedPlatform }

func Version() string { return "" }

func NewClient(configJSON string) (uintptr, error) { return 0, ErrUnsupportedPlatform }

func SetProxyAuth(client uintptr, user, pass string) error { return ErrUnsupportedPlatform }

func CloseClient(client uintptr) {}

func Start(client uintptr, opts StartOpts) (*Req, error) { return nil, ErrUnsupportedPlatform }

func (r *Req) Headers(ctx context.Context) (status int, proto string, fields []HeaderField, connJSON string, err error) {
	return 0, "", nil, "", ErrUnsupportedPlatform
}

func (r *Req) Read(ctx context.Context, p []byte) (int, error) { return 0, ErrUnsupportedPlatform }

func (r *Req) Write(p []byte) error { return ErrUnsupportedPlatform }

func (r *Req) Finish() error { return ErrUnsupportedPlatform }

func (r *Req) Cancel() {}

func (r *Req) Free() {}
