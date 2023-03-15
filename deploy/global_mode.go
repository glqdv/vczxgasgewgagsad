package deploy

import (
	"runtime"

	"gitee.com/dark.H/gs"
)

func SetGlobalMode(i int) {
	switch runtime.GOOS {
	case "darwin":
		gs.Str("networksetup -setsocksfirewallproxy wi-fi 127.0.0.1 %d").F(i).Exec()
		gs.Str("networksetup -setsocksfirewallproxystate wi-fi on").Exec()
	case "windows":
		ProxySet(i)
	}
}

func SetGlobalModeOff() {
	switch runtime.GOOS {
	case "darwin":
		gs.Str("networksetup -setsocksfirewallproxystate wi-fi off").Exec()
	case "windows":
		ProxySet(0)

	}
}

func IsOpenGlobalState() bool {
	switch runtime.GOOS {
	case "darwin":
		return gs.Str("networksetup -getsocksfirewallproxy wi-fi").Exec().In("Enabled: Yes")
	case "windows":
		return gs.Str(`powershell.exe -windowstyle hidden -Command (Get-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings\").ProxyEnable`).Exec().In("1")
	}

	return false
}
