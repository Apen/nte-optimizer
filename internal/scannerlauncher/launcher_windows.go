//go:build windows

package scannerlauncher

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"nte-optimizer/internal/scanlog"
)

func runElevated(helper, projectDir, outputDir, cancelFile string, seconds int) error {
	return runElevatedWithRunner(helper, projectDir, outputDir, cancelFile, seconds, runPowerShell)
}

func runElevatedWithRunner(helper, projectDir, outputDir, cancelFile string, seconds int, run func(string) ([]byte, error)) error {
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
	commandLineQuote := func(value string) string { return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"` }
	arguments := strings.Join([]string{
		"-login-capture",
		"-login-seconds", strconv.Itoa(seconds),
		"-output-dir", commandLineQuote(outputDir),
		"-cancel-file", commandLineQuote(cancelFile),
		"-scan-log", commandLineQuote(scanlog.Path(filepath.Dir(filepath.Dir(outputDir)))),
	}, " ")
	script := fmt.Sprintf(
		"$p = Start-Process -FilePath %s -WorkingDirectory %s -ArgumentList %s -Verb RunAs -Wait -PassThru; exit $p.ExitCode",
		quote(helper), quote(projectDir), quote(arguments),
	)
	if output, err := run(script); err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("the administrator scanner did not finish successfully: %s", message)
	}
	return nil
}

func runPowerShell(script string) ([]byte, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	// PowerShell only brokers UAC and waits for the real scanner process. Hide
	// this intermediary console while keeping the useful nte-scan console open.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.CombinedOutput()
}
