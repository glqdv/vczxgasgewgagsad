package clientcontroll

import (
	"encoding/base64"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

// Proxy is a HTTPS forward proxy.
type Proxy struct {
	AuthUser            string
	AuthPass            string
	Avoid               string
	HandleBody          func(p1 net.Conn, host string, afterConnected func(p1, p2 net.Conn))
	ForwardingHTTPProxy *httputil.ReverseProxy
	DestDialTimeout     time.Duration
	DestReadTimeout     time.Duration
	DestWriteTimeout    time.Duration
	ClientReadTimeout   time.Duration
	ClientWriteTimeout  time.Duration
	myServer            *http.Server
}

var WaitClose = make(chan int, 5)

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	select {
	case <-WaitClose:
		if err := p.myServer.Close(); err != nil {
			log.Fatal(err)
		}
	default:

		if p.AuthUser != "" && p.AuthPass != "" {
			user, pass, ok := parseBasicProxyAuth(r.Header.Get("Proxy-Authorization"))
			if !ok || user != p.AuthUser || pass != p.AuthPass {
				// p.Logger.Info("Authorization attempt with invalid credentials")
				http.Error(w, http.StatusText(http.StatusProxyAuthRequired), http.StatusProxyAuthRequired)
				return
			}
		}

		if r.URL.Scheme == "http" {
			p.handleHTTP(w, r)
		} else {
			p.handleTunneling(w, r)
		}
	}

}

func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if p.Avoid != "" && strings.Contains(r.Host, p.Avoid) == true {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusMethodNotAllowed)
		return
	}
	p.ForwardingHTTPProxy.ServeHTTP(w, r)
}

func (p *Proxy) handleTunneling(w http.ResponseWriter, r *http.Request) {

	if p.Avoid != "" && strings.Contains(r.Host, p.Avoid) == true {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusMethodNotAllowed)
		return
	}

	if r.Method != http.MethodConnect {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	go p.HandleBody(clientConn, r.Host, func(p1, p2 net.Conn) {
	})
	// now := time.Now()
	// clientConn.SetReadDeadline(now.Add(p.ClientReadTimeout))
	// clientConn.SetWriteDeadline(now.Add(p.ClientWriteTimeout))
	// destConn.SetReadDeadline(now.Add(p.DestReadTimeout))
	// destConn.SetWriteDeadline(now.Add(p.DestWriteTimeout))

	// go transfer(destConn, clientConn)
	// go transfer(clientConn, destConn)
}

func transfer(dest io.WriteCloser, src io.ReadCloser) {
	defer func() { _ = dest.Close() }()
	defer func() { _ = src.Close() }()
	_, _ = io.Copy(dest, src)
}

// parseBasicProxyAuth parses an HTTP Basic Authorization string.
// "Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==" returns ("Aladdin", "open sesame", true).
func parseBasicProxyAuth(authz string) (username, password string, ok bool) {
	const prefix = "Basic "
	if !strings.HasPrefix(authz, prefix) {
		return
	}
	c, err := base64.StdEncoding.DecodeString(authz[len(prefix):])
	if err != nil {
		return
	}
	cs := string(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		return
	}
	return cs[:s], cs[s+1:], true
}

// NewForwardingHTTPProxy retuns a new reverse proxy that takes an incoming
// request and sends it to another server, proxying the response back to the
// client.
//
// See: https://golang.org/pkg/net/http/httputil/#ReverseProxy
func NewForwardingHTTPProxy() *httputil.ReverseProxy {
	director := func(req *http.Request) {
		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			req.Header.Set("User-Agent", "")
		}
	}
	// TODO:(alesr) Use timeouts specified via flags to customize the default
	// transport used by the reverse proxy.
	return &httputil.ReverseProxy{
		// ErrorLog: logger,
		Director: director,
	}
}
