package servercontroll

import (
	"sync"

	"gitee.com/dark.H/ProxyZ/connections/base"
	"gitee.com/dark.H/ProxyZ/connections/prokcp"
	"gitee.com/dark.H/ProxyZ/connections/proquic"
	"gitee.com/dark.H/ProxyZ/connections/protls"
	"gitee.com/dark.H/gs"
)

var (
	lock         = sync.RWMutex{}
	ErrTypeCount = gs.Dict[int]{
		"tls":  0,
		"kcp":  0,
		"quic": 0,
	}
	lastUse = 0
)

func GetProxy(proxyType ...string) *base.ProxyTunnel {
	if proxyType == nil {

		if Tunnels.Count() == 0 {
			tunnel := NewProxy("quic")
			AddProxy(tunnel)
			return tunnel
		} else {
			lock.Lock()
			lastUse += 1
			lastUse = lastUse % Tunnels.Count()
			lock.Unlock()
			tunnel := Tunnels.Nth(lastUse)
			return tunnel
		}
	} else {
		var tu *base.ProxyTunnel
		Tunnels.Every(func(no int, i *base.ProxyTunnel) {
			if i.GetConfig().ProxyType == proxyType[0] {
				tu = i
			}
		})
		if tu == nil {
			tunnel := NewProxy(proxyType[0])
			AddProxy(tunnel)
			return tunnel
		} else {
			return tu
		}
	}
}

func AddProxy(c *base.ProxyTunnel) {
	lock.Lock()
	Tunnels = append(Tunnels, c)
	lock.Unlock()
}

func DelProxy(name string) (found bool) {

	e := gs.List[*base.ProxyTunnel]{}
	for _, tun := range Tunnels {
		if tun == nil {
			continue
		}
		if tun.GetConfig().ID == name {
			base.ClosePortUFW(tun.GetConfig().ServerPort)
			if num, ok := ErrTypeCount[tun.GetConfig().ProxyType]; ok {
				num += 1
				lock.Lock()
				ErrTypeCount[tun.GetConfig().ProxyType] = num
				lock.Unlock()
			}
			tun.SetWaitToClose()
			found = true
			continue
		} else {
			e = e.Add(tun)
		}
	}
	GLOCK.Lock()
	Tunnels = e
	GLOCK.Unlock()
	return
}

func NewProxyByErrCount() (c *base.ProxyTunnel) {
	tlsnum := ErrTypeCount["tls"]
	kcpnum := ErrTypeCount["kcp"]
	quicum := ErrTypeCount["quic"]
	if quicum < tlsnum {
		c = NewProxy("quic")
	} else {
		if kcpnum < tlsnum {

			c = NewProxy("kcp")
		} else {
			c = NewProxy("tls")
		}

	}

	AddProxy(c)
	return
}

func GetProxyByID(name string) (c *base.ProxyTunnel) {
	for _, tun := range Tunnels {
		if tun.GetConfig().ID == name {
			return tun
		} else {

		}
	}
	return
}

func NewProxy(tp string) *base.ProxyTunnel {
	switch tp {
	case "tls":
		config := base.RandomConfig()

		protocl := protls.NewTlsServer(config)
		tunel := base.NewProxyTunnel(protocl)
		return tunel
	case "kcp":
		config := base.RandomConfig()
		protocl := prokcp.NewKcpServer(config)
		tunel := base.NewProxyTunnel(protocl)
		return tunel
	case "quic":
		config := base.RandomConfig()
		protocl := proquic.NewQuicServer(config)
		tunel := base.NewProxyTunnel(protocl)
		return tunel
	default:
		config := base.RandomConfig()
		protocl := protls.NewTlsServer(config)
		tunel := base.NewProxyTunnel(protocl)
		return tunel
	}
}
