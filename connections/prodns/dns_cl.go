package prodns

import (
	"encoding/json"
	"sync"
	"time"

	"gitee.com/dark.H/gn"
	"gitee.com/dark.H/gs"
)

var (
	dnsQueryCache = make(chan string, 100)
	dnsReplyCache = make(chan *DNSRecord, 100)
	dnslock       = sync.RWMutex{}
)

func dnslockarea(d func()) {
	dnslock.Lock()
	d()
	dnslock.Unlock()
}

func SendDNS(server gs.Str, domains ...string) (reply gs.Dict[string]) {
	// ip2host = make(gs.Dict[string])

	if !server.StartsWith("https://") {
		server = "https://" + server
	}
	if !server.EndsWith(":55443") {
		server += ":55443"
	}
	server += "/z-dns"
	q := server.AsRequest().SetMethod("post").SetBody(gs.Dict[any]{
		"hosts": gs.List[string](domains).Join(","),
	}.Json())
	q.HTTPS = true
	tq := gn.AsReq(q)

	// tq = true
	tq.Timeout = 4
	// tq.Println("end")
	if res := tq.Go(); res.Err != nil {
		gs.Str(res.Err.Error()).Color("r").Println("dns query err")
		return nil
	} else {
		buf := res.Body()
		d := gs.Dict[any]{}
		err := json.Unmarshal(buf.Bytes(), &d)
		if err != nil {
			gs.Str(err.Error()).Color("r").Println()
			buf.Color("y").Println("Detail")
		} else {
			if st, ok := d["status"]; ok && st.(string) == "ok" {
				reply = make(gs.Dict[string])
				gs.Dict[any](d["msg"].(map[string]any)).Every(func(k string, v any) {
					if k == "0.0.0.0" {
						return
					}
					reply[k] = v.(string)
				})
			}
		}

	}
	return
}

func BackgroundBatchSend(server string, ifclose *bool, routeErrNum *int) {
	tick := time.NewTicker(100 * time.Millisecond)
	collected := gs.List[string]{}
	gs.Str("Start DNS Collection").Color("g", "B").Println("DNS")
	defer func() {
		gs.Str("CLSING DNS Collection").Color("g", "B").Println("DNS")
		// dnslockarea(func() {
		if dnsQueryCache != nil {
			for len(dnsQueryCache) > 0 {
				<-dnsQueryCache
			}
			// close(dnsQueryCache)
			gs.Str("close  DNS query").Color("g", "B").Println("DNS")
		}
		if dnsReplyCache != nil {
			for len(dnsReplyCache) > 0 {
				<-dnsQueryCache
			}
			// close(dnsReplyCache)
			gs.Str("close  DNS reply").Color("g", "B").Println("DNS")
		}
		gs.Str("CLOSE DNS Collect ion").Color("g", "B").Println("DNS")
		// dnsQueryCache = make(chan string, 100)
		// dnsReplyCache = make(chan *DNSRecord, 100)

		// })

	}()
	for {
		if *ifclose {
			break
		}
		select {
		case <-tick.C:
			// tick.Reset(100 * time.Millisecond)
			if len(collected) > 0 {
				go func(c ...string) {
					if reply := SendDNS(gs.Str(server), c...); reply != nil && len(reply) > 0 {
						o := gs.Dict[*DNSRecord]{}
						reply.Every(func(ip, dom string) {
							if ip == "0.0.0.0" {
								return
							}
							if ol, ok := o[dom]; ok {
								ol.IPs = ol.IPs.Add(ip)
							} else {
								o[dom] = &DNSRecord{
									Host: dom,
									IPs:  gs.List[string]{ip},
								}
							}
						})
						o.Every(func(k string, v *DNSRecord) {
							dnsReplyCache <- v
						})
						if *routeErrNum > 0 {
							*routeErrNum -= 1
						}
					} else {
						*routeErrNum += 1
					}

				}(collected...)
				collected = gs.List[string]{}
			}
		case one := <-dnsQueryCache:
			collected = collected.Add(one)
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
	time.Sleep(2 * time.Second)

}

func Query(domain string) {
	if dnsQueryCache != nil {
		dnsQueryCache <- domain
	}

}

func Reply(domain string) *DNSRecord {
	st := time.Now().Add(7 * time.Second)
loop:
	for {
		select {
		case one, ok := <-dnsReplyCache:
			if ok {
				// dnslockarea(func() {
				if one.Host == domain {
					gs.Str("%15s").F(one.IPs[0]).Color("g").Add(domain).Println(gs.Str("dns reply").Color("g"))
					return one
				} else {
					if dnsReplyCache != nil {
						dnsReplyCache <- one
					}
				}

			}
		default:
			time.Sleep(50 * time.Millisecond)
			if time.Now().After(st) {
				break loop
			}
		}

	}
	o := &DNSRecord{
		Host: domain,
	}
	gs.Str(domain).Add(gs.Str(" timeout").Color("r")).Println(gs.Str("dns reply").Color("g"))
	return o

}
