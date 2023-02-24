package prodns

import (
	"encoding/base64"
	"errors"
	"net"
	"strconv"

	"gitee.com/dark.H/gs"
	"github.com/miekg/dns"
)

var RDNS = "8.8.8.8:53"

type ConnecitonHandler interface {
	ConnectRemote() (con net.Conn, id string, proxyType string, err error)
	ErrRecord(eid string, i int)
	Health() float32
}

type DNSServerHandler interface {
	Shutdown() error
	ListenAndServe() error
}

func GetDNSServer(lport int, cons ConnecitonHandler, before func()) DNSServerHandler {
	if before != nil {
		before()
	}

	srv := &dns.Server{Addr: ":" + strconv.Itoa(lport), Net: "udp"}
	srv.Handler = &DNSHandler{
		cons:     cons,
		IsServer: false,
	}

	return srv
}

func PackDNS(msg *dns.Msg) gs.Str {
	buf, err := msg.Pack()
	if err != nil {
		gs.Str(err.Error()).Color("r").Println("DNS")
		return ""
	}
	return gs.Str("dns://" + base64.StdEncoding.EncodeToString(buf))

}

func UnpackDNS(dnsmsg gs.Str) (msg *dns.Msg, err error) {
	if dnsmsg.StartsWith("dns://") {
		msg = new(dns.Msg)
		rmsg := dnsmsg.Split("dns://")[1]
		buf, err2 := base64.StdEncoding.DecodeString(rmsg.Str())
		if err2 != nil {
			return nil, err2
		}
		err = msg.Unpack(buf)
	} else {
		err = errors.New("not dns:// start")
	}
	return
}

func ReplyDNS(dnsmsg gs.Str) (replyMsg gs.Str, err error) {

	r, err2 := UnpackDNS(dnsmsg)
	if err2 != nil {
		return "", err2
	}
	if len(r.Question) == 0 {
		return "", errors.New("no question in dns req")
	}
	// gs.Str(r.String()).Println()
	host := r.Question[0].Name
	H := gs.Str(host)
	if H.EndsWith(".") {
		H = H.Slice(0, -1)
	}

	if ips, err2 := net.LookupIP(H.Str()); err != nil {
		err = err2
		return
	} else {
		// mm := new(dns.Msg)
		for _, ip := range ips {
			if ip.String() == "" || ip.String() == "0.0.0.0" {
				continue
			}
			if ip.To4() != nil {
				// ipv4  := ip.IP.To4()

				r.Answer = append(r.Answer, &dns.A{
					Hdr: dns.RR_Header{
						Name:   H.Str() + ".",
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    80,
					},
					A: ip,
				})
			}

		}
		// gs.Str(r.String()).Println("dns ans")
		replyMsg = PackDNS(r)
	}
	return
}

func SetRDNS(rdns string) {
	RDNS = rdns
}
