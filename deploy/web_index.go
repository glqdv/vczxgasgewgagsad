package deploy

import "net/http"

func Web_Index(wt http.ResponseWriter, rq *http.Request) {
	if !AuthCheck(wt, rq) {
		return
	}
	base := DefaultData()
	mapContent, _ := Render("/index/map.html", nil)
	base.LayoutContent = mapContent
	// base2 := AddPage{}
	// base2.Type = "socks5"
	// add, _ := Render("/index/add.html", ObjToJsonMap(base2))
	// base.LayoutContent = add
	content, err := Render("/public/layout.html", base)
	if err != nil {
		wt.WriteHeader(400)
		wt.Write([]byte(content))
	} else {
		wt.Write([]byte(content))
	}
}
