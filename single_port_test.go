package main

import (
	"errors"
	"net"
	"testing"

	"github.com/valyala/fasthttp"
)

func TestSinglePortFromEnv(t *testing.T) {
	tests := []struct {
		raw      string
		wantPort uint
		wantOn   bool
		wantErr  bool
	}{
		{raw: "", wantOn: false},
		{raw: "10000", wantPort: 10000, wantOn: true},
		{raw: " 8080 ", wantPort: 8080, wantOn: true},
		{raw: "0", wantErr: true},
		{raw: "70000", wantErr: true},
		{raw: "abc", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Setenv(envPort, tt.raw)
			port, on, err := singlePortFromEnv()
			if tt.wantErr {
				if !errors.Is(err, errPortInvalid) {
					t.Fatalf("err = %v, want errPortInvalid", err)
				}
				return
			}
			if err != nil || port != tt.wantPort || on != tt.wantOn {
				t.Fatalf("got (%d, %v, %v), want (%d, %v, nil)", port, on, err, tt.wantPort, tt.wantOn)
			}
		})
	}
}

func TestIsAPIPath(t *testing.T) {
	cases := map[string]bool{
		"/v1":            true,
		"/v1/":           true,
		"/v1/products":   true,
		"/v1/images/abc": true,
		"/":              false,
		"/v1x":           false,
		"/v10/products":  false,
		"/assets/app.js": false,
		"/orders/v1":     false,
	}
	for path, want := range cases {
		if got := isAPIPath([]byte(path)); got != want {
			t.Errorf("isAPIPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{name: "none", want: ""},
		{name: "forwarded first entry", headers: map[string]string{"X-Forwarded-For": "203.0.113.7, 10.0.0.1"}, want: "203.0.113.7"},
		{name: "cloudflare wins", headers: map[string]string{"CF-Connecting-IP": "198.51.100.2", "X-Forwarded-For": "203.0.113.7"}, want: "198.51.100.2"},
		{name: "true client ip before forwarded", headers: map[string]string{"True-Client-IP": "2001:db8::1", "X-Forwarded-For": "203.0.113.7"}, want: "2001:db8::1"},
		{name: "garbage skipped", headers: map[string]string{"CF-Connecting-IP": "nope", "X-Forwarded-For": "203.0.113.9"}, want: "203.0.113.9"},
		{name: "all garbage", headers: map[string]string{"X-Forwarded-For": "unknown"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h fasthttp.RequestHeader
			for k, v := range tt.headers {
				h.Set(k, v)
			}
			got := clientIP(&h)
			if tt.want == "" {
				if got != nil {
					t.Fatalf("got %v, want nil", got)
				}
				return
			}
			if !got.Equal(net.ParseIP(tt.want)) {
				t.Fatalf("got %v, want %s", got, tt.want)
			}
		})
	}
}

func TestSinglePortHandlerRoutesAndSetsClientIP(t *testing.T) {
	var hit, seenIP string
	api := func(ctx *fasthttp.RequestCtx) { hit, seenIP = "api", ctx.RemoteIP().String() }
	store := func(ctx *fasthttp.RequestCtx) { hit, seenIP = "store", ctx.RemoteIP().String() }
	handler := newSinglePortHandler(api, store)

	for path, want := range map[string]string{"/v1/products": "api", "/": "store", "/product/abc": "store"} {
		var ctx fasthttp.RequestCtx
		ctx.Request.SetRequestURI(path)
		ctx.Request.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
		handler(&ctx)
		if hit != want {
			t.Fatalf("%s routed to %s, want %s", path, hit, want)
		}
		if seenIP != "203.0.113.7" {
			t.Fatalf("%s saw client ip %s, want 203.0.113.7", path, seenIP)
		}
	}
}
