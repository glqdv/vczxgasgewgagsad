package router

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"gitee.com/dark.H/ProxyZ/asset"
	"gitee.com/dark.H/ProxyZ/connections/prodns"
	"gitee.com/dark.H/gn"
	"gitee.com/dark.H/gs"
)

func redSocksStart(i int) {
	// gs.Str()
}

func GetIface() (string, string) {

	is, err := net.Interfaces()
	if err != nil {
		return "wlan", ""
	}
	for _, i := range is {
		ad, err := i.Addrs()
		// gs.Str(i.Name).Println()
		if err == nil {
			for _, a := range ad {
				ip := a.String()
				if gs.Str(ip).In("127.0.0.1") {
					continue
				}
				if gs.Str(ip).In(".1/") {
					if gs.Str(i.Name).In("Guest") {
						continue
					}
					if gs.Str(i.Name).In("guest") {
						continue
					}
					ip = string(gs.Str(ip).Split("/")[0])
					return i.Name, ip
				} else if gs.Str(ip).EndsWith(".1") {
					if gs.Str(i.Name).In("Guest") {
						continue
					}
					if gs.Str(i.Name).In("guest") {
						continue
					}

					return i.Name, ip
				}
			}
		}
	}
	return "", ""
}

func IPTahbleRouteSet(Pre string) string {
	iface, gatewayip := GetIface()
	prodns.SetConfigIP(gatewayip)
	gs.Str("iface: %s | ip: %s").F(iface, gatewayip).Println("firewall")
	if res := gn.AsReq(gs.Str("http://localhost:35555/z-api").AsRequest().SetMethod("post").SetBody(gs.Dict[any]{
		"op": "test",
	}.Json())).Go(); res.Err == nil {
		d := gs.Dict[any]{}
		err := json.Unmarshal(res.Body().Bytes(), &d)

		if err == nil {
			d2 := gs.AsList[any](d["msg"])
			render := gs.Str(`
iptables -P INPUT ACCEPT
iptables -P FORWARD ACCEPT
iptables -P OUTPUT ACCEPT
iptables -t nat -N REDSOCKS
iptables -t nat -A $${PRE} -i $${IFACE} -p tcp -j REDSOCKS
# redirct dns
iptables -t nat -A $${PRE} -p udp --dport  53 -j REDIRECT --to-ports 60053
%s
iptables -t nat -A REDSOCKS -d 0.0.0.0/8 -j RETURN
iptables -t nat -A REDSOCKS -d 10.0.0.0/8 -j RETURN
iptables -t nat -A REDSOCKS -d 127.0.0.0/8 -j RETURN
iptables -t nat -A REDSOCKS -d 192.168.0.0/16 -j RETURN
iptables -t nat -A REDSOCKS -d 169.254.0.0/16 -j RETURN
iptables -t nat -A REDSOCKS -d 172.16.0.0/12 -j RETURN
iptables -t nat -A REDSOCKS -d 224.0.0.0/4 -j RETURN
iptables -t nat -A REDSOCKS -d 240.0.0.0/4 -j RETURN


iptables -t nat -A REDSOCKS -p tcp -s $${IP} --dport $${PORT} -j RETURN
iptables -t nat -A REDSOCKS -d $${IP} -j RETURN
iptables -t nat -A REDSOCKS -p tcp -j REDIRECT --to-ports 1081
iptables -t nat -A $${PRE} -p tcp -j REDSOCKS`)
			tmp := gs.Str("")
			d2.Every(func(no int, i any) {
				i2 := gs.AsDict[any](i)
				tmp += gs.Str("iptables -t nat -A REDSOCKS -d %s -j RETURN\n").F(i2["Host"])
			})
			return string(render.F(tmp).Format(gs.Dict[string]{
				"IP":    gatewayip,
				"IFACE": iface,
				"PRE":   Pre,
			}))

		}
	}
	hpath := gs.Str("~").ExpandUser().PathJoin(".config", "proxy-z.host.conf")
	gs.Str("~").ExpandUser().PathJoin(".config").Mkdir()
	if hpath.IsExists() {
		buf := hpath.MustAsFile()
		dd := gs.Dict[any]{}
		if err := json.Unmarshal(buf.Bytes(), &dd); err == nil {
			d2 := gs.AsList[any](dd["msg"])
			render := gs.Str(`iptables -P INPUT ACCEPT
iptables -P FORWARD ACCEPT
iptables -P OUTPUT ACCEPT
iptables -t nat -N REDSOCKS
iptables -t nat -A $${PRE} -i $${IFACE} -p tcp -j REDSOCKS
# redirct dns
iptables -t nat -A $${PRE} -p udp --dport  53 -j REDIRECT --to-ports 60053
%s
iptables -t nat -A REDSOCKS -d 0.0.0.0/8 -j RETURN
iptables -t nat -A REDSOCKS -d 10.0.0.0/8 -j RETURN
iptables -t nat -A REDSOCKS -d 127.0.0.0/8 -j RETURN
iptables -t nat -A REDSOCKS -d 192.168.0.0/16 -j RETURN
iptables -t nat -A REDSOCKS -d 169.254.0.0/16 -j RETURN
iptables -t nat -A REDSOCKS -d 172.16.0.0/12 -j RETURN
iptables -t nat -A REDSOCKS -d 224.0.0.0/4 -j RETURN
iptables -t nat -A REDSOCKS -d 240.0.0.0/4 -j RETURN


iptables -t nat -A REDSOCKS -p tcp -s $${IP} --dport $${PORT} -j RETURN
iptables -t nat -A REDSOCKS -d $${IP} -j RETURN
iptables -t nat -A REDSOCKS -p tcp -j REDIRECT --to-ports 1081
iptables -t nat -A $${PRE} -p tcp -j REDSOCKS`)
			tmp := gs.Str("")
			d2.Every(func(no int, i any) {
				i2 := gs.AsDict[any](i)
				tmp += gs.Str("iptables -t nat -A REDSOCKS -d %s -j RETURN\n").F(i2["Host"])
			})
			return string(render.F(tmp).Format(gs.Dict[string]{
				"IP":    gatewayip,
				"IFACE": iface,
				"PRE":   Pre,
			}))
		}
	}

	return ""
}

