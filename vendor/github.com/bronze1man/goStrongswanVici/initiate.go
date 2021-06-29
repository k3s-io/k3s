package goStrongswanVici

import (
	"fmt"
)

// Initiate is used to initiate an SA. This is the
// equivalent of `swanctl --initiate -c childname`
func (c *ClientConn) Initiate(child string, ike string) (err error) {
	inMap := map[string]interface{}{}
	if child != "" {
		inMap["child"] = child
	}
	if ike != "" {
		inMap["ike"] = ike
	}
	msg, err := c.Request("initiate", inMap)
	if err != nil {
		return err
	}
	if msg["success"] != "yes" {
		return fmt.Errorf("unsuccessful Initiate: %v", msg["errmsg"])
	}
	return nil
}
