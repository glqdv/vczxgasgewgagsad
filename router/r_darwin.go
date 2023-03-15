//go:build darwin && !linux && !windows && !js
// +build darwin,!linux,!windows,!js

package router

// import (
// 	"gitee.com/dark.H/gn"
// 	"gitee.com/dark.H/gs"
// )

// func redSocksStart(i int) {
// 	// gs.Str()
// }

// func GetIface(k ...string) string {
// 	wl := "wl"
// 	if k != nil {
// 		wl = k[0]
// 	}
// 	return string(gs.Str(`ifconfig | awk '{print $1}' | grep %s| xargs`).F(wl).Exec())
// }

// func GetIP(ifaces ...string) string {

// 	iface := ""
// 	if ifaces != nil {
// 		iface = ifaces[0]
// 	} else {
// 		iface = GetIface()
// 	}

// 	return gs.Str(`ifconfig %s | grep 192 | awk '{print $2}' | awk -F : '{print $2}')`).F(iface).Exec().Str()
// }

// func IPTahbleRouteSet() {
// 	if res := gn.AsReq(gs.Str("http://localhost:35555/z-api").AsRequest().SetMethod("post").SetBody(gs.Dict[any]{
// 		"op": "test",
// 	}.Json())).Go(); res.Err == nil {

// 		if d := res.Body().Json(); d != nil {
// 			d2 := gs.AsList[any](d["msg"])
// 			render := gs.Str(`iptables -t nat -F
// iptables -t nat -N REDSOCKS
// iptables -t nat -A PREROUTING -i $IFACE -p tcp -j REDSOCKS
// # redirct dns
// iptables -t nat -A PREROUTING -p udp --dport  53 -j REDIRECT --to-ports 60053
// %s
// iptables -t nat -A REDSOCKS -d 0.0.0.0/8 -j RETURN
// iptables -t nat -A REDSOCKS -d 10.0.0.0/8 -j RETURN
// iptables -t nat -A REDSOCKS -d 127.0.0.0/8 -j RETURN
// iptables -t nat -A REDSOCKS -d 169.254.0.0/16 -j RETURN
// iptables -t nat -A REDSOCKS -d 172.16.0.0/12 -j RETURN
// iptables -t nat -A REDSOCKS -d 224.0.0.0/4 -j RETURN
// iptables -t nat -A REDSOCKS -d 240.0.0.0/4 -j RETURN

// iptables -t nat -A REDSOCKS -p tcp -s ${IP} --dport ${PORT} -j RETURN
// iptables -t nat -A REDSOCKS -d $IP -j RETURN
// iptables -t nat -A REDSOCKS -p tcp -j REDIRECT --to-ports 1081
// iptables -t nat -A PREROUTING -p tcp -j REDSOCKS`)
// 			tmp := gs.Str("")
// 			d2.Every(func(no int, i any) {
// 				i2 := gs.AsDict[any](i)
// 				tmp += gs.Str("iptables -t nat -A REDSOCKS -d %s -j RETURN\n").F(i2["Host"])
// 			})
// 			render.F(tmp)
// 		}
// 	}
// }
