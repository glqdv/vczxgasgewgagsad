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
	"gitee.com/dark.H/ProxyZ/router"
	"gitee.com/dark.H/ProxyZ/servercontroll"
	"gitee.com/dark.H/ProxyZ/vpn"
	"gitee.com/dark.H/gs"
)

var (
	errInvalidWrite = errors.New("invalid write result")
	ErrRouteISBreak = errors.New("route is break")
	cityMap         = gs.Dict[string]{
		"Los Angeles":    "US",
		"Los-Angeles":    "US",
		"Seattle":        "US",
		"Dallas":         "US",
		"Tokyo":          "Japen",
		"Chicago":        "US",
		"Atlanta":        "US",
		"London":         "UK",
		"Singapore":      "Singa.",
		"Silicon Valley": "US",
		"Osaka":          "Japen",
		"New Jersey":     "US",
		"Miami":          "US",
		"Toronto":        "Cana",
		"Santiago":       "Chile",
		"Stockholm":      "Swit",
		"Honolulu":       "US",
		"Paris":          "Fran",
		"Warsaw":         "Polan",
		"Mardri":         "Spain",
		"Frankfurt":      "German",
		"Amsterdam":      "Neth..",
		"Seoul":          "Koral",
		"Sydney":         "Austr",
	}
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
	if startHTTPProxy {
		go cli.HttpListen()
	}

	cli.Socks5Listen()

}

type SmuxorQuicClient interface {
	NewConnnect() (c net.Conn, err error)
	Close() error
	IsClosed() bool
	ID() string
	GetProxyType() string
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
	IfStartDNS     bool
	inited         bool
	IsBreak        bool
	IsRunning      bool
	vpnHandler     *vpn.VPNHandler
	setTimer       *time.Timer
	failedHost     Set[string]
	GetNewRoute    func() string
	proxyProfiles  chan *base.ProtocolConfig
	initProfiles   int
	confNum        int
	errCon         int
	errorid        gs.Dict[int]
	ReportingMark  gs.Dict[bool]
	statusSignal   gs.Strs
}

func NewClientControll(addr string, listenport int) *ClientControl {
	addr = Wrap(addr)
	gs.Str("New Client Controll:" + addr).Println()
	c := &ClientControl{
		Addr:           gs.Str(addr),
		ListenPort:     listenport,
		ClientNum:      100,
		DnsServicePort: 60053,
		lastUse:        -1,
		confNum:        10,
		errorid:        make(gs.Dict[int]),
		ReportingMark:  make(gs.Dict[bool]),
		proxyProfiles:  make(chan *base.ProtocolConfig, 10),
	}
	for i := 0; i < c.ClientNum; i++ {
		c.SmuxClients = append(c.SmuxClients, nil)
	}
	c.statusSignal = gs.Str("*").Color("w", "B").Add("|").Repeat(c.ClientNum).Slice(0, -1).Split("|")
	return c
}

