//go:build !windows

package scannerlauncher

import "fmt"

func runElevated(_, _, _, _ string, _ int) error {
	return fmt.Errorf("élévation administrateur non prise en charge sur cette plateforme")
}
