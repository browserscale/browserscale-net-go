<div align="center">

<img src="https://browserscale.cloud/icon-512.png" width="72" height="72" alt="browserscale">

# browserscale-net-go

**Requests with the browser's own fingerprint.**

Chromium's real network stack as a Go HTTP client: the same TLS ClientHello and HTTP/2 framing as the browserscale browser, so your requests carry the same JA3/JA4 and Akamai fingerprint — indistinguishable, on the wire, from the real thing.

[![Go Reference](https://img.shields.io/badge/go.dev-reference-0a0a0a?style=flat-square&logo=go)](https://pkg.go.dev/github.com/browserscale/browserscale-net-go)
[![go](https://img.shields.io/badge/go-1.25%2B-0a0a0a?style=flat-square&logo=go)](https://go.dev)
[![platform](https://img.shields.io/badge/platform-windows%2Famd64-0a0a0a?style=flat-square&logo=windows)](#requirements)

[Docs](https://browserscale.cloud/docs) · [Quickstart](https://browserscale.cloud/docs/quickstart) · [Discord](https://discord.gg/SfE9C9K28D)

</div>

---

```bash
go get github.com/browserscale/browserscale-net-go
```

```go
package main

import (
	"context"
	"io"
	"log"
	"os"

	bsnet "github.com/browserscale/browserscale-net-go"
)

func main() {
	// One client is one profile: its own connection pool, TLS cache and proxy.
	c, err := bsnet.New(bsnet.Options{
		Proxy: "http://user:pass@gw.example:7000", // optional; http:// or socks5://
	})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// Your headers go out in the exact order and case you write them.
	resp, err := c.Do(context.Background(), &bsnet.Request{
		Method: "GET",
		URL:    "https://tls.peet.ws/api/all",
		Header: bsnet.Header{
			{"sec-ch-ua", `"Chromium";v="155", "Not;A=Brand";v="24"`},
			{"sec-ch-ua-mobile", "?0"},
			{"sec-ch-ua-platform", `"Windows"`},
			{"user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36"},
			{"accept", "*/*"},
			{"accept-encoding", "gzip, deflate, br, zstd"}, // decoded for you
			{"accept-language", "en-US,en;q=0.9"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	ct, _ := resp.Header.Get("Content-Type") // names match case-insensitively
	log.Printf("content-type %s", ct)
	log.Printf("%d %s over %s (%s)", resp.Status, resp.Proto, resp.Conn.Protocol, resp.Conn.TLSVersionName())
	io.Copy(os.Stdout, resp.Body)
}
```

The engine ships inside the package — nothing to install, no cgo. Go 1.25+.

## What you won't find elsewhere

- **Chromium's real fingerprint, not an impression of it.** The bytes come from Chromium's own network stack, so the TLS ClientHello and HTTP/2 framing are the browser's — same JA3/JA4, same Akamai fingerprint — and they track the browser as it updates.
- **Nothing added behind your back.** Your method, your header fields in the exact order and case you write them, your body — on the wire as-is. No headers invented, no reordering; only the transport headers the browser's stack always sets (`Host`, `Connection`, `Content-Length`) go first, exactly where Chrome puts them. What the library owns is everything *below* the request: TLS, HTTP/2, pooling, proxies and DNS.
- **Bodies decoded like the browser decodes them.** Send Chrome's `accept-encoding: gzip, deflate, br, zstd` and read plain bytes back — gzip, deflate, Brotli and zstd are unpacked by Chromium's own decoders. Need the raw bytes? Set `DisableDecompression`.
- **Continue a real browser session.** Hand it a browserscale cloud browser and get a request client on the same proxy exit, with the same cookies, User-Agent and client hints already loaded — cheap request traffic that picks up exactly where the browser left off.
- **Proxies that just work.** HTTP and SOCKS with preemptive auth, Encrypted ClientHello on by default (part of the fingerprint), and typed net errors that tell a proxy failure apart from an origin one so you can rotate and retry.
- **A cookie jar that behaves like a browser.** Domain, path, secure and expiry matching with browser cookie ordering — opt-in, so the `cookie` header stays exactly where you put it.

## Let the client fill a header: `AUTO`

Order is yours, but you don't have to carry the values. Give a client a cookie `Jar` and a set of `Defaults`, then drop `bsnet.AUTO` in as a header value and the client fills it in Go, right before the request goes out:

```go
jar := bsnet.NewJar()
jar.Set(/* seed the session's cookies */)

c, _ := bsnet.New(bsnet.Options{
	Proxy: proxy,
	Jar:   jar,
	Defaults: bsnet.Header{ // values AUTO resolves to, by name
		{"user-agent", ua},
		{"accept-language", "en-US,en;q=0.9"},
		{"sec-ch-ua", secCHUA},
	},
})

resp, _ := c.Do(ctx, &bsnet.Request{
	URL: url,
	Header: bsnet.Header{
		{"sec-ch-ua", bsnet.AUTO},       // from Defaults
		{"cookie", bsnet.AUTO},          // from the Jar, for this URL
		{"user-agent", bsnet.AUTO},      // from Defaults
		{"accept", "*/*"},               // or just write it
		{"accept-language", bsnet.AUTO},
	},
})
```

`cookie` is filled from the jar for the request URL; every other `AUTO` header is filled from `Defaults`, matched by name **case-insensitively** (so `"Cookie"` and `"cookie"` both work, HTTP/1.1- or HTTP/2-style). `AUTO` only ever replaces the *value* — the field's name, case and **position stay exactly where you put them**. If there's nothing to fill it with, the field is dropped, so a literal `AUTO` or an empty header never reaches the wire. With a jar attached, the client also folds each response's `set-cookie` back into it automatically — attach a jar and forget.

## Continuing a cloud-browser session

`bsnet.FromCloudBrowser` turns a live browserscale cloud browser into a request client that keeps the session going — same proxy, same cookies, same identity. It wires the jar and identity in for you, so every header can be `AUTO`:

```go
// browser is a *browserscale.CloudBrowser you already rented.
profile, err := bsnet.FromCloudBrowser(ctx, browser)
if err != nil {
	log.Fatal(err)
}
defer profile.Close()

resp, err := profile.Client.Do(ctx, &bsnet.Request{
	Method: "GET",
	URL:    url,
	Header: bsnet.Header{
		{"sec-ch-ua", bsnet.AUTO},
		{"sec-ch-ua-mobile", bsnet.AUTO},
		{"sec-ch-ua-platform", bsnet.AUTO},
		{"cookie", bsnet.AUTO},
		{"user-agent", bsnet.AUTO},
		{"accept", "*/*"},
		{"accept-language", bsnet.AUTO},
	},
})
```

One call reads the session's identity, proxy and cookies, then hands back a client wired to the same proxy with the jar preloaded and the identity as `Defaults`.

## Requirements

Windows (amd64) only for now. On any other platform `New` returns `ErrUnsupportedPlatform`; more platforms are on the way.

Get an API key in your [panel](https://browserscale.cloud/panel/account).

## Ecosystem

| | |
| --- | --- |
| [**browserscale-go**](https://github.com/browserscale/browserscale-go) | The browser automation SDK for Go |
| [**browserscale-ts**](https://github.com/browserscale/browserscale-ts) | The same SDK for TypeScript, in Node.js and the browser |
| [**browserscale studio**](https://github.com/browserscale/browserscale) | Build bots by chatting with an agent; desktop app and `browserscale` CLI |
| [**MCP server**](https://browserscale.cloud/docs/guides/agentic-coding) | The browser as tools for Cursor, Claude, Codex or any MCP client |

<div align="center">

[browserscale.cloud](https://browserscale.cloud)

</div>
