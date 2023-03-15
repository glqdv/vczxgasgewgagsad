package deploy

import (
	"log"
	"os"
	"path"
)

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
