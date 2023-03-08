package deploy

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"gitee.com/dark.H/ProxyZ/clientcontroll"
	"gitee.com/dark.H/ProxyZ/servercontroll"
	"gitee.com/dark.H/gn"
	"gitee.com/dark.H/gs"
	"gitee.com/dark.H/gt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"golang.org/x/crypto/ssh"
)

var (
	BU = gs.Str(`mkdir -p  /tmp/repo_update/GoR ; cd /tmp/repo_update/GoR && wget -c 'https://go.dev/dl/go1.19.5.linux-amd64.tar.gz' && tar -zxf go1.19.5.linux-amd64.tar.gz ; /tmp/repo_update/GoR/go/bin/go version;`)
	B  = gs.Str(`ps aux | grep './Puzzle' | grep -v grep| awk '{print $2}' | xargs kill -9 ;export PATH="$PATH:/tmp/repo_update/GoR/go/bin" ; cd  /tmp/repo_update &&  git clone https://github.com/glqdv/vczxgasgewgagsad.git  && cd vczxgasgewgagsad &&  go mod tidy && go build -o Puzzle;  ulimit -n 4096 ;sysctl -w net.core.rmem_max=2500000 ;./Puzzle -h; ./Puzzle -d  && sleep 2 ; rm -rf /tmp/repo_update `)

	DOWNADDR = ""
)

type Onevps struct {
	Host             string        `json:"Host"`
	Pwd              string        `json:"Pwd"`
	Location         string        `json:"Location"`
	Tag              string        `json:"Tag"`
	Speed            string        `json:"Speed"`
	ConnectedQuality time.Duration `json:"ConnectedQuality"`
	IDS              int           `json:"IDS"`
}

func SetDownloadAddr(s string) {
	DOWNADDR = s
}

func Auth(name, host, passwd string, callbcak func(c *ssh.Client, s *ssh.Session)) {

	sshConfig := &ssh.ClientConfig{
		User: name,
		Auth: []ssh.AuthMethod{
			ssh.Password(passwd),
		},
		Timeout:         15 * time.Second,
		HostKeyCallback: ssh.HostKeyCallback(func(hostname string, remote net.Addr, key ssh.PublicKey) error { return nil }),
	}
	keyFile := gs.Str("~").ExpandUser().PathJoin(".ssh", "id_rsa")
	if keyFile.IsExists() {
		if keybuf := keyFile.MustAsFile(); keybuf != "" {
			signal, err := ssh.ParsePrivateKey(keybuf.Bytes())
			if err == nil {
				sshConfig.Auth = append(sshConfig.Auth,
					ssh.PublicKeys(signal),
				)
			}
		}

	}
	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	client, err := ssh.Dial("tcp", host, sshConfig)

	if err != nil {
		gs.Str(err.Error()).Println("Err")
		return
	}
	defer client.Close()

	// start session
	sess, err := client.NewSession()
	if err != nil {
		log.Fatal("session:", err)
	}
	defer sess.Close()
	callbcak(client, sess)
}

func DepOneHost(user, host, pwd string) {
	Auth(user, host, pwd, func(client *ssh.Client, sess *ssh.Session) {
		gs.Str("success auth by ssh use :%s@%s/%s").F(user, host, pwd).Color("g").Println()
		var out bytes.Buffer
		sess.Stdout = &out
		err := sess.Run(BU.Str())
		// err := sess.Run(string(DevStr.F(DOWNADDR)))
		if err != nil {
			gs.Str(err.Error()).Color("r").Println(host)
			// }
			return
		} else {
			gs.Str(out.String()).Trim().Color("g").Println(host)
		}
		sess.Close()
		var out2 = bytes.NewBuffer([]byte{})
		sess2, err := client.NewSession()
		if err != nil {
			gs.Str(err.Error()).Color("r").Println("Err")
			return
		}
		sess2.Stdout = out2

		err = sess2.Run(B.Str())
		if err != nil {
			gs.Str(err.Error()).Color("r").Println(host)
			return
		} else {
			// gs.Str(out2.String()).Color("g").Println(host)
		}

	})
}

