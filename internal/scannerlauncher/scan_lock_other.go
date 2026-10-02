//go:build !windows

package scannerlauncher

func AcquireScanLock(string) (func(), error) {
	return func() {}, nil
}
