package vpn

import (
	"errors"
	"log"
	"math/rand"
	"net"
	"runtime"
	"time"

	"gitee.com/dark.H/gs"
	"github.com/songgao/water"
)

type VPNHandler struct {
	iface    *water.Interface
	CIDR     string
	MyIP     string
	isServer bool
}

func NewVPNHandlerServer(cirs ...string) (v *VPNHandler) {
	v = &VPNHandler{
		CIDR:     "172.16.99.1/24",
		isServer: true,
	}
	if cirs != nil {
		v.CIDR = cirs[0]
	}
	return
}
func NewVPNHandlerClient(cirs ...string) (v *VPNHandler) {
	clientID := rand.New(rand.NewSource(time.Now().Unix())).Int()%251 + 2
	v = &VPNHandler{
		CIDR: gs.Str("172.16.99.1/24").Str(),
		MyIP: gs.Str("172.16.99.%d").F(clientID).Str(),
	}
	if cirs != nil {
		v.CIDR = cirs[0]
	}
	return
}

func (v *VPNHandler) Init() (err error) {
	// c :=

	iface, err := water.New(water.Config{
		DeviceType: water.TUN,
	})
	if err != nil {
		gs.Str("[VPN] " + err.Error() + "\n").ToFile("/tmp/z.log")
		return
	}
	v.iface = iface
	gs.Str("VPN Virutal Device interface build:"+iface.Name()).Color("c", "B").Println("VPN")
	return v.configVpn()

}

func (v *VPNHandler) configVpn() error {
	iface := v.iface
	cidr, iface := v.CIDR, v.iface
	ip, ipNet, err := net.ParseCIDR(cidr)
	// minIp := ipNet.IP.To4()

	gs.Str("sudo ifconfig %s inet %s %s up").F(iface.Name(), ip.String(), v.MyIP).Println("to").Exec()
	if v.iface != nil {
		return errors.New("no iface init !!")
	}

	os := runtime.GOOS
	// ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		log.Panicf("error cidr %v", cidr)
	}
	if os == "linux" {
		gs.Str("/sbin/ip link set dev %s  mtu 1500 ; sleep 1 ; /sbin/ip addr add %s dev %s ;sleep 1 ; /sbin/ip link set dev %s up ").F(iface.Name(), cidr, iface.Name(), iface.Name()).Exec()
		if v.iptableCheck() {
			gs.Str("device IP (%s) setup success!").F(v.MyIP).Color("g").Println("VPN")
		} else {
			gs.Str("device IP (%s) setup failed!").F(v.MyIP).Color("r").Println("VPN")
			return errors.New(gs.Str("device IP (%s) setup failed!").F(v.MyIP).Color("r").Str())
		}
	} else if os == "darwin" {
		minIp := ipNet.IP.To4()
		minIp[3]++

		gs.Str("sudo ifconfig %s inet %s %s up").F(iface.Name(), ip.String(), minIp.String()).Println()
		res := gs.Str("sudo ifconfig %s inet %s %s up").F(iface.Name(), ip.String(), minIp.String()).Println("to").Exec()
		gs.Str(res).Println("VPN")
	} else if os == "windows" {
		log.Printf("please install openvpn client,see this link:%v", "https://github.com/OpenVPN/openvpn")
		log.Printf("open new cmd and enter:netsh interface ip set address name=\"%v\" source=static addr=%v mask=%v gateway=none", iface.Name(), ip.String(), ipNet.Mask.String())
		return errors.New(gs.Str("not support os:%v").F(os).Str())
	} else {
		log.Printf("not support os:%v", os)
		return errors.New(gs.Str("not support os:%v").F(os).Str())
	}
	if v.isServer {
		v.iptableConfig()
		if v.iptableCheck() {
			gs.Str("(%s) iptables route build successful ").F(v.iface.Name()).Color("g").Println("VPN")
		} else {
			gs.Str("(%s) iptables route build failed ").F(v.iface.Name()).Color("r").Println("VPN")
			return errors.New(gs.Str("(%s) iptables route build failed ").F(v.iface.Name()).Color("r").Str())
		}
	}
	return nil
}

func (v *VPNHandler) iptableConfig() {
	if runtime.GOOS == "linux" {
		gs.Str(`echo 1 > /proc/sys/net/ipv4/ip_forward; sysctl -p ;iptables -t nat -A POSTROUTING -s 172.16.99.0/24 -o %s -j MASQUERADE`).F(v.iface.Name())

	}
}
func (v *VPNHandler) iptableCheck() bool {
	if res := gs.Str(`iptables -t nat -C POSTROUTING -s 172.16.99.0/24 -o %s -j MASQUERADE`).F(v.iface.Name()).Exec(); res.In("No chain/target/match by that name.") {
		return false
	}
	return true
}

func (v *VPNHandler) iptableClear() {
	if runtime.GOOS == "linux" {
		gs.Str(`echo 0 > /proc/sys/net/ipv4/ip_forward; sysctl -p ;iptables -t nat -D POSTROUTING -s 172.16.99.0/24 -o %s -j MASQUERADE`).F(v.iface.Name())
	}
}

func (v *VPNHandler) ipConfigCheck() bool {
	if res := gs.Str(`/sbin/ip addr show  %s`).F(v.iface.Name()).Exec(); res.In("does not exist.") {
		return false
	} else {
		return true
	}
}

func (v *VPNHandler) Close() error {
	return nil
}

func (v *VPNHandler) CloseVPN() error {
	if v == nil {
		return nil
	}
	if v.iface != nil {
		defer func() {
			if v.isServer {
				v.iptableClear()
			}
		}()
		return v.iface.Close()
	}
	return errors.New("no vpn device init !")
}

func (v *VPNHandler) Write(b []byte) (n int, err error) {
	if v.iface != nil {
		return v.iface.Write(b)
	} else {
		return -1, errors.New("no vpn device init !")
	}
}

func (v *VPNHandler) Read(b []byte) (n int, err error) {
	if v.iface != nil {
		n, err = v.iface.Read(b)

		return
	} else {
		return -1, errors.New("no vpn device init !")
	}
}
