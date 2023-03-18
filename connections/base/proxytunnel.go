package base

import (
	"errors"
	"io"
	"net"
	"sync"
	"syscall"
	"time"

	"gitee.com/dark.H/ProxyZ/connections/prodns"
	"gitee.com/dark.H/ProxyZ/connections/prosmux"
	"gitee.com/dark.H/ProxyZ/connections/prosocks5"
	"gitee.com/dark.H/gs"
)

type Protocol interface {
	GetListener() net.Listener
	GetConfig() *ProtocolConfig
	AcceptHandle(waitTime time.Duration, handle func(con net.Conn) error) (err error)
	TryClose()
	GetAliveIPS() gs.List[string]
	DelCon(con net.Conn)
}

type ProxyTunnel struct {
	cons      gs.List[net.Conn]
	alive     int
	lock      sync.RWMutex
	protocl   Protocol
	UseSmux   bool
	On        bool
	ZeroToDel bool
	// vpnHandler   *vpn.VPNHandler
	ControllFunc func(rawHost string, con net.Conn) (err error)
}

func NewProxyTunnel(procol Protocol) *ProxyTunnel {
	p := new(ProxyTunnel)
	p.protocl = procol
	p.UseSmux = true

	return p
}
func (pt *ProxyTunnel) Start(after func()) (err error) {
	pt.On = true
	go func() {
		err := pt.Server(after)
		if err != nil {
			gs.Str("Start proxy err:" + err.Error()).Color("r").Println("service")
		}
	}()
	return
}

func (pt *ProxyTunnel) Server(after func()) (err error) {
	serverPort := pt.GetConfig().ServerPort
	defer func() {
		pt.On = false
		ClosePortUFW(serverPort)
		after()
	}()

	if pt.protocl == nil {
		return errors.New("no protocol set in ProxyTunnel")
	}
	gs.Str("%s in %d id: %s").F(pt.GetConfig().ProxyType, serverPort, pt.GetConfig().ID).Println("service")

	if pt.GetConfig().ProxyType == "quic" {
		// gs.Str(pt.GetConfig().ID + "|" + pt.GetConfig().ProxyType + "| addr:" + pt.GetConfig().RemoteAddr()).Println("Start Quic Server ")
		pt.protocl.AcceptHandle(1*time.Minute, func(con net.Conn) error {
			pt.HandleConnAsync(con)
			return nil
		})

	} else if pt.UseSmux {
		// gs.Str(pt.GetConfig().ID + "|" + pt.GetConfig().ProxyType + "| addr:" + pt.GetConfig().RemoteAddr()).Println("Start Smux Tunnel")
		smux := prosmux.NewSmuxServer(pt.protocl, func(con net.Conn) (err error) {
			pt.HandleConnAsync(con)
			return
		})
		return smux.Server()

	} else {
		// gs.Str(pt.GetConfig().ID + "|" + pt.GetConfig().ProxyType + "| addr:" + pt.GetConfig().RemoteAddr()).Println("Start Tunnel")
		pt.protocl.AcceptHandle(1*time.Minute, func(con net.Conn) error {
			pt.HandleConnAsync(con)
			return nil
		})

	}

	return
}

func (pt *ProxyTunnel) SetWaitToClose() {
	pt.protocl.TryClose()

}

func (pt *ProxyTunnel) SetProtocol(procol Protocol) {
	pt.protocl = procol

}

func (pt *ProxyTunnel) GetConfig() *ProtocolConfig {
	if pt.protocl == nil {
		return nil
	}
	return pt.protocl.GetConfig()
}

func (pt *ProxyTunnel) DelCon(con net.Conn) {
	pt.protocl.DelCon(con)
}

func (pt *ProxyTunnel) SetControllFunc(l func(rawHost string, con net.Conn) (err error)) {
	pt.ControllFunc = l
}

// func (pt *ProxyTunnel) HandleConnTun(con net.Conn) {
// 	defer con.Close()
// 	pt.PipeReadWriteCloser(con, pt.vpnHandler)
// }

