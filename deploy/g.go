package deploy

import (
	"encoding/json"
	"time"

	"gitee.com/dark.H/gn"
	"gitee.com/dark.H/gs"
)

func SetP(i int) {
	ProxySet(i)
}

func AutoLogin(auth ...string) {
	apath := gs.Str("~").ExpandUser().PathJoin(".config", "proxy-z.auth.conf")
	gs.Str("~").ExpandUser().PathJoin(".config").Mkdir()

	username := ""
	pwd := ""
	con := false
	last := ""
	proxy := ""
	if auth != nil && len(auth) == 2 {
		username = auth[0]
		pwd = auth[1]
		con = true
	} else {
		auth_txt := gs.Str(apath)
		if auth_txt.IsExists() {
			gs.Str("use auth session:" + apath).Println()
			buf := auth_txt.MustAsFile()
			e := make(gs.Dict[string])
			err := json.Unmarshal(buf.Bytes(), &e)
			if err != nil {
				gs.Str(err.Error()).Color("r").Println("+")
				return
			}
			var ok bool
			if username, ok = e["name"]; ok {
				if pwd, ok = e["password"]; ok {
					con = true
					if last, ok = e["last"]; ok {
					}
					if proxy, ok = e["proxy"]; ok {

					}
				}
			}
		} else {
			gs.Str("no auth session:" + apath).Println()
		}
	}
	gs.Str("User:%s Pwd:%s / last: %s / if Proxy : %s").F(username, pwd, last, proxy).Color("g", "B", "F").Println("Check")
	for {

		if con {
			gs.Str("Auto login").Println()
			if last != "" {
				res := gn.AsReq(gs.Str("http://localhost:35555/z-login").AsRequest().SetMethod("post").SetBody(gs.Dict[any]{
					"name":     username,
					"password": pwd,
					"last":     last,
					"proxy":    proxy,
				}.Json())).Go()
				if res.Err != nil {
				} else {
					if res.Body().In("Login Success!") {
						break
					}

				}
			}

			gs.Dict[any]{
				"name":     username,
				"password": pwd,
				"last":     last,
				"proxy":    proxy,
			}.Json().ToFile(apath.Str(), gs.O_NEW_WRITE)
		} else {
			res := gn.AsReq(gs.Str("http://localhost:35555/z-login").AsRequest().SetMethod("post").SetBody(gs.Dict[any]{
				"name":     username,
				"password": pwd,
				"last":     last,
				"proxy":    proxy,
			}.Json())).Go()
			if res.Err != nil {

			} else {
				if res.Body().In("Login Success!") {
					break
				}

			}
			time.Sleep(5 * time.Second)
		}
		gs.Dict[any]{
			"name":     username,
			"password": pwd,
			"last":     last,
			"proxy":    proxy,
		}.Json().ToFile(apath.Str(), gs.O_NEW_WRITE)

	}

}
