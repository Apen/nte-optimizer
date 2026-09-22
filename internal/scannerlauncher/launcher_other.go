//go:build !windows

package scannerlauncher

import "fmt"

func runElevated(_, _, _, _ string, _ int) error {
	return fmt.Errorf("administrator elevation is not supported on this platform")
}