func (c *ClientControl) Init() {
	c.lastUse = 0
	if c.proxyProfiles != nil {
		for len(c.proxyProfiles) > 0 {
			<-c.proxyProfiles
		}
		gs.Str("Clar Proxy Profiles ").Println("Init")
	}
	c.proxyProfiles = make(chan *base.ProtocolConfig, 10)
	c.initProfiles = 0
	c.errorid = make(gs.Dict[int])
	c.ErrCount = 0
	c.inited = false
	c.dnsservice = false
	c.IsRunning = false
	c.IsBreak = false
	c.RouteErrCount = 0
	time.Sleep(1 * time.Second)
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

func (c *ClientControl) SetIfStartDNS(b bool) {
	c.IfStartDNS = b
}

func (c *ClientControl) TryClose() {
	c.closeFlag = true
	c.SetRouteLoc("Closing...")
	go func() {
		if c, err := net.Dial("tcp", string(gs.Str("127.0.0.1:%d").F(c.ListenPort))); err == nil {
			time.Sleep(100 * time.Millisecond)
			c.Close()
			gs.Str("Send Close Signal").Println("Close")

		}
		// c.SetRouteLoc("Closed")
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

func (c *ClientControl) IfRunning() bool {
	return c.IsRunning
}

func (c *ClientControl) GetRouteLoc() string {
	if !c.IsRunning {
		return "Connecting ...."
	}
	fs := gs.Str(c.Loc).SplitSpace()
	if len(fs) == 0 {
		return c.Loc
	}
	e := fs[:fs.Len()-1].Join(" ")
	last := fs[fs.Len()-1]
	// gs.Str("'%s'").F(e).Println("Loc")
	if ee, ok := cityMap[e.Str()]; ok {
		return ee + " " + last.Str()
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

	prodns.Clear()

	c.Addr = gs.Str(host)
	c.Init()
	gs.Str("server init !").Color("g").Println()
	prodns.SetDNSAddr(host)
	if err := c.Socks5Listen(); err == ErrRouteISBreak {
		gs.Str("Gs. is Break ").Println()
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
			if k > 4 {

				if _, ok := c.ReportingMark[id]; !ok {
					c.ReportingMark[id] = true
					ids = ids.Add(id)
				}
			}
		}

	})

	left := len(ids)

	ids.Every(func(no int, i string) {
		gs.Str(i).Println("Need Replace")
	})
	// w := sync.WaitGroup{}
	for left > 0 {
		select {
		case thisconf := <-c.proxyProfiles:
			if ids.In(thisconf.ID) {
				errconf = thisconf
				errnum := c.errorid[errconf.ID]
				c.LockArea(func() {
					c.ErrCount -= errnum
				})
				c.ErrSoGetNew(errconf.ID, errnum)
				gs.Str("Report Err: %d").F(left).Color("y").Println("ReportErrorProxy")
				left -= 1
			} else {
				c.proxyProfiles <- thisconf
			}
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

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
	reply, err = servercontroll.HTTPSPost(addr+"/proxy-err", data, 10)
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
			delete(c.ReportingMark, id)
		})
		c.FixBuildSmux(id)
		// gs.Str("Can not Re Proxy ! \n\t").Add(reply.Color("r")).Println("Big Err")
	} else {

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
	reply, err = servercontroll.HTTPSPost(addr+"/z-set", data, 10)
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

		reply, err = servercontroll.HTTPSPost(addr+"/proxy-get", data, 10)
		if err != nil {
			time.Sleep(1 * time.Second)
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
	if prodns.IfStart {
		return
	}
	for _i := 0; _i < 10; _i++ {

		if !c.dnsservice {
			port := c.DnsServicePort
			dd := StartDNS(port, c, func() {
				c.dnsservice = true
			})

			gs.Str("Wait Initialization finish .....").Color("g").Println()
			for !c.inited {
				time.Sleep(1 * time.Second)
			}
			prodns.SetDNSAddr(c.Addr.Str())
			// go prodns.BackgroundBatchSend(&c.RouteErrCount)
			gs.Str("Start DNS (%s)").F(gs.Str(":%d").F(port).Color("g")).Println("dns")
			err := dd.ListenAndServe()
			if err != nil {
				gs.Str("DNS (%s) | err : %s").F(gs.Str(":%d").F(port).Color("g"), err.Error()).Println("dns")
			}
			return
		} else {
			if router.IsRouter() {
				gs.Str("Can no start dns because dnservice not allow !").Color("r").Println("wait 2s try again:" + gs.S(_i))
				time.Sleep(2 * time.Second)

			} else {
				break
			}
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

	if inied != nil && inied[0] {

	} else {
		if !c.InitializationTunnels() {
			c.IsRunning = false
			c.IsBreak = true
			return ErrRouteISBreak
		}
	}

	go c.DNSListen()

	if c.ListenPort != 0 {
		var l net.Listener

		for {
			l, err = net.Listen("tcp", "0.0.0.0:"+gs.S(c.ListenPort).Str())
			if err != nil {
				if gs.Str(err.Error()).In("bind: address already in use") {
					time.Sleep(1 * time.Second)
					continue
				} else {
					log.Fatal(err)
					time.Sleep(1 * time.Second)
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
			if c.RouteErrCount > 15 {
				c.ChangeNextRoute()
				break MLoop
			} else if c.ErrCount > 70 {
				c.ChangeNextRoute()
				break MLoop
			}
			if c.ErrCount > 12 {

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
			c.AliveCount += 1
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
					c.AliveCount -= 1
					return
				}

				raw, host, _, err := prosocks5.GetLocalRequest(&socks5con)
				if err != nil {
					gs.Str(err.Error()).Println("socks5 get host")
					return
				}

				if len(raw) > 9 && raw[0] == 5 && raw[3] == 1 {
					ip := net.IP(raw[4:8]).String()
					// gs.Str(ip).Println("===>")
					if prodns.IsLocal(ip) {
						port := binary.BigEndian.Uint16(raw[8:10])
						c.tcppipe(socks5con, gs.Str(ip+":%d").F(port))
						c.AliveCount -= 1
						return
					} else if ip == "99.254.254.254" {

						gs.Str("==== Config me ====").Color("b").Println("Config")
						c.RedirectConfig(socks5con)
						c.AliveCount -= 1
						return
					}

				}

				// if c.regionFilter(socks5con, raw, host) {
				// 	return
				// }

				c.OnBodyBeforeGetRemote(socks5con, true, raw, host)

			}(socks5con)

		}
	}
	c.closed = true

	return
}

func (c *ClientControl) RedirectConfig(l net.Conn) {
	defer l.Close()
	l.Write(prosocks5.Socks5Confirm)
	_t := make([]byte, 4096)
	l.Read(_t)
	ip := router.GetGatewayIP()
	gs.Str("Config IP :" + ip).Color("b").Println("To Config")
	l.Write([]byte(gs.Str(`HTTP/1.1 302 Found
Location: http://%s:35555`).F(ip) + "\r\n\r\n"))

}

func (c *ClientControl) LogTest(raw []byte, host, l string) {
	if len(l) > 5 {
		l = l[:5]
	}
	if host == "" {
		if raw[3] == 1 && len(raw) > 9 {
			ip := net.IP(raw[4 : 4+net.IPv4len]).String()
			host := prodns.SearchIP(ip)
			if host != "" {
				// gs.Str(gs.Str("%15s").F(ip)+gs.Str("(%s)         \r").F(host).Color("y")).Color("m", "U").Print("tcp " + l)
			} else {
				// gs.Str(gs.Str("%15s").F(ip)+"(x)         \r").Color("m", "U").Print("tcp " + l)
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
		if i > 1 {
			c.RouteErrCount += 1
		}
		c.errCon += 1
	})
}

func (c *ClientControl) ErrVanish(eid string) {
	c.LockArea(func() {
		if c.errorid[eid] > 0 {
			c.errorid[eid] -= 1
		}
		if c.ErrCount > 0 {
			c.ErrCount -= 1
		}

		if c.RouteErrCount > 0 {
			c.RouteErrCount -= 1
		}

	})
}

func (c *ClientControl) OnBodyBeforeGetRemote(socks5con net.Conn, isSocks5 bool, raw []byte, host string) (err error) {
	defer socks5con.Close()
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
	remotecon, eid, proxyType, err = c.ConnectRemote()
	c.LogTest(raw, host, proxyType)
	// c.LogTest(raw, host, "get con")
	c.LogStat()
	if err != nil {
		if !gs.Str(err.Error()).In("timeout") && !gs.Str(err.Error()).In("EOF") {
			gs.Str(err.Error()).Println("connect proxy server err")
		} else {
			c.LogErr("build", err, host, eid, proxyType)
		}
		c.ErrRecord(eid, 1)
		c.LockArea(func() {
			c.RouteErrCount += 1
		})
		if remotecon != nil {
			remotecon.Close()
		}
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
	if err != nil {
		if err.Error() != "timeout" {
			base.ErrToFile("err in client controll.go :160", err)
		}

		c.ErrRecord(eid, 1)

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
		c.LockArea(func() {
			c.AliveCount -= 1
		})
		// continued = true
		return
	}
	if bytes.Equal(_buf, prosocks5.Socks5Confirm) {
		if isSocks5 {
			_, err = socks5con.Write(_buf)
			if err != nil {
				c.LogErr("rely", err, host, eid, proxyType)

				c.ErrRecord(eid, 1)
				c.LockArea(func() {
					c.AliveCount -= 1
				})
				remotecon.Close()
				return continued, err
			}
		}
	}

	c.ErrVanish(eid)

	err = nil
	c.LogStat()
	remotecon.SetReadDeadline(time.Now().Add(24 * time.Hour))
	c.Pipe(socks5con, remotecon)
	socks5con.Close()
	remotecon.Close()
	c.LockArea(func() {
		c.AliveCount -= 1
		if c.acceptCount > 655300 {
			c.acceptCount = 1
			c.errCon = 0
		}
	})

	return
}

func (c *ClientControl) LogStat() {
	gs.S("%s %d/%d/%d\r").F(gs.Str("[status]").Color("B"), c.AliveCount, c.ErrCount, c.acceptCount).Print()
}

func (c *ClientControl) ChangeNextRoute() {
	if c.GetNewRoute != nil {

		l := c.GetNewRoute()
		go func() {
			gs.Str("Wait 1s then Change Route: " + l).Color("r").Println("Change")
			time.Sleep(1 * time.Second)
			c.ChangeRoute(l)
		}()

	} else {
		gs.Str("no getNewRoute Function !!!!!").Color("r", "B").Println()
	}
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

func (c *ClientControl) FixBuildSmux(errid string) {
	for no := 0; no < len(c.SmuxClients); no++ {
		if e := c.SmuxClients[no]; e != nil {
			eid := e.ID()
			if eid == errid {
				if err, conf := c.RebuildSmux(no); err == nil {
					gs.Str("Rebuild %d tunnel -> %s").F(no, conf.ProxyType).Color("g").Println("FixBuild")
				}
			}
		}
	}
}

func (c *ClientControl) RebuildSmux(no int) (err error, conf *base.ProtocolConfig) {
	// gs.Str("b").Println("test1")
	switch no % 3 {
	case 0:
		conf = c.GetAviableProxy("quic")
	case 1:
		conf = c.GetAviableProxy("tls")
	case 2:
		conf = c.GetAviableProxy("tls")
	default:
		conf = c.GetAviableProxy("quic")
	}

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

		if c.SmuxClients[no] != nil {
			c.SmuxClients[no].Close()
		}
		cc := prosmux.NewSmuxClient(singleTunnelConn, conf.ID, conf.ProxyType)
		c.LockArea(func() {
			c.SmuxClients[no] = nil
			c.SmuxClients[no] = cc
		})

	} else if conf.ProxyType == "quic" {

		if c.SmuxClients[no] != nil {
			c.SmuxClients[no].Close()
		}
		qc, err := proquic.NewQuicClient(conf)
		if err != nil {
			return errors.New("[quic-rebuild] " + err.Error() + conf.RemoteAddr()), conf
		}
		c.LockArea(func() {
			c.SmuxClients[no] = nil
			c.SmuxClients[no] = qc

		})

	} else {
		if err == nil {
			err = errors.New("tls/kcp only :  now method is :" + conf.ProxyType)
		}
		return err, conf
	}
	return nil, conf
}

func (c *ClientControl) GetSession(debug bool) (con net.Conn, err error, id, proxyType string) {
	if debug {
		gs.Str("before get id").Color("y").Println(id)
	}

	c.LockArea(func() {
		c.lastUse += 1
		c.lastUse = c.lastUse % c.ClientNum
	})
	var _c *base.ProtocolConfig
	if debug {
		gs.Str("before get session").Color("y", "U").Println(id)
	}

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
				if e != nil {
					id = e.ID()
				}

				break
			}
		}
		if err != nil {

			gs.Str("before get connection").Color("r").Println(id)
			return nil, err, id, proxyType
		}
	}
	if e != nil && e.ID() != "" {
		id = e.ID()
	}
	if debug {
		gs.Str("before get connection").Color("c").Println(id)
	}

	if e != nil && e.IsClosed() {
		err, _c = c.RebuildSmux(c.lastUse)
		if _c != nil {
			id = _c.ID
			proxyType = _c.ProxyType
		}
		if err != nil {
			gs.Str("err before create new connection").Color("r").Println(id)
			return nil, errors.New(err.Error() + " in rebuild "), id, proxyType
		}
		if debug {
			gs.Str("before create new connection").Color("c", "U").Println(id)
		}

		con, err = e.NewConnnect()
		if debug {
			gs.Str("create new connection").Color("c", "B").Println(id)
		}

	} else {
		if e != nil {
			proxyType = e.GetProxyType()
			if debug {
				gs.Str("before create new connection").Color("c", "U").Println(id)
			}
			con, err = e.NewConnnect()
			if debug {
				gs.Str("create new connection").Color("c", "B").Println(id)
			}

			return
		} else {
			gs.Str("err create new connection").Color("r").Println(id)
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

func (c *ClientControl) ShowChannelStatus(channelID int, ProxyType string, status int) {

	c.LockArea(func() {
		msgs := c.statusSignal
		switch status {
		case 1:
			msgs[channelID] = gs.Str('*').Color("r", "F", "B")
		case 2:
			switch ProxyType {
			case "tls":
				msgs[channelID] = gs.Str('*').Color("g", "B")
			case "kcp":
				msgs[channelID] = gs.Str('*').Color("y", "B")
			case "quic":
				msgs[channelID] = gs.Str('*').Color("m", "B")
			default:
				msgs[channelID] = gs.Str('*').Color("g", "B")
			}

		}
		gs.Str("[%s] %s \r").F(c.Addr, msgs.Join("")).Print()
		c.statusSignal = msgs
	})

}

func (c *ClientControl) BuildChannel(channelID int, errnum *int, wait *sync.WaitGroup) {

	defer wait.Done()

	c.ShowChannelStatus(channelID, "Unknow", 0)

	err, conf := c.RebuildSmux(channelID)
	if err == ErrRouteISBreak {
		*errnum += 1
	}

	pt := "unknow"
	if err != nil {
		c.ShowChannelStatus(channelID, "Unknow", 1)
		if err != nil {
			base.ErrToFile("RebuildSmux Er", err)
		}

	} else {
		if conf != nil {
			pt = conf.ProxyType
		}
		c.ShowChannelStatus(channelID, pt, 2)

	}

}

func (c *ClientControl) InitializationTunnels() (use bool) {
	wait := sync.WaitGroup{}
	var errnum = 0

	for i := 0; i < c.ClientNum; i++ {
		wait.Add(1)
		time.Sleep(50 * time.Millisecond)
		go c.BuildChannel(i, &errnum, &wait)
	}

	wait.Wait()
	time.Sleep(1 * time.Second)
	c.inited = true
	use = true
	if errnum > c.confNum/2 {
		c.SetRouteLoc("this is break, try next !!!")
		use = false
	} else {
		gs.Str("Tunnel Build Successful").Color("g", "F").Println()
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

func (c *ClientControl) ConnectRemote() (con net.Conn, id, proxyType string, err error) {

	// connted := false

	con, err, id, proxyType = c.GetSession(false)
	if err != nil {
		// gs.Str("rebuild smux").Println("connect remote")
		con, err, id, proxyType = c.GetSession(true)
		if err != nil {
			c.ErrRecord(id, 2)
		}
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
		buf := make([]byte, 4096)
		copyBuffer(dst, src, buf, 1800)
		p1.Close()
		p2.Close()
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
