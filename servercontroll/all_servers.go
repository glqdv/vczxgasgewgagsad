package servercontroll

import (
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"gitee.com/dark.H/ProxyZ/connections/base"
	"gitee.com/dark.H/ProxyZ/update"
	"gitee.com/dark.H/ProxyZ/vpn"
	"gitee.com/dark.H/gs"
)

func setupHandler(www string) http.Handler {
	mux := http.NewServeMux()
	base.CloseALLPortUFW()

	go func() {
		for {
			time.Sleep(30 * time.Minute)
			if time.Now().Hour() == 0 {
				gs.Str("Start Refresh All Routes").Println()
				ids := gs.List[string]{}
				Tunnels.Every(func(no int, i *base.ProxyTunnel) {
					ids = append(ids, i.GetConfig().ID)
				})

				ids.Every(func(no int, i string) {
					DelProxy(i)
				})
			}
		}
	}()
	if len(www) > 0 {
		mux.HandleFunc("/z-files", func(w http.ResponseWriter, r *http.Request) {
			fs := gs.List[any]{}
			gs.Str(www).Ls().Every(func(no int, i gs.Str) {
				isDir := i.IsDir()
				name := i.Basename()
				size := i.FileSize()
				fs = fs.Add(gs.Dict[any]{
					"name":  name,
					"isDir": isDir,
					"size":  size,
				})
			})
			Reply(w, fs, true)

		})
		mux.Handle("/z-files-d/", http.StripPrefix("/z-files-d/", http.FileServer(http.Dir(www))))
		mux.HandleFunc("/z-files-u", uploadFileFunc(www))
	}
	mux.HandleFunc("/z-info", func(w http.ResponseWriter, r *http.Request) {
		d, err := Recv(r.Body)
		if err != nil {
			w.WriteHeader(400)
			Reply(w, err, false)
		}
		if d == nil {
			Reply(w, "alive", true)
		}
	})

	mux.HandleFunc("/z-set", func(w http.ResponseWriter, r *http.Request) {
		d, err := Recv(r.Body)
		if err != nil {
			w.WriteHeader(400)
			Reply(w, err, false)
			return
		}

		if d == nil {
			Reply(w, "no val", false)
			return
		}
		if name, ok := d["name"]; ok {
			if val, ok := d["val"]; ok {
				gs.Str(val.(string)).Color("g", "B").Println(name.(string))
				switch name.(string) {
				case "vpn":
					switch val.(string) {
					case "on", "start", "Start":
						vpnHandler := vpn.NewVPNHandlerServer()
						vpnHandler.Init()
						Tunnels.Every(func(no int, i *base.ProxyTunnel) {
							i.SetVPN(vpnHandler)
						})
					case "off", "clear", "stop", "kill":
						Tunnels.Every(func(no int, i *base.ProxyTunnel) {
							i.ClearVPN()
						})

					}
				}
				Reply(w, "Good ", true)
				return

			}

		}
	})

	mux.HandleFunc("/proxy-info", func(w http.ResponseWriter, r *http.Request) {
		ids := []string{}
		proxy := gs.Dict[int]{
			"tls":  0,
			"quic": 0,
			"kcp":  0,
		}
		aliveProxy := gs.Dict[int]{
			"tls":  0,
			"quic": 0,
			"kcp":  0,
		}

		Tunnels.Every(func(no int, i *base.ProxyTunnel) {
			proxy[i.GetConfig().ProxyType] += 1
			aliveProxy[i.GetConfig().ProxyType] += i.GetConnectNum()
			ids = append(ids, i.GetConfig().ID)
		})
		Reply(w, gs.Dict[any]{
			"ids":   ids,
			"alive": aliveProxy,
			"proxy": proxy,
			"err":   ErrTypeCount,
		}, true)
	})

	mux.HandleFunc("/z-dns", func(w http.ResponseWriter, r *http.Request) {
		d, err := Recv(r.Body)
		if err != nil {
			Reply(w, err, false)
			return
		}

		if hostsStr, ok := d["hosts"]; ok {
			res := gs.Dict[any]{}
			for _, host := range gs.Str(hostsStr.(string)).Split(",") {
				gs.Str(host).Println("Query DNS")
				if ips, err := net.LookupHost(host.Str()); err == nil {
					for _, _ip := range ips {
						res[_ip] = host
					}
				}
			}
			Reply(w, res, true)

		} else {
			Reply(w, "no dns", false)
		}
	})

	mux.HandleFunc("/z-ufw-close-all", func(w http.ResponseWriter, r *http.Request) {
		base.CloseALLPortUFW()
		Reply(w, base.GetUFW(), true)

	})

	mux.HandleFunc("/z-ufw", func(w http.ResponseWriter, r *http.Request) {
		d, err := Recv(r.Body)
		if err != nil {
			Reply(w, err, false)
			return
		}
		if d == nil {
			Reply(w, base.GetUFW(), true)
			return
		}
		if port, ok := d["port"]; ok && port != nil {
			switch port.(type) {
			case int:
				if op, ok := d["op"]; ok && op != nil {
					if op == "" {
						base.OpenPortUFW(port.(int))
					}
				} else {
					base.ClosePortUFW(port.(int))
				}

			case string:
				t := gs.Str(port.(string))
				if t.In("\n") {
					t.Split("\n").Every(func(no int, i gs.Str) {
						pi, err := strconv.Atoi(i.Trim().Str())
						if err != nil {
						} else {
							base.ClosePortUFW(pi)
						}
					})
				} else if t.In(",") {
					t.Split(",").Every(func(no int, i gs.Str) {
						pi, err := strconv.Atoi(i.Trim().Str())
						if err != nil {
						} else {
							base.ClosePortUFW(pi)
						}
					})
				} else {

				}
				i, err := strconv.Atoi(port.(string))
				if err != nil {
				} else {
					base.ClosePortUFW(i)
				}
			}
		}
		Reply(w, base.GetUFW(), true)
	})

	mux.HandleFunc("/proxy-get", func(w http.ResponseWriter, r *http.Request) {
		d, err := Recv(r.Body)
		if err != nil {
			Reply(w, err, false)
			return
		}
		if d == nil {
			tu := GetProxy()
			if !tu.On {

				afterID := tu.GetConfig().ID
				// gs.Str("start id" + afterID).Println()
				err := tu.Start(func() {
					DelProxy(afterID)
				})
				if err != nil {
					Reply(w, err, false)
					return
				}
			}
			str := tu.GetConfig()
			// gs.Str(str.ProxyType + " in port %d").F(str.ServerPort).Color("g").Println("get")
			Reply(w, str, true)
		} else {
			if proxyType, ok := d["type"]; ok {
				switch proxyType.(type) {
				case string:
					tu := GetProxy(proxyType.(string))
					if !tu.On {
						afterID := tu.GetConfig().ID
						err := tu.Start(func() {
							DelProxy(afterID)
						})
						if err != nil {
							Reply(w, err, false)
							return
						}
					}
					str := tu.GetConfig()
					gs.Str(str.ProxyType + " in port %d").F(str.ServerPort).Color("g").Println("get")
					Reply(w, str, true)
				default:
					tu := GetProxy()
					if !tu.On {
						afterID := tu.GetConfig().ID
						err := tu.Start(func() {
							DelProxy(afterID)
						})
						if err != nil {
							Reply(w, err, false)
							return
						}
					}
					str := tu.GetConfig()
					gs.Str(str.ProxyType + " in port %d").F(str.ServerPort).Color("g").Println("get")
					Reply(w, str, true)
				}
			} else {
				tu := GetProxy()
				if !tu.On {
					afterID := tu.GetConfig().ID
					err := tu.Start(func() {
						DelProxy(afterID)
					})
					if err != nil {
						Reply(w, err, false)
						return
					}
				}
				str := tu.GetConfig()
				gs.Str(str.ProxyType + " in port %d").F(str.ServerPort).Color("g").Println("get")
				Reply(w, str, true)
			}
		}
	})

	mux.HandleFunc("/z-log", func(w http.ResponseWriter, r *http.Request) {
		if gs.Str("/tmp/z.log").IsExists() {
			fp, err := os.Open("/tmp/z.log")
			if err != nil {
				w.Write([]byte(err.Error()))
			} else {
				defer fp.Close()
				io.Copy(w, fp)
			}
		} else {
			w.Write(gs.Str("/tmp/z.log not exists !!!").Bytes())
		}
	})

	mux.HandleFunc("/c-log", func(w http.ResponseWriter, r *http.Request) {
		if gs.Str("/tmp/z.log").IsExists() {
			defer gs.Str("").ToFile("/tmp/z.log", gs.O_NEW_WRITE)
			fp, err := os.Open("/tmp/z.log")
			if err != nil {
				w.Write([]byte(err.Error()))
			} else {
				defer fp.Close()
				io.Copy(w, fp)
			}
		} else {
			w.Write(gs.Str("/tmp/z.log not exists !!!").Bytes())
		}
	})

	mux.HandleFunc("/__close-all", func(w http.ResponseWriter, r *http.Request) {
		ids := gs.List[string]{}
		Tunnels.Every(func(no int, i *base.ProxyTunnel) {
			ids = append(ids, i.GetConfig().ID)
		})

		ids.Every(func(no int, i string) {
			DelProxy(i)
		})
		Reply(w, gs.Dict[gs.List[string]]{
			"ids": ids,
		}, true)
	})

	mux.HandleFunc("/z11-update", func(w http.ResponseWriter, r *http.Request) {
		ids := gs.List[string]{}
		Tunnels.Every(func(no int, i *base.ProxyTunnel) {
			ids = append(ids, i.GetConfig().ID)
		})

		ids.Every(func(no int, i string) {
			DelProxy(i)
		})
		go update.Update(func(info string, ok bool) {
			Reply(w, info, ok)
		})
		Reply(w, "updaing... wait 3 s", true)

		// os.Exit(0)
		// }
	})

	mux.HandleFunc("/proxy-err", func(w http.ResponseWriter, r *http.Request) {

		d, err := Recv(r.Body)

		if err != nil {
			w.WriteHeader(400)
			Reply(w, err, false)
		}
		if id, ok := d["Host"]; ok && id != nil {
			switch id.(type) {
			case []any:
				hosts := id.([]any)
				wait := sync.WaitGroup{}
				errn := 0
				for _, h := range hosts {
					wait.Add(1)
					go func(hh string, w *sync.WaitGroup) {
						defer w.Done()
						if !CanReachHOST(hh) {
							errn += 1
						}
					}(h.(string), &wait)
				}
				wait.Wait()
				if errn == 0 {
					Reply(w, "status ok:"+gs.S(id), false)
					return
				}
			case string:
				if CanReachHOST(id.(string)) {
					Reply(w, "status ok:"+id.(string), false)
					return
				}
			}

		}
		if id, ok := d["ID"]; ok && id != nil {
			idstr := id.(string)
			gs.Str(idstr).Color("r").Println("proxy-err")
			DelProxy(idstr)
		}
		tu := NewProxyByErrCount()
		afterID := tu.GetConfig().ID
		err = tu.Start(func() {
			DelProxy(afterID)
		})
		if err != nil {
			Reply(w, err, false)
			return
		}
		c := tu.GetConfig()
		Reply(w, c, true)

	})

	mux.HandleFunc("/proxy-new", func(w http.ResponseWriter, r *http.Request) {
		tu := NewProxy("tls")

		str := tu.GetConfig()
		Reply(w, str, true)
	})

	mux.HandleFunc("/proxy-del", func(w http.ResponseWriter, r *http.Request) {
		d, err := Recv(r.Body)
		if err != nil {
			w.WriteHeader(400)
			Reply(w, err, false)
		}
		configName := d["msg"].(string)

		str := DelProxy(configName)
		Reply(w, str, true)
	})
	return mux
}
