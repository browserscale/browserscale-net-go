//go:build darwin && arm64

package native

import (
	_ "embed"

	"github.com/ebitengine/purego"
)

//go:embed wrc_net_darwin_arm64.dylib.flate
var libCompressed []byte

const libName = "libwrc_net.dylib"

// purego loads the dylib without cgo, so the package still builds with
// CGO_ENABLED=0 and cross-compiles from other platforms.
func openLibrary(path string) (func(name string) (uintptr, error), error) {
	lib, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	return func(name string) (uintptr, error) {
		return purego.Dlsym(lib, name)
	}, nil
}

func call(fn uintptr, args ...uintptr) uintptr {
	r, _, _ := purego.SyscallN(fn, args...)
	return r
}
