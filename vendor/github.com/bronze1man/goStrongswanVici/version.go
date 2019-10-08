package goStrongswanVici

type Version struct {
	Daemon  string `json:"daemon"`
	Version string `json:"version"`
	Sysname string `json:"sysname"`
	Release string `json:"release"`
	Machine string `json:"machine"`
}

func (c *ClientConn) Version() (out *Version, err error) {
	msg, err := c.Request("version", nil)
	if err != nil {
		return
	}
	out = &Version{}
	err = ConvertFromGeneral(msg, out)
	return
}
