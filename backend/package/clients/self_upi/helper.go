package selfUPIClient

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const qrImageMaxBytes = 2 << 20

var upiIDPattern = regexp.MustCompile(`^[a-zA-Z0-9.\-_]{2,256}@[a-zA-Z]{2,64}$`)

func parse(data string) (c client, err error) {
	defer func() {
		if r := recover(); r != nil {
			c, err = client{}, fmt.Errorf("%w: %v", ErrInitializePanicked, r)
		}
	}()
	c, err = utils.FromStringToObjectJSON[client](data)
	if err != nil {
		return client{}, fmt.Errorf("%w: %w", ErrDataInvalid, err)
	}
	c.UPIID = strings.TrimSpace(c.UPIID)
	if !upiIDPattern.MatchString(c.UPIID) {
		return client{}, ErrUPIIDInvalid
	}
	c.QRImage = strings.Join(strings.Fields(c.QRImage), "")
	c.qrMIMEType, err = validateQRImage(c.QRImage)
	if err != nil {
		return client{}, err
	}
	c.qrDataURI = "data:" + c.qrMIMEType + ";base64," + c.QRImage
	return c, nil
}

func validateQRImage(encoded string) (string, error) {
	if encoded == "" {
		return "", ErrQRImageInvalid
	}
	if base64.StdEncoding.DecodedLen(len(encoded)) > qrImageMaxBytes {
		return "", ErrQRImageTooLarge
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrQRImageInvalid, err)
	}
	mime := http.DetectContentType(decoded)
	if !strings.HasPrefix(mime, "image/") {
		return "", ErrQRImageInvalid
	}
	return mime, nil
}
