//go:build !linux || !cgo

package cmds

func EvacuateCgroup2() error {
	return nil
}
