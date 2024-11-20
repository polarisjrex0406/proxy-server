package pkg

import (
	"io"
)

type Sessions interface {
	io.Closer
	Start(*Request) error
	Cached(*Request) (Provider, bool)
}
