package prosmux

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/xtaci/smux"
)

var FGCOLORS = []func(a ...interface{}) string{
	color.New(color.FgYellow, color.Bold).SprintFunc(),
	color.New(color.FgRed, color.Bold).SprintFunc(),
	color.New(color.FgGreen, color.Bold).SprintFunc(),
	color.New(color.FgBlue, color.Bold).SprintFunc(),
}
var BGCOLORS = []func(a ...interface{}) string{
	color.New(color.BgYellow, color.Bold).SprintFunc(),
	color.New(color.BgRed, color.Bold).SprintFunc(),
	color.New(color.BgGreen, color.Bold).SprintFunc(),
	color.New(color.BgBlue, color.Bold).SprintFunc(),
}

type ProxyHandler interface {
	AcceptHandle(waiter time.Duration, handle func(con net.Conn) error) error
	TryClose()
	DelCon(con net.Conn)
}

type SmuxConfig struct {
	Mode            string `json:"mode"`
	NoDelay         int    `json:"nodelay"`
	Interval        int    `json:"interval"`
	Resend          int    `json:"resend"`
	NoCongestion    int    `json:"nocongeestion"`
	AutoExpire      int    `json:"autoexpire"`
	ScavengeTTL     int    `json:"scavengettl"`
	MTU             int    `json:"mtu"`
	SndWnd          int    `json:"sndwnd"`
	RcvWnd          int    `json:"rcvwnd"`
	DataShard       int    `json:"datashard"`
	ParityShard     int    `json:"parityshard"`
	KeepAlive       int    `json:"keepalive"`
	SmuxBuf         int    `json:"smuxbuf"`
	StreamBuf       int    `json:"streambuf"`
	AckNodelay      bool   `json:"acknodelay"`
	SocketBuf       int    `json:"socketbuf"`
	WrapProxyServer ProxyHandler
	ClientConn      net.Conn
	clienConf       *smux.Config
	Session         *smux.Session
	ZeroToDel       *bool
	ProxyType       string
	handleStream    func(conn net.Conn) (err error)
}

func (kconfig *SmuxConfig) SetAsDefault() {
	kconfig.Mode = "fast4"

	kconfig.KeepAlive = 10
	kconfig.MTU = 1350
	kconfig.DataShard = 10
	kconfig.ParityShard = 3
	kconfig.SndWnd = 2048 * 2
	kconfig.RcvWnd = 2048 * 2
	kconfig.ScavengeTTL = 600
	kconfig.AutoExpire = 7
	kconfig.SmuxBuf = 4194304 * 2
	kconfig.StreamBuf = 2097152 * 2
	kconfig.AckNodelay = false
	kconfig.SocketBuf = 4194304 * 2
}

func NewSmuxServer(proxyServer ProxyHandler, handle func(con net.Conn) (err error)) (s *SmuxConfig) {
	s = new(SmuxConfig)
	s.WrapProxyServer = proxyServer
	s.handleStream = handle
	s.SetAsDefault()
	return
}

func NewSmuxServerNull() (s *SmuxConfig) {
	s = new(SmuxConfig)
	// s.Listener = listener
	// s.handleStream = handle
	s.SetAsDefault()
	return
}

func NewSmuxClient(conn net.Conn, proxyType string) (s *SmuxConfig) {
	s = new(SmuxConfig)
	// Create a multiplexer using smux
	// conf := s.GenerateConfig()
	s.ClientConn = conn
	s.SetAsDefault()
	s.ProxyType = proxyType
	s.clienConf = s.GenerateConfig()
	// s.UpdateMode()
	if s.ClientConn == nil {
		return nil
	}
	mux, err := smux.Client(s.ClientConn, s.clienConf)
	// ColorD(s)
	if err != nil {
		fmt.Println(err)
		return
	}
	s.Session = mux
	return
}
func (s *SmuxConfig) IsClosed() bool {
	if s.Session == nil {
		return false
	}
	return s.Session.IsClosed()
}

func (s *SmuxConfig) GetProxyType() string {
	return s.ProxyType
}

func (s *SmuxConfig) NewConnnect() (con net.Conn, err error) {

	// Create a new stream on the multiplexer
	if s.Session == nil {
		return nil, errors.New("session closed")
	}
	if !s.Session.IsClosed() {
		stream, err2 := s.Session.OpenStream()
		if err2 != nil {
			s.Session, err2 = smux.Client(s.ClientConn, s.clienConf)

			if err2 != nil || s.Session.IsClosed() {

				return nil, errors.New("[reclient smux err conn is break]: " + err2.Error())
			}

			if stream, err = s.Session.OpenStream(); err != nil {
				return nil, errors.New("[re open smux session err]: " + err.Error())
			} else {
				return stream, nil
			}
		}
		return stream, nil

	} else {
		return nil, errors.New("session closed")
	}

}

