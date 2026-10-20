package browserscalenet

import "testing"

func TestHeaderLookupIsCaseInsensitive(t *testing.T) {
	h := Header{
		{"content-type", "text/html"},
		{"set-cookie", "a=1"},
		{"Set-Cookie", "b=2"},
	}

	if v, ok := h.Get("Content-Type"); !ok || v != "text/html" {
		t.Fatalf(`Get("Content-Type") = %q, %v; want "text/html", true`, v, ok)
	}
	if _, ok := h.Get("x-missing"); ok {
		t.Fatal(`Get("x-missing") found a value`)
	}

	got := h.Values("SET-COOKIE")
	if len(got) != 2 || got[0] != "a=1" || got[1] != "b=2" {
		t.Fatalf(`Values("SET-COOKIE") = %q; want [a=1 b=2]`, got)
	}
}