func LogOneHost(user, host, pwd string) {
	Auth(user, host, pwd, func(client *ssh.Client, sess *ssh.Session) {
		gs.Str("success auth by ssh use :%s@%s/%s").F(user, host, pwd).Color("g").Println()
		var out bytes.Buffer
		sess.Stdout = &out
		err := sess.Run("cat /tmp/z.log")
		// err := sess.Run(string(DevStr.F(DOWNADDR)))
		if err != nil {
			gs.Str(err.Error()).Color("r").Println(host)
			// }
			return
		} else {
			gs.Str(out.String()).Trim().Color("g").Println(host)
		}
		sess.Close()
	})
}

func DepBySSH(sshStr string) {
	user := "root"
	host := ""
	pwd := ""
	if gs.Str(sshStr).In("@") {
		gs.Str(sshStr).Split("@").Every(func(no int, i gs.Str) {
			if no == 0 {
				user = i.Str()
			} else {
				if i.In("/") {
					i.Split("/").Every(func(no int, i gs.Str) {
						if no == 0 {
							host = i.Str()
						} else {
							pwd = i.Str()
						}
					})

				} else {
					host = i.Str()
				}
			}
		})
	} else {
		i := gs.Str(sshStr)
		if i.In("/") {
			i.Split("/").Every(func(no int, i gs.Str) {
				if no == 0 {
					host = i.Str()
				} else {
					pwd = i.Str()
				}
			})
		} else {
			host = i.Str()
		}
	}
	if !gs.Str(host).In(":") {
		host += ":22"
	}
	if user != "" && host != "" {
		DepOneHost(user, host, pwd)
	} else {
		gs.Str("user:%s host:%s pwd:%s").F(user, host, pwd).Println()
	}
}

func GetConfig(user string, pwd string) (err error) {
	REPO_TMP := gs.TMP.PathJoin("repo")
	defer REPO_TMP.Rm()
	REPO_PATH := REPO_TMP.PathJoin("pz")
	if REPO_PATH.IsExists() {
		REPO_PATH.Rm()
	}
	repoUrl := "https://gitee.com/dark.H/"
	_, err = git.PlainClone(REPO_PATH.Str(), false, &git.CloneOptions{
		URL:      repoUrl,
		Progress: os.Stdout,
	})
	return err
}

func (o *Onevps) Println() {
	w := gs.Str("tag:%s ").F(o.Tag).Color("b", "B") + gs.Str("host: %s ").F(o.Host).Color("g") + gs.Str("loc: "+o.Location).Color("m", "B") + gs.Str(" root@"+o.Host+":22/"+o.Pwd).Color("U", "B")
	w.Println()
}

func (o *Onevps) Log() {
	LogOneHost("root", o.Host+":22", o.Pwd)
}

func (o *Onevps) Build() {
	DepOneHost("root", o.Host+":22", o.Pwd)
}

func SearchFromVultr(tag, api string) (vpss gs.List[*Onevps]) {
	a := gs.Str(api)
	if a.StartsWith("https://") {
		api = a.Split("https://")[1].Split(":")[0].Str()
	}
	nt := gs.Str("https://api.vultr.com/v1/server/list").AsRequest()
	nt = nt.SetMethod(gs.Str("GET")).SetHead("API-Key", gs.Str(api))
	// nt.Color("g").Println(tag)
	r := gn.AsReq(nt).HTTPS()
	r.Build()

	if res := r.Go(); res.Str != "" {
		// res.Str.Println()

		data := res.Body().Json()
		data.Every(func(k string, v any) {
			vals := data[k].(map[string]interface{})
			vtag := vals["tag"].(string)
			passwd := vals["default_password"].(string)
			host := vals["main_ip"].(string)
			location := vals["location"].(string)
			if gs.Str(vtag + host + location).In(tag) {
				vpss = vpss.Add(&Onevps{
					Host:     host,
					Tag:      tag,
					Pwd:      passwd,
					Location: location,
				})
			}
		})
	}

	return
}

func (o *Onevps) Update() {
	servercontroll.SendUpdate(o.Host)
}

