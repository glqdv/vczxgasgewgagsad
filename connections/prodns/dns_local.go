package prodns

import (
	"net"
	"runtime"
	"sync"
	"time"

	"gitee.com/dark.H/ProxyZ/connections/prosocks5"
	"gitee.com/dark.H/ProxyZ/geo"

	"gitee.com/dark.H/gs"
	"github.com/miekg/dns"
)

var (
	ip2host            = make(gs.Dict[string])
	local2host         = make(gs.Dict[string])
	fuzzyHost          = gs.List[string]{}
	domainsToAddresses = make(map[string]*DNSRecord)
	// DEBUG              = false
)

type DNSRecord struct {
	Host    string
	IPs     gs.List[string]
	timeout time.Time
}

type DNSHandler struct {
	RemoteDNS string
	cons      ConnecitonHandler
	IsServer  bool
	queryWait int
	lock      sync.RWMutex
}

func (this *DNSHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)
	// fin := false
	defer w.Close()
	if len(r.Question) == 0 {
		w.WriteMsg(&msg)
		return
	}
	switch r.Question[0].Qtype {
	case dns.TypeA:
		msg.Authoritative = true
		domain := msg.Question[0].Name
		if this.ResolveCache(w, msg) {
			return
		}
		this.lock.Lock()
		this.queryWait += 1
		this.lock.Unlock()
		if this.IsRouter() {

			if IsLocal(domain) {
				gs.Str(domain).Println("dns cn")
				if this.ResolveLocal(w, msg) {
					this.lock.Lock()
					this.queryWait -= 1
					this.lock.Unlock()
					return
				}
			}
			for i := 0; i < 3; i++ {
				if this.ResolveRemoteOld(w, msg) {
					this.lock.Lock()
					this.queryWait -= 1
					this.lock.Unlock()
					return
				}
			}

			// if this.ResolveTest(w, msg) {
			// 	return
			// }
		}

		// if this.ResolveRemote(w, msg) {
		// 	this.lock.Lock()
		// 	this.queryWait -= 1
		// 	this.lock.Unlock()
		// 	return
		// }

	}
	w.WriteMsg(&msg)
}

func (this *DNSHandler) IsRouter() bool {
	return runtime.GOOS == "linux" && (runtime.GOARCH == "arm" || runtime.GOARCH == "arm64")
}

func (this *DNSHandler) ResolveLocal(w dns.ResponseWriter, msg dns.Msg) bool {
	domain := msg.Question[0].Name
	if r, err := ReplyDNS(PackDNS(&msg)); err == nil {
		if replymsg, err := UnpackDNS(r); err == nil {
			if len(replymsg.Answer) > 0 {
				record := &DNSRecord{
					timeout: time.Now().Add(1 * time.Hour),
				}
				for _, o := range replymsg.Answer {
					if o.Header().Rrtype == dns.TypeA && o.Header().Class == dns.ClassINET {
						ip := o.(*dns.A).A.String()
						if ip != "" && ip != "0.0.0.0" {
							record.IPs = record.IPs.Add(ip)
						}
					}
				}
				this.lock.Lock()
				domainsToAddresses[domain] = record
				record.IPs.Every(func(no int, i string) {
					ip2host[i] = domain
					local2host[i] = domain
				})
				this.lock.Unlock()
				w.WriteMsg(replymsg)
				if len(record.IPs) > 0 {
					gs.Str("(" + msg.Question[0].Name + ")").Color("y").Add(gs.Str(record.IPs[0]).Color("m")).Println("dns reply CN")
				}
				return true
			}
		}

		// return
	}
	return false
}

func SetConfigIP(ip string) {
	domainsToAddresses["config.me."] = &DNSRecord{
		Host:    "config.me.",
		IPs:     gs.List[string]{"99.254.254.254"},
		timeout: time.Now().Add(9999 * time.Hour),
	}
	domainsToAddresses["local.me."] = &DNSRecord{
		Host:    "local.me.",
		IPs:     gs.List[string]{"99.254.254.254"},
		timeout: time.Now().Add(9999 * time.Hour),
	}
}

