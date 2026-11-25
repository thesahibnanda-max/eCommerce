package selfUPIClient

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"
)

var pngHeader = base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\n0000"))

func TestNewClient_EmbeddedData(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("embedded self_upi.json is invalid: %v", err)
	}
	if !upiIDPattern.MatchString(c.GetUPIID()) {
		t.Fatalf("upi id %q invalid", c.GetUPIID())
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(c.GetUPIQR())
	if err != nil {
		t.Fatalf("qr not base64: %v", err)
	}
	if !bytes.HasPrefix(decoded, []byte{0xff, 0xd8, 0xff}) || !bytes.HasSuffix(decoded, []byte{0xff, 0xd9}) {
		t.Fatalf("qr is not a complete jpeg")
	}
	if c.GetUPIQRMIMEType() != "image/jpeg" {
		t.Fatalf("mime %q", c.GetUPIQRMIMEType())
	}
	if c.GetUPIQRDataURI() != "data:image/jpeg;base64,"+c.GetUPIQR() {
		t.Fatalf("data uri mismatch")
	}
}

func TestParse(t *testing.T) {
	c, err := parse("{\"text\":\"  shop@ybl \",\"image\":\"" + pngHeader[:8] + "\\n " + pngHeader[8:] + "\"}")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.UPIID != "shop@ybl" || c.QRImage != pngHeader || c.qrMIMEType != "image/png" {
		t.Fatalf("got %+v", c)
	}
	if c.qrDataURI != "data:image/png;base64,"+pngHeader {
		t.Fatalf("data uri %q", c.qrDataURI)
	}
}

func TestParse_Rejects(t *testing.T) {
	huge := strings.Repeat("A", base64.StdEncoding.EncodedLen(qrImageMaxBytes+1))
	cases := []struct {
		name string
		data string
		want error
	}{
		{"not json", "{", ErrDataInvalid},
		{"empty upi id", `{"text":"","image":"` + pngHeader + `"}`, ErrUPIIDInvalid},
		{"no handle", `{"text":"shop","image":"` + pngHeader + `"}`, ErrUPIIDInvalid},
		{"bad handle", `{"text":"shop@y8l","image":"` + pngHeader + `"}`, ErrUPIIDInvalid},
		{"empty image", `{"text":"shop@ybl","image":""}`, ErrQRImageInvalid},
		{"bad base64", `{"text":"shop@ybl","image":"***"}`, ErrQRImageInvalid},
		{"truncated base64", `{"text":"shop@ybl","image":"` + pngHeader + `+"}`, ErrQRImageInvalid},
		{"not image", `{"text":"shop@ybl","image":"` + base64.StdEncoding.EncodeToString([]byte("hello world")) + `"}`, ErrQRImageInvalid},
		{"too large", `{"text":"shop@ybl","image":"` + huge + `"}`, ErrQRImageTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parse(tc.data); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNewClient_Concurrent(t *testing.T) {
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := NewClient()
			if err != nil || c.GetUPIID() == "" || c.GetUPIQR() == "" {
				t.Errorf("NewClient: %v", err)
			}
		}()
	}
	wg.Wait()
}
