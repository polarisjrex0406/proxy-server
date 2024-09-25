package internal

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
)

type ForwardProxy struct {
}

func (p *ForwardProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	fmt.Printf("ForwardProxy - ServeHTTP - %v\n", req.Method)
	if req.Method == http.MethodConnect {
		ProxyConnect(w, req)
	} else {
		http.Error(w, "this proxy only supports CONNECT", http.StatusMethodNotAllowed)
	}
}

func ProxyConnect(w http.ResponseWriter, req *http.Request) {
	log.Printf("CONNECT requested to %v (from %v)", req.Host, req.RemoteAddr)
	targetConn, err := net.Dial("tcp", req.Host)
	if err != nil {
		log.Println("failed to dial to target", req.Host)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	hj, ok := w.(http.Hijacker)
	if !ok {
		log.Fatal("http server doesn't support hijacking connection")
	}

	clientConn, _, err := hj.Hijack()
	if err != nil {
		log.Fatal("http hijacking failed")
	}

	log.Println("tunnel established")
	go TunnelConn(targetConn, clientConn)
	go TunnelConn(clientConn, targetConn)
}

func TunnelConn(dst io.WriteCloser, src io.ReadCloser) {
	io.Copy(dst, src)
	dst.Close()
	src.Close()
}
