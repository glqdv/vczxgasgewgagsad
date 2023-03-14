package deploy

import (
	"net/http"

	"gitee.com/dark.H/gs"
)

func Web_Routes(wt http.ResponseWriter, rq *http.Request) {
	if !AuthCheck(wt, rq) {
		return
	}
	content := DefaultData()
	// base2 := AddPage{}
	// base2.Type = "socks5"
	// add, _ := Render("/index/add.html", ObjToJsonMap(base2))
	// base.LayoutContent = add

	if rq.Method == "GET" {
		hlistContent, err := Render("/index/hlist.html", gs.Dict[any]{
			"Web_base_url": "",
			"Task_id":      "",
			"Client_id":    "",
			"Columns": []string{
				"Remark",
				"Type",
				"Addr",
			},
			"CanDel": true,
		})
		content.LayoutContent = hlistContent
		content.Menu = "chains"
		out, err := Render("/public/layout.html", content)
		if err != nil {
			wt.WriteHeader(400)
			wt.Write([]byte(out))
		} else {
			wt.Write([]byte(out))
		}
	}
}
