// Package serialui provides implementation of User Interface built around
// keyboard interaction using commands and log buffers.
//
// Subpackages provide implemenations of primitives used by this package.
package serialui

// TODO: Proper documentation for serial UI model.

type UI interface {
	ColorMsg(buffer, sender string, format string, args ...interface{})
	Error(buffer, format string, args ...interface{})
	Msg(buffer, sender string, format string, args ...interface{})
	Write(b []byte) (int, error)

	ReadLine() (buffer string, line string, err error)

	SetCurrentBuffer(name string)
	CurrentBuffer() string

	Close() error
}