func (s *SmuxConfig) Close() error {
	if s.Session != nil {
		return s.Session.Close()
	}

	return nil
}

func (kconfig *SmuxConfig) UpdateMode() {
	// kconfig.Mode = mode
	switch kconfig.Mode {
	case "normal":
		kconfig.NoDelay, kconfig.Interval, kconfig.Resend, kconfig.NoCongestion = 0, 40, 2, 1
	case "fast":
		kconfig.NoDelay, kconfig.Interval, kconfig.Resend, kconfig.NoCongestion = 0, 30, 2, 1
	case "fast2":
		kconfig.NoDelay, kconfig.Interval, kconfig.Resend, kconfig.NoCongestion = 1, 20, 2, 1
	case "fast3":
		kconfig.NoDelay, kconfig.Interval, kconfig.Resend, kconfig.NoCongestion = 1, 10, 2, 1
	case "fast4":
		kconfig.NoDelay, kconfig.Interval, kconfig.Resend, kconfig.NoCongestion = 1, 5, 2, 1
	}

	// ColorL("kcp mode", kconfig.Mode)
}

func (kconfig *SmuxConfig) GenerateConfig() *smux.Config {
	smuxConfig := smux.DefaultConfig()
	kconfig.UpdateMode()
	// smuxConfig.Version = 2
	smuxConfig.MaxReceiveBuffer = kconfig.SmuxBuf
	smuxConfig.MaxStreamBuffer = kconfig.StreamBuf
	smuxConfig.KeepAliveInterval = time.Duration(kconfig.KeepAlive) * time.Second
	if err := smux.VerifyConfig(smuxConfig); err != nil {
		log.Fatalf("%+v", err)
	}
	return smuxConfig
}

func (m *SmuxConfig) Server() (err error) {
	// ColorD(m)

	m.WrapProxyServer.AcceptHandle(1*time.Minute, func(con net.Conn) error {
		go m.AccpetStream(con)
		return nil
	})

	// for {
	// LOOP:
	// 	// Accept a TCP connection
	// 	select {
	// 	case <-wait10minute.C:
	// 	default:
	// 		if *m.ZeroToDel {
	// 			m.Listener.Close()
	// 			break LOOP
	// 		} else {
	// 			wait10minute.Reset(10 * time.Minute)
	// 		}

	// 	}
	// 	conn, err := m.Listener.Accept()
	// 	if err != nil {
	// 		time.Sleep(10 * time.Second)
	// 		gs.Str(err.Error()).Println("smux raw conn accpet err")
	// 		m.Listener.Close()
	// 		break
	// 	}

	// 	go m.AccpetStream(conn)
	// }
	// m.Listener.Close()
	return nil
	// return err
}

func (m *SmuxConfig) SetHandler(handler func(con net.Conn) (err error)) {
	m.handleStream = handler
}

func (m *SmuxConfig) AccpetStream(conn net.Conn) (err error) {
	smuxconfig := m.GenerateConfig()
	err = smux.VerifyConfig(smuxconfig)
	if err != nil {
		panic(err)
	}

	// Use smux to multiplex the connection
	mux, err := smux.Server(conn, smuxconfig)
	if err != nil {
		// fmt.Println(err)
		return
	}

	// Use WaitGroup to wait for all streams to finish
	var wg sync.WaitGroup
	for {
		// Accept a new stream
		stream, err := mux.AcceptStream()
		if err != nil {
			// fmt.Println(err)
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			// gs.Str("comming").Println("smux session accpet")
			m.handleStream(stream)
		}()
	}

	// Wait for all streams to finish before closing the multiplexer
	wg.Wait()
	mux.Close()

	return
}

func ColorD(args interface{}, join ...string) {

	if b, err := json.Marshal(args); err == nil {
		var data map[string]interface{}
		// yellow := FGCOLORS[0]
		if err := json.Unmarshal(b, &data); err == nil {
			var S []string
			c := 0
			for k, v := range data {
				// ColorD(data)
				S = append(S, fmt.Sprint(k, ": ", FGCOLORS[c](v)))
				c++
				c %= len(BGCOLORS)
			}
			if len(join) == 0 {
				fmt.Println(strings.Join(S, "\n"))
			}

		}
	}
}
