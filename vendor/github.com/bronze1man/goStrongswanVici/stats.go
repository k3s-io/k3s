package goStrongswanVici

// Stats returns IKE daemon statistics and load information.
func (c *ClientConn) Stats() (msg map[string]interface{}, err error) {
	msg, err = c.Request("stats", nil)
	return
}
