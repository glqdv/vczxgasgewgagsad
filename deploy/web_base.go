package deploy

import (
	"bytes"
	"net/http"
	"text/template"

	"gitee.com/dark.H/ProxyZ/asset"
	"gitee.com/dark.H/gs"
)

type DataIndex struct {
	WebBaseUrl    string
	IsAdmin       bool
	Username      string
	Menu          string
	LayoutContent string
}

func DefaultData() DataIndex {
	return DataIndex{
		WebBaseUrl: "",
		Menu:       "index",
		IsAdmin:    true,
		Username:   "who-am-i",
	}
}

func AuthCheck(wt http.ResponseWriter, rq *http.Request) bool {
	if globalClient.Routes.Count() == 0 {
		http.Redirect(wt, rq, "/z-login", http.StatusSeeOther)
		return false
	}
	return true
}

func Render(name string, data interface{}) (string, error) {
	buffer := bytes.NewBuffer([]byte{})
	buf, err2 := asset.Asset("Resources/web/views" + name)
	if err2 != nil {
		return "", err2
	}
	tmp, err3 := template.New(name).Parse(string(buf))
	if err3 != nil {
		return "", err3
	}
	err4 := tmp.Execute(buffer, data)
	return buffer.String(), err4
}

func LogOut(wt http.ResponseWriter, rq *http.Request) {
	globalClient.Routes = gs.List[*Onevps]{}
}
