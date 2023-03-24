package router

func IfOpenGloabl() bool {
	return IfProxyStart()
}

func SetGloabl(port int) {
	ProxySet(port)
}

func SetGloablOff() {
	StopProxy()
}
