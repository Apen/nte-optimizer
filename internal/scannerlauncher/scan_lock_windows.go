//go:build windows

package scannerlauncher

import (
	"errors"
	"fmt"
	"runtime"
	"sync"

	"golang.org/x/sys/windows"
)

// AcquireScanLock prevents concurrent scans from this account and Windows
// session, including scans started by another application instance. The mutex
// is acquired by the calling goroutine and released on its pinned OS thread.
func AcquireScanLock(_ string) (func(), error) {
	runtime.LockOSThread()

	name, err := windows.UTF16PtrFromString(`Local\NTEOptimizer.Scan.v1`)
	if err != nil {
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("create scan lock identity: %w", err)
	}
	handle, createErr := windows.CreateMutex(nil, false, name)
	if handle == 0 {
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("create scan lock: %w", createErr)
	}
	if createErr != nil && !errors.Is(createErr, windows.ERROR_ALREADY_EXISTS) {
		_ = windows.CloseHandle(handle)
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("create scan lock: %w", createErr)
	}

	state, waitErr := windows.WaitForSingleObject(handle, 0)
	if waitErr != nil {
		_ = windows.CloseHandle(handle)
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("wait for scan lock: %w", waitErr)
	}
	switch state {
	case windows.WAIT_OBJECT_0, windows.WAIT_ABANDONED:
		var once sync.Once
		return func() {
			once.Do(func() {
				_ = windows.ReleaseMutex(handle)
				_ = windows.CloseHandle(handle)
				runtime.UnlockOSThread()
			})
		}, nil
	case uint32(windows.WAIT_TIMEOUT):
		_ = windows.CloseHandle(handle)
		runtime.UnlockOSThread()
		return nil, ErrScanAlreadyRunning
	default:
		_ = windows.CloseHandle(handle)
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("unexpected scan lock wait result: %d", state)
	}
}
