package errhelper

import (
	"fmt"
	"io"
)

type e struct {
	prefix string
	nested error
}

func (err e) Error() string {
	return err.prefix + ": " + err.nested.Error()
}

func (err e) Unwrap() error {
	return err.nested
}

// H structure is a simple utility that does two things:
// - Collects on-failure cleanup functions
// - Wraps errors on failure
//
// Intended use is as follows:
//      func foo() error {
//          h := errhelper.New("foo")
//
//          complexThing, err = initA()
//          if err != nil {
//              return h.Fail(err)
//          }
//          h.CleanupClose(complexThing.Close)
//
//          anotherThing, err = initB()
//          if err != nil {
//              // Will close complexThing.
//              return h.Fail(err)
//          }
//
//          ...
//      }
type H struct {
	deferList []func()
	Wrap      func(error) error
}

// Cleanup adds a function to be executed on error.
func (h H) Cleanup(f func()) {
	h.deferList = append(h.deferList, f)
}

// CleanupClose add a io.Closer to be clsoed on error.
func (h H) CleanupClose(c io.Closer) {
	h.deferList = append(h.deferList, func() {
		c.Close()
	})
}

// Fail executes functions previously pased to Cleanup in the reverse order
// and return an error wrapped using h.Wrap function.
//
// If you just want to execute cleanup functions - use RunCleanup.
//
// Does nothing if err is nil.
func (h *H) Fail(err error) error {
	if err == nil {
		return nil
	}

	h.RunCleanup()
	return h.Wrap(err)
}

// Fail executes functions previously pased to Cleanup.
// The cleanup list is reset.
func (h *H) RunCleanup() {
	list := h.deferList
	for i := len(list) - 1; i >= 0; i-- {
		list[i]()
		// "Pop" elements from the defer list so in case of panic we will not
		// reexecute the same functions on the next RunCleanup.
		h.deferList = h.deferList[:i]
	}
}

func New(format string, args ...interface{}) H {
	prefix := fmt.Sprintf(format, args...)

	return H{
		Wrap: func(err error) error {
			return e{prefix: prefix, nested: err}
		},
	}
}
