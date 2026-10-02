//go:build !windows

package main

import (
	"context"
	"fmt"
	"time"
)

func captureLoginAutoWithProgressAndRecovery(context.Context, string, time.Duration, bool, func(string, string)) (string, error) {
	return "", fmt.Errorf("guided capture is available only on Windows")
}
