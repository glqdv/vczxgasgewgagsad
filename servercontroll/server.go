package servercontroll

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"gitee.com/dark.H/ProxyZ/asset"
	"gitee.com/dark.H/ProxyZ/connections/base"

	"gitee.com/dark.H/gs"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
)

var (
	CERT    = "Resources/pem/cert.pem"
	KEYPEM  = "Resources/pem/key.pem"
	Tunnels = gs.List[*base.ProxyTunnel]{}
	GLOCK   = sync.RWMutex{}
)

func Reply(w io.Writer, msg any, status bool) {
	if status {
		fmt.Fprintf(w, string(gs.Dict[any]{
			"status": "ok",
			"msg":    msg,
		}.Json()))
	} else {
		fmt.Fprintf(w, string(gs.Dict[any]{
			"status": "fail",
			"msg":    msg,
		}.Json()))

	}
}

func Recv(r io.Reader) (d gs.Dict[any], err error) {
	buf, err := ioutil.ReadAll(r)
	if err != io.EOF && err != nil {
		// w.WriteHeader(400)
		return nil, err
	}
	if len(buf) == 0 {
		return nil, nil
	}
	d = make(gs.Dict[any])
	err = json.Unmarshal(buf, &d)
	return
}

func HTTP3Server(serverAddr, wwwDir string, useQuic bool) {
	base.OpenPortUFW(gs.Str(serverAddr).Split(":")[1].TryInt())
	quicConf := &quic.Config{}
	handler := setupHandler(wwwDir)

	cerPEM, err := asset.Asset(CERT)
	if err != nil {
		log.Fatal(err)
	}
	keyPEM, err := asset.Asset(KEYPEM)
	if err != nil {
		log.Fatal(err)
	}

	// Load the certificate and private key
	cert, err := tls.X509KeyPair(cerPEM, keyPEM)
	if err != nil {
		panic(err)
	}

	// Create a TLS configuration
	certpool := x509.NewCertPool()
	certpool.AppendCertsFromPEM(cerPEM)
	tlsconfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            certpool,
		ClientCAs:          certpool,
		InsecureSkipVerify: false,
	}

	if useQuic {
		server := http3.Server{
			Handler:    handler,
			Addr:       serverAddr,
			QuicConfig: quicConf,
			TLSConfig:  tlsconfig,
		}
		// Bind to a port and listen for incoming connections
		gs.Str(server.Addr).Println("QUIC HTTP")
		err = server.ListenAndServe()
		if err != nil {
			log.Println("listen server tls err:", err)
		}

	} else {
		server := &http.Server{
			Handler:      handler,
			ReadTimeout:  7 * time.Second,
			WriteTimeout: 12 * time.Second,
			IdleTimeout:  120 * time.Second,
			Addr:         serverAddr,
			TLSConfig:    tlsconfig,
		}
		gs.Str(server.Addr).Println("TLS HTTP")
		ln, err := tls.Listen("tcp", serverAddr, tlsconfig)
		if err != nil {
			log.Println("listen server tls err:", err)
		}
		server.Serve(ln)
		// server.Serve()
		if err != nil {
			log.Println("https server tls err:", err)
		}
	}
}
