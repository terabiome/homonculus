package templator

import "io"

type Template interface {
	Execute(wr io.Writer, data any) error
}
