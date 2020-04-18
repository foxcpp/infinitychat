// Package serialui provides implementation of User Interface built around
// keyboard interaction using commands and log buffers.
//
// Subpackages provide implemenations of primitives used by this package.
package serialui

// TODO: Proper documentation for serial UI model.

type UI interface {
	ColorMsg(prefix string, local bool, format string, args ...interface{})
	Error(prefix string, local bool, format string, args ...interface{})
	Msg(prefix string, local bool, format string, args ...interface{})
	Write(b []byte) (int, error)

	ReadLine() (string, error)

	SetCurrentChat(descriptor string)
	CurrentChat() string

	Close() error
}
