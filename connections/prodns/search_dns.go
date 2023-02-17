package prodns

import "gitee.com/dark.H/gs"

func LoadLocalRule(path string) {
	if e := gs.Str(path); e.IsExists() {
		local2host = make(gs.Dict[string])
		e.MustAsFile().EveryLine(func(lineno int, line gs.Str) {
			if line.Trim().StartsWith("#") {
				return
			}
			if line.Trim() == "" {
				return
			}
			if line.Trim().In(" ") {
				return
			}
			if line.In("*") {
				fuzzyHost = fuzzyHost.Add(line.Trim().Str())
				line.Trim().Color("m").Println("bypass")
			} else {
				local2host[line.Trim().Str()] = "local"
			}

		})
	}
}

func SearchIP(ip string) (doamin string) {
	if domai, ok := ip2host[ip]; ok {
		doamin = domai
	}
	return
}

func IsLocal(ip string) (ok bool) {

	_, ok = local2host[ip]
	if !ok {
		fuzzyHost.Every(func(no int, i string) {
			ok = gs.Str(ip).In(gs.Str(i).Replace("*", ""))

		})
	}
	return
}

func Clear() {
	domainsToAddresses = make(map[string]*DNSRecord)
	gs.Str("~").ExpandUser().PathJoin(".config").Mkdir()
	s := gs.Str("~").ExpandUser().PathJoin(".config", "local.conf")
	s.Dirname().Mkdir()
	LoadLocalRule(s.Str())
	gs.Str("Clear DNS Cache ").Color("c", "B", "F").Println("DNS Clear")
}
