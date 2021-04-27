package goStrongswanVici

import (
	"fmt"
)

type UnloadConnRequest struct {
	Name string `json:"name"`
}

func (c *ClientConn) UnloadConn(r *UnloadConnRequest) error {
	reqMap := &map[string]interface{}{}
	ConvertToGeneral(r, reqMap)
	msg, err := c.Request("unload-conn", *reqMap)
	if err != nil {
		return err
	}

	if msg["success"] != "yes" {
		return fmt.Errorf("[Unload-Connection] %s", msg["errmsg"])
	}

	return nil
}
