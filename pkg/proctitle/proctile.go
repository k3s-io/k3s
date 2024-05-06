//go:build linux
// +build linux

package proctitle

import (
	"github.com/erikdubbelboer/gspt"
)

func SetProcTitle(cmd string) {
	gspt.SetProcTitle(cmd)
}
