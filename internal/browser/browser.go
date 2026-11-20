// Package browser opens a URL in the user's default web browser, and a file
// in the program for its type.
package browser

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Open hands url to the operating system's URL handler and returns once the
// handler has been started.
func Open(url string) error {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return fmt.Errorf("refusing to open %q: not an http or https URL", url)
	}
	if err := hand(url); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return nil
}

// OpenFile hands a file to the program the operating system opens its type
// with, and returns once that has been started. The caller decides which
// files are safe to open: a program file would be run.
func OpenFile(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("refusing to open %q: not a full path", path)
	}
	if err := hand(path); err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	return nil
}

// hand gives a URL or a path to the operating system's handler.
func hand(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
