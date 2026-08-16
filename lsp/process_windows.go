//go:build windows

package lsp

import (
	"os/exec"
	"syscall"
)

// configureBackgroundProcess prevents console-based language servers from
// creating a visible console window when they are launched by the GUI app.
func configureBackgroundProcess(cmd *exec.Cmd) {
	const createNoWindow = 0x08000000
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
