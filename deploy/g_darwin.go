//go:build darwin && !linux && !windows && !js
// +build darwin,!linux,!windows,!js

package deploy

func IfProxyStart() bool {
	return false
}
func ProxySet(i int) {
}
