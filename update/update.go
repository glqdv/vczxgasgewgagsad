package update

import (
	"log"
	"os"
	"path"
	"runtime"
	"time"

	"gitee.com/dark.H/gs"
	"github.com/go-git/go-git/v5"
)

// mkdir -p  /tmp/repo_update/GoR ; cd /tmp/repo_update/GoR && wget -c 'https://go.dev/dl/go1.19.5.linux-amd64.tar.gz' && tar -zxvf go1.19.5.linux-amd64.tar.gz ; /tmp/repo_update/GoR/go/bin/go version
var (
	REPO_TMP = gs.Str("/tmp/repo_update")
	BU       = gs.Str(`mkdir -p  /tmp/repo_update/GoR ; cd /tmp/repo_update/GoR && wget -c 'https://go.dev/dl/go1.19.5.%s-%s.tar.gz' && tar -zxvf go1.19.5.%s-%s.tar.gz ; /tmp/repo_update/GoR/go/bin/go version`)
	B        = gs.Str(`export PATH="$PATH:/tmp/repo_update/GoR/go/bin" ; cd %s &&  go mod tidy && go build -o Puzzle ; sysctl -w net.core.rmem_max=2500000  ;ls `)
)

func Update(beforeExit func(info string, ok bool), repo ...string) {
	repoUrl := "https://github.com/glqdv/vczxgasgewgagsad.git"
	if repo != nil {
		repoUrl = repo[0]

	}
	info := gs.Str("")
	defer REPO_TMP.Rm()
	REPO_TMP.Mkdir()
	gs.Str("Platform : %s | Architecture: %s").F(gs.Platform, runtime.GOARCH).Color("b").Println()
	if res := BU.F(gs.Platform, runtime.GOARCH, gs.Platform, runtime.GOARCH).Exec(); !res.In(`go version go1.19.5`) {
		gs.Str("build golang env failed").Color("r").Println("Err")
		info += gs.Str("\nbuild golang env failed").Color("r")
		beforeExit(info.Str(), false)

		return
	} else {
		gs.Str("build golang env success!").Color("g").Println()
		info += gs.Str("\nbuild golang env success!").Color("g")
	}

	REPO_PATH := REPO_TMP.PathJoin("pz")
	if REPO_PATH.IsExists() {
		REPO_PATH.Rm()
	}
	_, err := git.PlainClone(REPO_PATH.Str(), false, &git.CloneOptions{
		URL:      repoUrl,
		Progress: os.Stdout,
	})
	if err != nil {
		gs.Str(err.Error()).Println("Err")
		info += gs.Str("\n" + err.Error()).Color("r")
		beforeExit(info.Str(), false)
		return
	}
	gs.Str("Repository cloned successfully").Color("g").Println()
	if res := B.F(REPO_PATH.Str()).Exec(); res.In("Puzzle") {
		gs.Str("Repository build successfully").Color("g").Println()
		info += gs.Str("\nRepository build successfully").Color("g")
	} else {
		gs.Str("Repository build failed").Color("r").Println("Err")
		info += gs.Str("\nRepository build failed").Color("r")
		beforeExit(info.Str(), false)
		return
	}

	Execute := REPO_PATH.PathJoin("Puzzle")
	args := os.Args
	args[0] = Execute.Str()
	gs.Str("Start New Version !").Color("g").Println()
	info += gs.Str("\nStart New Version !").Color("g")
	beforeExit(info.Str(), true)
	Daemon(args, "/tmp/z.log")
	time.Sleep(3 * time.Second)
	gs.Str("Exit old Version !").Color("g").Println()
	os.Exit(0)
}

func Daemon(args []string, LOG_FILE string) {
	createLogFile := func(fileName string) (fd *os.File, err error) {
		dir := path.Dir(fileName)
		if _, err = os.Stat(dir); err != nil && os.IsNotExist(err) {
			if err = os.MkdirAll(dir, 0755); err != nil {
				log.Println(err)
				return
			}
		}
		if fd, err = os.Create(fileName); err != nil {
			log.Println(err)
			return
		}
		return
	}
	if LOG_FILE != "" {
		logFd, err := createLogFile(LOG_FILE)
		if err != nil {
			log.Println(err)
			return
		}
		defer logFd.Close()

		cmdName := args[0]
		newProc, err := os.StartProcess(cmdName, args, &os.ProcAttr{
			Files: []*os.File{logFd, logFd, logFd},
		})
		if err != nil {
			log.Fatal("daemon error:", err)
			return
		}
		log.Printf("Start-Deamon: run in daemon success, pid: %v\nlog : %s", newProc.Pid, LOG_FILE)
	} else {
		cmdName := args[0]
		newProc, err := os.StartProcess(cmdName, args, &os.ProcAttr{
			Files: []*os.File{nil, nil, nil},
		})
		if err != nil {
			log.Fatal("daemon error:", err)
			return
		}
		log.Printf("Start-Deamon: run in daemon success, pid: %v\n", newProc.Pid)
	}
	return
	// }
}
