package goStrongswanVici

import (
	"fmt"
)

type Pool struct {
	PoolMapping map[string]interface{} `json:"pools"`
}

type PoolMapping struct {
	Addrs              string   `json:"addrs"`
	DNS                []string `json:"dns,omitempty"`
	NBNS               []string `json:"nbns,omitempty"`
	ApplicationVersion []string `json:"7,omitempty"`
	InternalIPv6Prefix []string `json:"18,omitempty"`
}

func (c *ClientConn) LoadPool(ph Pool) error {
	requestMap := map[string]interface{}{}

	err := ConvertToGeneral(ph.PoolMapping, &requestMap)

	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("error creating request: %v", err)
	}

	msg, err := c.Request("load-pool", requestMap)

	if msg["success"] != "yes" {
		return fmt.Errorf("unsuccessful LoadPool: %v", msg["success"])
	}

	return nil
}
