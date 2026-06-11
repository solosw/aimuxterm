package terminal

// terminalIO is the internal interface for Read/Write/Resize/Close operations.
type terminalIO interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Resize(cols, rows uint16) error
	Close() error
}

// Session represents a single terminal session (local or SSH).
type Session struct {
	ID string
	io terminalIO
}

// Read reads output from the session.
func (s *Session) Read(buf []byte) (int, error) { return s.io.Read(buf) }

// Write writes input to the session.
func (s *Session) Write(data []byte) (int, error) { return s.io.Write(data) }

// Resize changes the terminal window size.
func (s *Session) Resize(rows, cols uint16) error { return s.io.Resize(cols, rows) }

// Close terminates the session.
func (s *Session) Close() error { return s.io.Close() }
