package util

import (
	"fmt"
	stdlog "log"
	"os"
	"path/filepath"
)

type serverErrorLogWriter struct {
	file *os.File
}

func (w *serverErrorLogWriter) Write(p []byte) (int, error) {
	return w.file.Write(p)
}

func NewServerErrorLog() *stdlog.Logger {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			panic(fmt.Sprintf("Failed to get user home directory: %v", err))
		}
		stateDir = filepath.Join(homeDir, ".local", "state")
	}

	appName := "tsgrok"
	logDir := filepath.Join(stateDir, appName)
	logPath := filepath.Join(logDir, "app.log")

	if err := os.MkdirAll(logDir, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create log directory %s: %v", logDir, err))
	}

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		panic(fmt.Sprintf("Failed to open server error log file %s: %v", logPath, err))
	}

	return stdlog.New(&serverErrorLogWriter{file: file}, "", stdlog.LstdFlags)
}
