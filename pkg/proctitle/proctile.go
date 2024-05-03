//go:build linux
// +build linux

package proctitle

import (
	"os"

	"github.com/erikdubbelboer/gspt"
)

func SetProcTitle(cmd string) {
	gspt.SetProcTitle(os.Args[0] + " agent")
}
