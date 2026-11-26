// Package closer reports errors from deferred Close calls instead of
// dropping them.
package closer

import (
	"errors"
	"io"
)

// Join closes c and joins its error into *errp. Defer it from a function
// with a named error result:
//
//	func read(path string) (err error) {
//		f, err := os.Open(path)
//		...
//		defer closer.Join(&err, f)
//
// *errp is left untouched when Close succeeds, so a sentinel error the
// function returned keeps its identity.
func Join(errp *error, c io.Closer) {
	if err := c.Close(); err != nil {
		*errp = errors.Join(*errp, err)
	}
}