func findProc(name string) (pids gs.List[int]) {
	gs.Str(`ps | grep %s  |grep -v grep | awk '{ print $1 } ' | xargs `).F(name).SplitSpace().Every(func(no int, i gs.Str) {
		if ei, err := strconv.Atoi(i.Str()); err == nil {
			pids = pids.Add(ei)
		}
	})
	return
}

func kill(name string) {
	Exec(gs.Str(`ps | grep %s | grep -v grep | awk '{ print $1 } ' | xargs kill -9 2>/dev/null;`).F(name))
}

func StartFireWall(proxyAddr string) {
	switch runtime.GOOS {
	case "linux":
		// ip := ""
		port := "1091"
		_, gatewayip := GetIface()
		gs.Str(proxyAddr).Split(":").Every(func(no int, i gs.Str) {
			if no == 0 {
				// ip = i.Trim().Str()
			} else if no == 1 {
				port = i.Trim().Str()
			}
		})
		isv7 := Exec(`cat /proc/cpuinfo`).In("ARMv7")
		if isv7 {
			gs.Str("armv7 start routing ....").Println("firewall")
			gs.Str(`
#base {
#    log_debug = on;
#    log_info = on;
#    redirector = iptables;
#    daemon = on;
#}
#            
#redsocks{
#    local_ip = 0.0.0.0;
#    local_port = 1081;
#    ip = $${ip};
#    port = $${port};
#    type = socks5;
#}

base {
    log_debug = on;
    log_info = on;
    redirector = iptables;
    daemon = on;
}

redsocks{
    bind = "0.0.0.0:1081";
    relay = "$${ip}:$${port}";
    type = socks5;
    autoproxy = 0;
    timeout = 12;
}

`).Format(gs.Dict[string]{
				"ip":   gatewayip,
				"port": port,
			}).ToFile("/tmp/redsocks.conf", gs.O_NEW_WRITE)
			gs.Str("gateway :" + gatewayip).Println("firewall")
			if !gs.Str("/usr/sbin/redsocks2").IsExists() {
				if buf, err := asset.Asset("Resources/bin/redsocks2-armv7"); err == nil {
					if _, fp, err := gs.Str("/usr/sbin/redsocks2").OpenFile(gs.O_NEW_WRITE); err == nil {
						fp.Write(buf)
						fp.Close()
						os.Chmod("/usr/sbin/redsocks2", os.FileMode(755))
					}
				}
			}
			Exec(`redsocks2 -c /tmp/redsocks.conf`)
		} else {
			gs.Str(`
base {
    log_debug = on;
    log_info = on;
    redirector = iptables;
    daemon = on;
}

redsocks{
    bind = "0.0.0.0:1081";
    relay = "$${ip}:$${port}";
    type = socks5;
    autoproxy = 0;
    timeout = 12;
}
`).Format(gs.Dict[string]{
				"ip":   gatewayip,
				"port": port,
			}).ToFile("/tmp/redsocks.conf", gs.O_NEW_WRITE)
			if !gs.Str("/usr/sbin/redsocks2").IsExists() {
				if buf, err := asset.Asset("Resources/bin/redsocks2"); err == nil {
					if _, fp, err := gs.Str("/usr/sbin/redsocks2").OpenFile(gs.O_NEW_WRITE); err == nil {
						fp.Write(buf)
						fp.Close()
						os.Chmod("/usr/sbin/redsocks2", os.FileMode(755))
					}
				}
			}
			Exec(`redsocks2 -c /tmp/redsocks.conf`)
		}

		Pre := "PREROUTING"
		if Exec("iptables -t nat  -L").In("prerouting_rule") {
			Pre = "prerouting_rule"
		}
		gs.Str(IPTahbleRouteSet(Pre)).Format(gs.Dict[string]{
			"PORT": port,
		}).ToFile("/etc/firewall.user", gs.O_NEW_WRITE)
		gs.Str("write rule to /etc/firewall.user ").Println("firewall")
		Exec("/etc/init.d/firewall restart 2> /dev/null;")
		if !Exec("iptables -t nat -L ").In("1091") {
			Exec("cat /etc/firewall.user | sh ")
		}
	}
}

