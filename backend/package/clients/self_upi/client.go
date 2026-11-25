package selfUPIClient

import (
	_ "embed"
)

//go:embed self_upi.json
var selfUPIData string

type Client interface {
	GetUPIQR() string
	GetUPIQRDataURI() string
	GetUPIQRMIMEType() string
	GetUPIID() string
}

type client struct {
	QRImage    string `json:"image"`
	UPIID      string `json:"text"`
	qrMIMEType string
	qrDataURI  string
}

func NewClient() (Client, error) {
	c, err := parse(selfUPIData)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *client) GetUPIQR() string {
	return c.QRImage
}

func (c *client) GetUPIQRDataURI() string {
	return c.qrDataURI
}

func (c *client) GetUPIQRMIMEType() string {
	return c.qrMIMEType
}

func (c *client) GetUPIID() string {
	return c.UPIID
}
