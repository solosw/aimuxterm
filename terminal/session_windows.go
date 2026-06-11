//go:build windows

package terminal

import (
	"os/exec"

	"github.com/UserExistsError/conpty"
)

// newLocalSession creates a local Windows ConPTY session.
func newLocalSession(id, cwd string) (*Session, error) {
	shell := "bash"
	if _, err := exec.LookPath("powershell.exe"); err == nil {
		shell = `powershell.exe -NoLogo -NoExit`
	} else {
		shell = `cmd.exe`
	}
	c, err := conpty.Start(shell)
	if err != nil {
		return nil, err
	}
	return &Session{ID: id, io: &winIO{cpty: c}}, nil
}

type winIO struct {
	cpty *conpty.ConPty
}

func (w *winIO) Read(buf []byte) (int, error)  { return w.cpty.Read(buf) }
func (w *winIO) Write(data []byte) (int, error) { return w.cpty.Write(data) }
func (w *winIO) Resize(cols, rows uint16) error { return w.cpty.Resize(int(cols), int(rows)) }
func (w *winIO) Close() error                   { return w.cpty.Close() }