func StopFirewall() {
	switch runtime.GOOS {
	case "linux":
		gs.Str("").ToFile("/etc/firewall.user", gs.O_NEW_WRITE)
		Exec("/etc/init.d/firewall restart 2> /dev/null;")
		if Exec("iptables -t nat -L ").In("1091") {
			Exec("iptables -t nat -F")
			Exec("/etc/init.d/firewall restart 2> /dev/null;")
		}
		kill("redsocks")
		// kill("red2socks")
	}
}

func CheckStatus() (can, startFirewall, status bool) {
	if runtime.GOOS == "linux" {
		if runtime.GOARCH == "arm" {
			can = true
		}
	}
	if res := gs.Str("/etc/firewall.user").MustAsFile(); res == "" {
		kill("redsocks")
		return
	} else {
		o := false
		res.EveryLine(func(lineno int, line gs.Str) {
			if line.Trim() != "" {
				if line.Trim()[0] != '#' {
					o = true
				}
			}
		})
		if !o {
			kill("redsocks")
			return
		}
		startFirewall = true
		if pids := findProc("redsocks"); pids.Count() > 0 {
			status = true
		}

	}
	return
}

func IsRouter() bool {
	return runtime.GOOS == "linux" && gs.Str(runtime.GOARCH).In("arm")
}

func IsOpen() bool {
	if IsRouter() {
		return Exec("iptables -t nat -L ").In("REDIRECT")
	}
	return false
}

