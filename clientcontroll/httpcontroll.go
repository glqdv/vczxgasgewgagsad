package clientcontroll

import (
	"flag"
	"log"
	"net"
	"net/http"
	"time"

	"gitee.com/dark.H/ProxyZ/connections/prosocks5"
	"gitee.com/dark.H/gs"
)

const (
	DEFAULT_TIMEOUT = 5 * time.Second
)

var (
	flagAuthUser     = flag.String("httpuser", "", "Server authentication username")
	flagAuthPass     = flag.String("httppass", "", "Server authentication password")
	IsStartHttpProxy = false
)

func (client *ClientControl) listenHttpProxy(httpPort int) (err error) {
	// c := zap.NewProductionConfig()
	// c.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	// logger, err := c.Build()
	if err != nil {
		log.Fatalln("Error: failed to initiate logger")
	}

	proxy := &Proxy{
		ForwardingHTTPProxy: NewForwardingHTTPProxy(),

		AuthUser:           *flagAuthUser,
		AuthPass:           *flagAuthPass,
		DestDialTimeout:    DEFAULT_TIMEOUT,
		DestReadTimeout:    DEFAULT_TIMEOUT,
		DestWriteTimeout:   DEFAULT_TIMEOUT,
		ClientReadTimeout:  DEFAULT_TIMEOUT,
		ClientWriteTimeout: DEFAULT_TIMEOUT,
		// Avoid:              ,
		HandleBody: func(p1 net.Conn, host string, afterConnected func(p1, p2 net.Conn)) {
			raw := prosocks5.HostToRaw(host, -1)
			client.OnBodyBeforeGetRemote(p1, false, raw, host)
		},
	}

	listenAddr := ":" + gs.S(httpPort)
	if client.srv != nil {
		gs.Str("Close old http listener").Println("http proxy")
		client.srv.Close()
	}
	client.srv = &http.Server{
		Addr:    listenAddr.Str(),
		Handler: proxy,
	}
	IsStartHttpProxy = true
	gs.Str("HTTP Proxy Listen in http://localhost:%d").F(httpPort).Println("Service")
	client.srv.ListenAndServe()

	return
}
