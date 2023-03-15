package clientcontroll

import "gitee.com/dark.H/ProxyZ/connections/prodns"

func StartDNS(port int, cons prodns.ConnecitonHandler, before func()) prodns.DNSServerHandler {
	return prodns.GetDNSServer(port, cons, before)
}
