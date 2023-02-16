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
	isRouter           = (runtime.GOOS == "linux" && runtime.GOARCH == "arm")
	ip2host            = make(gs.Dict[string])
	domainsToAddresses = make(map[string]*DNSRecord)
)

type DNSRecord struct {
	IPs     gs.List[string]
	timeout time.Time
}

type DNSHandler struct {
	RemoteDNS string
	cons      ConnecitonHandler
	IsServer  bool

	lock sync.RWMutex
}

func (this *DNSHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)
	// fin := false
	if len(r.Question) == 0 {
		w.WriteMsg(&msg)
		return
	}
	switch r.Question[0].Qtype {
	case dns.TypeA:
		msg.Authoritative = true

		if this.ResolveCache(w, msg) {
			return
		}

		if isRouter {
			domain := msg.Question[0].Name
			if gs.Str(domain).EndsWith(".cn") {
				if this.ResolveLocal(w, msg) {
					return
				}
			}
			if gs.Str(domain).In("bilibili") {
				if this.ResolveLocal(w, msg) {
					return
				}
			}

			if this.ResolveRemote(w, msg) {
				return
			}

			// if this.ResolveTest(w, msg) {
			// 	return
			// }
		}

		if this.ResolveRemote(w, msg) {
			return
		}

	}
	w.WriteMsg(&msg)
}

func SearchIP(ip string) (doamin string) {
	if domai, ok := ip2host[ip]; ok {
		doamin = domai
	}
	return
}

func Clear() {
	domainsToAddresses = make(map[string]*DNSRecord)
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

func (this *DNSHandler) ResolveRemote(w dns.ResponseWriter, msg dns.Msg) bool {
	domain := msg.Question[0].Name
	data := PackDNS(&msg)
	for i := 0; i < 3; i++ {
		conn, err, eid, _ := this.cons.ConnectRemote()
		if err != nil && i < 2 {

			continue
		}
		if err != nil && i == 2 {
			this.cons.ErrRecord(eid, 2)
			break
		}
		defer conn.Close()
		r := prosocks5.HostToRaw(data.Str(), 99)
		conn.Write(r)
		replyB := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(6 * time.Second))
		gs.Str(domain).Color("y").Println(gs.Str("query").Color("b"))
		if n, err := conn.Read(replyB); err != nil {
			if len(msg.Question) > 0 {
				qn := msg.Question[0].Name
				if i == 2 {
					gs.Str("[%d] health:%.2f%% [%s/%s] dns send err:"+err.Error()).F(i, this.cons.Health(), qn, eid).Color("r", "B").Println("dns")
					this.cons.ErrRecord(eid, 2)
				}

			}

			continue
		} else {
			if replymsg, err := UnpackDNS(gs.Str(string(replyB[:n]))); err != nil {
				gs.Str("dns unpack err:"+err.Error()).Color("r", "B").Println("dns")
				continue
			} else {
				ip := ""
				if len(replymsg.Answer) > 0 {
					record := &DNSRecord{
						timeout: time.Now().Add(1 * time.Hour),
					}
					for _, o := range replymsg.Answer {
						if o.Header().Rrtype == dns.TypeA && o.Header().Class == dns.ClassINET {
							ip = o.(*dns.A).A.String()
							if ip != "" && ip != "0.0.0.0" {
								record.IPs = record.IPs.Add(ip)
							}
						}
					}
					if record.IPs.Count() > 0 {
						this.lock.Lock()
						domainsToAddresses[domain] = record
						record.IPs.Every(func(no int, i string) {
							ip2host[i] = domain
						})
						this.lock.Unlock()
					}
				}
				if len(replymsg.Question) > 0 {

					gs.Str("(" + msg.Question[0].Name + ")").Color("y").Add(gs.Str(ip).Color("m")).Println("dns remote")
				}
				w.WriteMsg(replymsg)
				return true
			}

		}
	}
	return false
}

func (this *DNSHandler) ResolveCache(w dns.ResponseWriter, msg dns.Msg) bool {

	domain := msg.Question[0].Name
	// data := PackDNS(&msg)
	addressR, ok := domainsToAddresses[domain]
	// gs.Str("query: %s").F(gs.Str(domain).Color("c", "U")).Println("dns")
	if ok {
		if time.Now().Before(addressR.timeout) {
			addressR.IPs.Every(func(no int, ip string) {
				if no == 0 {

					gs.Str("(" + msg.Question[0].Name + ")").Color("y").Add(gs.Str(ip).Color("m")).Println("dns cache")

				}
				msg.Answer = append(msg.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 80},
					A:   net.ParseIP(ip),
				})
			})

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
