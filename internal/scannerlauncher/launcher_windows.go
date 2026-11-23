//go:build windows

package scannerlauncher

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"nte-optimizer/internal/scanlog"
)

func runElevated(helper, projectDir, outputDir, cancelFile string, seconds int) error {
	return runElevatedWithRunner(helper, projectDir, outputDir, cancelFile, seconds, runElevatedProcess)
}

func runElevatedWithRecovery(helper, projectDir, outputDir, cancelFile string, seconds int, stopPktmonFirst bool) error {
	return runElevatedWithRecoveryWithRunner(helper, projectDir, outputDir, cancelFile, seconds, stopPktmonFirst, runElevatedProcess)
}

func runElevatedWithRunner(helper, projectDir, outputDir, cancelFile string, seconds int, run func(string, string, string) ([]byte, error)) error {
	return runElevatedWithRecoveryWithRunner(helper, projectDir, outputDir, cancelFile, seconds, false, run)
}

func runElevatedWithRecoveryWithRunner(helper, projectDir, outputDir, cancelFile string, seconds int, stopPktmonFirst bool, run func(string, string, string) ([]byte, error)) error {
	return runElevatedOptionsWithRunner(helper, projectDir, outputDir, cancelFile, Options{Seconds: seconds, StopPktmonFirst: stopPktmonFirst}, run)
}

func runElevatedWithOptions(helper, projectDir, outputDir, cancelFile string, options Options) error {
	return runElevatedOptionsWithRunner(helper, projectDir, outputDir, cancelFile, options, runElevatedProcess)
}

func runElevatedOptionsWithRunner(helper, projectDir, outputDir, cancelFile string, options Options, run func(string, string, string) ([]byte, error)) error {
	argumentList := []string{
		"-login-capture",
		"-login-seconds", strconv.Itoa(options.Seconds),
		"-output-dir", syscall.EscapeArg(outputDir),
		"-cancel-file", syscall.EscapeArg(cancelFile),
		"-scan-log", syscall.EscapeArg(scanlog.Path(filepath.Dir(filepath.Dir(outputDir)))),
	}
	if options.StopPktmonFirst {
		argumentList = append(argumentList, "-stop-pktmon-first")
	}
	if options.RetainCapture {
		argumentList = append(argumentList, "-retain-capture")
	}
	arguments := strings.Join(argumentList, " ")
	if output, err := run(helper, projectDir, arguments); err != nil {
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

const (
	seeMaskNoCloseProcess = 0x00000040
	swShowNormal          = 1
)

var shellExecuteEx = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

type shellExecuteInfo struct {
	cbSize        uint32
	fMask         uint32
	hwnd          uintptr
	verb          *uint16
	file          *uint16
	parameters    *uint16
	directory     *uint16
	nShow         int32
	hInstApp      uintptr
	idList        uintptr
	class         *uint16
	hkeyClass     uintptr
	hotKey        uint32
	iconOrMonitor uintptr
	process       windows.Handle
}

func runElevatedProcess(helper, projectDir, arguments string) ([]byte, error) {
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return nil, err
	}
	file, err := windows.UTF16PtrFromString(helper)
	if err != nil {
		return nil, err
	}
	parameters, err := windows.UTF16PtrFromString(arguments)
	if err != nil {
		return nil, err
	}
	directory, err := windows.UTF16PtrFromString(projectDir)
	if err != nil {
		return nil, err
	}

	info := shellExecuteInfo{
		cbSize:     uint32(unsafe.Sizeof(shellExecuteInfo{})),
		fMask:      seeMaskNoCloseProcess,
		verb:       verb,
		file:       file,
		parameters: parameters,
		directory:  directory,
		nShow:      swShowNormal,
	}
	result, _, callErr := shellExecuteEx.Call(uintptr(unsafe.Pointer(&info)))
	runtime.KeepAlive(verb)
	runtime.KeepAlive(file)
	runtime.KeepAlive(parameters)
	runtime.KeepAlive(directory)
	runtime.KeepAlive(info)
	if result == 0 {
		if callErr == nil || callErr == syscall.Errno(0) {
			callErr = windows.ERROR_GEN_FAILURE
		}
		return nil, callErr
	}
	if info.process == 0 {
		return nil, fmt.Errorf("administrator scanner process handle is unavailable")
	}
	defer windows.CloseHandle(info.process)

	waitResult, err := windows.WaitForSingleObject(info.process, windows.INFINITE)
	if err != nil {
		return nil, fmt.Errorf("wait for administrator scanner: %w", err)
	}
	if waitResult != windows.WAIT_OBJECT_0 {
		return nil, fmt.Errorf("wait for administrator scanner returned %d", waitResult)
	}
	var exitCode uint32
	if err := windows.GetExitCodeProcess(info.process, &exitCode); err != nil {
		return nil, fmt.Errorf("read administrator scanner exit code: %w", err)
	}
	if exitCode != 0 {
		return nil, fmt.Errorf("exit status %d", exitCode)
	}
	return nil, nil
}
