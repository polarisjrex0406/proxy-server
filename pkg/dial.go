package pkg

import (
	"net"
)

type Protocol string

const (
	HTTP   Protocol = "http"
	SOCKS5 Protocol = "socks5"
)

// Dialer - used by providers to connect to the upstream proxy
type Dialer interface {
	Protocol() Protocol
	Dial(uri []byte, addr string, username, password []byte) (net.Conn, error)
}
