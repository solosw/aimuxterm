//go:build !windows

package lsp

import "os/exec"

func configureBackgroundProcess(cmd *exec.Cmd) {}
