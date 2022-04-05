//go:build !linux || !cgo
// +build !linux !cgo

package cmds

func EvacuateCgroup2() error {
	return nil
}
