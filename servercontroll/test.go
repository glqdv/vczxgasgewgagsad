package servercontroll

import (
	"net"
	"time"

	"gitee.com/dark.H/gs"
)

var (
	NotHost = gs.List[string]{}
)

func TestServer(server string) (t time.Duration, IDS gs.List[string]) {
	st := time.Now()
	ok := true
	f := ""
	if !gs.Str(server).In(":55443") {
		server += ":55443"
	}
	if gs.Str(server).In("://") {
		f = "https://" + gs.Str(server).Split("://")[1].Str()
	} else {
		f = "https://" + gs.Str(server).Str()
	}

	if res, err := HTTPSGet(f + "/proxy-info"); err == nil {
		res.Json().Every(func(k string, v any) {
			if k == "status" {
				// gs.S(v).Color("g").Println(server)
				if v != "ok" {
					gs.Str("server is not alive !" + v.(string)).Color("r").Println(server)
					ok = false
				}
			} else if k == "msg" {
				switch v.(type) {
				case map[string]any:
					v := gs.AsDict[any](v)
					idsS := v["ids"].([]any)
					// gs.Str("id: %d").F(len(idsS)).Println(server)
					for _, i := range idsS {
						IDS = IDS.Add(i.(string))
					}

				}

			} else if k == "ids" {
				idsS := v.([]any)
				// gs.Str("id: %d").F(len(idsS)).Println()
				for _, i := range idsS {
					IDS = IDS.Add(i.(string))
				}
			}
		})
	} else {
		gs.Str("err:" + err.Error()).Color("r").Println("TestServer")
		return time.Duration(30000) * time.Hour, IDS
	}
	if !ok {
		return time.Duration(30000) * time.Hour, IDS
	}
	return time.Since(st), IDS
}

func SendUpdate(server string) {
	f := ""
	if !gs.Str(server).In(":55443") {
		server += ":55443"
	}
	if gs.Str(server).In("://") {
		f = "https://" + gs.Str(server).Split("://")[1].Str()
	} else {
		f = "https://" + gs.Str(server).Str()
	}
	res, err := HTTPSPost(f+"/z11-update", nil)
	if err == nil {
		res.Json().Every(func(k string, v any) {
			gs.S(v).Color("g").Println(server + " > " + k)
		})
	}

}

func CanReachHOST(host string) bool {
	con, err := net.DialTimeout("tcp", host, 7*time.Second)
	if err == nil {
		defer con.Close()
		return true
	}

	return false
}
