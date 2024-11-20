package pkg

import (
	"bytes"
	"encoding/base64"
	"net"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/omimic12/proxy-server/constants"
)

func extractCredentials(req *http.Request) ([]byte, string, error) {
	authStr := req.Header.Get(constants.HeaderProxyAuthorization)

	if authStr == "" {
		return nil, "", ErrMissingAuth
	}

	username, password, ok := parseBasicAuth([]byte(authStr))
	if !ok {
		return nil, "", ErrMissingAuth
	}

	return username, password, nil
}

func parseRequest(hostname string, username []byte, password string, req *Request, parser UsernameParser) error {
	req.Password = password
	req.Host = hostname
	req.Target = hostname
	if strings.Contains(req.Target, ":") {
		var err error
		req.Target, _, err = net.SplitHostPort(req.Host)
		if err != nil {
			return err
		}
	}

	err := parser.Parse(username, req)
	if err != nil {
		return err
	}

	req.ID = RequestKey(req.Password, uuid.New().String())

	return nil
}

func cleanRequestHeaders(request *http.Request) {
	request.Header.Del(constants.HeaderProxyAuthenticate)
	request.Header.Del(constants.HeaderProxyAuthorization)
}

func parseBasicAuth(credentials []byte) (username []byte, password string, ok bool) {
	if !bytes.EqualFold(credentials[:6], strBasic) {
		return
	}

	var buf = make([]byte, base64.StdEncoding.DecodedLen(len(credentials)))
	w, err := base64.StdEncoding.Decode(buf, credentials[6:])
	if err != nil {
		return
	}
	buf = buf[:w]
	s := bytes.IndexByte(buf, ':')
	if s < 0 {
		return
	}
	return buf[:s], string(buf[s+1:]), true
}
