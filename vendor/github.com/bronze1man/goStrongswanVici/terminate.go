package goStrongswanVici

import (
	"fmt"
)

type TerminateRequest struct {
	Child    string `json:"child,omitempty"`
	Ike      string `json:"ike,omitempty"`
	Child_id string `json:"child-id,omitempty"`
	Ike_id   string `json:"ike-id,omitempty"`
	Force    string `json:"force,omitempty"`
	Timeout  string `json:"timeout,omitempty"`
	Loglevel string `json:"loglevel,omitempty"`
}

// To be simple, kill a client that is connecting to this server. A client is a sa.
//Terminates an SA while streaming control-log events.
func (c *ClientConn) Terminate(r *TerminateRequest) (err error) {
	err = handlePanic(func() (err error) {
		reqMap := &map[string]interface{}{}
		ConvertToGeneral(r, reqMap)
		msg, err := c.Request("terminate", *reqMap)
		if err != nil {
			return
		}
		if msg["success"] != "yes" {
			return fmt.Errorf("[Terminate] %s", msg["errmsg"])
		}
		return
	})
	return
}
