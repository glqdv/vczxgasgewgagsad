package deploy

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"text/template"
	"time"

	"gitee.com/dark.H/ProxyZ/asset"
	"gitee.com/dark.H/ProxyZ/clientcontroll"
	"gitee.com/dark.H/ProxyZ/connections/prodns"
	"gitee.com/dark.H/ProxyZ/router"
	"gitee.com/dark.H/gs"
)

type ClientInterface interface {
	TryClose()
	ChangeRoute(string) bool
	Socks5Listen(ini ...bool) error
	HttpListen() error
	DNSListen()
	ChangePort(int)
	GetRoute() string
	GetRouteLoc() string
	SetRouteLoc(loc string)
	SetChangeRoute(f func() string)
	IfRunning() bool
	ChangeProxyType(tp string)
	GetListenPort() (socks5port, httpport, dnsport int)
}

type HTTPAPIConfig struct {
	ClientConf ClientInterface
	Routes     gs.List[*Onevps]
	Logined    bool
}

var (
	globalClient = &HTTPAPIConfig{}
	LOCAL_PORT   = 1091
)

func LoadPage(name string, data any) []byte {
	buf, _ := asset.Asset("Resources/pages/" + name)
	text := string(buf)
	buffer := bytes.NewBuffer([]byte{})
	t, _ := template.New(name).Parse(text)
	// gs.S(data).Println()
	t.Execute(buffer, data)
	return buffer.Bytes()
}

