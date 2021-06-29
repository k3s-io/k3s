package goStrongswanVici

import (
	"fmt"
	"strconv"
)

//from list-sa event
type IkeSa struct {
	Uniqueid        string               `json:"uniqueid"` //called ike_id in terminate() argument.
	Version         string               `json:"version"`
	State           string               `json:"state"` //had saw: ESTABLISHED
	Local_host      string               `json:"local-host"`
	Local_port      string               `json:"local-port"`
	Local_id        string               `json:"local-id"`
	Remote_host     string               `json:"remote-host"`
	Remote_port     string               `json:"remote-port"`
	Remote_id       string               `json:"remote-id"`
	Remote_xauth_id string               `json:"remote-xauth-id"` //client username
	Initiator       string               `json:"initiator"`
	Initiator_spi   string               `json:"initiator-spi"`
	Responder_spi   string               `json:"responder-spi"`
	Encr_alg        string               `json:"encr-alg"`
	Encr_keysize    string               `json:"encr-keysize"`
	Integ_alg       string               `json:"integ-alg"`
	Integ_keysize   string               `json:"integ-keysize"`
	Prf_alg         string               `json:"prf-alg"`
	Dh_group        string               `json:"dh-group"`
	Established     string               `json:"established"`
	Rekey_time      string               `json:"rekey-time"`
	Reauth_time     string               `json:"reauth-time"`
	Remote_vips     []string             `json:"remote-vips"`
	Child_sas       map[string]Child_sas `json:"child-sas"` //key means child-sa-name(conn name in ipsec.conf)
}

type Child_sas struct {
	Reqid         string   `json:"reqid"`
	State         string   `json:"state"` //had saw: INSTALLED
	Mode          string   `json:"mode"`  //had saw: TUNNEL
	Protocol      string   `json:"protocol"`
	Encap         string   `json:"encap"`
	Spi_in        string   `json:"spi-in"`
	Spi_out       string   `json:"spi-out"`
	Cpi_in        string   `json:"cpi-in"`
	Cpi_out       string   `json:"cpi-out"`
	Encr_alg      string   `json:"encr-alg"`
	Encr_keysize  string   `json:"encr-keysize"`
	Integ_alg     string   `json:"integ-alg"`
	Integ_keysize string   `json:"integ-keysize"`
	Prf_alg       string   `json:"prf-alg"`
	Dh_group      string   `json:"dh-group"`
	Esn           string   `json:"esn"`
	Bytes_in      string   `json:"bytes-in"` //bytes into this machine
	Packets_in    string   `json:"packets-in"`
	Use_in        string   `json:"use-in"`
	Bytes_out     string   `json:"bytes-out"` // bytes out of this machine
	Packets_out   string   `json:"packets-out"`
	Use_out       string   `json:"use-out"`
	Rekey_time    string   `json:"rekey-time"`
	Life_time     string   `json:"life-time"`
	Install_time  string   `json:"install-time"`
	Local_ts      []string `json:"local-ts"`
	Remote_ts     []string `json:"remote-ts"`
}

func (s *Child_sas) GetBytesIn() uint64 {
	num, err := strconv.ParseUint(s.Bytes_in, 10, 64)
	if err != nil {
		return 0
	}
	return num
}

func (s *Child_sas) GetBytesOut() uint64 {
	num, err := strconv.ParseUint(s.Bytes_out, 10, 64)
	if err != nil {
		return 0
	}
	return num
}

// To be simple, list all clients that are connecting to this server .
// A client is a sa.
// Lists currently active IKE_SAs
func (c *ClientConn) ListSas(ike string, ike_id string) (sas []map[string]IkeSa, err error) {
	sas = []map[string]IkeSa{}
	var eventErr error
	//register event
	err = c.RegisterEvent("list-sa", func(response map[string]interface{}) {
		sa := &map[string]IkeSa{}
		err = ConvertFromGeneral(response, sa)
		if err != nil {
			fmt.Printf("list-sa event error: %s\n", err)
			eventErr = err
			return
		}
		sas = append(sas, *sa)
		//fmt.Printf("event %#v\n", response)
	})
	if err != nil {
		return
	}
	if eventErr != nil {
		return
	}

	inMap := map[string]interface{}{}
	if ike != "" {
		inMap["ike"] = ike
	}
	if ike_id != "" {
		inMap["ike_id"] = ike_id
	}
	_, err = c.Request("list-sas", inMap)
	if err != nil {
		return
	}
	//fmt.Printf("request finish %#v\n", sas)
	err = c.UnregisterEvent("list-sa")
	if err != nil {
		return
	}
	return
}

//a vpn conn in the strongswan server
type VpnConnInfo struct {
	IkeSa
	Child_sas
	IkeSaName   string //looks like conn name in ipsec.conf, content is same as ChildSaName
	ChildSaName string //looks like conn name in ipsec.conf
}

func (c *VpnConnInfo) GuessUserName() string {
	if c.Remote_xauth_id != "" {
		return c.Remote_xauth_id
	}
	if c.Remote_id != "" {
		return c.Remote_id
	}
	return ""
}

// a helper method to avoid complex data struct in ListSas
// if it only have one child_sas ,it will put it into info.Child_sas
func (c *ClientConn) ListAllVpnConnInfo() (list []VpnConnInfo, err error) {
	sasList, err := c.ListSas("", "")
	if err != nil {
		return
	}
	list = make([]VpnConnInfo, len(sasList))
	for i, sa := range sasList {
		info := VpnConnInfo{}
		if len(sa) != 1 {
			fmt.Printf("[vici.ListAllVpnConnInfo] warning: len(sa)[%d]!=1\n", len(sa))
		}
		for ikeSaName, ikeSa := range sa {
			info.IkeSaName = ikeSaName
			info.IkeSa = ikeSa
			//if len(ikeSa.Child_sas) != 1 {
			//	fmt.Println("[vici.ListAllVpnConnInfo] warning: len(ikeSa.Child_sas)[%d]!=1", len(ikeSa.Child_sas))
			//}
			for childSaName, childSa := range ikeSa.Child_sas {
				info.ChildSaName = childSaName
				info.Child_sas = childSa
				break
			}
			break
		}
		if len(info.IkeSa.Child_sas) == 1 {
			info.IkeSa.Child_sas = nil
		}
		list[i] = info
	}
	return
}
