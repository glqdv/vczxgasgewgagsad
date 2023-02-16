package proquic

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"time"

	"gitee.com/dark.H/ProxyZ/connections/base"
	"github.com/quic-go/quic-go"
)

type QuicClient struct {
	addr      string
	isclosed  bool
	tlsconfig *tls.Config
	qcon      quic.Connection
}

func NewQuicClient(config *base.ProtocolConfig) (qc *QuicClient, err error) {
	qc = new(QuicClient)
	qc.addr = config.RemoteAddr()
	qc.tlsconfig, _ = config.GetQuicConfig()
	cc, _ := context.WithTimeout(context.Background(), 10*time.Second)
	// gs.Str("test be").Println("quic config")
	conn, err := quic.DialAddrContext(cc, qc.addr, qc.tlsconfig, nil)
	// conn, err := quic.DialAddr(qc.addr, tlsconfig, nil)

	if err != nil {
		qc.isclosed = true
		return qc, err
	}
	qc.qcon = conn
	return
}

func (qc *QuicClient) IsClosed() bool {
	return qc.isclosed
}

func (q *QuicClient) NewConnnect() (con net.Conn, err error) {
	if q.IsClosed() || q.qcon == nil {
		return nil, errors.New("dia quic err")
	}
	conn := q.qcon
	var stream quic.Stream
	// gs.Str("open stream !!").Println()
	stream, err = conn.OpenStream()

	if err != nil {

		cc, _ := context.WithTimeout(context.Background(), 10*time.Second)
		if conn, err := quic.DialAddrContext(cc, q.addr, q.tlsconfig, nil); err != nil {
			q.isclosed = true
			q.isclosed = true
			return nil, errors.New("[try agin quic new connect err]: " + err.Error())
		} else {
			q.qcon = conn
		}
		stream, err = conn.OpenStream()
		if err != nil {
			return nil, errors.New("[try agin  quic reconnect err]: " + err.Error())
		}
	}
	qq := WrapQuicNetConn(stream)
	return qq, err
}

func (q *QuicClient) Close() error {
	q.isclosed = true
	if q.qcon != nil {
		return q.qcon.CloseWithError(quic.ApplicationErrorCode(0), "closd")
	} else {
		return errors.New("no qcon")
	}

}
