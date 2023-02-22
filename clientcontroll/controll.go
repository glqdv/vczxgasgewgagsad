package clientcontroll

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"gitee.com/dark.H/ProxyZ/connections/base"
	"gitee.com/dark.H/ProxyZ/connections/prodns"
	"gitee.com/dark.H/ProxyZ/connections/prokcp"
	"gitee.com/dark.H/ProxyZ/connections/proquic"
	"gitee.com/dark.H/ProxyZ/connections/prosmux"
	"gitee.com/dark.H/ProxyZ/connections/prosocks5"
	"gitee.com/dark.H/ProxyZ/connections/protls"
	"gitee.com/dark.H/ProxyZ/servercontroll"
	"gitee.com/dark.H/ProxyZ/vpn"
	"gitee.com/dark.H/gs"
)

var (
	errInvalidWrite = errors.New("invalid write result")
	ErrRouteISBreak = errors.New("route is break")
	backupRoute     = make(chan *ClientControl)
)

func RunLocal(server string, l int, startDNS, startHTTPProxy bool) {

	if r, _ := servercontroll.TestServer(server); r > 5*time.Minute {
		gs.Str("Test Failed").Println()
		os.Exit(0)
		return
	} else {
		gs.Str("server build time: %s ").F(r).Println("test")
	}

	cli := NewClientControll(server, l)
	if startDNS {
		go cli.DNSListen()
	}
	if startHTTPProxy {
		go cli.HttpListen()
	}

	cli.Socks5Listen()

}

func PrepareRoute(server string, l int) bool {
	cli := NewClientControll(server, l)
	if cli.InitializationTunnels() {
		select {
		case backupRoute <- cli:

		default:

		}
		return true
	}
	return false

}

type SmuxorQuicClient interface {
	NewConnnect() (c net.Conn, err error)
	Close() error
	IsClosed() bool
}

type ClientControl struct {
	SmuxClients []SmuxorQuicClient
	CacheConns  gs.List[net.Conn]
	// nowconf        *base.ProtocolConfig
	Loc            string
	ClientNum      int
	ListenPort     int
	DnsServicePort int
	ErrCount       int
	RouteErrCount  int
	AliveCount     int
	lastUse        int
	acceptCount    int
	lock           sync.RWMutex
	islocked       bool
	Addr           gs.Str
	stdout         io.WriteCloser
	closed         bool
	closeFlag      bool
	Usevpn         bool
	dnsservice     bool
	inited         bool
	IsRunning      bool
	vpnHandler     *vpn.VPNHandler
	setTimer       *time.Timer
	failedHost     Set[string]
	GetNewRoute    func() string
	CloseDNS       func() error
	proxyProfiles  chan *base.ProtocolConfig
	initProfiles   int
	confNum        int
	errCon         int
	errorid        gs.Dict[int]
}

func NewClientControll(addr string, listenport int) *ClientControl {
	addr = Wrap(addr)
	gs.Str("New Client Controll:" + addr).Println()
	c := &ClientControl{
		Addr:           gs.Str(addr),
		ListenPort:     listenport,
		ClientNum:      30,
		DnsServicePort: 60053,
		lastUse:        -1,
		confNum:        10,
		errorid:        make(gs.Dict[int]),
		proxyProfiles:  make(chan *base.ProtocolConfig, 10),
	}
	for i := 0; i < c.ClientNum; i++ {
		c.SmuxClients = append(c.SmuxClients, nil)
	}
	return c
}

func (c *ClientControl) Init() {
	c.lastUse = -1
	c.proxyProfiles = make(chan *base.ProtocolConfig, 10)
	c.initProfiles = 0
	c.errorid = make(gs.Dict[int])
	c.ErrCount = 0
	c.inited = false
	c.dnsservice = false
	c.IsRunning = false
	c.RouteErrCount = 0
}

func RecvMsg(reply gs.Str) (di any, o bool) {
	d := reply.Json()
	if c, ok := d["status"]; ok {
		if c.(string) == "ok" {
			o = true
		}

		di = d["msg"]
		return
	} else {
		o = false
		return
	}
}

func (c *ClientControl) TryClose() {
	c.closeFlag = true
	go func() {
		if c, err := net.Dial("tcp", string(gs.Str("127.0.0.1:%d").F(c.ListenPort))); err == nil {
			time.Sleep(100 * time.Millisecond)
			c.Close()
			gs.Str("Send Close Signal").Println("Close")
		}
	}()
}

func (c *ClientControl) SetChangeRoute(f func() string) {
	gs.Str("----- set change route function -------").Println("init")
	c.GetNewRoute = f
}

func (c *ClientControl) GetRoute() string {

	e := c.Addr
	if e.In("://") {
		e = e.Split("://")[1]
	}
	if e.In(":") {
		e = e.Split(":")[0]
	}
	return e.Str()
}

func (c *ClientControl) GetRouteLoc() string {
	if !c.IsRunning {
		return "Connecting ...."
	}
	return c.Loc
}

