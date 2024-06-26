package prodns

import (
	"regexp"

	"gitee.com/dark.H/gs"
)

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
		for _, i := range fuzzyHost {
			if ok {
				break
			}
			if gs.Str(i).In("*") {
				if testC, err := regexp.Compile(string(gs.Str(i).Replace("*", ".*"))); err == nil {
					if testC.MatchString(ip) {
						ok = true
					}
				}
			} else {
				if i == ip {
					ok = true
				}
			}
		}
	}
	return
}

func Clear() {
	names := gs.List[string]{}
	for n := range domainsToAddresses {
		names = names.Add(n)
	}
	names.Every(func(no int, i string) {
		delete(domainsToAddresses, i)
	})
	gs.Str("Clear dns cache").Color("g").Println()
	domainsToAddresses = make(map[string]*DNSRecord)
	gs.Str("~").ExpandUser().PathJoin(".config").Mkdir()
	s := gs.Str("~").ExpandUser().PathJoin(".config", "local.conf")
	s.Dirname().Mkdir()
	LoadLocalRule(s.Str())
}
