//go:build windows && !linux && !darwin && !js
// +build windows,!linux,!darwin,!js

package router

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

func IfProxyStart() bool {
	TMP2 := `
	Get-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings\" -Name ProxyEnable
	`
	result, err := exec.Command("powershell.exe", "-command", TMP2).Output()
	if err != nil {
		return false
	}
	if !strings.Contains(string(result), "ProxyEnable  : 1") {
		return false
	}
	return false
}

func ProxySet(port int) {
	sd := fmt.Sprintf("127.0.0.1:%d", port+1)
	if port == 0 {
		sd = ""
	}
	TMP := `

	Function SetSystemProxy($Addr = $null) {
		Write-Output $Addr
		$proxyReg = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings";
	
		if ($Addr -eq $null) {
			Set-ItemProperty -Path $proxyReg -Name ProxyEnable -Value 0;
			Set-ItemProperty -Path $proxyReg -Name ProxyOverride -Value "*.local";
			return;
		}
		
		Set-ItemProperty -Path $proxyReg -Name ProxyServer -Value $Addr;
		Set-ItemProperty -Path $proxyReg -Name ProxyEnable -Value 1;
		Set-ItemProperty -Path $proxyReg -Name ProxyOverride -Value "*.local;localhost;192.168.*.*;127.0.0.1";
		
		if ($Addr -eq ""){
			Set-ItemProperty -Path $proxyReg -Name ProxyEnable -Value 0;
			Set-ItemProperty -Path $proxyReg -Name ProxyOverride -Value "*.local";
			return;
		}
	}	
	` + fmt.Sprintf("SetSystemProxy '%s'", sd)
	exe := exec.Command("powershell.exe", "-Command", TMP)
	exe.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	exe.Output()
}

func StopProxy() {
	TMP2 := `
	Set-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings" -Name ProxyServer -Value ""
	`
	exec.Command("powershell.exe", "-command", TMP2).Output()
	TMP2 = `
	Set-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings" -Name ProxyEnable -Value "0"
	`
	exec.Command("powershell.exe", "-command", TMP2).Output()
}
