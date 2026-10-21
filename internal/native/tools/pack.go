//go:build ignore

// Command pack compresses a freshly built wrc_net library into the DEFLATE
// blob the native package embeds for its platform. Run it from the module
// root:
//
//	go run ./internal/native/tools/pack.go [-target windows_amd64|darwin_arm64] [path-to-library]
//
// The target defaults to windows_amd64; with no path it reads the in-tree
// Chromium build output for that target.
package main

import (
	"compress/flate"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type target struct {
	// build is the library in the in-tree Chromium build output.
	build string
	out   string
}

var targets = map[string]target{
	"windows_amd64": {build: `../_chromium/src/out/WrcNet/wrc_net.dll`, out: `internal/native/wrc_net_windows_amd64.dll.flate`},
	"darwin_arm64":  {build: `../_chromium/src/out/WrcNetMac/libwrc_net.dylib`, out: `internal/native/wrc_net_darwin_arm64.dylib.flate`},
}

func main() {
	name := flag.String("target", "windows_amd64", "windows_amd64 or darwin_arm64")
	flag.Parse()
	t, ok := targets[*name]
	if !ok {
		fatal(fmt.Errorf("unknown target %q", *name))
	}
	src := t.build
	if flag.NArg() > 0 {
		src = flag.Arg(0)
	}

	raw, err := os.ReadFile(src)
	if err != nil {
		fatal(err)
	}
	sum := sha256.Sum256(raw)

	if err := os.MkdirAll(filepath.Dir(t.out), 0o755); err != nil {
		fatal(err)
	}
	f, err := os.Create(t.out)
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

	info, _ := os.Stat(t.out)
	fmt.Printf("packed %s\n  src    %d bytes\n  blob   %d bytes\n  sha256 %s\n",
		t.out, len(raw), info.Size(), hex.EncodeToString(sum[:]))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "pack:", err)
	os.Exit(1)
}
