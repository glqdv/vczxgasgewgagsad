package proquic

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/quic-go/quic-go"
)

var (
	CERT              = "Resources/pem/cert.pem"
	KEYPEM            = "Resources/pem/key.pem"
	SHARED_TLS_CONFIG *tls.Config
	SHARED_TLS_KEY    = ""
)

type QuicConn struct {
	steam quic.Stream
}

func WrapQuicNetConn(s quic.Stream) (qc *QuicConn) {
	return &QuicConn{
		steam: s,
	}
}

func (quic *QuicConn) LocalAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.IPv4(byte(127), byte(0), byte(0), byte(1)),
		Port: 0,
	}
}
func (quic *QuicConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.IPv4(byte(0), byte(0), byte(0), byte(1)),
		Port: 0,
	}
}

func (quic *QuicConn) Read(buf []byte) (n int, err error) {
	return quic.steam.Read(buf)
}

func (quic *QuicConn) Write(buf []byte) (n int, err error) {
	return quic.steam.Write(buf)
}

func (quic *QuicConn) Close() (err error) {
	return quic.steam.Close()
}

func (quic *QuicConn) SetReadDeadline(t time.Time) (err error) {
	return quic.steam.SetReadDeadline(t)
}

func (quic *QuicConn) SetWriteDeadline(t time.Time) (err error) {
	return quic.steam.SetWriteDeadline(t)
}

func (quic *QuicConn) SetDeadline(t time.Time) (err error) {
	return quic.steam.SetDeadline(t)
}
