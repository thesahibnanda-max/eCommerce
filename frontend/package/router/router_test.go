package frontendRouter

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

type fakeHandler struct {
	panic bool
}

func (f fakeHandler) ServeStatic(c fiber.Ctx) error {
	if f.panic {
		panic("boom")
	}
	return c.SendString("static:" + c.Path())
}

func Test_NewRouter_RequiresHandler(t *testing.T) {
	if _, err := NewRouter(nil); !errors.Is(err, ErrHandlerRequired) {
		t.Fatalf("err = %v", err)
	}
}

func Test_Routes(t *testing.T) {
	r, err := NewRouter(fakeHandler{})
	if err != nil {
		t.Fatal(err)
	}
	app := r.(*routerImpl).app
	for _, path := range []string{"/", "/cart", "/assets/a.js", "/p/1/x"} {
		resp, err := app.Test(httptest.NewRequest("GET", path, nil))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || string(body) != "static:"+path {
			t.Fatalf("%s = %d %s", path, resp.StatusCode, body)
		}
	}
	resp, err := app.Test(httptest.NewRequest("POST", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d, want 405", resp.StatusCode)
	}
}

func Test_PanicRecovered(t *testing.T) {
	r, err := NewRouter(fakeHandler{panic: true})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := r.(*routerImpl).app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 500 || string(body) != "Internal Server Error" {
		t.Fatalf("panic = %d %s", resp.StatusCode, body)
	}
}

func Test_ListenAndServe(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	r, err := NewRouter(fakeHandler{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errs := r.ListenAndServeOnPort(ctx, uint(port))
	var resp *http.Response
	for range 50 {
		if resp, err = http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/x"); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("server never came up: %v", err)
	}
	resp.Body.Close()
	cancel()
	select {
	case err, ok := <-errs:
		if ok && err != nil {
			t.Fatalf("shutdown err = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no shutdown")
	}
}
