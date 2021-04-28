package goStrongswanVici

import (
	"fmt"
)

type certPayload struct {
	Typ  string `json:"type"` // (X509|X509_AC|X509_CRL)
	Flag string `json:"flag"` // (CA|AA|OCSP|NONE)
	Data string `json:"data"`
}

func (c *ClientConn) LoadCertificate(s string, typ string, flag string) (err error) {
	requestMap := &map[string]interface{}{}

	var k = certPayload{
		Typ:  typ,
		Flag: flag,
		Data: s,
	}

	if err = ConvertToGeneral(k, requestMap); err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	msg, err := c.Request("load-cert", *requestMap)

	if err != nil {
		return fmt.Errorf("unsuccessful loadCert: %v", err.Error())
	}

	if msg["success"] != "yes" {
		return fmt.Errorf("unsuccessful loadCert: %v", msg["success"])
	}

	return nil
}
