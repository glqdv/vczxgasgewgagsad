//go:build darwin && !linux && !windows && !js
// +build darwin,!linux,!windows,!js

package router

import "gitee.com/dark.H/gs"

func IfProxyStart() bool {
	return !gs.Str(`networksetup -getwebproxy "Wi-Fi"`).Exec().In("Enabled: No")
}

func StopProxy() {
	gs.Str(`networksetup -setwebproxystate "Wi-Fi" off`).Exec()
	// gs.Str(`networksetup -setsecurewebproxy "Wi-Fi" `).F(i + 1).Exec(
	gs.Str(`networksetup -setsecurewebproxystate "Wi-Fi" off`).Exec()
}

func ProxySet(i int) {
	gs.Str(`networksetup -setsecurewebproxy "Wi-Fi" 127.0.0.1 %d`).F(i + 1).Exec()
	gs.Str(`networksetup -setsecurewebproxystate "Wi-Fi" on`).Exec()
	gs.Str(`networksetup -setwebproxy "Wi-Fi" 127.0.0.1 %d`).F(i + 1).Exec()
	gs.Str(`networksetup -setwebproxystate "Wi-Fi" on`).Exec()
}
