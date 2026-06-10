package common

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// SysLog logs a system-level info message
// This is a compatibility wrapper that uses slog internally
func SysLog(s string) {
	l := ensureSlogInit()
	// Use custom format to match legacy output: [SYS] timestamp | message
	t := time.Now()
	_, _ = fmt.Fprintf(gin.DefaultWriter, "[SYS] %v | %s \n", t.Format("2006/01/02 - 15:04:05"), s)
	// Also log to slog for structured logging
	l.Info(s, "source", "system")
}

// SysError logs a system-level error message
// This is a compatibility wrapper that uses slog internally
func SysError(s string) {
	l := ensureSlogInit()
	// Use custom format to match legacy output: [SYS] timestamp | message
	t := time.Now()
	_, _ = fmt.Fprintf(gin.DefaultErrorWriter, "[SYS] %v | %s \n", t.Format("2006/01/02 - 15:04:05"), s)
	// Also log to slog for structured logging
	l.Error(s, "source", "system")
}

// FatalLog logs a fatal message and exits the program
func FatalLog(v ...any) {
	l := ensureSlogInit()
	t := time.Now()
	msg := fmt.Sprint(v...)
	_, _ = fmt.Fprintf(gin.DefaultErrorWriter, "[FATAL] %v | %v \n", t.Format("2006/01/02 - 15:04:05"), msg)
	// Also log to slog for structured logging
	l.Log(context.Background(), slog.LevelError+4, msg, "source", "fatal")
	os.Exit(1)
}

// LogStartupSuccess logs the startup success message with formatted output
func LogStartupSuccess(startTime time.Time, port string) {
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	// Get network IPs
	networkIps := GetNetworkIps()

	// Print blank line for spacing
	fmt.Fprintf(gin.DefaultWriter, "\n")

	// Print the main success message
	fmt.Fprintf(gin.DefaultWriter, "  \033[32m%s %s\033[0m  ready in %d ms\n", SystemName, Version, durationMs)
	fmt.Fprintf(gin.DefaultWriter, "\n")

	// Skip fancy startup message in container environments
	if !IsRunningInContainer() {
		// Print local URL
		fmt.Fprintf(gin.DefaultWriter, "  ➜  \033[1mLocal:\033[0m   http://localhost:%s/\n", port)
	}

	// Print network URLs
	for _, ip := range networkIps {
		fmt.Fprintf(gin.DefaultWriter, "  ➜  \033[1mNetwork:\033[0m http://%s:%s/\n", ip, port)
	}

	// Print blank line for spacing
	fmt.Fprintf(gin.DefaultWriter, "\n")

	// Also log to slog for structured logging
	LogInfo(context.Background(), "Server started",
		"name", SystemName,
		"version", Version,
		"port", port,
		"startup_time_ms", durationMs,
	)
}

// SysLogf logs a formatted system-level info message
func SysLogf(format string, args ...any) {
	SysLog(fmt.Sprintf(format, args...))
}

// SysErrorf logs a formatted system-level error message
func SysErrorf(format string, args ...any) {
	SysError(fmt.Sprintf(format, args...))
}
