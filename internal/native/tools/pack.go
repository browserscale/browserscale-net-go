//go:build ignore

// Command pack compresses the freshly built wrc_net.dll into the DEFLATE blob
// embedded by the native package. Run it from the module root:
//
//	go run ./internal/native/tools/pack.go [path-to-wrc_net.dll]
//
// With no argument it defaults to the in-tree Chromium build output.
package main

import (
	"compress/flate"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultDLL = `..\_chromium\src\out\WrcNet\wrc_net.dll`
	outPath    = `internal/native/wrc_net_windows_amd64.dll.flate`
)

func main() {
	src := defaultDLL
	if len(os.Args) > 1 {
		src = os.Args[1]
	}

	raw, err := os.ReadFile(src)
	if err != nil {
		fatal(err)
	}
	sum := sha256.Sum256(raw)

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fatal(err)
	}
	f, err := os.Create(outPath)
	if err != nil {
		fatal(err)
	}
	zw, err := flate.NewWriter(f, flate.BestCompression)
	if err != nil {
		fatal(err)
	}
	if _, err := zw.Write(raw); err != nil {
		fatal(err)
	}
	if err := zw.Close(); err != nil {
		fatal(err)
	}
	if err := f.Close(); err != nil {
		fatal(err)
	}

	info, _ := os.Stat(outPath)
	fmt.Printf("packed %s\n  src    %d bytes\n  blob   %d bytes\n  sha256 %s\n",
		outPath, len(raw), info.Size(), hex.EncodeToString(sum[:]))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "pack:", err)
	os.Exit(1)
}
