//go:build windows && amd64

package native

import (
	_ "embed"
	"syscall"
	"unsafe"
)

//go:embed wrc_net_windows_amd64.dll.flate
var libCompressed []byte

const libName = "wrc_net.dll"

func openLibrary(path string) (func(name string) (uintptr, error), error) {
	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return nil, err
	}
	return func(name string) (uintptr, error) {
		proc, err := dll.FindProc(name)
		if err != nil {
			return 0, err
		}
		return proc.Addr(), nil
	}, nil
}

func call(fn uintptr, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(fn, args...)
	return r
}

// The Windows x64 ABI passes a struct larger than 8 bytes as a pointer to a
// copy, so each wrc_bytes argument is one pointer.
func setProxyAuth(client uintptr, user, pass wrcBytes) uintptr {
	rc, _, _ := p.setProxyAuth.Call(client,
		uintptr(unsafe.Pointer(&user)),
		uintptr(unsafe.Pointer(&pass)))
	return rc
}