func (o *Onevps) Test() time.Duration {
	// var IDS gs.List[string]
	// l := sync.RWMutex{}
	// var l time.Duration = 0
	var ids gs.List[string]

	l, ids := servercontroll.TestServer(o.Host)
	// gs.Str("Test Connect:" + l.String()).Println(o.Host)
	ol := l
	ol += servercontroll.TestHost(o.Host)
	ol /= 2
	// gs.Str("Test Web:" + ol.String()).Println(o.Host)
	// s.Wait()
	o.ConnectedQuality = ol
	o.IDS = ids.Count()
	o.Speed = o.ConnectedQuality.String()
	return o.ConnectedQuality
}

func SyncToGit(gitrepo, gitName, gitPwd, loginName, loginPwd string, vpss gs.List[*Onevps]) {
	text := gs.Str("")
	vpss.Every(func(no int, i *Onevps) {
		text += gs.Str(i.Location + "|" + i.Host + "\n")
	})
	EncryptedText := text.Trim().Enrypt(loginPwd)
	tmprepo := gs.TMP.PathJoin("repot-sync-tmp")
	repoUrl := gitrepo
	if tmprepo.IsExists() {
		tmprepo.Rm()
	}
	repo, err := git.PlainClone(tmprepo.Str(), false, &git.CloneOptions{
		URL:      repoUrl,
		Progress: os.Stdout,
	})
	if err != nil {
		gs.Str(err.Error()).Println("Err")
		log.Fatal(err)
	}

	fileP := tmprepo.PathJoin(loginName)
	EncryptedText.ToFile(fileP.Str(), gs.O_NEW_WRITE)

	work, err := repo.Worktree()
	if err != nil {
		gs.Str(err.Error()).Println("Err")
		log.Fatal(err)
	}
	fileP.Color("b").Println("git:add file")
	_, err = work.Add(fileP.Basename().Str())
	if err != nil {
		gs.Str(err.Error()).Println("Err")
		log.Fatal(err)
	}
	_, err = work.Commit("example go-git commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "John Doe",
			Email: "john@doe.org",
			When:  time.Now(),
		},
	})

	if err != nil {
		gs.Str(err.Error()).Println("Err")
		log.Fatal(err)
	}
	gs.Str("Commit ok ").Color("g").Println("git")
	err = repo.Push(&git.PushOptions{
		RemoteName: "origin",
		// RefSpecs:   []config.RefSpec{config.RefSpec("+refs/heads/" + nameInfoObj.Version + ":refs/heads/" + nameInfoObj.Version)},
		Auth: &githttp.BasicAuth{
			Username: gitName,
			Password: gitPwd,
		},
	})
	if err != nil {
		gs.Str(err.Error()).Println("Err")
		log.Fatal(err)
	}
	gs.Str("Push ok ").Color("g").Println("git")
}