func (this *DNSHandler) ResolveRemoteOld(w dns.ResponseWriter, msg dns.Msg) bool {
	domain := msg.Question[0].Name
	if gs.Str(domain).EndsWith(".lan.") {
		oldClass := msg.Question[0].Qclass
		msg.Question[0] = dns.Question{
			Name:   string(gs.Str(domain).Replace(".lan.", ".")),
			Qtype:  dns.TypeA,
			Qclass: oldClass,
		}
	}
	data := PackDNS(&msg)
	gs.Str("before con").Color("y").Println(domain)
	conn, eid, _, err := this.cons.ConnectRemote()
	if err != nil {
		gs.Str("no conn to dns :" + eid).Color("r").Println("Dns get con errr")
		this.cons.ErrRecord(eid, 2)
		return false
	}

	defer conn.Close()
	// data.Println()

	r := prosocks5.HostToRaw(data.Str(), 99)
	conn.Write(r)
	replyB := make([]byte, 4096)
	gs.Str("before read").Color("y", "U").Println(domain)
	conn.SetReadDeadline(time.Now().Add(12 * time.Second))
	// gs.Str(domain).Color("y").Println(gs.Str("query remote").Color("b"))
	if n, err := conn.Read(replyB); err != nil {
		if len(msg.Question) > 0 {
			qn := msg.Question[0].Name
			// gs.Str("health:%.2f%% [%s/%s] dns read err:"+err.Error()).F(this.cons.Health(), qn, eid).Color("r", "B").Println("dns")
			gs.Str(qn).Color("r").Println("query")
			this.cons.ErrRecord(eid, 1)

		}

		return false
	} else {
		gs.Str("before parse").Color("m").Println(domain)
		if replymsg, err := UnpackDNS(gs.Str(string(replyB[:n]))); err != nil {
			gs.Str("dns unpack err:"+err.Error()).Color("r", "B").Println("dns")
			return false
		} else {
			ip := ""
			if len(replymsg.Answer) > 0 {
				record := &DNSRecord{
					timeout: time.Now().Add(5 * time.Minute),
					Host:    domain,
				}
				for _, o := range replymsg.Answer {
					if o.Header().Rrtype == dns.TypeA && o.Header().Class == dns.ClassINET {
						ip = o.(*dns.A).A.String()
						if ip != "" && ip != "0.0.0.0" {
							record.IPs = record.IPs.Add(ip)
						}
					}
				}
				gs.Str("before cache").Color("m", "U").Println(domain)
				this.lock.Lock()
				if record.IPs.Count() > 0 {
					domainsToAddresses[domain] = record
					record.IPs.Every(func(no int, i string) {
						ip2host[i] = domain
					})
				}
				this.lock.Unlock()
			}
			if len(replymsg.Question) > 0 {
				gs.Str("(" + msg.Question[0].Name + ")").Color("y").Add(gs.Str(ip).Color("m")).Println("dns remote")
			}
			gs.Str("before write").Color("g").Println(domain)
			w.WriteMsg(replymsg)
			gs.Str("before vanish").Color("g", "U").Println(domain)
			this.cons.ErrVanish(eid)
			gs.Str("before vanish").Color("g", "B").Println(domain)
			return true
		}
	}

	return false
}

func (this *DNSHandler) ResolveRemote(w dns.ResponseWriter, msg dns.Msg) bool {
	domain := msg.Question[0].Name

	if gs.Str(domain).EndsWith(".lan.") {
		oldClass := msg.Question[0].Qclass
		msg.Question[0] = dns.Question{
			Name:   string(gs.Str(domain).Replace(".lan.", ".")),
			Qtype:  dns.TypeA,
			Qclass: oldClass,
		}
	}
	domain = msg.Question[0].Name
	record := &DNSRecord{
		// Host: domain,
	}
	if reply := SendDNS(gs.Str(DNSServerAddr), domain); reply != nil && len(reply) > 0 {

		reply.Every(func(ip, dom string) {
			// gs.Str(ip + " - " + dom).Println()
			if ip == "0.0.0.0" {
				return
			}
			record.Host = dom
			record.IPs = record.IPs.Add(ip)
		})
	}

	record.timeout = time.Now().Add(5 * time.Minute)

	if record.IPs.Count() > 0 {
		this.lock.Lock()
		if record.Host == domain {
			domainsToAddresses[domain] = record
		} else {
			gs.Str("Err position").Color("r").Println()
		}

		record.IPs.Every(func(no int, i string) {
			if i == "0.0.0.0" {
				return
			}
			ip2host[i] = domain
		})
		this.lock.Unlock()
	}
	record.IPs.Every(func(no int, ip string) {
		if ip != "0.0.0.0" {
			msg.Answer = append(msg.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 80},
				A:   net.ParseIP(ip),
			})
		}
	})

	w.WriteMsg(&msg)
	return true

}

func (this *DNSHandler) ResolveCache(w dns.ResponseWriter, msg dns.Msg) bool {

	domain := msg.Question[0].Name
	// data := PackDNS(&msg)
	addressR, ok := domainsToAddresses[domain]
	// gs.Str("query: %s").F(gs.Str(domain).Color("c", "U")).Println("dns")
	if ok {
		if time.Now().Before(addressR.timeout) {
			if addressR.IPs.Count() > 0 {
				ips := addressR.IPs
				i := gs.RAND.Int() % len(ips)
				ip := ips[i]
				msg.Answer = append(msg.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 80},
					A:   net.ParseIP(ip),
				})
				gs.Str("(" + msg.Question[0].Name + ")").Color("y").Add(gs.Str(ip).Color("m")).Println("dns cache")
				ips.Every(func(no int, nip string) {
					if nip != ip {
						msg.Answer = append(msg.Answer, &dns.A{
							Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 80},
							A:   net.ParseIP(nip),
						})
					}
				})

			}

			w.WriteMsg(&msg)
			return true
		} else {
			gs.Str(msg.Question[0].Name).Println(gs.Str("dns expired").Color("y"))
			this.lock.Lock()
			delete(domainsToAddresses, domain)
			this.lock.Unlock()
		}
	}
	return false
}

// func (this *DNSHandler) SaveCache()

func (this *DNSHandler) ResolveTest(w dns.ResponseWriter, msg dns.Msg) bool {
	domain := msg.Question[0].Name
	s := geo.Host2GEO(domain)
	if s.Count() > 0 && s[0].InCN() {

		gs.Str("(" + msg.Question[0].Name + ")").Color("y").Add(gs.Str(s[0].IP).Color("m")).Println("dns cn")
		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 80},
			A:   net.ParseIP(s[0].IP),
		})
		record := &DNSRecord{
			timeout: time.Now().Add(1 * time.Hour),
		}
		record.IPs = record.IPs.Add(s[0].IP)
		this.lock.Lock()
		domainsToAddresses[domain] = record
		this.lock.Unlock()
		w.WriteMsg(&msg)
		return true
	}
	return false
}