func (c *ClientControl) SetRouteLoc(loc string) {
	c.Loc = loc
}

func (c *ClientControl) ChangeRoute(host string) bool {

	if c.closeFlag {

		c.Addr = gs.Str(Wrap(host))
		gs.Str("Change Client Controll:" + c.Addr).Println()
	} else {
		gs.Str("server is not closed !").Color("r").Println()
	}
	for {
		time.Sleep(1 * time.Second)
		if c.closed {
			break
		}
	}
	c.LockArea(func() {
		gs.Str("Clear All Session").Color("g").Println()
		for _i := 0; _i < len(c.SmuxClients); _i++ {
			if c.SmuxClients[_i] != nil {
				c.SmuxClients[_i].Close()
				c.SmuxClients[_i] = nil
			}

		}
	})

	time.Sleep(1 * time.Second)

	prodns.Clear()

	if c.CloseDNS != nil {
		c.CloseDNS()
		gs.Str("Close DNS Service ").Color("g").Println()
	}
	c.Addr = gs.Str(host)
	gs.Str("server closed !").Color("g").Println()
	c.Init()

	go c.DNSListen()
	gs.Str("Start New route:" + c.Addr).Color("g").Println()
	if err := c.Socks5Listen(); err == ErrRouteISBreak {
		return false
	} else {
		return true
	}
}

func (c *ClientControl) ChangePort(port int) {
	c.ListenPort = port
}