func (pt *ProxyTunnel) HandleConnAsync(con net.Conn) {
	// if pt.vpnHandler != nil {
	// 	pt.HandleConnTun(con)
	// 	return
	// }

	host, _, _, err := prosocks5.GetServerRequest(con)
	if err != nil {
		gs.Str(err.Error()).Println("GetServerRequest | err")
		ErrToFile("Server HandleConnection", err)
		// con.Close()
		pt.DelCon(con)
		return
	} else {
		// gs.Str(host).Println("host|ready")
	}

	pt.lock.Lock()
	pt.cons = pt.cons.Add(con)
	pt.alive += 1
	pt.lock.Unlock()
	defer func() {
		pt.lock.Lock()
		pt.alive -= 1
		pt.lock.Unlock()
	}()
	if gs.Str(host).StartsWith("R://") {
		if pt.ControllFunc != nil {
			if err := pt.ControllFunc(host, con); err != nil {
				ErrToFile("server controll func ", err)
			}
		}
	} else if gs.Str(host).StartsWith("dns://") || gs.Str(host).StartsWith("[dns://") {
		host = string(gs.Str(host).Split("[")[1].Split("]")[0])
		pt.DnsNormal(host, con)
	} else {
		pt.TcpNormal(host, con)
	}
}

func (pt *ProxyTunnel) GetClientNum() int {
	return pt.alive
}

func (pt *ProxyTunnel) GetClientIP() gs.List[string] {
	return pt.protocl.GetAliveIPS()
}

func (pt *ProxyTunnel) DnsNormal(host string, con net.Conn) (err error) {
	defer pt.DelCon(con)

	// c.SingleInflight = true
	// laddr := net.UDPAddr{
	// 	IP:   net.ParseIP("[::1]"),
	// 	Port: 12345,
	// 	Zone: "",
	// }
	// c.Dialer = &net.Dialer{
	// 	Timeout:   1 * time.Second,
	// 	LocalAddr: &laddr,
	// }
	gs.Str(host).Println("query")
	if msg, err := prodns.ReplyDNS(gs.Str(host)); err != nil || msg == "" {
		gs.Str(err.Error()).Println("DNS")
		return err
	} else {

		con.Write(msg.Bytes())
		if m, err := prodns.UnpackDNS(msg); err == nil && m != nil {
			if len(m.Question) > 0 {
				if len(m.Answer) > 0 {
					gs.Str("%s -> %s ").F(gs.Str(m.Question[0].Name).Color("y"), gs.Str(m.Answer[0].String()).Color("g")).Println("dns")
				}
			}
		}

	}
	return
}

func (pt *ProxyTunnel) TcpNormal(host string, con net.Conn) (err error) {
	defer pt.DelCon(con)
	remoteConn, err := net.Dial("tcp", host)
	if err != nil {
		if ne, ok := err.(*net.OpError); ok && (ne.Err == syscall.EMFILE || ne.Err == syscall.ENFILE) {
			// log too many open file error
			// EMFILE is process reaches open file limits, ENFILE is system limit
			ErrToFile("dial error too many file!!:", err)
		} else {
			ErrToFile("tcp normal", err)
		}
		gs.Str(host + "|" + err.Error()).Println("host|failed")
		// log.Println("X connect to ->", host)
		return err
	}
	// gs.Str(host).Println("host|ok")
	// con.SetWriteDeadline(time.Now().Add(2 * time.Minute))
	_, err = con.Write(prosocks5.Socks5Confirm)
	if err != nil {
		ErrToFile("back con is break", err)
		remoteConn.Close()
		return
	}
	gs.Str(host).Println("host|build")
	pt.Pipe(remoteConn, con)
	return
}

func (pt *ProxyTunnel) Pipe(p1, p2 net.Conn) {
	var wg sync.WaitGroup
	// var wait = 39 * time.Second
	wg.Add(1)
	streamCopy := func(dst net.Conn, src net.Conn, fr, to net.Addr) {
		// startAt := time.Now()
		Copy(dst, src)
		// dst.SetReadDeadline(time.Now().Add(wait))
		p1.Close()
		p2.Close()
		// }()
	}

	go func(p1, p2 net.Conn) {
		wg.Done()
		streamCopy(p1, p2, p2.RemoteAddr(), p1.RemoteAddr())
	}(p1, p2)
	streamCopy(p2, p1, p1.RemoteAddr(), p2.RemoteAddr())
	wg.Wait()
}

func (pt *ProxyTunnel) PipeReadWriteCloser(p1, p2 io.ReadWriteCloser) {
	var wg sync.WaitGroup
	// var wait = 39 * time.Second
	wg.Add(1)
	streamCopy := func(dst, src io.ReadWriteCloser) {
		Copy(dst, src)
		p1.Close()
		p2.Close()

	}
	go func(p1, p2 io.ReadWriteCloser) {
		wg.Done()
		streamCopy(p1, p2)
	}(p1, p2)
	streamCopy(p2, p1)
	wg.Wait()
}