func ReleaseRedsocks() {
	isv7 := runtime.GOARCH != "arm64"
	gs.Str("Is Armv7:%v").F(isv7).Println()
	if runtime.GOOS == "linux" && gs.Str(runtime.GOARCH).In("arm") {

		if !gs.Str("/etc/init.d/proxy-z").IsExists() {
			_, fp, err := gs.Str("/etc/init.d/proxy-z").OpenFile(gs.O_NEW_WRITE)
			if err == nil {
				if len(os.Args) > 0 {
					if buf, err := asset.Asset("Resources/bin/z-proxy"); err == nil {
						fp.Write(buf)

					}

				}
				fp.Close()
				if gs.Str("/etc/init.d/proxy-z").IsExists() {
					os.Chmod(gs.Str("/etc/init.d/proxy-z").Str(), os.FileMode(755))
					Exec("/etc/init.d/proxy-z enable").Println("Set Service Start Auth!")
				}
			}
		}

		if !gs.Str("/.cache/geodb").IsExists() {
			gs.Str("/.cache").Mkdir()
			_, fp, err := gs.Str("/.cache/geodb").OpenFile(gs.O_NEW_WRITE)
			if err == nil {
				if buf, err := asset.Asset("Resources/bin/geodb"); err == nil {
					fp.Write(buf)
				}
				fp.Close()
			}
		}

		if !gs.Str("/usr/local/bin/proxy-z").IsExists() {
			gs.Str("/usr/local/bin").Mkdir()
			_, fp, err := gs.Str("/usr/local/bin/proxy-z").OpenFile(gs.O_NEW_WRITE)
			if err == nil {
				if len(os.Args) > 0 {
					if buf, err := ioutil.ReadFile(os.Args[0]); err == nil {
						fp.Write(buf)

					}

				}
				fp.Close()

				if gs.Str("/usr/local/bin/proxy-z").IsExists() {
					Exec("chmod +x /usr/local/bin/proxy-z")
				}
			}
		}
		if isv7 {
			gs.Str("release redsocks v7 ").Println()
			if !gs.Str("/usr/sbin/redsocks2").IsExists() {
				if buf, err := asset.Asset("Resources/bin/redsocks2-armv7"); err == nil {
					if _, fp, err := gs.Str("/usr/sbin/redsocks2").OpenFile(gs.O_NEW_WRITE); err == nil {
						fp.Write(buf)
						fp.Close()
						os.Chmod("/usr/sbin/redsocks2", os.FileMode(755))
					}
					if buf, err := asset.Asset("Resources/bin/libevent_core-2.1.so.7"); err == nil {
						if _, fp, err := gs.Str("/usr/lib/libevent_core-2.1.so.7").OpenFile(gs.O_NEW_WRITE); err == nil {
							fp.Write(buf)
							fp.Close()
							os.Chmod("/usr/lib/libevent_core-2.1.so.7", os.FileMode(644))
						}
					}

				} else {
					gs.Str("realse failed:" + err.Error()).Color("r").Println()
				}
			}

		} else {
			gs.Str("release redsocks ").Println()
			if !gs.Str("/usr/sbin/redsocks2").IsExists() {
				if buf, err := asset.Asset("Resources/bin/redsocks2"); err == nil {
					if _, fp, err := gs.Str("/usr/sbin/redsocks2").OpenFile(gs.O_NEW_WRITE); err == nil {
						fp.Write(buf)
						fp.Close()
						os.Chmod("/usr/sbin/redsocks2", os.FileMode(755))
					}
				}
			}
			if !gs.Str("/usr/sbin/lcd-btn.py").IsExists() {
				_, fp, err := gs.Str("/usr/sbin/lcd-btn.py").OpenFile(gs.O_NEW_WRITE)
				if err == nil {
					if len(os.Args) > 0 {
						if buf, err := asset.Asset("Resources/script/c.py"); err == nil {
							fp.Write(buf)
						}
					}
					fp.Close()

				}
			}
			if gs.Str("/usr/sbin/lcd-btn.py").IsExists() {
				gs.Str("Start BTN Controller System!").Println()
				if !Exec("ps ").In("/usr/sbin/lcd-btn.py") {
					go Exec("/usr/bin/python /usr/sbin/lcd-btn.py >> /tmp/z-lcd.log")
				}

			}
		}

	}
}

func Exec(str gs.Str) gs.Str {
	var args []string
	// sep := "\n"
	if runtime.GOOS == "windows" {
		// sep = "\r\n"
		args = []string{"C:\\Windows\\System32\\Cmd.exe", "/C"}
	} else {
		args = []string{"sh", "-c"}
	}
	PATH := os.Getenv("PATH")
	if PATH == "" {
		PATH = "/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin"
	}
	args = append(args, str.String())
	cmd := exec.Command(args[0], args[1:]...)
	outbuffer := bytes.NewBuffer([]byte{})
	cmd.Stdout = outbuffer
	cmd.Stderr = outbuffer
	cmd.Run()
	return gs.Str(outbuffer.String())
}

func IsDNSRunning() bool {
	return Exec("netstat -anup").In("60053")
}
