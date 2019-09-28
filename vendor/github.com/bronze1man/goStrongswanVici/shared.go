// this file contains the functions for managing shared secrets

package goStrongswanVici

import (
	"fmt"
)

type Key struct {
	ID     string   `json:"id,omitempty"`
	Typ    string   `json:"type"`
	Data   string   `json:"data"`
	Owners []string `json:"owners"`
}

type UnloadKeyRequest struct {
	ID string `json:"id"`
}

type keyList struct {
	Keys []string `json:"keys"`
}

// load a shared secret into the IKE daemon
func (c *ClientConn) LoadShared(key *Key) error {
	requestMap := &map[string]interface{}{}

	err := ConvertToGeneral(key, requestMap)

	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	msg, err := c.Request("load-shared", *requestMap)
	if msg["success"] != "yes" {
		return fmt.Errorf("unsuccessful loadSharedKey: %v", msg["errmsg"])
	}

	return nil
}

// unload (delete) a shared secret from the IKE daemon
func (c *ClientConn) UnloadShared(key *UnloadKeyRequest) error {
	requestMap := &map[string]interface{}{}

	err := ConvertToGeneral(key, requestMap)

	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	msg, err := c.Request("unload-shared", *requestMap)
	if msg["success"] != "yes" {
		return fmt.Errorf("unsuccessful loadSharedKey: %v", msg["errmsg"])
	}

	return nil
}

// get a the names of the shared secrets currently loaded
func (c *ClientConn) GetShared() ([]string, error) {
	msg, err := c.Request("get-shared", nil)
	if err != nil {
		fmt.Errorf("Error making request: %v", err)
		return nil, err
	}

	keys := &keyList{}

	err = ConvertFromGeneral(msg, keys)
	if err != nil {
		fmt.Errorf("Error converting data: %v", err)
	}

	return keys.Keys, err
}
