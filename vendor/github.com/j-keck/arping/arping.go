// Package arping is a native go library to ping a host per arp datagram, or query a host mac address
//
// The currently supported platforms are: Linux and BSD.
//
//
// The library requires raw socket access. So it must run as root, or with appropriate capabilities under linux:
// `sudo setcap cap_net_raw+ep <BIN>`.
//
//
// Examples:
//
//   ping a host:
//   ------------
//     package main
//     import ("fmt"; "github.com/j-keck/arping"; "net")
//
//     func main(){
//       dstIP := net.ParseIP("192.168.1.1")
//       if hwAddr, duration, err := arping.Ping(dstIP); err != nil {
//         fmt.Println(err)
//       } else {
//         fmt.Printf("%s (%s) %d usec\n", dstIP, hwAddr, duration/1000)
//       }
//     }
//
//
//   resolve mac address:
//   --------------------
//     package main
//     import ("fmt"; "github.com/j-keck/arping"; "net")
//
//     func main(){
//       dstIP := net.ParseIP("192.168.1.1")
//       if hwAddr, _, err := arping.Ping(dstIP); err != nil {
//         fmt.Println(err)
//       } else {
//         fmt.Printf("%s is at %s\n", dstIP, hwAddr)
//       }
//     }
//
//
//   check if host is online:
//   ------------------------
//     package main
//     import ("fmt"; "github.com/j-keck/arping"; "net")
//
//     func main(){
//       dstIP := net.ParseIP("192.168.1.1")
//       _, _, err := arping.Ping(dstIP)
//       if err == arping.ErrTimeout {
//         fmt.Println("offline")
//       }else if err != nil {
//         fmt.Println(err.Error())
//       }else{
//         fmt.Println("online")
//       }
//     }
//
package arping

import (
	"errors"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"
)

var (
	// ErrTimeout error
	ErrTimeout = errors.New("timeout")

	verboseLog = log.New(ioutil.Discard, "", 0)
	timeout    = time.Duration(500 * time.Millisecond)
)

// Ping sends an arp ping to 'dstIP'
func Ping(dstIP net.IP) (net.HardwareAddr, time.Duration, error) {
	iface, err := findUsableInterfaceForNetwork(dstIP)
	if err != nil {
		return nil, 0, err
	}
	return PingOverIface(dstIP, *iface)
}

// PingOverIfaceByName sends an arp ping over interface name 'ifaceName' to 'dstIP'
func PingOverIfaceByName(dstIP net.IP, ifaceName string) (net.HardwareAddr, time.Duration, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, 0, err
	}
	return PingOverIface(dstIP, *iface)
}

// PingOverIface sends an arp ping over interface 'iface' to 'dstIP'
func PingOverIface(dstIP net.IP, iface net.Interface) (net.HardwareAddr, time.Duration, error) {
	srcMac := iface.HardwareAddr
	srcIP, err := findIPInNetworkFromIface(dstIP, iface)
	if err != nil {
		return nil, 0, err
	}

	broadcastMac := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	request := newArpRequest(srcMac, srcIP, broadcastMac, dstIP)

	if err := initialize(iface); err != nil {
		return nil, 0, err
	}
	defer deinitialize()

	type PingResult struct {
		mac      net.HardwareAddr
		duration time.Duration
		err      error
	}
	pingResultChan := make(chan PingResult)

	go func() {
		// send arp request
		verboseLog.Printf("arping '%s' over interface: '%s' with address: '%s'\n", dstIP, iface.Name, srcIP)
		if sendTime, err := send(request); err != nil {
			pingResultChan <- PingResult{nil, 0, err}
		} else {
			for {
				// receive arp response
				response, receiveTime, err := receive()

				if err != nil {
					pingResultChan <- PingResult{nil, 0, err}
					return
				}

				if response.IsResponseOf(request) {
					duration := receiveTime.Sub(sendTime)
					verboseLog.Printf("process received arp: srcIP: '%s', srcMac: '%s'\n",
						response.SenderIP(), response.SenderMac())
					pingResultChan <- PingResult{response.SenderMac(), duration, err}
					return
				}

				verboseLog.Printf("ignore received arp: srcIP: '%s', srcMac: '%s'\n",
					response.SenderIP(), response.SenderMac())
			}
		}
	}()

	select {
	case pingResult := <-pingResultChan:
		return pingResult.mac, pingResult.duration, pingResult.err
	case <-time.After(timeout):
		return nil, 0, ErrTimeout
	}
}

// GratuitousArp sends an gratuitous arp from 'srcIP'
func GratuitousArp(srcIP net.IP) error {
	iface, err := findUsableInterfaceForNetwork(srcIP)
	if err != nil {
		return err
	}
	return GratuitousArpOverIface(srcIP, *iface)
}

// GratuitousArpOverIfaceByName sends an gratuitous arp over interface name 'ifaceName' from 'srcIP'
func GratuitousArpOverIfaceByName(srcIP net.IP, ifaceName string) error {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return err
	}
	return GratuitousArpOverIface(srcIP, *iface)
}

// GratuitousArpOverIface sends an gratuitous arp over interface 'iface' from 'srcIP'
func GratuitousArpOverIface(srcIP net.IP, iface net.Interface) error {
	srcMac := iface.HardwareAddr
	broadcastMac := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	request := newArpRequest(srcMac, srcIP, broadcastMac, srcIP)

	if err := initialize(iface); err != nil {
		return err
	}
	defer deinitialize()
	verboseLog.Printf("gratuitous arp over interface: '%s' with address: '%s'\n", iface.Name, srcIP)
	_, err := send(request)
	return err
}

// EnableVerboseLog enables verbose logging on stdout
func EnableVerboseLog() {
	verboseLog = log.New(os.Stdout, "", 0)
}

// SetTimeout sets ping timeout
func SetTimeout(t time.Duration) {
	timeout = t
}
