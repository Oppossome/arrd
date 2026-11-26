// Package console gives arrd somewhere to write when it is built as a
// Windows GUI program (-ldflags -H=windowsgui), which starts without a
// console window and so without stdout or stderr.
//
// On other platforms the standard streams are always left as they are.
package console
