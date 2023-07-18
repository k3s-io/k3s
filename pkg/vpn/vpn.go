package vpn

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/k3s-io/k3s/pkg/util"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	tailscaleIf = "tailscale0"
)

type TailscaleOutput struct {
	TailscaleIPs []string `json:"TailscaleIPs"`
}

// VPNInfo includes node information of the VPN. It is a general struct in case we want to add more vpn integrations
type VPNInfo struct {
	IPv4Address  net.IP
	IPv6Address  net.IP
	NodeID       string
	ProviderName string
	VPNInterface string
}

// vpnCliAuthInfo includes auth information of the VPN. It is a general struct in case we want to add more vpn integrations
type vpnCliAuthInfo struct {
	Name             string
	JoinKey          string
	ControlServerURL string
}

// StartVPN starts the VPN interface. General function in case we want to add more vpn integrations
func StartVPN(vpnAuthConfigFile string) error {
	authInfo, err := getVPNAuthInfo(vpnAuthConfigFile)
	if err != nil {
		return err
	}

	logrus.Infof("Starting VPN: %s", authInfo.Name)
	switch authInfo.Name {
	case "tailscale":
		args := []string{
			"up", "--authkey", authInfo.JoinKey, "--timeout=30s", "--reset",
		}
		if authInfo.ControlServerURL != "" {
			args = append(args, "--login-server", authInfo.ControlServerURL)
		}
		output, err := util.ExecCommand("tailscale", args)
		if err != nil {
			return errors.Wrap(err, "tailscale up failed: "+output)
		}
		logrus.Debugf("Output from tailscale up: %v", output)
		return nil
	default:
		return fmt.Errorf("Requested VPN: %s is not supported. We currently only support tailscale", authInfo.Name)
	}
}

// GetVPNInfo returns a VPNInfo object with details about the VPN. General function in case we want to add more vpn integrations
func GetVPNInfo(vpnAuth string) (VPNInfo, error) {
	authInfo, err := getVPNAuthInfo(vpnAuth)
	if err != nil {
		return VPNInfo{}, err
	}

	if authInfo.Name == "tailscale" {
		return getTailscaleInfo()
	}
	return VPNInfo{}, nil
}

// getVPNAuthInfo returns the required authInfo object
func getVPNAuthInfo(vpnAuth string) (vpnCliAuthInfo, error) {
	var authInfo vpnCliAuthInfo
	vpnParameters := strings.Split(vpnAuth, ",")
	for _, vpnKeyValues := range vpnParameters {
		vpnKeyValue := strings.Split(vpnKeyValues, "=")
		switch vpnKeyValue[0] {
		case "name":
			authInfo.Name = vpnKeyValue[1]
		case "joinKey":
			authInfo.JoinKey = vpnKeyValue[1]
		case "controlServerURL":
			authInfo.ControlServerURL = vpnKeyValue[1]
		default:
			return vpnCliAuthInfo{}, fmt.Errorf("VPN Error. The passed VPN auth info includes an unknown parameter: %v", vpnKeyValue[0])
		}
	}

	if err := isVPNConfigOK(authInfo); err != nil {
		return authInfo, err
	}
	return authInfo, nil
}

// isVPNConfigOK checks that the config is complete
func isVPNConfigOK(authInfo vpnCliAuthInfo) error {
	if authInfo.Name == "tailscale" {
		if authInfo.JoinKey == "" {
			return errors.New("VPN Error. Tailscale requires a JoinKey")
		}
		if authInfo.ControlServerURL != "" {
			if _, err := url.Parse(authInfo.ControlServerURL); err != nil {
				return fmt.Errorf("VPN Error. Invalid control server URL for Tailscale: %w", err)
			}
		}
		return nil
	}

	return errors.New("Requested VPN: " + authInfo.Name + " is not supported. We currently only support tailscale")
}

// getTailscaleInfo returns the IPs of the interface
func getTailscaleInfo() (VPNInfo, error) {
	output, err := util.ExecCommand("tailscale", []string{"status", "--json"})
	if err != nil {
		return VPNInfo{}, fmt.Errorf("failed to run tailscale status --json: %v", err)
	}

	logrus.Debugf("Output from tailscale status --json: %v", output)

	var tailscaleOutput TailscaleOutput
	err = json.Unmarshal([]byte(output), &tailscaleOutput)
	if err != nil {
		return VPNInfo{}, fmt.Errorf("failed to unmarshal tailscale output: %v", err)
	}

	// Errors are ignored because the interface might not have ipv4 or ipv6 addresses (that's the only possible error)
	ipv4Address, _ := util.GetFirst4String(tailscaleOutput.TailscaleIPs)
	ipv6Address, _ := util.GetFirst6String(tailscaleOutput.TailscaleIPs)

	return VPNInfo{IPv4Address: net.ParseIP(ipv4Address), IPv6Address: net.ParseIP(ipv6Address), NodeID: "", ProviderName: "tailscale", VPNInterface: tailscaleIf}, nil
}