func _switch(i string) (float64, error) {
	I := gs.Str(i)
	var err error
	var t float64
	if I.In("ms") {
		t, err = strconv.ParseFloat(I.Replace("ms", "").Str(), 64)
	} else if I.In("s") {
		t, err = strconv.ParseFloat(I.Replace("s", "").Str(), 64)
		t = t * 1000
	} else if I.In("minute") {
		t, err = strconv.ParseFloat(I.Replace("minute", "").Str(), 64)
		t = t * 1000 * 60
	} else if I.In("hour") {
		t, err = strconv.ParseFloat(I.Replace("hour", "").Str(), 64)
		t = t * 1000 * 60 * 60
	}
	return t, err

}
func localSetupHandler() http.Handler {
	mux := http.NewServeMux()
	apath := gs.Str("~").ExpandUser().PathJoin(".config", "proxy-z.auth.conf")
	hpath := gs.Str("~").ExpandUser().PathJoin(".config", "proxy-z.host.conf")
	gs.Str("~").ExpandUser().PathJoin(".config").Mkdir()
	s := gs.Str("~").ExpandUser().PathJoin(".config", "local.conf")
	s.Dirname().Mkdir()
	prodns.LoadLocalRule(s.Str())
	user := ""
	pwd := ""
	last := ""
	ST := false
	if router.IsOpen() {
		ST = true
	}
	proxy := ""
	go func() {
		inter := time.NewTicker(10 * time.Minute)
		dnsCheck := time.NewTicker(30 * time.Second)
		for {
			select {
			case <-dnsCheck.C:
				if globalClient.ClientConf != nil {
					if !router.IsDNSRunning() {
						if router.IsRouter() {
							gs.Str("DNS Service is Closed").Color("r").Println("Check")
							go globalClient.ClientConf.DNSListen()
						}
					} else {
						// gs.Str("DNS Service is RUNNING").Color("g").Println("Check")
					}
				}
				if router.IsStartRouteMode() {
					if !router.IsRedsocksRunning() || !router.IsRouteRedirectOk() {
						gs.Str("Route iptables/redsocks Closed").Color("r").Println("Check")
						router.RestartRouterMode()

					} else {
						// gs.Str("Route iptables/redsocks RUNNING").Color("g").Println("Check")
					}
				}
			case <-inter.C:

				if globalClient.Routes.Count() > 0 {
					globalClient.Routes = TestRoutes(globalClient.Routes)
					globalClient.Routes.Every(func(no int, i *Onevps) {
						if globalClient.ClientConf != nil && i.Host == globalClient.ClientConf.GetRoute() {
							loc := i.Location
							loc += " " + gs.Str("%4dms").F(i.ConnectedQuality.Milliseconds()).Str()
							if globalClient.ClientConf != nil {
								globalClient.ClientConf.SetRouteLoc(loc)
							}
						}

					})

					if globalClient.ClientConf != nil {
						globalClient.ClientConf.SetChangeRoute(func() string {
							var it *Onevps

							globalClient.Routes.Every(func(no int, i *Onevps) {
								if it == nil {
									it = i
								} else {

									fi, err1 := _switch(i.Speed)
									fit, err2 := _switch(it.Speed)
									if err1 == nil && err2 == nil {
										if fit < fi {
											it = i
										}
									}
								}
							})
							if it != nil {
								return it.Host
							} else {
								return ""
							}

						})

					}
				}

			default:
				time.Sleep(12 * time.Second)
			}
		}
	}()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if globalClient.Routes.Count() == 0 {

			http.Redirect(w, r, "/z-login", http.StatusSeeOther)

			return
		}

		if r.Method == "GET" {
			// globalClient.Routes.Every(func(no int, i *Onevps) {
			// 	i.Println()
			// })
			w.Write(LoadPage("map.html", globalClient.Routes))
			// w.Write(LoadPage("route.html", globalClient.Routes))
			return
		}
	})
	mux.HandleFunc("/log", func(w http.ResponseWriter, r *http.Request) {
		name := r.Form.Get("name")
		if name == "" {
			fp, err := os.Open("/tmp/z.log")
			if err != nil {
				w.Write([]byte(err.Error()))
			}
			defer fp.Close()
			io.Copy(w, fp)
		} else {
			fp, err := os.Open("/tmp/z-lcd.log")
			if err != nil {
				w.Write([]byte(err.Error()))
			}
			defer fp.Close()
			io.Copy(w, fp)
		}
		return

	})
	mux.HandleFunc("/z-rule", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Write(LoadPage("local.html", nil))
		} else {
			d, err := Recv(r.Body)
			if err != nil {
				w.WriteHeader(400)
				Reply(w, err, false)
				return
			}
			op := d["op"].(string)
			switch op {
			case "get-rule":
				if s := gs.Str("~").ExpandUser().PathJoin(".config", "local.conf"); s.IsExists() {
					Reply(w, s.MustAsFile(), true)
					return
				} else {
					Reply(w, "# no rule ", true)
					return
				}
			case "set-rule":
				if rule, ok := d["rule"]; ok {
					s := gs.Str("~").ExpandUser().PathJoin(".config", "local.conf")
					s.Dirname().Mkdir()
					gs.Str(rule.(string)).ToFile(s.Str(), gs.O_NEW_WRITE)
					gs.Str(rule.(string)).Println("Update Local Rule")
					prodns.LoadLocalRule(s.Str())
					Reply(w, rule, true)
					return
				} else {
					if s := gs.Str("~").ExpandUser().PathJoin(".config", "local.conf"); s.IsExists() {
						Reply(w, s.MustAsFile(), true)
						return
					} else {
						Reply(w, "# no rule ", true)
						return
					}
				}

			}
		}
	})

	mux.HandleFunc("/z-login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Write(LoadPage("login.html", nil))
		} else {
			// fmt.Println(r.Body)
			d, err := Recv(r.Body)
			if err != nil {
				w.WriteHeader(400)
				Reply(w, err, false)
				return
			}
			user = d["name"].(string)
			pwd = d["password"].(string)
			// gs.Str(user + ":" + pwd).Println()

			if e, ok := d["last"]; ok {
				last = e.(string)
				// useLast = true
			}

			if e, ok := d["proxy"]; ok {
				proxy = e.(string)

			}
			gs.Str("Start pull all routes ").Println("init")
			if vpss := GitGetAccount("https://"+string(gs.Str("55594657571e515d5f1f5653405b1c7a1d53541c555946").Derypt("2022")), user, pwd); vpss.Count() > 0 {
				globalClient.Routes = vpss
				if vpss.Count() > 0 {
					gs.Dict[any]{
						"msg": vpss,
					}.Json().ToFile(hpath.Str(), gs.O_NEW_WRITE)
				}
				useLast := false
				gs.Str("Pull Routes:%d").F(vpss.Count()).Println("init")
				if proxy == "ok" {
					go func() {
						router.StopFirewall()
						time.Sleep(3 * time.Second)
						router.StartFireWall("127.0.0.1:" + gs.S(LOCAL_PORT).Str())
					}()
				}
				if last != "" {
					globalClient.Routes.Every(func(no int, i *Onevps) {
						if i.Host == last {
							useLast = true
						}
					})
				}
				d.Json().ToFile(apath.Str(), gs.O_NEW_WRITE)
				gs.Str("start test route").Println("login")
				go TestRoutes(globalClient.Routes)
				if useLast {
					gs.Str("use last login :" + last).Println("login")
					if globalClient.ClientConf == nil {
						globalClient.ClientConf = clientcontroll.NewClientControll(last, LOCAL_PORT)
						// globalClient.ClientConf
						globalClient.ClientConf.SetChangeRoute(func() string {
							var it *Onevps

							globalClient.Routes.Every(func(no int, i *Onevps) {
								if it == nil {
									it = i
								} else {

									fi, err1 := _switch(i.Speed)
									fit, err2 := _switch(it.Speed)
									if err1 == nil && err2 == nil {
										if fit < fi {
											it = i
										}
									}
								}
							})
							if it != nil {
								return it.Host
							} else {
								return ""
							}

						})
						loc := "Unknow"

						globalClient.Routes.Every(func(no int, i *Onevps) {
							if i.Host == last {
								loc = i.Location
								loc += " " + gs.Str("%5s").F(i.ConnectedQuality.String()).Str()
							}
						})
						globalClient.ClientConf.SetRouteLoc(loc)
						go globalClient.ClientConf.Socks5Listen()

					} else {
						gs.Str("Close Old!").Color("g", "B").Println("Swtich")
						globalClient.ClientConf.TryClose()
						go func() {
							if !globalClient.ClientConf.ChangeRoute(last) {
								globalClient.ClientConf.SetRouteLoc("this is break, try next !!!")
							}

						}()
					}
				} else {

				}

				Reply(w, "", true)
				return
			} else {
				gs.Str("update route failed").Println("init")
				// INFO := ""
				w.WriteHeader(400)
				Reply(w, "", false)
			}
		}
	})

	mux.HandleFunc("/z-route", func(w http.ResponseWriter, r *http.Request) {
		if globalClient.Routes.Count() == 0 {
			http.Redirect(w, r, "/z-login", http.StatusSeeOther)
			return
		}
		d, err := Recv(r.Body)
		if err != nil {
			w.WriteHeader(400)
			Reply(w, err, false)
			return
		}
		if d == nil {
			Reply(w, "alive", true)
			return
		}

		if op, ok := d["op"]; ok {
			gs.S(op).Println("z-router")
			switch op {
			case "open/close":
				if ST {
					router.StopFirewall()
					proxy = ""
					Reply(w, "stop", true)
					if user != "" && pwd != "" && last != "" {
						gs.Dict[any]{
							"name":     user,
							"password": pwd,
							"last":     last,
							"proxy":    "",
						}.Json().ToFile(apath.Str(), gs.O_NEW_WRITE)
					}
					// Reply(w, "> China-net", true)
					ST = false
					return
				} else {
					router.StartFireWall("127.0.0.1:" + gs.S(LOCAL_PORT).Str())
					ST = true
					if globalClient.ClientConf != nil {
						Reply(w, ">"+globalClient.ClientConf.GetRoute(), true)
					} else {
						Reply(w, "> no host ", true)
					}

					return
				}

			case "start":
				router.StartFireWall("127.0.0.1:" + gs.S(LOCAL_PORT).Str())
				if user != "" && pwd != "" && last != "" {
					gs.Dict[any]{
						"name":     user,
						"password": pwd,
						"last":     last,
						"proxy":    "ok",
					}.Json().ToFile(apath.Str(), gs.O_NEW_WRITE)
				}
				Reply(w, "start", true)
				return
			case "check":
				if router.IsRouter() {
					can, ifstart, status := router.CheckStatus()
					Reply(w, gs.Dict[any]{
						"can":     can,
						"running": ifstart,
						"healty":  status,
					}, true)

				} else {

					Reply(w, gs.Dict[any]{
						"can":     false,
						"running": false,
						"healty":  false,
					}, true)

				}
				return
			case "stop":
				router.StopFirewall()
				proxy = ""
				Reply(w, "stop", true)
				if user != "" && pwd != "" && last != "" {
					gs.Dict[any]{
						"name":     user,
						"password": pwd,
						"last":     last,
						"proxy":    "",
					}.Json().ToFile(apath.Str(), gs.O_NEW_WRITE)
				}
				return
			}
		}
		Reply(w, "err", false)

	})

	mux.HandleFunc("/z-api", func(w http.ResponseWriter, r *http.Request) {
		if globalClient.Routes.Count() == 0 {
			http.Redirect(w, r, "/z-login", http.StatusSeeOther)
			return
		}
		// if globalClient.ClientConf == nil {
		// 	http.Redirect(w, r, "/z-login", http.StatusSeeOther)
		// 	return
		// }
		d, err := Recv(r.Body)
		if err != nil {
			w.WriteHeader(400)
			Reply(w, err, false)
			return
		}
		if d == nil {
			Reply(w, "alive", true)
			return
		}
		// gs.S(d).Println("API")
		if op, ok := d["op"]; ok {
			switch op {
			case "connect":
				if user, ok := d["user"]; ok && user != nil {
					if pwd, ok := d["pwd"]; ok && pwd != nil {
						go func() {
							if vpss := GitGetAccount("https://"+string(gs.Str("55594657571e515d5f1f5653405b1c7a1d53541c555946").Derypt("2022")), user.(string), pwd.(string)); vpss.Count() > 0 {
								globalClient.Routes = vpss
							}
						}()
						Reply(w, "ok", true)
						return
					}
				}
			case "change":
				if proxyTp, ok := d["proxy-type"]; ok {
					go globalClient.ClientConf.ChangeProxyType(proxyTp.(string))
					Reply(w, "change proxy :"+proxyTp.(string), true)
				} else {
					Reply(w, "faled", false)
				}
				return
			case "global":
				if host, ok := d["state"]; ok && host != nil {
					switch host.(type) {
					case string:
						switch host.(string) {
						case "on", "start":
							sport, _, _ := globalClient.ClientConf.GetListenPort()
							SetGlobalMode(sport)
						default:
							SetGlobalModeOff()
						}
					}
					Reply(w, "ok", true)
					return
				}
				Reply(w, "failed", false)
				return

			case "switch":
				if globalClient.ClientConf != nil && !globalClient.ClientConf.IfRunning() {
					Reply(w, "wait ...", false)
					return
				}
				if host, ok := d["host"]; ok && host != nil {
					gs.Str(host.(string)).Color("g", "B").Println("Swtich")
					if globalClient.ClientConf == nil {
						globalClient.ClientConf = clientcontroll.NewClientControll(host.(string), LOCAL_PORT)
						// globalClient.ClientConf
						last = host.(string)
						gs.Dict[any]{
							"name":     user,
							"password": pwd,
							"last":     host,
							"proxy":    proxy,
						}.Json().ToFile(apath.Str(), gs.O_NEW_WRITE)
						loc := "Unknow"
						globalClient.Routes.Every(func(no int, i *Onevps) {
							if i.Host == last {
								loc = i.Location
								loc += " " + gs.Str("%4dms").F(i.ConnectedQuality.Milliseconds()).Str()
							}
						})
						globalClient.ClientConf.SetRouteLoc(loc)

						go globalClient.ClientConf.Socks5Listen()
					} else {
						gs.Str("Close Old!").Color("g", "B").Println("Swtich")
						globalClient.ClientConf.TryClose()
						last = host.(string)
						loc := "Unknow"
						globalClient.Routes.Every(func(no int, i *Onevps) {
							if i.Host == last {
								loc = i.Location
								loc += " " + gs.Str("%4dms").F(i.ConnectedQuality.Milliseconds()).Str()
							}
						})
						globalClient.ClientConf.SetRouteLoc(loc)
						go globalClient.ClientConf.ChangeRoute(host.(string))
						gs.Dict[any]{
							"name":     user,
							"password": pwd,
							"last":     host,
							"proxy":    proxy,
						}.Json().ToFile(apath.Str(), gs.O_NEW_WRITE)
					}
					Reply(w, "ok", true)
				} else {
					gs.Str("No Host So use next").Println("Switch")
					nowhost := ""
					nowix := -1
					if globalClient.ClientConf != nil {
						// Reply(w, "no init ", true)
						globalClient.ClientConf.TryClose()
						nowhost = globalClient.ClientConf.GetRoute()
						// return
					} else {

						nowhost = string(globalClient.Routes[0].Host)
						globalClient.ClientConf = clientcontroll.NewClientControll(nowhost, LOCAL_PORT)

						go globalClient.ClientConf.Socks5Listen()
						gs.Dict[any]{
							"name":     user,
							"password": pwd,
							"last":     nowhost,
							"proxy":    proxy,
						}.Json().ToFile(apath.Str(), gs.O_NEW_WRITE)
						Reply(w, "!"+nowhost, true)
						return
					}
					var oo *Onevps = nil
					loc := "unknow"
					gs.Str("Choose:" + nowhost).Println("Switch")
					globalClient.Routes.Every(func(no int, i *Onevps) {
						if i.Host == nowhost {
							nowix = no
						}
						if oo == nil {
							oo = i
						} else {
							if no == nowix+1 {
								oo = i
								loc = i.Location
								loc += " " + gs.Str("%4dms").F(i.ConnectedQuality.Milliseconds()).Str()
							}
						}

					})
					if globalClient.ClientConf != nil {
						globalClient.ClientConf.SetRouteLoc(loc)
						go globalClient.ClientConf.ChangeRoute(oo.Host)
					}
					// last = host.(string)
					gs.Dict[any]{
						"name":     user,
						"password": pwd,
						"last":     oo.Host,
						"proxy":    proxy,
					}.Json().ToFile(apath.Str(), gs.O_NEW_WRITE)
					Reply(w, "!"+oo.Host, true)
				}
				return
			case "check":
				if globalClient.ClientConf != nil {
					ee := ""
					if ST {
						ee = "route"
						Reply(w, gs.Dict[any]{
							"mode":    ee,
							"loc":     globalClient.ClientConf.GetRouteLoc(),
							"running": globalClient.ClientConf.GetRoute(),
						}, true)
					} else {
						Reply(w, gs.Dict[any]{
							"mode":    "",
							"loc":     "China",
							"running": globalClient.ClientConf.GetRoute(),
						}, true)

					}
				} else {
					Reply(w, gs.Dict[any]{
						"mode":    "",
						"loc":     "China-no-login",
						"running": "login first!",
					}, true)
				}
				return

			case "test":
				Reply(w, globalClient.Routes, true)
				return

			}
		}
		Reply(w, "err", false)

	})
	return mux

}

func LocalAPI(openbrowser, global bool) {
	server := &http.Server{
		Handler:      localSetupHandler(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		Addr:         "0.0.0.0:35555",
	}
	if !openbrowser {
		go func() {
			time.Sleep(2 * time.Second)
			if runtime.GOOS == "windows" {
				gs.Str("start http://localhost:35555/").Exec()
			} else if runtime.GOOS == "darwin" {
				gs.Str("open http://localhost:35555/").Exec()
			}
		}()
	}

	if global {
		SetGlobalMode(LOCAL_PORT)
	}
	err := server.ListenAndServe()
	if err != nil {
		gs.Str(err.Error()).Color("r").Println("Err")
	}
}

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
	// fmt.Println(gs.S(buf))
	if d := gs.Str(buf).Json(); len(d) > 0 {
		return d, nil
	}
	return nil, nil
}

func GetState() string {
	if router.IsOpen() {
		return "router"
	} else {
		return ""
	}
}
