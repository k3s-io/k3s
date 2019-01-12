// +build darwin freebsd openbsd

package arping

import (
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"syscall"
	"time"
)

var bpf *os.File
var bpfFd int
var buflen int

var bpfArpFilter = []syscall.BpfInsn{
	// make sure this is an arp packet
	*syscall.BpfStmt(syscall.BPF_LD+syscall.BPF_H+syscall.BPF_ABS, 12),
	*syscall.BpfJump(syscall.BPF_JMP+syscall.BPF_JEQ+syscall.BPF_K, 0x0806, 0, 1),
	// if we passed all the tests, ask for the whole packet.
	*syscall.BpfStmt(syscall.BPF_RET+syscall.BPF_K, -1),
	// otherwise, drop it.
	*syscall.BpfStmt(syscall.BPF_RET+syscall.BPF_K, 0),
}

func initialize(iface net.Interface) (err error) {
	verboseLog.Println("search available /dev/bpfX")
	for i := 0; i <= 10; i++ {
		bpfPath := fmt.Sprintf("/dev/bpf%d", i)
		bpf, err = os.OpenFile(bpfPath, os.O_RDWR, 0666)
		if err != nil {
			verboseLog.Printf("  open failed: %s - %s\n", bpfPath, err.Error())
		} else {
			verboseLog.Printf("  open success: %s\n", bpfPath)
			break
		}
	}
	bpfFd = int(bpf.Fd())
	if bpfFd == -1 {
		return errors.New("unable to open /dev/bpfX")
	}

	if err := syscall.SetBpfInterface(bpfFd, iface.Name); err != nil {
		return err
	}

	if err := syscall.SetBpfImmediate(bpfFd, 1); err != nil {
		return err
	}

	buflen, err = syscall.BpfBuflen(bpfFd)
	if err != nil {
		return err
	}

	if err := syscall.SetBpf(bpfFd, bpfArpFilter); err != nil {
		return err
	}

	if err := syscall.FlushBpf(bpfFd); err != nil {
		return err
	}

	return nil
}

func send(request arpDatagram) (time.Time, error) {
	_, err := syscall.Write(bpfFd, request.MarshalWithEthernetHeader())
	return time.Now(), err
}

func receive() (arpDatagram, time.Time, error) {
	buffer := make([]byte, buflen)
	n, err := syscall.Read(bpfFd, buffer)
	if err != nil {
		return arpDatagram{}, time.Now(), err
	}

	//
	// FreeBSD uses a different bpf header (bh_tstamp differ in it's size)
	// https://www.freebsd.org/cgi/man.cgi?bpf(4)#BPF_HEADER
	//
	var bpfHdrLength int
	if runtime.GOOS == "freebsd" {
		bpfHdrLength = 26
	} else {
		bpfHdrLength = 18
	}

	// skip bpf header + 14 bytes ethernet header
	var hdrLength = bpfHdrLength + 14

	return parseArpDatagram(buffer[hdrLength:n]), time.Now(), nil
}

func deinitialize() error {
	return bpf.Close()
}
