package base

// func ProxyEnd(lastConn net.Conn, host string) (err error) {

// 	// utils.ColorL("func:", "handleRemote")

// 	closed := false
// 	if strings.ContainsRune(host, 0x00) {
// 		log.Println("invalid domain name.")
// 		closed = true
// 		return
// 	}
// 	num := serve.GetAliveNum()
// 	remote, err := net.Dial("tcp", host)
// 	if err != nil {
// 		if ne, ok := err.(*net.OpError); ok && (ne.Err == syscall.EMFILE || ne.Err == syscall.ENFILE) {
// 			// log too many open file error
// 			// EMFILE is process reaches open file limits, ENFILE is system limit
// 			log.Println(fmt.Sprintf("%d/%d", num, serve.AcceptConn), "dial error too many file!!:", err)
// 		} else {
// 			log.Println(fmt.Sprintf("%d/%d", num, serve.AcceptConn), "handleRemote", host, "Err", err)
// 		}
// 		// log.Println("X connect to ->", host)
// 		return
// 	}

// 	switch serve.Plugin {
// 	case "ss":
// 		utils.ColorL(fmt.Sprintf("%d/%d", num, serve.AcceptConn), "handleRemote", host, "Shadowsocks", "ok")
// 	default:
// 		_, err = conn.Write(Socks5ConnectedRemote)
// 		if err != nil {
// 			utils.ColorL(fmt.Sprintf("%d/%d", num, serve.AcceptConn), "Err", err)
// 		}
// 		utils.ColorL(fmt.Sprintf("%d/%d", num, serve.AcceptConn), "handleRemote", host, "ok")

// 	}

// 	defer func() {
// 		if !closed {
// 			remote.Close()
// 		}
// 	}()

// 	serve.Pipe(conn, remote)
// }
