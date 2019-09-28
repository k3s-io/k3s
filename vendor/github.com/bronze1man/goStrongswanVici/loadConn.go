package goStrongswanVici

import (
	"fmt"
)

type Connection struct {
	ConnConf map[string]IKEConf `json:"connections"`
}

type IKEConf struct {
	LocalAddrs  []string               `json:"local_addrs"`
	RemoteAddrs []string               `json:"remote_addrs,omitempty"`
	Proposals   []string               `json:"proposals,omitempty"`
	Version     string                 `json:"version"` //1 for ikev1, 0 for ikev1 & ikev2
	Encap       string                 `json:"encap"`   //yes,no
	KeyingTries string                 `json:"keyingtries"`
	RekeyTime   string                 `json:"rekey_time"`
	DPDDelay    string                 `json:"dpd_delay,omitempty"`
	LocalAuth   AuthConf               `json:"local"`
	RemoteAuth  AuthConf               `json:"remote"`
	Pools       []string               `json:"pools,omitempty"`
	Children    map[string]ChildSAConf `json:"children"`
}

type AuthConf struct {
	ID         string `json:"id"`
	Round      string `json:"round,omitempty"`
	AuthMethod string `json:"auth"` // (psk|pubkey)
	EAP_ID     string `json:"eap_id,omitempty"`
}

type ChildSAConf struct {
	Local_ts      []string `json:"local_ts"`
	Remote_ts     []string `json:"remote_ts"`
	ESPProposals  []string `json:"esp_proposals,omitempty"` //aes128-sha1_modp1024
	StartAction   string   `json:"start_action"`            //none,trap,start
	CloseAction   string   `json:"close_action"`
	ReqID         string   `json:"reqid,omitempty"`
	RekeyTime     string   `json:"rekey_time"`
	ReplayWindow  string   `json:"replay_window,omitempty"`
	Mode          string   `json:"mode"`
	InstallPolicy string   `json:"policies"`
	UpDown        string   `json:"updown,omitempty"`
	Priority      string   `json:"priority,omitempty"`
	MarkIn        string   `json:"mark_in,omitempty"`
	MarkOut       string   `json:"mark_out,omitempty"`
	DpdAction     string   `json:"dpd_action,omitempty"`
	LifeTime      string   `json:"life_time,omitempty"`
}

func (c *ClientConn) LoadConn(conn *map[string]IKEConf) error {
	requestMap := &map[string]interface{}{}

	err := ConvertToGeneral(conn, requestMap)

	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	msg, err := c.Request("load-conn", *requestMap)

	if msg["success"] != "yes" {
		return fmt.Errorf("unsuccessful LoadConn: %v", msg["errmsg"])
	}

	return nil
}
