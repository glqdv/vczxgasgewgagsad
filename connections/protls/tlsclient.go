package protls

import (
	"crypto/tls"
	"net"
	"time"

	"gitee.com/dark.H/ProxyZ/connections/base"
)

func ConnectTls(config *base.ProtocolConfig) (con net.Conn, err error) {
	dst := config.RemoteAddr()
	tlsconfig, _ := config.GetTlsConfig()
	d := &net.Dialer{Timeout: 12 * time.Second}
	return tls.DialWithDialer(d, "tcp", dst, tlsconfig)
}
