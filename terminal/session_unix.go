//go:build !windows

package terminal

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// newLocalSession creates a local Unix PTY session.
func newLocalSession(id, cwd string) (*Session, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	c := exec.Command(shell)
	if cwd != "" {
		c.Dir = cwd
	}
	c.Env = append(os.Environ(), "TERM=xterm-256color")
	f, err := pty.StartWithSize(c, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		return nil, err
	}
	return &Session{ID: id, io: &unixIO{f: f, cmd: c}}, nil
}

type unixIO struct {
	f   *os.File
	cmd *exec.Cmd
}

func (u *unixIO) Read(buf []byte) (int, error)  { return u.f.Read(buf) }
func (u *unixIO) Write(data []byte) (int, error) { return u.f.Write(data) }
func (u *unixIO) Resize(cols, rows uint16) error {
	return pty.Setsize(u.f, &pty.Winsize{Rows: rows, Cols: cols})
}
func (u *unixIO) Close() error {
	u.f.Close()
	return u.cmd.Wait()
}
