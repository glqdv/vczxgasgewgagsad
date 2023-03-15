package base

import (
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"time"

	"gitee.com/dark.H/gs"
	"github.com/xtaci/kcp-go"
	"golang.org/x/crypto/pbkdf2"
)

var (
	readTimeout time.Duration
)

type ProtocolConfig struct {
	ID           string      `json:"id"`
	Server       interface{} `json:"server"`
	ServerPort   int         `json:"server_port"`
	LocalPort    int         `json:"local_port"`
	LocalAddress string      `json:"local_address"`
	Password     string      `json:"password"`
	Method       string      `json:"method"` // encryption method
	Tag          string      `json:"tag"`
	CryptoMethod string      `json:"c-method"`
	ProxyType    string      `json:"proxy-type"`
	// following options are only used by server
	PortPassword map[string]string `json:"port_password"`
	Timeout      int               `json:"timeout"`
	LastPing     int               `json:"last_ping"`
	// following options are only used by client

	// The order of servers in the client config is significant, so use array
	// instead of map to preserve the order.
	ServerPassword string `json:"server_password"`

	// shadowsocks options
	SSPassword  string `json:"ss_password"`
	OldSSPwd    string `json:"ss_old"`
	SSMethod    string `json:"ss_method"`
	SALT        string `json:"salt"`
	EBUFLEN     int    `json:"buflen"`
	Type        string `json:"type"`
	IsErr       bool
	OtherConfig gs.Dict[any]
}

// GeneratePassword by config
func (config *ProtocolConfig) GeneratePassword(plugin ...string) (en kcp.BlockCrypt) {
	klen := 32
	if strings.Contains(config.Method, "128") {
		klen = 16
	}
	mainMethod := strings.Split(config.Method, "-")[0]
	var keyData []byte
	if config.SALT == "" && config.EBUFLEN == 0 {
		keyData = pbkdf2.Key([]byte(config.Password), []byte("kcpKCPkcp"), 1024, klen, sha1.New)

		if plugin != nil {
			keyData = pbkdf2.Key([]byte(config.Password), []byte("kcpKCPkcp"), 4096, klen, sha1.New)
		}
	} else {
		keyData = pbkdf2.Key([]byte(config.Password), []byte(config.SALT), config.EBUFLEN, klen, sha1.New)
	}

	switch mainMethod {

	case "des":
		en, _ = kcp.NewTripleDESBlockCrypt(keyData[:klen])
	case "tea":
		en, _ = kcp.NewTEABlockCrypt(keyData[:klen])
	case "simple":
		en, _ = kcp.NewSimpleXORBlockCrypt(keyData[:klen])
	case "xtea":
		en, _ = kcp.NewXTEABlockCrypt(keyData[:klen])
	default:
		en, _ = kcp.NewAESBlockCrypt(keyData[:klen])
	}

	return
}

func (config *ProtocolConfig) Json() string {
	buf, _ := json.Marshal(config)
	return string(buf)
}

func (config *ProtocolConfig) RemoteAddr() string {
	return gs.Str("%s:%d").F(config.Server, config.ServerPort).Str()
}

func (config *ProtocolConfig) GetTlsConfig() (conf *tls.Config, ok bool) {
	// SHARED_TLS_KEY = (gs.S(cerPEM) + "|" + gs.S(keyPEM)).Str()
	if config.ProxyType != "tls" {
		return nil, false
	}
	fs := gs.Str(config.Password).Split("|", 2)
	cerPEM, keyPEM := fs[0].Trim(), fs[1].Trim()
	// Load the certificate and private key
	cert, err := tls.X509KeyPair(cerPEM.Bytes(), keyPEM.Bytes())
	if err != nil {
		gs.Str(config.Json()).Println("err config ?")
		panic(err)
	}
	certpool := x509.NewCertPool()
	certpool.AppendCertsFromPEM(cerPEM.Bytes())
	ok = true
	conf = &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            certpool,
		ClientCAs:          certpool,
		InsecureSkipVerify: true,
	}
	return
}

func (config *ProtocolConfig) GetQuicConfig() (conf *tls.Config, ok bool) {
	// SHARED_TLS_KEY = (gs.S(cerPEM) + "|" + gs.S(keyPEM)).Str()
	if config.ProxyType != "quic" {
		return nil, false
	}
	fs := gs.Str(config.Password).Split("|", 2)
	cerPEM, keyPEM := fs[0].Trim(), fs[1].Trim()
	// Load the certificate and private key
	cert, err := tls.X509KeyPair(cerPEM.Bytes(), keyPEM.Bytes())
	if err != nil {
		gs.Str(config.Json()).Println("err config ?")
		panic(err)
	}
	certpool := x509.NewCertPool()
	certpool.AppendCertsFromPEM(cerPEM.Bytes())
	ok = true
	conf = &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            certpool,
		ClientCAs:          certpool,
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-stream"},
	}
	return
}

// GetServerArray get serverfdf
func (config *ProtocolConfig) GetServerArray() []string {
	// Specifying multiple servers in the "server" options is deprecated.
	// But for backward compatibility, keep this.
	if config.Server == nil {
		return nil
	}
	single, ok := config.Server.(string)
	if ok {
		return []string{single}
	}
	arr, ok := config.Server.([]interface{})
	if ok {
		serverArr := make([]string, len(arr), len(arr))
		for i, s := range arr {
			serverArr[i], ok = s.(string)
			if !ok {
				goto typeError
			}
		}
		return serverArr
	}
typeError:
	panic(fmt.Sprintf("Config.Server type error %v", reflect.TypeOf(config.Server)))
}

// ParseConfig parse path to json
func ParseConfig(path string) (config *ProtocolConfig, err error) {
	file, err := os.Open(path) // For read access.
	if err != nil {
		return
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return
	}

	config = &ProtocolConfig{}
	if err = json.Unmarshal(data, config); err != nil {
		return nil, err
	}
	readTimeout = time.Duration(config.Timeout) * time.Second
	return
}

func RandomConfig() *ProtocolConfig {
	c := new(ProtocolConfig)
	port := GiveAPort()
	c.ServerPort = port
	c.Server = "0.0.0.0"
	c.ServerPassword = string(gs.Str("").RandStr(16))
	c.Password = string(gs.Str("").RandStr(16))
	c.SALT = string(gs.Str("").RandStr(8))
	c.Method = "aes-256"
	c.EBUFLEN = 4096
	c.ID = string(gs.Str("").RandStr(32))
	return c
}

func JsonConfig(jsonStr string) (config *ProtocolConfig, err error) {
	config = new(ProtocolConfig)
	err = json.Unmarshal([]byte(jsonStr), config)
	if err != nil {
		return
	}
	if config.ID == "" {
		config.ID = string(gs.Str("").RandStr(32))
	}
	return
}
