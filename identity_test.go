package browserscalenet

import "testing"

// The sec-ch-ua values are the ones Google Chrome of that version sends.
func TestChromeBrands(t *testing.T) {
	for major, want := range map[int]string{
		120: `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
		124: `"Chromium";v="124", "Google Chrome";v="124", "Not-A.Brand";v="99"`,
		131: `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`,
		157: `"Not-A.Brand";v="99", "Google Chrome";v="157", "Chromium";v="157"`,
	} {
		if got := chromeBrands(major); got != want {
			t.Errorf("chromeBrands(%d)\n got  %s\n want %s", major, got, want)
		}
	}
}

func TestEngineIdentity(t *testing.T) {
	const url = "https://example.com/"

	t.Run("the Windows browser, wherever the client runs", func(t *testing.T) {
		id := newEngineIdentity("157.0.8103.39")
		for name, want := range map[string]string{
			"user-agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/157.0.0.0 Safari/537.36",
			"sec-ch-ua":          `"Not-A.Brand";v="99", "Google Chrome";v="157", "Chromium";v="157"`,
			"sec-ch-ua-mobile":   "?0",
			"sec-ch-ua-platform": `"Windows"`,
			"accept-language":    "en-US,en;q=0.9",
			"accept-encoding":    "gzip, deflate, br, zstd",
			"accept":             "", // depends on the request, so it is not part of the identity
			"sec-fetch-mode":     "",
		} {
			if got := id.value(name, url, false); got != want {
				t.Errorf("%s: got %q, want %q", name, got, want)
			}
		}
	})

	t.Run("unknown version has no identity", func(t *testing.T) {
		for _, id := range []*engineIdentity{
			newEngineIdentity(""),
			newEngineIdentity("x.0.1.2"),
		} {
			if id != nil {
				t.Errorf("got an identity %v, want none", id.fixed)
			}
			if got := id.value("user-agent", url, false); got != "" {
				t.Errorf("value on no identity: got %q", got)
			}
		}
	})
}

func TestChromeAcceptEncoding(t *testing.T) {
	for _, c := range []struct {
		url      string
		hasRange bool
		want     string
	}{
		{"https://example.com/", false, "gzip, deflate, br, zstd"},
		{"HTTPS://example.com/", false, "gzip, deflate, br, zstd"},
		{"http://example.com/", false, "gzip, deflate"},
		{"http://localhost:8080/", false, "gzip, deflate, br, zstd"},
		{"http://app.localhost/", false, "gzip, deflate, br, zstd"},
		{"http://127.0.0.1:8080/", false, "gzip, deflate, br, zstd"},
		{"http://[::1]:8080/", false, "gzip, deflate, br, zstd"},
		{"http://localhost.example.com/", false, "gzip, deflate"},
		{"https://example.com/video.mp4", true, "identity"},
	} {
		if got := chromeAcceptEncoding(c.url, c.hasRange); got != c.want {
			t.Errorf("%s (range %v): got %q, want %q", c.url, c.hasRange, got, c.want)
		}
	}
}

func TestResolveAutoFallsBackToTheEngine(t *testing.T) {
	engine := newEngineIdentity("157.0.8103.39")
	const url = "https://example.com/"
	const ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/157.0.0.0 Safari/537.36"

	t.Run("without defaults every identity header is the engine's", func(t *testing.T) {
		got := resolveAuto(Header{
			{"sec-ch-ua", AUTO},
			{"sec-ch-ua-mobile", AUTO},
			{"sec-ch-ua-platform", AUTO},
			{"User-Agent", AUTO},
			{"accept", AUTO}, // not part of the identity: dropped
			{"accept-encoding", AUTO},
			{"accept-language", AUTO},
			{"cookie", AUTO}, // no jar: dropped
		}, nil, nil, engine, url)
		assertHeader(t, got, Header{
			{"sec-ch-ua", `"Not-A.Brand";v="99", "Google Chrome";v="157", "Chromium";v="157"`},
			{"sec-ch-ua-mobile", "?0"},
			{"sec-ch-ua-platform", `"Windows"`},
			{"User-Agent", ua},
			{"accept-encoding", "gzip, deflate, br, zstd"},
			{"accept-language", "en-US,en;q=0.9"},
		})
	})

	t.Run("a default takes the place of the engine's value, header by header", func(t *testing.T) {
		defaults := buildDefaults(Header{{"Accept-Language", "de-DE,de;q=0.9"}})
		got := resolveAuto(Header{{"user-agent", AUTO}, {"accept-language", AUTO}}, nil, defaults, engine, url)
		assertHeader(t, got, Header{{"user-agent", ua}, {"accept-language", "de-DE,de;q=0.9"}})
	})

	t.Run("a literal value is left alone", func(t *testing.T) {
		got := resolveAuto(Header{{"user-agent", "mine"}, {"accept-language", AUTO}}, nil, nil, engine, url)
		assertHeader(t, got, Header{{"user-agent", "mine"}, {"accept-language", "en-US,en;q=0.9"}})
	})

	t.Run("accept-encoding follows the request", func(t *testing.T) {
		got := resolveAuto(Header{{"accept-encoding", AUTO}}, nil, nil, engine, "http://example.com/")
		assertHeader(t, got, Header{{"accept-encoding", "gzip, deflate"}})
		got = resolveAuto(Header{{"Range", "bytes=0-99"}, {"accept-encoding", AUTO}}, nil, nil, engine, url)
		assertHeader(t, got, Header{{"Range", "bytes=0-99"}, {"accept-encoding", "identity"}})
	})

	// What Chrome sends on a navigation to a plain-HTTP address that is not
	// this machine, and to one that is.
	t.Run("no client hints on plain HTTP, except to the local machine", func(t *testing.T) {
		in := Header{
			{"sec-ch-ua", AUTO},
			{"sec-ch-ua-mobile", AUTO},
			{"sec-ch-ua-platform", AUTO},
			{"user-agent", AUTO},
			{"accept-encoding", AUTO},
			{"accept-language", AUTO},
		}
		got := resolveAuto(in, nil, nil, engine, "http://192.0.2.10:8080/")
		assertHeader(t, got, Header{
			{"user-agent", ua},
			{"accept-encoding", "gzip, deflate"},
			{"accept-language", "en-US,en;q=0.9"},
		})
		got = resolveAuto(in, nil, nil, engine, "http://127.0.0.1:8080/")
		assertHeader(t, got, Header{
			{"sec-ch-ua", `"Not-A.Brand";v="99", "Google Chrome";v="157", "Chromium";v="157"`},
			{"sec-ch-ua-mobile", "?0"},
			{"sec-ch-ua-platform", `"Windows"`},
			{"user-agent", ua},
			{"accept-encoding", "gzip, deflate, br, zstd"},
			{"accept-language", "en-US,en;q=0.9"},
		})
	})
}

// The engine in this process reports a version the identity can be built from.
func TestIdentityOfLoadedEngine(t *testing.T) {
	c, err := New(Options{})
	if err == ErrUnsupportedPlatform {
		t.Skip("no engine for this platform")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.engine == nil {
		t.Fatal("the loaded engine has no identity")
	}
	if got := c.engine.value("sec-ch-ua", "https://example.com/", false); got == "" {
		t.Fatal("sec-ch-ua is empty")
	}
}
