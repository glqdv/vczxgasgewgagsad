package servercontroll

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"time"

	"gitee.com/dark.H/ProxyZ/connections/base"
	"gitee.com/dark.H/ProxyZ/connections/prokcp"
	"gitee.com/dark.H/ProxyZ/connections/proquic"
	"gitee.com/dark.H/ProxyZ/connections/prosmux"
	"gitee.com/dark.H/ProxyZ/connections/prosocks5"
	"gitee.com/dark.H/ProxyZ/connections/protls"
	"gitee.com/dark.H/gs"
)

var (
	NotHost = gs.List[string]{}
)

func TestServer(server string) (t time.Duration, IDS gs.List[string]) {
	st := time.Now()
	ok := true
	f := ""
	if !gs.Str(server).In(":55443") {
		server += ":55443"
	}
	if gs.Str(server).In("://") {
		f = "https://" + gs.Str(server).Split("://")[1].Str()
	} else {
		f = "https://" + gs.Str(server).Str()
	}

	if res, err := HTTPSGet(f + "/proxy-info"); err == nil {

		res.Json().Every(func(k string, v any) {
			if k == "status" {
				// gs.S(v).Color("g").Println(server)
				if v != "ok" {
					gs.Str("server is not alive !" + v.(string)).Color("r").Println(server)
					ok = false
				}
			} else if k == "msg" {
				switch v.(type) {
				case map[string]any:
					v := gs.AsDict[any](v)
					idsS := v["ids"].([]any)
					// gs.Str("id: %d").F(len(idsS)).Println(server)
					for _, i := range idsS {
						IDS = IDS.Add(i.(string))
					}

				}

			} else if k == "ids" {
				idsS := v.([]any)
				// gs.Str("id: %d").F(len(idsS)).Println()
				for _, i := range idsS {
					IDS = IDS.Add(i.(string))
				}
			}
		})
	} else {
		gs.Str("err:" + err.Error()).Color("r").Println("TestServer")
		return time.Duration(30000) * time.Millisecond, IDS
	}
	if !ok {
		return time.Duration(30000) * time.Millisecond, IDS
	}
	return time.Since(st), IDS
}

func SendUpdate(server string) {
	f := ""
	if !gs.Str(server).In(":55443") {
		server += ":55443"
	}
	if gs.Str(server).In("://") {
		f = "https://" + gs.Str(server).Split("://")[1].Str()
	} else {
		f = "https://" + gs.Str(server).Str()
	}
	res, err := HTTPSPost(f+"/z11-update", nil)
	if err == nil {
		res.Json().Every(func(k string, v any) {
			gs.S(v).Color("g").Println(server + " > " + k)
		})
	}

}

func CanReachHOST(host string) bool {
	con, err := net.DialTimeout("tcp", host, 7*time.Second)
	if err == nil {
		defer con.Close()
		return true
	}

	return false
}

func GetProxyConfig(host, tp string) *base.ProtocolConfig {
	data := gs.Dict[any]{
		"type": tp,
	}
	H := gs.Str(host)
	if !H.StartsWith("http") {
		host = "https://" + host
	}
	if !H.In(":55443") {
		host += ":55443"
	}
	server := gs.Str(host).Split("/")[1].Split(":")[0]
	reply, err := HTTPSPost(host+"/proxy-get", data, 10)
	if err != nil {
		gs.Str(err.Error()).Color("r").Println()
		return nil
	}
	d := gs.Dict[any]{}
	err = json.Unmarshal([]byte(reply), &d)
	if err != nil {
		gs.Str(err.Error()).Color("r").Println()
		return nil
	}
	if c, ok := d["status"]; ok {
		if c.(string) == "ok" {
			di := d["msg"]
			buf, err := json.Marshal(di)
			if err != nil {
				gs.Str(err.Error()).Color("r").Println()
				return nil
			}
			conf := new(base.ProtocolConfig)
			// gs.Str("read 2").Color("g").Println("test1.7")
			if err := json.Unmarshal(buf, conf); err != nil {
				gs.Str(err.Error()).Color("r").Println()
				return nil
			}
			// gs.Str("read 3").Color("g").Println("test1.7")
			if conf.Server == "0.0.0.0" {
				conf.Server = server
			}
		}

		return nil
	} else {
		return nil
	}

}

func GetConn(conf *base.ProtocolConfig) (net.Conn, error) {
	// gs.Str("a").Println("test3")
	var singleTunnelConn net.Conn
	var err error
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
		return nil, err
	}

	// gs.Str("--> "+conf.RemoteAddr()).Color("y", "B").Println(conf.ProxyType)
	if singleTunnelConn != nil && conf.ProxyType != "quic" {
		smux := prosmux.NewSmuxClient(singleTunnelConn, conf.ID, conf.ProxyType)
		return smux.NewConnnect()
	} else if conf.ProxyType == "quic" {
		// gs.Str("test Enter be").Println(conf.ProxyType)
		qc, err := proquic.NewQuicClient(conf)
		if err != nil {
			return nil, err
		}
		return qc.NewConnnect()

	} else {
		if err == nil {
			err = errors.New("tls/kcp only :  now method is :" + conf.ProxyType)
		}
		return nil, err
	}
}

func TestHost(host string) time.Duration {
	wait := sync.WaitGroup{}
	tim := gs.List[time.Duration]{}
	gs.List[string]{
		"https://www.google.com",
		"https://www.bing.com",
		"https://twitter.com",
	}.Every(func(no int, domain string) {
		wait.Add(1)
		go func(w *sync.WaitGroup) {
			defer w.Done()
			tp := "quic"
			if no == 1 {
				tp = "kcp"
			} else if no == 2 {
				tp = "tls"
			}
			config := GetProxyConfig(host, tp)
			if config == nil {
				tim = tim.Add((3000 * time.Millisecond))
				return
			}
			st := time.Now()
			con, err := GetConn(config)
			if err != nil {
				tim = tim.Add((3000 * time.Millisecond))
				return
			}
			con.SetReadDeadline(time.Now().Add(7 * time.Second))
			defer con.Close()

			data := prosocks5.HostToRaw(domain, 443)
			con.Write(data)
			re := make([]byte, 200)
			if n, err := con.Read(re); err == nil {
				if bytes.Equal(re[:n], prosocks5.Socks5Confirm) {
					tim = tim.Add(time.Since(st))
				}
			} else {
				tim = tim.Add((3000 * time.Millisecond))
				return
			}
		}(&wait)
	})
	wait.Wait()
	c := time.Duration(0)
	tim.Every(func(no int, i time.Duration) {
		c += i
	})

	return c / 3
}
