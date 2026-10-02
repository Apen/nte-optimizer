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

func runElevatedWithRecovery(helper, projectDir, outputDir, cancelFile string, seconds int, stopPktmonFirst bool) error {
	return runElevatedWithRecoveryWithRunner(helper, projectDir, outputDir, cancelFile, seconds, stopPktmonFirst, runPowerShell)
}

func runElevatedWithRunner(helper, projectDir, outputDir, cancelFile string, seconds int, run func(string) ([]byte, error)) error {
	return runElevatedWithRecoveryWithRunner(helper, projectDir, outputDir, cancelFile, seconds, false, run)
}

func runElevatedWithRecoveryWithRunner(helper, projectDir, outputDir, cancelFile string, seconds int, stopPktmonFirst bool, run func(string) ([]byte, error)) error {
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
	commandLineQuote := func(value string) string { return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"` }
	argumentList := []string{
		"-login-capture",
		"-login-seconds", strconv.Itoa(seconds),
		"-output-dir", commandLineQuote(outputDir),
		"-cancel-file", commandLineQuote(cancelFile),
		"-scan-log", commandLineQuote(scanlog.Path(filepath.Dir(filepath.Dir(outputDir)))),
	}
	if stopPktmonFirst {
		argumentList = append(argumentList, "-stop-pktmon-first")
	}
	arguments := strings.Join(argumentList, " ")
	script := fmt.Sprintf(
		"$p = Start-Process -FilePath %s -WorkingDirectory %s -ArgumentList %s -Verb RunAs -Wait -PassThru; exit $p.ExitCode",
		quote(helper), quote(projectDir), quote(arguments),
	)
	if output, err := run(script); err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		code := scannerHelperFailureCode(message)
		if isUACCancellation(message) {
			code = "uac_cancelled"
		}
		return &DiagnosticError{Code: code, Err: fmt.Errorf("the administrator scanner did not finish successfully: %s", message)}
	}
	return nil
}

func isUACCancellation(message string) bool {
	message = strings.ToLower(message)
	for _, marker := range []string{"operation was canceled", "operation was cancelled", "annulé par", "annulée par", "annule par", "annulee par", "0x4c7", "1223"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func scannerHelperFailureCode(message string) string {
	message = strings.ToLower(message)
	switch {
	case strings.Contains(message, "local scan diagnostic is not writable"):
		return "diagnostic_log_not_writable"
	case strings.Contains(message, "pktmon is already capturing"):
		return "pktmon_already_running"
	case strings.Contains(message, "query pktmon status"):
		if strings.Contains(message, "access denied") || strings.Contains(message, "accès refusé") || strings.Contains(message, "acces refuse") {
			return "pktmon_permission_denied"
		}
		return "pktmon_status_failed"
	case strings.Contains(message, "pktmon failed to start"):
		if strings.Contains(message, "already running") || strings.Contains(message, "already active") {
			return "pktmon_already_running"
		}
		if strings.Contains(message, "access denied") || strings.Contains(message, "accès refusé") || strings.Contains(message, "acces refuse") {
			return "pktmon_permission_denied"
		}
		return "pktmon_start_failed"
	case strings.Contains(message, "input directory is not writable") || strings.Contains(message, "create input directory"):
		return "input_not_writable"
	case strings.Contains(message, "failed to stop pktmon") || strings.Contains(message, "could not be stopped"):
		return "pktmon_stop_failed"
	case strings.Contains(message, "conversion failed"):
		return "etl_conversion_failed"
	case strings.Contains(message, "incomplete capture"):
		return "capture_incomplete"
	case strings.Contains(message, "capture interrupted") || strings.Contains(message, "scan cancelled"):
		return "capture_interrupted"
	case strings.Contains(message, "local diagnostic log"):
		return "diagnostic_log_not_writable"
	default:
		return "scanner_helper_failed"
	}
}

func runPowerShell(script string) ([]byte, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	// PowerShell only brokers UAC and waits for the real scanner process. Hide
	// this intermediary console while keeping the useful nte-scan console open.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.CombinedOutput()
}
