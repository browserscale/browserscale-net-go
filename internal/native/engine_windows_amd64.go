//go:build windows && amd64

package native

import (
	_ "embed"
	"syscall"
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