func VultrMode(server string) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Search in all vps by tag ['exit' to exit] >>")
		tag, _ := reader.ReadString('\n')
		tag = gs.Str(tag).Trim().Str()
		// tag := gt.TypedInput("Search Tag[exit] >")
		if tag == "exit" {
			break
		}
		devs := SearchFromVultr(tag, server)
		devs.Every(func(no int, i *Onevps) {
			i.Println()
		})

		fmt.Print("build/ test / update / sync / log >>>")
		handler, _ := reader.ReadString('\n')
		switch gs.Str(handler).Trim() {
		case "log":
			waiter := sync.WaitGroup{}
			devs.Every(func(no int, i *Onevps) {
				waiter.Add(1)
				go func() {
					defer waiter.Done()
					i.Log()
				}()
			})
			waiter.Wait()

			fmt.Print("enter to continue")
			reader.ReadString('\n')
		case "build":
			waiter := sync.WaitGroup{}
			devs.Every(func(no int, i *Onevps) {
				waiter.Add(1)
				go func() {
					defer waiter.Done()
					i.Build()
				}()
			})
			waiter.Wait()

			fmt.Print("enter to continue")
			reader.ReadString('\n')
		case "test":
			waiter := sync.WaitGroup{}
			ts := gs.List[gs.Str]{}
			lock := sync.RWMutex{}
			devs.Every(func(no int, i *Onevps) {
				waiter.Add(1)
				go func() {
					defer waiter.Done()
					ti := i.Test()
					lock.Lock()
					ts = ts.Add(gs.Str("%s-%s:%d").F(i.Location, i.Host, ti))
					lock.Unlock()
				}()
			})
			waiter.Wait()
			ts.Sort(func(l, r gs.Str) bool {
				return l.Split(":").Nth(1).TryLong() > r.Split(":").Nth(1).TryLong()
			})
			ts.Every(func(no int, i gs.Str) {
				t := i.Split(":").Nth(0)
				used := time.Duration(i.Split(":").Nth(1).TryLong())
				gs.Str("%s : %s").F(t, used).Color("g", "B").Println("test")
			})
			fmt.Print("enter to continue")
			reader.ReadString('\n')
		case "update":
			waiter := sync.WaitGroup{}
			devs.Every(func(no int, i *Onevps) {
				waiter.Add(1)
				go func() {
					defer waiter.Done()
					i.Update()

				}()
			})
			waiter.Wait()
			fmt.Print("enter to continue")
			reader.ReadString('\n')
		case "sync":
			fmt.Print("git url:")
			repo, _ := reader.ReadString('\n')
			fmt.Print("git name:")
			gitname, _ := reader.ReadString('\n')
			fmt.Print("git pwd:")
			gitpwd, _ := reader.ReadString('\n')
			fmt.Print("set login name:")
			loginname, _ := reader.ReadString('\n')

			fmt.Print("set login pwd:")
			loginpwd, _ := reader.ReadString('\n')
			SyncToGit(gs.Str(repo).Trim().Str(), gs.Str(gitname).Trim().Str(), gs.Str(gitpwd).Trim().Str(), gs.Str(loginname).Trim().Str(), gs.Str(loginpwd).Trim().Str(), devs)
			fmt.Print("enter to continue")
			reader.ReadString('\n')
		}

	}

}

func GitGetAccount(gitrepo string, namePwd ...string) (vpss gs.List[*Onevps]) {
	name := ""
	pwd := ""

	if namePwd != nil {
		name = namePwd[0]
		if len(namePwd) > 1 {
			pwd = namePwd[1]
		}
	}
	tmprepo := gs.TMP.PathJoin("repot-sync-tmp")
	defer tmprepo.Rm()
	repoUrl := gitrepo
	if tmprepo.IsExists() {
		err := tmprepo.Rm()
		if err != nil {
			gs.Str(err.Error()).Println("Err")
			return
		}
	}
	_, err := git.PlainClone(tmprepo.Str(), false, &git.CloneOptions{
		URL: repoUrl,
		// Progress: os.Stdout,
	})
	if err != nil {
		gs.Str(err.Error()).Add(gs.Str(tmprepo).Color("r")).Println("Err")
		return
	}
	reader := bufio.NewReader(os.Stdin)
	if name == "" {
		gs.Str("login name:").Color("u").Print()
		name, _ = reader.ReadString('\n')
		name = gs.Str(name).Trim().Str()
	}
	filename := tmprepo.PathJoin(name)
	if !filename.IsExists() {
		gs.Str("Login Failed no such config ! "+name).Color("r", "B").Println("login")
		gs.Str(filename).Color("r", "B").Println("login")
		return
	} else {
		gs.Str("Login config ready!"+name).Color("g", "B").Println("login")
	}

	if pwd == "" {
		gs.Str("login pwd:").Color("u").Print()
		pwd, _ = reader.ReadString('\n')
		pwd = gs.Str(pwd).Trim().Str()
	}
	if encrpytedBuf := filename.MustAsFile(); encrpytedBuf != "" {
		if plain := encrpytedBuf.Derypt(pwd); plain.In(".") {

			plain.EveryLine(func(lineno int, line gs.Str) {
				// line.Color("g").Println("Route")
				if line.In("|") {
					vpss = append(vpss, &Onevps{
						Location: line.Split("|").Nth(0).Trim().Str(),
						Host:     line.Split("|").Nth(1).Trim().Str(),
					})

				} else {
					vpss = append(vpss, &Onevps{
						Host: line.Trim().Str(),
					})
				}
			})
			gs.Str("login success !").Color("g", "B").Println("login")

		}
	}

	return

}

