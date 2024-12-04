package dialer

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net"
	"time"

	"github.com/omimic12/proxy-server/pkg"
	"github.com/valyala/bytebufferpool"
)

var (
	byteHTTP11  = []byte(" HTTP/1.1\r\n")
	byteConnect = []byte("CONNECT ")
	// byteOKStatus = []byte(" 200")
	byteColon = []byte(":")
)

var (
// errBadStatusCode = errors.New("bad status code")
)

const (
	strProxyBasicAuth = "Proxy-Authorization: Basic "
)

var (
	strRn = []byte("\r\n")
)

type HTTP struct {
	readDeadline time.Duration
	dialTimeout  time.Duration
}

func NewHTTP(dialTimeout, readDeadline time.Duration) *HTTP {
	return &HTTP{dialTimeout: dialTimeout, readDeadline: readDeadline}
}

func (d *HTTP) Dial(uri []byte, addr string, username, password []byte) (rc net.Conn, err error) {
	rc = nil
	err = fmt.Errorf("error while http dial")
	if d.dialTimeout > 0 {
		// rc, err = fasthttp.DialTimeout(addr, d.dialTimeout)
	} else {
		// rc, err = fasthttp.Dial(addr)
	}
	if err != nil {
		return
	}

	r := bytebufferpool.Get()
	r.Write(byteConnect) //nolint:errcheck
	r.Write(uri)         //nolint:errcheck
	r.Write(byteHTTP11)  //nolint:errcheck
	if username != nil && password != nil {
		r.WriteString(strProxyBasicAuth)                                                                      //nolint:errcheck
		r.WriteString(base64.StdEncoding.EncodeToString(bytes.Join([][]byte{username, password}, byteColon))) //nolint:errcheck
		r.Write(strRn)                                                                                        //nolint:errcheck
	}
	r.Write(strRn) //nolint:errcheck

	_, err = r.WriteTo(rc)
	bytebufferpool.Put(r)
	if err != nil {
		return
	}

	// err = rc.SetReadDeadline(time.Now().Add(d.dialTimeout))
	// if err != nil {
	// 	return nil, err
	// }

	// buf := make([]byte, 1024)
	// n, err := rc.Read(buf)
	// if err != nil {
	// 	rc.Close() //nolint:errcheck
	// 	return nil, err
	// }

	// if !bytes.Contains(buf[:n], byteOKStatus) {
	// 	rc.Close() //nolint:errcheck
	// 	return nil, errBadStatusCode
	// }

	return
}

func (d *HTTP) Protocol() pkg.Protocol {
	return pkg.HTTP
}
