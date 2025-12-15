//go:build !linux || !cgo

package cmds

func forkIfLoggingOrReaping() error {
	return nil
}
