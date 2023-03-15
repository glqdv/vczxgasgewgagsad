package protls

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"gitee.com/dark.H/ProxyZ/asset"
	"gitee.com/dark.H/ProxyZ/connections/base"
	"gitee.com/dark.H/gs"
)

var (
	CERT              = "Resources/pem/cert.pem"
	KEYPEM            = "Resources/pem/key.pem"
	SHARED_TLS_CONFIG *tls.Config
	SHARED_TLS_KEY    = ""
)

// KcpServer used for server
type TlsServer struct {
	config    *base.ProtocolConfig
	tlsconfig *tls.Config
	ips       gs.Dict[bool]
	// RedirectMode  bool
	// TunnelChan     chan Channel
	// TcpListenPorts map[string]int
	AcceptConn int
	lock       sync.RWMutex
	ZeroToDel  bool
	// RedirectBook  *utils.Config
}

func GetTlsConfig() *tls.Config {
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
			InsecureSkipVerify: true,
		}
	}
	return SHARED_TLS_CONFIG

}

func (tlsServer *TlsServer) Record(con net.Addr) {
	ip := con.String()
	if tlsServer.ips == nil {
		tlsServer.ips = make(gs.Dict[bool])
	}
	if _, ok := tlsServer.ips[ip]; !ok {
		tlsServer.lock.Lock()
		tlsServer.ips[ip] = true
		tlsServer.lock.Unlock()
	}
}

func (tlsServer *TlsServer) DelRecord(con net.Conn) {
	if tlsServer.ips == nil {
		tlsServer.ips = make(gs.Dict[bool])
	}
	ip := con.RemoteAddr().String()
	if _, ok := tlsServer.ips[ip]; !ok {
		tlsServer.lock.Lock()
		delete(tlsServer.ips, ip)
		tlsServer.lock.Unlock()
	}

}

func (tlsServer *TlsServer) GetAliveIPS() gs.List[string] {
	ds := gs.List[string]{}
	for k := range tlsServer.ips {
		ds = append(ds, k)
	}
	return ds
}

func (tlsServer *TlsServer) AcceptHandle(waitTime time.Duration, handle func(con net.Conn) error) (err error) {
	wait10minute := time.NewTicker(1 * time.Minute)
	listener := tlsServer.GetListener()
	for {
	LOOP:
		select {
		case <-wait10minute.C:
			break LOOP
		default:
			if tlsServer.ZeroToDel {
				break
			} else {
				wait10minute.Reset(waitTime)
			}
		}
		if listener == nil {
			return errors.New("listenre is closed")
		}

		con, err := listener.Accept()
		if err != nil {
			listener.Close()
			return err
		}
		tlsServer.Record(con.RemoteAddr())
		go handle(con)
	}
	// return
}

func (tlsServer *TlsServer) TryClose() {
	tlsServer.ZeroToDel = true
}

func (tlsserver *TlsServer) Accept() (con net.Conn, err error) {
	listener := tlsserver.GetListener()
	if listener == nil {
		return nil, errors.New("get listener err! in kcp")
	}
	con, err = listener.Accept()
	if err != nil {
		return
	}
	tlsserver.AcceptConn += 1
	return
}

func (kserver *TlsServer) DelCon(con net.Conn) {
	con.Close()
	kserver.DelRecord(con)
	kserver.AcceptConn -= 1
}

func (tlsserver *TlsServer) GetListener() net.Listener {
	address := gs.Str("%s:%d").F(tlsserver.config.Server, tlsserver.config.ServerPort).Str()
	listenr, err := tls.Listen("tcp", address, tlsserver.tlsconfig)
	if err != nil {
		return nil
	}
	return listenr
}

func (kserver *TlsServer) GetConfig() *base.ProtocolConfig {
	return kserver.config
}

func NewTlsServer(config *base.ProtocolConfig) *TlsServer {
	k := new(TlsServer)

	k.tlsconfig = GetTlsConfig()
	config.Password = SHARED_TLS_KEY
	config.ProxyType = "tls"
	k.config = config

	return k
}
