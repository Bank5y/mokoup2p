package daemon_test

import (
	"testing"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/daemontest"
	"github.com/Bank5y/mokoup2p/server/servertest"
)

func TestServerAddressWithoutSchemeDefaultsToHTTPOnPort8080(t *testing.T) {
	cases := map[string]string{
		"1.2.3.4":                  "http://1.2.3.4:8080",
		"1.2.3.4:9000":             "http://1.2.3.4:9000",
		"p2p.example.com":          "http://p2p.example.com:8080",
		" p2p.example.com:9000/ ":  "http://p2p.example.com:9000",
		"[2001:db8::1]":            "http://[2001:db8::1]:8080",
		"[2001:db8::1]:9000":       "http://[2001:db8::1]:9000",
		"https://p2p.example.com":  "https://p2p.example.com",
		"http://1.2.3.4":           "http://1.2.3.4",
		"HTTP://1.2.3.4:81/":       "http://1.2.3.4:81",
		"https://p2p.example.com/": "https://p2p.example.com",
	}
	d := daemontest.Start(t)
	for in, want := range cases {
		st, err := d.IPC.SetServer(ctx, in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if st.ServerURL != want {
			t.Errorf("%q: want %q, got %q", in, want, st.ServerURL)
		}
	}
}

func TestInvalidServerAddressIsRejected(t *testing.T) {
	d := daemontest.Start(t)
	for _, in := range []string{"", "   ", "ftp://1.2.3.4", "http://", "1.2.3.4:abc", "a b", "not a url"} {
		_, err := d.IPC.SetServer(ctx, in)
		if code := servertest.ErrorCode(err); code != api.ErrBadRequest {
			t.Errorf("%q: want %s, got %v", in, api.ErrBadRequest, err)
		}
	}
}
