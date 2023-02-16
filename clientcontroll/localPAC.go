package clientcontroll

import (
	"net"
	"runtime"

	"gitee.com/dark.H/ProxyZ/connections/prosocks5"
	"gitee.com/dark.H/ProxyZ/geo"
	"gitee.com/dark.H/gs"
)

var (
	isRUNGEO = (runtime.GOOS == "linux" && runtime.GOARCH == "arm")
)

func (c *ClientControl) regionFilter(comcon net.Conn, raw []byte, h string) bool {
	host := gs.Str(h)
	if host.In("://") {
		return false
	}
	if c.inCN(host) {
		c.tcppipe(comcon, host)
		return true
	}
	return false
}

func (c *ClientControl) tcppipe(comcon net.Conn, host gs.Str) {
	con, err := net.Dial("tcp", host.Str())
	if err != nil {
		gs.Str("%s conneting err ").F(host).Color("r").Println("CN")
		return
	}
	_, err = comcon.Write(prosocks5.Socks5Confirm)
	if err != nil {
		con.Close()
		gs.Str("%s reply socks5 err ").F(host).Color("r").Println("CN")
		return
	}
	gs.Str("%s %s\t\t        ").F(gs.Str("[connected] E/A:%d/%d ").F(c.ErrCount, c.acceptCount).Color("w", "B")+gs.S(c.AliveCount), host).Color("g").Add("\r").Print()
	c.Pipe(comcon, con)
}

func (c *ClientControl) inCN(host gs.Str) bool {
	// port := ""
	domain := gs.Str("")
	if host.In(":") {
		// port = host.Split(":")[1].Str()
		domain = host.Split(":")[0]
	}

	if domain.StartsWith("[") && domain.EndsWith("]") {
		domain = domain.Slice(1, -1)
	}
	if domain.EndsWith(".gstatic.com") {
		return false
	}
	if domain.EndsWith(".cn") {
		return true
	} else {
		if isRUNGEO {
			s := geo.Host2GEO(domain.Str())
			if s.Count() > 0 {
				ww := s[0].InCN()
				if ww {
					s[0].Str().Color("m", "B").Println("CN")
				}
				return ww
			}
		}
	}
	return false
}
