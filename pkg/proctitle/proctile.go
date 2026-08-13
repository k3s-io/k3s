//go:build linux

package proctitle

import (
	"github.com/erikdubbelboer/gspt"
)

var SetProcTitle = gspt.SetProcTitle
