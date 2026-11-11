package browserscalenet

import "strings"

// AUTO is a sentinel header value. Put it as the value of a header you want the
// client to fill for you, and Do resolves it in Go before the request reaches
// the engine:
//
//   - "cookie" is filled from the client's Jar for the request URL.
//   - any other header is filled from the client's Defaults, matched by name
//     case-insensitively (so {"User-Agent", AUTO} and {"user-agent", AUTO} both
//     resolve, whether you write HTTP/1.1-style or HTTP/2-style casing).
//   - a header Defaults has no value for is filled with what Google Chrome of
//     the engine's version sends on Windows, whatever the client runs on:
//     user-agent, sec-ch-ua, sec-ch-ua-mobile, sec-ch-ua-platform,
//     accept-language ("en-US,en;q=0.9") and accept-encoding. As in Chrome,
//     the three sec-ch-ua headers are left out and accept-encoding is
//     "gzip, deflate" on plain HTTP to another machine. So a client made with
//     nothing but New presents itself as the Chrome its TLS and HTTP/2 come
//     from, and one made by FromCloudBrowser as the session's browser.
//
// AUTO only ever replaces a value: the field's name, case and position are left
// exactly as you wrote them, so you decide where on the wire it sits. If there
// is nothing to fill it with, the field is dropped entirely, so the wire never
// carries a literal AUTO or an empty header. That is the case for headers that
// depend on what kind of request it is, such as accept or the sec-fetch family.
//
// Its value is an opaque, control-character-wrapped string you are not meant to
// type by hand — always use the constant.
const AUTO = "\x00bsnet/auto\x00"

// resolveAuto returns h with every AUTO-valued field replaced from the jar (for
// cookie), from defaults, or else from the engine's identity, dropping fields
// it cannot fill. Non-AUTO fields pass through untouched, preserving their
// exact name and case. It allocates a new slice only when h actually contains
// an AUTO field.
func resolveAuto(h Header, jar *Jar, defaults map[string]string, engine *engineIdentity, reqURL string) Header {
	hasAuto, hasRange := false, false
	for _, f := range h {
		if f[1] == AUTO {
			hasAuto = true
		} else if strings.EqualFold(f[0], "range") {
			hasRange = true
		}
	}
	if !hasAuto {
		return h
	}

	out := make(Header, 0, len(h))
	for _, f := range h {
		if f[1] != AUTO {
			out = append(out, f)
			continue
		}
		var v string
		if name := strings.ToLower(f[0]); name == "cookie" {
			if jar != nil {
				v = jar.Cookie(reqURL)
			}
		} else if v = defaults[name]; v == "" {
			v = engine.value(name, reqURL, hasRange)
		}
		if v != "" {
			out = append(out, Field{f[0], v}) // keep the name's original case
		}
	}
	return out
}

// buildDefaults turns a Header of default values into a case-insensitive lookup
// keyed by lower-cased name. Later fields win on a duplicate name.
func buildDefaults(h Header) map[string]string {
	if len(h) == 0 {
		return nil
	}
	m := make(map[string]string, len(h))
	for _, f := range h {
		m[strings.ToLower(f[0])] = f[1]
	}
	return m
}