func (c *ClientControl) ReportErrorProxy() (conf *base.ProtocolConfig) {
	// if c.setTimer != nil {
	var errconf *base.ProtocolConfig
	ids := gs.List[string]{}

	c.LockArea(func() {
		for id, k := range c.errorid {
			if k > 0 {
				ids = ids.Add(id)

			}
		}

	})
	// LOOP:

	left := len(ids)
	gs.Str("Report Err: %d [start]").F(left).Color("y").Println("ReportErrorProxy")
	w := sync.WaitGroup{}
	for left > 0 {
		select {
		case thisconf := <-c.proxyProfiles:
			if ids.In(thisconf.ID) {
				errconf = thisconf
				w.Add(1)
				go func(ww *sync.WaitGroup) {
					defer ww.Done()
					errnum := c.errorid[errconf.ID]

					c.LockArea(func() {
						c.ErrCount -= errnum
					})

					c.ErrSoGetNew(errconf.ID, errnum)
					gs.Str("Report Err: %d").F(left).Color("y").Println("ReportErrorProxy")
				}(&w)
				left -= 1
			} else {
				c.proxyProfiles <- thisconf
			}
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
	w.Wait()
	for i := 0; i < c.ClientNum; i++ {
		if se := c.SmuxClients[i]; se != nil {
			se.Close()
			c.SmuxClients[i] = nil
		}
	}
	// c.InitializationTunnels()
	return
}

func (c *ClientControl) ErrSoGetNew(id string, ernum int) {
	// }
	var addr string
	// useTls := false
	// TAAGGGGGGGG
	if id == "" {
		gs.Str("not found !").Println()
		return
	}
	gs.Str(c.Addr.Str()+" Need Re Proxy Change!  ProxyType: "+id).Color("c", "B").Println("Route")
	addr = Wrap(c.Addr.Str())
	gs.Str("Err Get New :" + addr).Println()
	var reply gs.Str
	data := gs.Dict[any]{
		"ID": id,
	}

	var err error
	reply, err = servercontroll.HTTPSPost("https://"+addr+"/proxy-err", data)
	if err != nil {
		c.LockArea(func() {
			c.RouteErrCount += 1
		})
	}
	if reply == "" {
		gs.Str(addr + "Report And Rebuild Failed").Color("r").Println("Route")
		c.LockArea(func() {
			c.RouteErrCount += 1
		})

		return
	}
	if obj, ok := RecvMsg(reply); ok {
		// fmt.Println(obj)
		buf, err := json.Marshal(obj)
		if err != nil {
			gs.Str(err.Error()).Println("Err Tr")
			return
		}
		conf := new(base.ProtocolConfig)

		if err := json.Unmarshal(buf, conf); err != nil {
			gs.Str("get aviable proxy client err :" + err.Error()).Println("Err")
			return
		}
		if conf.Server == "0.0.0.0" {
			conf.Server = gs.Str(addr).Split(":")[0].Trim()
		}
		gs.Str(addr + " Rebuild Success! in " + conf.ID + " ProxyType: " + conf.ProxyType).Color("g").Println("Route")
		c.LockArea(func() {
			c.proxyProfiles <- conf
			c.errorid[conf.ID] = 0
			if c.RouteErrCount > 0 {
				c.RouteErrCount -= 1
			}
		})
		gs.Str("Can not Re Proxy ! \n\t").Add(reply.Color("r")).Println("Big Err")
	}

}

func (c *ClientControl) LockArea(d func()) {
	c.lock.Lock()
	// gs.Str("Lock Area-----------------------------").EndPrintln(" [lock]")
	d()
	// gs.Str("Lock Area-----------------------------").EndPrintln(" [unlock]")
	c.lock.Unlock()
}

func (c *ClientControl) GetListenPort() (socks5port, httpport, dnsport int) {
	socks5port = c.ListenPort
	httpport = socks5port + 1
	dnsport = c.DnsServicePort
	return
}

func (c *ClientControl) ConfigServer(name, val string) bool {

	addr := Wrap(c.Addr.Str())
	gs.Str("Config Server:" + addr).Println()
	var reply gs.Str
	var data gs.Dict[any]
	data = nil
	data = gs.Dict[any]{
		"name": name,
		"val":  val,
	}
	var err error
	reply, err = servercontroll.HTTPSPost(addr+"/z-set", data)
	if err != nil {

		c.LockArea(func() {
			c.RouteErrCount += 1
		})
	}
	if reply == "" {
		return false
	}
	if _, ok := RecvMsg(reply); ok {
		return true
	}
	return false
}

func (c *ClientControl) ClearAllRoute() bool {
	addr := Wrap(c.Addr.Str())
	gs.Str("Clear All Route:" + addr).Println()
	var reply gs.Str
	var err error
	reply, err = servercontroll.HTTPSGet(addr + "/__close-all")
	if err != nil {

		c.LockArea(func() {
			c.RouteErrCount += 1
		})
		return false
	}
	if reply == "" {
		return false
	}
	if _, ok := RecvMsg(reply); ok {
		return true
	}
	return false
}

func (c *ClientControl) ClearALLOpenUFW() bool {
	addr := Wrap(c.Addr.Str())
	gs.Str("Clear All Open UFW:" + addr).Println()
	var reply gs.Str
	var err error
	reply, err = servercontroll.HTTPSGet(addr + "/z-ufw-close")
	if err != nil {

		c.LockArea(func() {
			c.RouteErrCount += 1
		})
		return false
	}
	if reply == "" {
		return false
	}
	if _, ok := RecvMsg(reply); ok {
		return true
	}
	return false
}

func (c *ClientControl) GetAviableProxy(tp ...string) (conf *base.ProtocolConfig) {
	I := 0
	c.LockArea(func() {
		I = c.initProfiles
	})
	if I >= c.confNum {
		// c.LockArea(func() {

	L:
		for {

			select {
			case conf = <-c.proxyProfiles:
				break L
			default:
				time.Sleep(100 * time.Millisecond)
				// gs.Str("Jump").Println("test1.4")
			}

		}
		// })
	}

	if conf != nil {
		return
	}

	c.LockArea(func() {
		// c.proxyProfiles <- conf
		c.initProfiles += 1
	})
	addr := Wrap(c.Addr.Str())
	// gs.Str("Get Proxy Profile: '" + addr + "/proxy-get'").Println()
	useTls := true

	var reply gs.Str
	var data gs.Dict[any]
	data = nil
	if tp != nil {
		data = gs.Dict[any]{

			"type": tp[0],
		}
	}
	var err error
	for _i := 0; _i < 5; _i++ {
		if useTls {
			reply, err = servercontroll.HTTPSPost(addr+"/proxy-get", data)
		} else {
			reply, err = servercontroll.HTTP3Post(addr+"/proxy-get", data)
		}
		if err != nil {

			continue
		}
		break
	}
	if err != nil {
		gs.Str("No Aviliable Config get!:" + err.Error()).Color("r").Println("Init")
		c.LockArea(func() {
			c.RouteErrCount += 1
			c.initProfiles -= 1

		})
		return nil
	}
	if reply == "" {
		gs.Str("No Aviliable Config get!").Color("r").Println("Init")
		c.LockArea(func() {
			c.RouteErrCount += 1
			c.initProfiles -= 1

		})
		return nil
	}

	// gs.Str("read:" + reply[:10]).Color("g").Println("test1.7")
	if obj, ok := RecvMsg(reply); ok {
		// fmt.Println(obj)
		// gs.Str("read 1").Color("g").Println("test1.7")
		buf, err := json.Marshal(obj)
		if err != nil {
			gs.Str(err.Error()).Println("Err Tr")
			c.LockArea(func() {
				c.RouteErrCount += 1
				c.initProfiles -= 1
			})
			return nil
		}
		conf = new(base.ProtocolConfig)
		// gs.Str("read 2").Color("g").Println("test1.7")
		if err := json.Unmarshal(buf, conf); err != nil {
			gs.Str("get aviable proxy client err :" + err.Error()).Println("Err")
			c.LockArea(func() {
				c.RouteErrCount += 1
				c.initProfiles -= 1
			})
			return nil
		}
		// gs.Str("read 3").Color("g").Println("test1.7")
		if conf.Server == "0.0.0.0" {
			conf.Server = WrapIPPort(addr)
		}

		// gs.Str("read 4").Color("g").Println("test1.7")
		c.LockArea(func() {
			c.errorid[conf.ID] = 0
			// gs.Str("import").Color("g").Println("test1.7")
		})
		// gs.Str("read 5").Color("g").Println("test1.7")

	} else {
		// gs.Str("bad").Color("r").Println("test1.7")
		c.LockArea(func() {
			c.RouteErrCount += 1
			c.initProfiles -= 1
		})
	}

	return
}

func (c *ClientControl) SetOutFile(out io.WriteCloser) {
	if c.stdout != nil {
		c.stdout.Close()
	}
	c.stdout = out
}

func (c *ClientControl) DNSListen() {
	if !c.dnsservice {
		port := c.DnsServicePort
		dd := StartDNS(port, c, func() {
			c.dnsservice = true

		})
		c.CloseDNS = dd.Shutdown
		for !c.inited {
			time.Sleep(1 * time.Second)
		}

		go prodns.BackgroundBatchSend(c.Addr.Str(), &c.closeFlag)
		gs.Str("Start DNS (%s)").F(gs.Str(":%d").F(port).Color("g")).Println("dns")
		err := dd.ListenAndServe()

		if err != nil {
			gs.Str("DNS (%s) | err : %s").F(gs.Str(":%d").F(port).Color("g"), err.Error()).Println("dns")
		}
	}
}

func (c *ClientControl) HttpListen() (err error) {
	l := c.ListenPort + 1
	c.listenHttpProxy(l)
	return
}

/*
**************************************************************
**************************************************************
CORE ！！！！！！！！
*/
func (c *ClientControl) Socks5Listen(inied ...bool) (err error) {

	RE_LISTEN := false
	if inied != nil && inied[0] {

	} else {
		if !c.InitializationTunnels() {
			return ErrRouteISBreak
		}
	}

	var bak *ClientControl
	if c.ListenPort != 0 {
		var l net.Listener

		for {
			l, err = net.Listen("tcp", "0.0.0.0:"+gs.S(c.ListenPort).Str())
			if err != nil {
				if gs.Str(err.Error()).In("bind: address already in use") {
					continue
				} else {
					log.Fatal(err)
				}
				gs.Str("already listen wait !!!").Println("service")
				// log.Fatal(err)
			}
			break
		}
		c.closeFlag = false
		c.closed = false
		c.acceptCount = 0
		lastAutoSwitch := time.Now()
		c.IsRunning = true
		gs.Str("Socks5 Start").Color("g", "B", "F").Println("service")
	MLoop:
		for {
			if c.RouteErrCount > 20 {
				if c.GetNewRoute != nil && !RE_LISTEN {
					// ### BUG
					l := c.GetNewRoute()
					go func() {
						for {
							if !PrepareRoute(l, c.ListenPort) {
								gs.Str("Wait 2 s try again!").Println()
								time.Sleep(2 * time.Second)
								l = c.GetNewRoute()
							} else {
								break
							}

						}
					}()
					RE_LISTEN = true
					select {
					case bak = <-backupRoute:
						gs.Str("Use Backup Route").Color("g", "B", "U").Println()
						break MLoop
					}

				} else {
					gs.Str("no getNewRoute Function !!!!!").Color("r", "B").Println()
				}
			}
			if c.ErrCount > 70 {
				RE_LISTEN = true
				break
			}
			if c.ErrCount > 7 {

				go c.ReportErrorProxy()
			}
			if c.closeFlag {
				break
			}

			socks5con, err := l.Accept()
			if time.Now().After(lastAutoSwitch.Add(30 * time.Second)) {
				go func() {
					c.AutoSwitchProfile()
					lastAutoSwitch = time.Now()
				}()

			}
			c.acceptCount += 1
			if err != nil {
				gs.S(err.Error()).Println("accept err")
				time.Sleep(3 * time.Second)
				continue
			}

			go func(socks5con net.Conn) {
				defer func() {
					socks5con.Close()
					c.acceptCount -= 1
				}()
				err := prosocks5.Socks5HandShake(&socks5con)
				if err != nil {
					gs.Str(err.Error()).Println("socks5 handshake")
					return
				}

				raw, host, _, err := prosocks5.GetLocalRequest(&socks5con)
				if err != nil {
					gs.Str(err.Error()).Println("socks5 get host")
					return
				}

				if len(raw) > 9 && raw[0] == 5 && raw[3] == 1 {
					if ip := net.IP(raw[4:8]).String(); prodns.IsLocal(ip) {
						port := binary.BigEndian.Uint16(raw[8:10])
						c.tcppipe(socks5con, gs.Str(ip+":%d").F(port))
						return
					}

				}

				// if c.regionFilter(socks5con, raw, host) {
				// 	return
				// }
				c.LogTest(raw, host, "accept")
				c.OnBodyBeforeGetRemote(socks5con, true, raw, host)

			}(socks5con)

		}
	}
	c.closed = true
	if RE_LISTEN {

		// c.ChangeNewRoute()
		c.CloseDNS()
		if bak != nil {
			c.SetRouteLoc(bak.GetRouteLoc())
			go bak.DNSListen()
			bak.Socks5Listen()
		}
		// c.closed = false
		// go c.DNSListen()
		// c.Socks5Listen()
	}

	return
}

func (c *ClientControl) LogTest(raw []byte, host, l string) {
	if host == "" {
		if raw[3] == 1 && len(raw) > 9 {
			ip := net.IP(raw[4 : 4+net.IPv4len]).String()
			host := prodns.SearchIP(ip)
			if host != "" {
				gs.Str(ip+string(gs.Str("(%s)").F(host).Color("y"))).Color("m", "U").Println("TEST-" + l)
			} else {
				gs.Str(ip+"(x)").Color("m", "U").Println("TEST-" + l)
			}

		}
	} else {
		gs.Str(host).Color("m", "U").Println("TEST-" + l)
	}
}

func (c *ClientControl) ErrRecord(eid string, i int) {
	c.LockArea(func() {
		c.errorid[eid] += i
		c.ErrCount += i
	})
}

func (c *ClientControl) OnBodyBeforeGetRemote(socks5con net.Conn, isSocks5 bool, raw []byte, host string) (err error) {
	if gs.Str(host).StartsWith("c://") {
		// c.SetOutFile(socks5con)
		socks5con.Write([]byte("END Controll :" + host))
		// c.CloseWriter()
		c.ControllCode(host)
		return
	}
	var remotecon net.Conn
	var eid string
	var proxyType string
	c.LogStat()
	// for tryTime := 0; tryTime < 2; tryTime += 1 {
	remotecon, err, eid, proxyType = c.ConnectRemote()
	// c.LogTest(raw, host, "get con")
	c.LogStat()
	if err != nil {
		if !gs.Str(err.Error()).In("timeout") && !gs.Str(err.Error()).In("EOF") {
			gs.Str(err.Error()).Println("connect proxy server err")
		} else {
			c.LogErr("build", err, host, eid, proxyType)
		}
		c.LockArea(func() {
			c.RouteErrCount += 2
			c.errorid[eid] += 2
			c.errCon += 1
		})

		// continue
		return
	}
	if remotecon == nil {
		log.Fatal("!!???@@ASFASGFS")
	}

	defer remotecon.Close()
	// var continued bool
	// if host == "" {
	// 	if len(raw) > 9 {
	// 		switch raw[3] {
	// 		case 1:
	// 			ip := net.IP(raw[4 : 4+net.IPv4len]).String()
	// 			port := binary.BigEndian.Uint16(raw[4+net.IPv4len : 6+net.IPv4len])
	// 			gs.Str(ip + ":" + gs.S(port).Str()).Println("ready")
	// 		}
	// 	}
	// } else {
	// 	gs.Str(host).Println("ready")
	// }

	_, err = c.OnBodyDo(socks5con, remotecon, proxyType, eid, isSocks5, raw, host)
	if err != nil {
		c.LockArea(func() {
			c.errCon += 1
		})

	}
	// if continued && c.ErrCount < 8 {
	// 	// continue
	// }
	// break
	// }

	return
}

func (c *ClientControl) OnBodyDo(socks5con, remotecon net.Conn, proxyType, eid string, isSocks5 bool, raw []byte, host string) (continued bool, err error) {
	_, err = remotecon.Write(raw)
	if err != nil {
		(gs.S(c.RouteErrCount) + gs.Str(" "+err.Error())).Println("connecting write|" + host)
		c.LockArea(func() {
			c.RouteErrCount += 1
		})
		continued = true
		return
	}
	// gs.Str(host).Color("g").Println("connect|write")
	_buf := make([]byte, len(prosocks5.Socks5Confirm))
	remotecon.SetReadDeadline(time.Now().Add(7 * time.Second))
	_, err = remotecon.Read(_buf)
	// c.LogTest(raw, host, " build")
	if err != nil {
		// gs.Str(err.Error()).Println("connecting read|" + host)
		if err.Error() != "timeout" {
			base.ErrToFile("err in client controll.go :160", err)
		}

		c.LockArea(func() {
			c.errorid[eid] += 3
			c.ErrCount += 3
			c.errCon += 1
		})
		// gs.Str("%s : %s").F(gs.Str(host).Color("y"), gs.Str(err.Error()).Color("r")).Println(gs.Str("Err  read E/A:%d/%d ").F(c.ErrCount, c.acceptCount))

		if host == "" {
			if len(raw) > 9 {
				switch raw[3] {
				case 1:
					ip := net.IP(raw[4 : 4+net.IPv4len]).String()
					port := binary.BigEndian.Uint16(raw[4+net.IPv4len : 6+net.IPv4len])
					c.LogErr("read", err, ip+":"+gs.S(port).Str(), eid, proxyType)
				}
			}
			// gs.S(raw).Println("err read buf")
		} else {
			c.LogErr("read", err, host, eid, proxyType)
		}

		continued = true
		return
	}
	if bytes.Equal(_buf, prosocks5.Socks5Confirm) {
		if isSocks5 {
			_, err = socks5con.Write(_buf)
			if err != nil {
				c.LogErr("rely", err, host, eid, proxyType)
				// gs.Str("%s : %s").F(gs.Str(host).Color("y"), gs.Str(err.Error()).Color("r")).Println(gs.Str("Err reply E/A:%d/%d ").F(c.ErrCount, c.acceptCount))
				c.LockArea(func() {
					c.errorid[eid] += 1
					c.ErrCount += 1
				})
				remotecon.Close()
				return continued, err
			}
		}
	}

	c.LockArea(func() {
		c.AliveCount += 1
		if c.ErrCount > 0 {
			c.ErrCount -= 1
			c.errorid[eid] -= 1
		}
		if c.RouteErrCount > 0 {
			c.RouteErrCount -= 1
		}
		if c.acceptCount > 655300 {
			c.acceptCount = 1
			c.errCon = 0
		}

	})
	err = nil
	// if host == "" {
	// if len(raw) > 9 {
	// switch raw[3] {
	// case 1:
	// ip := net.IP(raw[4 : 4+net.IPv4len]).String()
	// port := binary.BigEndian.Uint16(raw[4+net.IPv4len : 6+net.IPv4len])
	// c.LogConnected(proxyType, ip+":"+gs.S(port).Str())

	// }
	// }

	// } else {
	// c.LogConnected(proxyType, host)
	// }
	c.LogStat()
	remotecon.SetReadDeadline(time.Now().Add(30 * time.Minute))
	c.Pipe(socks5con, remotecon)
	socks5con.Close()
	remotecon.Close()
	c.LockArea(func() {
		c.AliveCount -= 1
	})

	return
}

func (c *ClientControl) LogStat() {
	gs.S("%s %d/%d/%d\r").F(gs.Str("[status]").Color("B"), c.AliveCount, c.ErrCount, c.acceptCount).Print()
}

func (c *ClientControl) ChangeNewRoute() {
	if c.GetNewRoute != nil {
		gs.Str("Get New Route .... ").Println("Switch Route")
		newroute := c.GetNewRoute()
		if newroute != "" {

			if !gs.Str(newroute).In(":") {
				newroute += ":55443"
			}
			if !gs.Str(newroute).In("://") {
				newroute = "https://" + newroute
			}

			c.Addr = gs.Str(newroute)
			gs.Str("Set New Route : " + c.Addr).Println("Switch Route")
			gs.Str("Clear old profiles ... " + c.Addr).Println("Switch Route")
			c.LockArea(func() {
				for len(c.proxyProfiles) > 0 {
					select {
					case oldc := <-c.proxyProfiles:
						gs.Str("Clear " + oldc.ID).Println("Switch Route")
					default:
						time.Sleep(100 * time.Millisecond)
					}
				}
				c.initProfiles = 0
				c.errorid = make(gs.Dict[int])
			})
			gs.Str("Clear  ok " + c.Addr).Println("Switch Route")
			if c.CloseDNS != nil {
				c.CloseDNS()
				gs.Str("Close DNS service").Println("Switch Route")
			}
			c.RouteErrCount = 0
			c.ErrCount = 0
			gs.Str("Close DNS Cache").Println("Switch Route")
			prodns.Clear()

		} else {
			gs.Str("Get New Route failed ....").Color("r").Println("Switch Route")
		}
	} else {
		gs.Str("No New Route Function").Color("r").Println("Switch Route")
	}
}

func (c *ClientControl) ChangeProxyType(tp string) {

	c.LockArea(func() {
		for len(c.proxyProfiles) > 0 {
			select {
			case <-c.proxyProfiles:
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
		c.initProfiles = 0
		c.errorid = make(gs.Dict[int])
	})
	for i := 0; i < c.confNum; i++ {
		c.GetAviableProxy(tp)
	}
	gs.Str("Change Proxy Type :"+tp).Color("y", "B").Println("Change Proxy")
	c.InitializationTunnels()

}

func (c *ClientControl) ControllCode(host string) {
	C := gs.Str(host)
	if C.StartsWith("c://change/") {
		changeProxyType := C.Split("c://change/").Nth(1).Trim()
		c.ChangeProxyType(changeProxyType.Str())
	}

}

func (c *ClientControl) RebuildSmux(no int) (err error, conf *base.ProtocolConfig) {
	// gs.Str("b").Println("test1")
	conf = c.GetAviableProxy()

	// gs.Str("g").Println("test2")
	if conf == nil {
		return ErrRouteISBreak, nil
	}
	// id = conf.ID
	c.LockArea(func() {
		c.proxyProfiles <- conf
	})
	// gs.Str("a").Println("test3")
	var singleTunnelConn net.Conn

	switch conf.ProxyType {
	case "tls":
		singleTunnelConn, err = protls.ConnectTls(conf)
	case "kcp":
		singleTunnelConn, err = prokcp.ConnectKcp(conf)
	case "quic":
	default:
		singleTunnelConn, err = prokcp.ConnectKcp(conf)
	}
	if err != nil {

		return errors.New(err.Error() + " [Rebuild Err] in Protocol " + conf.ProxyType), conf
	}

	// gs.Str("--> "+conf.RemoteAddr()).Color("y", "B").Println(conf.ProxyType)
	if singleTunnelConn != nil && conf.ProxyType != "quic" {
		if len(c.SmuxClients) <= no {
			c.SmuxClients = append(c.SmuxClients, prosmux.NewSmuxClient(singleTunnelConn))
		} else {
			if c.SmuxClients[no] != nil {
				c.SmuxClients[no].Close()
			}
			cc := prosmux.NewSmuxClient(singleTunnelConn)
			c.lock.Lock()
			c.SmuxClients[no] = nil
			c.SmuxClients[no] = cc
			c.lock.Unlock()

		}
	} else if conf.ProxyType == "quic" {
		// gs.Str("test Enter be").Println(conf.ProxyType)
		if len(c.SmuxClients) <= no {

			qc, err := proquic.NewQuicClient(conf)
			if err != nil {

				return errors.New("[quic-rebuild] " + err.Error() + conf.RemoteAddr()), conf
			}
			c.SmuxClients = append(c.SmuxClients, qc)
		} else {

			if c.SmuxClients[no] != nil {
				c.SmuxClients[no].Close()
			}
			qc, err := proquic.NewQuicClient(conf)
			if err != nil {
				return errors.New("[quic-rebuild] " + err.Error() + conf.RemoteAddr()), conf
			}
			c.lock.Lock()
			c.SmuxClients[no] = nil
			c.SmuxClients[no] = qc
			c.lock.Unlock()

		}
	} else {
		if err == nil {
			err = errors.New("tls/kcp only :  now method is :" + conf.ProxyType)
		}
		return err, conf
	}
	return nil, conf
}

func (c *ClientControl) GetSession() (con net.Conn, err error, id, proxyType string) {
	c.lock.Lock()
	c.lastUse += 1
	c.lastUse = c.lastUse % c.ClientNum
	c.lock.Unlock()
	var _c *base.ProtocolConfig

	e := c.SmuxClients[c.lastUse]
	if e == nil {
		for _i := 0; _i < 2; _i++ {
			err, _c = c.RebuildSmux(c.lastUse)
			if _c != nil {
				id = _c.ID
				proxyType = _c.ProxyType
			}

			e = c.SmuxClients[c.lastUse]
			if err == nil {
				break
			}
		}
		if err != nil {
			return nil, err, id, proxyType
		}
	}
	if e != nil && e.IsClosed() {
		err, _c = c.RebuildSmux(c.lastUse)
		if _c != nil {
			id = _c.ID
			proxyType = _c.ProxyType
		}
		if err != nil {
			return nil, errors.New(err.Error() + " in rebuild "), id, proxyType
		}
		con, err = e.NewConnnect()
	} else {
		if e != nil {
			con, err = e.NewConnnect()
			return
		} else {
			return nil, errors.New("no session create!!"), id, proxyType
		}

	}

	return
}

func (c *ClientControl) Write(buf string) {
	if c.stdout != nil {
		c.stdout.Write([]byte(buf))
	}
}

func (c *ClientControl) CloseWriter() {
	if c.stdout != nil {
		c.stdout.Close()
		c.stdout = nil
	}
}

func (c *ClientControl) InitializationTunnels() (use bool) {
	wait := sync.WaitGroup{}
	l := sync.RWMutex{}
	msgs := gs.Str("*").Color("y").Add("|").Repeat(c.ClientNum).Slice(0, -1).Split("|")
	cc := 0
	var conf *base.ProtocolConfig
	var err error
	var errnum = 0
	for i := 0; i < c.ClientNum; i++ {
		wait.Add(1)
		time.Sleep(100 * time.Millisecond)
		go func(no int, w *sync.WaitGroup) {
			defer wait.Done()

			err, conf = c.RebuildSmux(no)
			if err == ErrRouteISBreak {
				errnum += 1
			}
			p := -1
			pt := "unknow"
			if err != nil {
				l.Lock()
				msgs[no] = gs.Str('*').Color("r", "F")
				l.Unlock()
				if conf != nil {
					p = conf.ServerPort
					pt = conf.ProxyType
				}
				gs.Str("[%s T:%2d %s in %d] %s \r").F(c.Addr, cc, pt, p, msgs.Join("")).Print()
				if err != nil {
					base.ErrToFile("RebuildSmux Er", err)
				}

			} else {
				l.Lock()
				switch conf.ProxyType {
				case "tls":
					msgs[no] = gs.Str('*').Color("g", "B")
				case "kcp":
					msgs[no] = gs.Str('*').Color("b", "B")
				case "quic":
					msgs[no] = gs.Str('*').Color("m", "B")
				default:
					msgs[no] = gs.Str('*').Color("w", "B")
				}

				cc += 1
				l.Unlock()
				if conf != nil {
					p = conf.ServerPort
					pt = conf.ProxyType
				}
				gs.Str("[%s T:%2d %s in %d] %s \r").F(c.Addr, cc, pt, p, msgs.Join("")).Print()

			}

		}(i, &wait)
	}

	wait.Wait()
	time.Sleep(1 * time.Second)
	if conf != nil {
		gs.Str("\nConnected %s :%d").F(conf.ProxyType, c.ClientNum).Color("g").Println(conf.ProxyType)
		c.inited = true
		use = true
	}
	if errnum > c.confNum/2 {
		c.SetRouteLoc("this is break, try next !!!")
		use = false
	}
	return
}

func (c *ClientControl) Health() float32 {
	healthy := float32(100) - (float32(c.errCon) * 100 / float32(c.acceptCount))
	return healthy
}

func (c *ClientControl) LogConnected(proxyType string, host string) {
	health := c.Health()
	gs.Str("%s %s                ").F(gs.Str("[connected] health:%.2f%% %s ").F(health, proxyType).Color("w", "B"), host).Color("g").Add("\n").Print()

}

func (c *ClientControl) LogErr(label string, err error, host, eid, proxyType string) {
	health := c.Health()
	gs.Str("%s %s").F(gs.Str("[%s] %s health:%.2f%% %s ").F(label, eid, health, proxyType).Color("w", "B"), gs.Str(err.Error()).Color("r")).Add("\n").Print()
}

func (c *ClientControl) ConnectRemote() (con net.Conn, err error, id, proxyType string) {

	// connted := false

	con, err, id, proxyType = c.GetSession()
	if err != nil {
		// gs.Str("rebuild smux").Println("connect remote")
		con, err, id, proxyType = c.GetSession()
	}
	// gs.Str("smxu connect ").Println()
	return
}

func (c *ClientControl) Pipe(p1, p2 net.Conn) {
	Pipe(p1, p2)
}

func Pipe(p1, p2 net.Conn) {
	var wg sync.WaitGroup
	// var wait int = 1800
	wg.Add(1)
	streamCopy := func(dst net.Conn, src net.Conn, fr, to net.Addr) {
		// startAt := time.Now()
		// Copy(dst, src, wait)
		buf := make([]byte, 8092)
		// io.CopyBuffer(dst, src, buf)
		copyBuffer(dst, src, buf, 1800)
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

// Memory optimized io.Copy function specified for this library
func Copy(dst io.Writer, src io.Reader, timeout ...int) (written int64, err error) {
	// If the reader has a WriteTo method, use it to do the copy.
	// Avoids an allocation and a copy.
	if wt, ok := src.(io.WriterTo); ok {
		return wt.WriteTo(dst)
	}
	// Similarly, if the writer has a ReadFrom method, use it to do the copy.
	if rt, ok := dst.(io.ReaderFrom); ok {
		if timeout != nil {
			src.(net.Conn).SetReadDeadline(time.Now().Add(time.Duration(timeout[0]) * time.Second))
		}
		return rt.ReadFrom(src)
	}

	// fallback to standard io.CopyBuffer
	buf := make([]byte, 4096)
	return copyBuffer(dst, src, buf, timeout...)
}

func copyBuffer(dst io.Writer, src io.Reader, buf []byte, timeout ...int) (written int64, err error) {
	if buf != nil && len(buf) == 0 {
		panic("empty buffer in CopyBuffer")
	}
	// If the reader has a WriteTo method, use it to do the copy.
	// Avoids an allocation and a copy.
	if wt, ok := src.(io.WriterTo); ok {
		return wt.WriteTo(dst)
	}
	// Similarly, if the writer has a ReadFrom method, use it to do the copy.
	if rt, ok := dst.(io.ReaderFrom); ok {
		if timeout != nil {
			src.(net.Conn).SetReadDeadline(time.Now().Add(time.Duration(timeout[0]) * time.Second))
		}
		return rt.ReadFrom(src)
	}
	if buf == nil {
		size := 32 * 1024
		if l, ok := src.(*io.LimitedReader); ok && int64(size) > l.N {
			if l.N < 1 {
				size = 1
			} else {
				size = int(l.N)
			}
		}
		buf = make([]byte, size)
	}
	for {
		if timeout != nil {
			src.(net.Conn).SetReadDeadline(time.Now().Add(time.Duration(timeout[0]) * time.Second))
		}
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errInvalidWrite
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}

func (c *ClientControl) AutoSwitchProfile() {
	maxid := ""
	maxnum := 0

	c.LockArea(func() {
		for id, enum := range c.errorid {
			if enum > maxnum {
				maxid = id
			}
		}
		if maxid != "" {
			delete(c.errorid, maxid)
		}

	})
	if maxid != "" {
	L:
		for {
			select {
			case conf := <-c.proxyProfiles:
				if conf.ID == maxid {
					c.LockArea(func() {
						c.initProfiles -= 1
					})
					newconf := c.GetAviableProxy()
					if newconf != nil {
						c.proxyProfiles <- newconf
						gs.Str(conf.ID + " [out]").Println("AutoSwitch")
						c.LockArea(func() {
							c.errorid[newconf.ID] = 0
						})
					}
					break L
				} else {
					c.proxyProfiles <- conf
				}
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

}
