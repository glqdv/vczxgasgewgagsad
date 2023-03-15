//go:build linux && !windows && !darwin
// +build linux,!windows,!darwin

package deploy

func IfProxyStart() bool {
	return false
}
func ProxySet(i int) {
}
