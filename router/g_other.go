//go:build linux && !windows && !darwin
// +build linux,!windows,!darwin

package router

func IfProxyStart() bool {
	return false
}
func ProxySet(i int) {
}

func StopProxy() {

}
