package prodns

import (
	"encoding/json"
	"time"

	"gitee.com/dark.H/gn"
	"gitee.com/dark.H/gs"
)

var (
	dnsQueryCache = make(chan string, 100)
	dnsReplyCache = make(chan *DNSRecord, 100)
)

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

func BackgroundBatchSend(server string, ifclose *bool) {
	tick := time.NewTicker(100 * time.Millisecond)
	collected := gs.List[string]{}
	gs.Str("Start DNS Collection").Color("g", "B").Println("DNS")
	defer func() {
		gs.Str("CLOSE DNS Collection").Color("g", "B").Println("DNS")
	}()
	for {
		if *ifclose {
			break
		}
		select {
		case <-tick.C:
			tick.Reset(100 * time.Millisecond)
			if len(collected) > 0 {
				go func(c ...string) {
					if reply := SendDNS(gs.Str(server), c...); len(reply) > 0 {
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
					}

				}(collected...)
				collected = gs.List[string]{}
			}
		case one := <-dnsQueryCache:
			collected = collected.Add(one)
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

}

func Query(domain string) {
	dnsQueryCache <- domain
}

func Reply(domain string) *DNSRecord {
	st := time.Now().Add(7 * time.Second)
loop:
	for {
		select {
		case one := <-dnsReplyCache:
			if one.Host == domain {
				gs.Str("%15s").F(one.IPs[0]).Color("g").Add(domain).Println("dns reply")
				return one
			} else {
				dnsReplyCache <- one
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
	gs.Str(domain).Add(gs.Str(" timeout").Color("r")).Println("dns reply")
	return o

}
