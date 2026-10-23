package browserscalenet

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProxyCredentialsReachTheProxy(t *testing.T) {
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("bob:s3cr3t-ü"))
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Proxy-Authorization") != want {
			w.Header().Set("Proxy-Authenticate", `Basic realm="proxy"`)
			w.WriteHeader(http.StatusProxyAuthRequired)
			return
		}
		io.WriteString(w, "via proxy "+r.URL.String())
	}))
	defer proxy.Close()

	c, err := New(Options{Proxy: strings.Replace(proxy.URL, "http://", "http://bob:s3cr3t-%C3%BC@", 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	resp, err := c.Do(ctx, &Request{URL: "http://example.com/hello"})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != 200 || string(body) != "via proxy http://example.com/hello" {
		t.Fatalf("got %d %q; want 200 from the proxy with the credentials", resp.Status, body)
	}
}
