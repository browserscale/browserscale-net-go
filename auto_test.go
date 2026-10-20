package browserscalenet

import "testing"

func TestResolveAuto(t *testing.T) {
	jar := NewJar()
	jar.Set(Cookie{Name: "sid", Value: "abc", Domain: "example.com", Path: "/"})
	defaults := buildDefaults(Header{
		{"user-agent", "UA/1"},
		{"accept-language", "en-US"},
	})
	const url = "https://example.com/"

	t.Run("fills cookie from jar and identity from defaults, keeping order and name case", func(t *testing.T) {
		in := Header{
			{"sec-ch-ua", `"x";v="1"`},
			{"Cookie", AUTO},     // HTTP/1.1-style casing, still matches
			{"User-Agent", AUTO}, // filled from defaults by lower-cased name
			{"accept", "*/*"},
		}
		got := resolveAuto(in, jar, defaults, url)
		want := Header{
			{"sec-ch-ua", `"x";v="1"`},
			{"Cookie", "sid=abc"},
			{"User-Agent", "UA/1"},
			{"accept", "*/*"},
		}
		assertHeader(t, got, want)
	})

	t.Run("drops cookie when jar has nothing for the url", func(t *testing.T) {
		got := resolveAuto(Header{{"cookie", AUTO}, {"accept", "*/*"}}, jar, defaults, "https://other.com/")
		assertHeader(t, got, Header{{"accept", "*/*"}})
	})

	t.Run("drops unknown auto header not in defaults", func(t *testing.T) {
		got := resolveAuto(Header{{"x-custom", AUTO}, {"accept", "*/*"}}, jar, defaults, url)
		assertHeader(t, got, Header{{"accept", "*/*"}})
	})

	t.Run("no AUTO returns the input unchanged without allocating", func(t *testing.T) {
		in := Header{{"user-agent", "literal"}, {"accept", "*/*"}}
		got := resolveAuto(in, jar, defaults, url)
		if &got[0] != &in[0] {
			t.Fatalf("expected input slice to be returned as-is")
		}
	})

	t.Run("nil jar and nil defaults just drop every AUTO", func(t *testing.T) {
		got := resolveAuto(Header{{"cookie", AUTO}, {"user-agent", AUTO}, {"accept", "*/*"}}, nil, nil, url)
		assertHeader(t, got, Header{{"accept", "*/*"}})
	})
}

func assertHeader(t *testing.T, got, want Header) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i][0] != want[i][0] || got[i][1] != want[i][1] {
			t.Fatalf("field %d: got %q=%q, want %q=%q", i, got[i][0], got[i][1], want[i][0], want[i][1])
		}
	}
}
