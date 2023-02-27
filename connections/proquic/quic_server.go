package proquic

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log"
	"net"
	"time"

	"gitee.com/dark.H/ProxyZ/asset"
	"gitee.com/dark.H/ProxyZ/connections/base"
	"gitee.com/dark.H/gs"
	"github.com/quic-go/quic-go"
)

type QuicServer struct {
	config     *base.ProtocolConfig
	tlsconfig  *tls.Config
	AcceptConn int
	ZeroToDel  bool
	handleConn func(con net.Conn) error
}

// type QuicLister struct {
// 	l quic.Listener
// }

// func (q *QuicConn) Accept() (con net.Conn, err error) {

// }

func GetQuicConfig() *tls.Config {
	if SHARED_TLS_CONFIG == nil {
		cerPEM, err := asset.Asset(CERT)
		if err != nil {
			log.Fatal(err)
		}
		keyPEM, err := asset.Asset(KEYPEM)
		if err != nil {
			log.Fatal(err)
		}
		SHARED_TLS_KEY = string(cerPEM) + "|" + string(keyPEM)

		// Load the certificate and private key
		cert, err := tls.X509KeyPair(cerPEM, keyPEM)
		if err != nil {
			panic(err)
		}
		certpool := x509.NewCertPool()
		certpool.AppendCertsFromPEM(cerPEM)

		SHARED_TLS_CONFIG = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            certpool,
			ClientCAs:          certpool,
			InsecureSkipVerify: false,
			NextProtos:         []string{"quic-echo-stream"},
		}
	}
	return SHARED_TLS_CONFIG

}

func (quicServe *QuicServer) GetListener() (ls net.Listener) {
	return
}

func (quicServe *QuicServer) AcceptHandle(waitTime time.Duration, handle func(con net.Conn) error) (err error) {
	address := gs.Str("%s:%d").F(quicServe.config.Server, quicServe.config.ServerPort).Str()
	wait10minute := time.NewTicker(1 * time.Minute)
	listener, err := quic.ListenAddr(address, quicServe.tlsconfig, nil)
	if err != nil {
		return err
	}
	quicServe.handleConn = handle
	for {
	LOOP:
		select {
		case <-wait10minute.C:
			break LOOP
		default:
			if quicServe.ZeroToDel {
				break
			} else {
				wait10minute.Reset(waitTime)
			}
		}
		if listener == nil {
			return errors.New("listener is closed!")
		}
		con, err := listener.Accept(context.Background())
		if err != nil {
			return err
		}
		go quicServe.accpeStream(con)
	}

	// return
}

func (quicServer *QuicServer) TryClose() {
	quicServer.ZeroToDel = true
}

func (quicServer *QuicServer) accpeStream(con quic.Connection) (err error) {
	// defer con.CloseWithError(quic.StreamErrorCode)
	for {
		stream, err := con.AcceptStream(context.Background())
		if err != nil {
			return err
		}

		go func() {
			quicServer.handleConn(WrapQuicNetConn(stream, con.RemoteAddr(), con.LocalAddr()))
			quicServer.AcceptConn -= 1
		}()
		quicServer.AcceptConn += 1
	}
	// return
}

func (quicServe *QuicServer) SetCon(handle func(con net.Conn) error) {
	quicServe.handleConn = handle
}

func (quicServe *QuicServer) DelCon(con net.Conn) {
	con.Close()
	quicServe.AcceptConn -= 1
}

// func (quicServe *QuicServer) GetListener() net.Listener {
// 	if err != nil {
// 		return nil
// 	}
// 	return listenr
// }

func (kserver *QuicServer) GetConfig() *base.ProtocolConfig {
	return kserver.config
}

func NewQuicServer(config *base.ProtocolConfig) *QuicServer {
	k := new(QuicServer)

	k.tlsconfig = GetQuicConfig()
	config.Password = SHARED_TLS_KEY
	config.ProxyType = "quic"
	k.config = config

	return k
}
