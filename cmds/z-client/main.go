package main

import (
	"bufio"
	"flag"
	"io"
	"os"
	"runtime"
	"time"

	"gitee.com/dark.H/ProxyZ/clientcontroll"
	"gitee.com/dark.H/ProxyZ/connections/prosocks5"
	"gitee.com/dark.H/ProxyZ/deploy"
	"gitee.com/dark.H/ProxyZ/router"
	"gitee.com/dark.H/ProxyZ/servercontroll"
	"gitee.com/dark.H/gn"
	"gitee.com/dark.H/gs"
	"gitee.com/dark.H/gt"
)

func main() {
	server := ""
	dev := false
	update := false
	vultrmode := false
	gitmode := false
	daemon := false
	httpmode := false
	noopenbrowser := false
	log := false
	// useVPN := false
	global := false
	// cli := false
	// configbuild := false

	flag.StringVar(&server, "H", "http://localhost:35555", "set server addr/set ssh name / set some other ")
	flag.IntVar(&deploy.LOCAL_PORT, "l", 1091, "set local socks5 listen port")

	flag.BoolVar(&dev, "dev", false, "use ssh to devploy proxy server ; example -H 'user@host:port/pwd' -dev ")
	flag.BoolVar(&update, "update", false, "set this server update by git")
	flag.BoolVar(&vultrmode, "vultr", false, "true to use vultr api to search host")
	flag.BoolVar(&gitmode, "git", false, "true to use git to login group proxy")
	flag.BoolVar(&httpmode, "http", false, "true to use http mode")
	flag.BoolVar(&noopenbrowser, "no-open", false, "true not open browser")
	flag.BoolVar(&daemon, "d", false, "true to run deamon")
	flag.BoolVar(&log, "log", false, "true to get log")
	flag.BoolVar(&global, "global", false, "true to set system proxy")

	// flag.BoolVar(&cli, "cli", false, "true to use cli-client")
	// flag.BoolVar(&configbuild, "", false, "true to use vultr api to build host group")

	flag.Parse()

	if dev {
		deploy.DepBySSH(server)
		os.Exit(0)
	}
	if vultrmode {
		deploy.VultrMode(server)
		os.Exit(0)
	}
	if update {
		servercontroll.SendUpdate(server)
		os.Exit(0)
	}
	if router.IsRouter() {
		gs.Str("Plat:%s/%s").F(runtime.GOOS, runtime.GOARCH).Color("g").Println()
		router.ReleaseRedsocks()
	} else {
		gs.Str("Plat:%s/%s").F(runtime.GOOS, runtime.GOARCH).Color("g").Println()
	}

	if daemon {
		logFile := gs.TMP.PathJoin("z.log").Str()
		args := []string{}
		for _, a := range os.Args {
			if a == "-d" {
				continue
			}
			args = append(args, a)
		}
		deploy.Daemon(args, logFile)
		time.Sleep(2 * time.Second)
		gs.Str("%s run background | log: %s").F(os.Args[0], logFile).Println("Daemon")
		os.Exit(0)
	}

	if gitmode {
		if !gs.Str(server).StartsWith("https://git") {
			server = "https://" + string(gs.Str("55594657571e515d5f1f5653405b1c7a1d53541c555946").Derypt("2022"))
		}
		server = deploy.GitMode(server)
		if server == "" {
			os.Exit(0)
		}
		if global {
			deploy.SetGlobalMode(deploy.LOCAL_PORT)
		}
		clientcontroll.RunLocal(server, deploy.LOCAL_PORT, true, true)
		os.Exit(0)
	}

	if httpmode {

		if auth := gs.List[string](flag.Args()); auth != nil && auth.Count() == 2 {
			noopenbrowser = true
			go deploy.AutoLogin(auth...)
		} else {
			go deploy.AutoLogin()
		}

		deploy.LocalAPI(noopenbrowser, global)

		os.Exit(0)
	}

	if gs.Str(server) != "" && !gs.Str(server).In("http://") {
		if global {
			deploy.SetGlobalMode(deploy.LOCAL_PORT)
		}
		clientcontroll.RunLocal(server, deploy.LOCAL_PORT, true, true)
		os.Exit(0)
	}
	if log {
		SeeLog(server)
		os.Exit(0)
	}

	SomeCmd(server)
}

func SeeLog(server string) {

	f := ""
	if !gs.Str(server).In(":55443") {
		server += ":55443"
	}
	if gs.Str(server).In("://") {
		f = "https://" + gs.Str(server).Split("://")[1].Str()
	} else {
		f = "https://" + gs.Str(server).Str()
	}
	req := gs.Str(f + "/c-log").AsRequest()
	r := gn.AsReq(req).Go().AsRequest().BodyReader()
	io.Copy(os.Stdout, r)
	os.Exit(0)

}

func SomeCmd(server string) {
	if server == "http://localhost:35555" {
		switch gt.Select[string](gs.List[string]{
			"change proxy type",
			"send code",
		}) {
		case "change proxy type":
			switch choose := gt.Select[string](gs.List[string]{
				"quic",
				"tls",
				"kcp",
			}); choose {
			default:
				gs.Str(prosocks5.SendControllCode(string("change/"+choose), deploy.LOCAL_PORT)).Println()
			}
		case "send code":
			gs.Str("send code >>").Print()
			l, _, err := bufio.NewReader(os.Stdin).ReadLine()
			if err != nil {
				gs.Str(err.Error()).Color("r").Println("Err")
				return
			}
			gs.Str(prosocks5.SendControllCode(string(l), deploy.LOCAL_PORT)).Println()

		}
	}
}
