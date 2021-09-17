package main

import (
	"fmt"

	"golang.org/x/sys/unix"
)

var rlimitMap = map[string]int{
	"RLIMIT_CPU":        unix.RLIMIT_CPU,
	"RLIMIT_FSIZE":      unix.RLIMIT_FSIZE,
	"RLIMIT_DATA":       unix.RLIMIT_DATA,
	"RLIMIT_STACK":      unix.RLIMIT_STACK,
	"RLIMIT_CORE":       unix.RLIMIT_CORE,
	"RLIMIT_RSS":        unix.RLIMIT_RSS,
	"RLIMIT_NPROC":      unix.RLIMIT_NPROC,
	"RLIMIT_NOFILE":     unix.RLIMIT_NOFILE,
	"RLIMIT_MEMLOCK":    unix.RLIMIT_MEMLOCK,
	"RLIMIT_AS":         unix.RLIMIT_AS,
	"RLIMIT_LOCKS":      unix.RLIMIT_LOCKS,
	"RLIMIT_SIGPENDING": unix.RLIMIT_SIGPENDING,
	"RLIMIT_MSGQUEUE":   unix.RLIMIT_MSGQUEUE,
	"RLIMIT_NICE":       unix.RLIMIT_NICE,
	"RLIMIT_RTPRIO":     unix.RLIMIT_RTPRIO,
	"RLIMIT_RTTIME":     unix.RLIMIT_RTTIME,
}

func strToRlimit(key string) (int, error) {
	rl, ok := rlimitMap[key]
	if !ok {
		return 0, fmt.Errorf("wrong rlimit value: %s", key)
	}
	return rl, nil
}