func TestRoutes(vpss gs.List[*Onevps]) (sorted gs.List[*Onevps]) {
	waiter := sync.WaitGroup{}
	vpss.Every(func(no int, i *Onevps) {
		waiter.Add(1)
		go func() {
			defer waiter.Done()
			i.Test()
		}()
	})
	waiter.Wait()
	return vpss.Sort(func(l, r *Onevps) bool {
		return l.ConnectedQuality < r.ConnectedQuality
	})
}

func GitMode(gitrepo string, namePwd ...string) string {
	vpss := GitGetAccount(gitrepo, namePwd...)
	gs.Str("Start testing !").Color("g", "B").Println("Routing")
	vpss = TestRoutes(vpss)
	vpss.Every(func(no int, i *Onevps) {
		gs.Str("[%2d] Host: %s Location: %s %s\n").F(no, gs.Str(i.Host).Color("b"), gs.Str(i.Location).Color("y"), i.ConnectedQuality).Print()
	})
	gs.Str("Choose route to listen:").Color("u").Print()
	reader := bufio.NewReader(os.Stdin)
	if namePwd == nil {
		routeNo, _ := reader.ReadString('\n')
		routeNo = gs.Str(routeNo).Trim().Str()

		route := vpss.Nth(gs.Str(routeNo).TryInt()).Host
		gs.Str("").ANSICursor(0, 0).ANSIEraseToEND().Print()
		return route
	} else {

		route := vpss.Nth(-1).Host
		gs.Str("").ANSICursor(0, 0).ANSIEraseToEND().Print()
		return route
	}

	return ""
}

var (
	waitlock    = sync.RWMutex{}
	cacheRoutes gs.List[*Onevps]
)

func RouteModeInit(gitrepo string, namepwd ...string) {
	waitlock.Lock()
	cacheRoutes = GitGetAccount(gitrepo, namepwd...)
	if cacheRoutes.Count() == 0 {
		gs.Str(gitrepo).Color("g").Add(namepwd).Color("r").Println("Err in init")
		os.Exit(1)
	}
	waitlock.Unlock()
	gs.Str("testing routes: %d").F(cacheRoutes.Count()).Color("g", "B").Println("Routing")
	RouteModeTest()
	go func() {
		itner := time.NewTicker(1 * time.Hour)
		for {
			select {
			case <-itner.C:
				gs.Str("testing routes: %d").F(cacheRoutes.Count()).Color("g", "B").Println("Routing")
				RouteModeTest()
			default:
				time.Sleep(1 * time.Minute)
			}
		}
	}()
}

func GetNewRoute() string {
	gs.Str("wait testing").Print()
	if cacheRoutes.Count() > 0 {
		no := 0
		waitlock.Lock()
		if cacheRoutes.Count() > 3 {
			no = gs.RAND.Int() % 3
		}
		e := cacheRoutes[no].Host
		waitlock.Unlock()
		gs.Str("             ").Print()
		return e
	}
	return ""
}

func RouteModeTest() {
	if cacheRoutes.Count() > 0 {
		waitlock.Lock()
		newcacheRouts := TestRoutes(cacheRoutes)
		cacheRoutes = newcacheRouts
		waitlock.Unlock()

	}
}

func QuietStdout(do func(e string)) {
	one := gt.Select[string](gs.List[string]{
		"https://github.com/",
		"https://gitee.com/",
		"https://gitcoffe.com/",
		"https://gitlab.com/",
		"https://git.me/",
	})

	do(one)
}

func RunLocalRouterMode(repo, name, pwd string, l int) {
	RouteModeInit(repo, name, pwd)
	server := GetNewRoute()
	cli := clientcontroll.NewClientControll(server, l)
	cli.GetNewRoute = GetNewRoute
	cli.Socks5Listen()

}
