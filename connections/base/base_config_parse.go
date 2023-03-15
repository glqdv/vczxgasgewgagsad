package base

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

func ParseURI(u string) (config *ProtocolConfig) {
	config = new(ProtocolConfig)
	parseURI(u, config)
	return
}

func parseURI(u string, cfg *ProtocolConfig) (string, error) {
	if u == "" {
		return "", nil
	}
	invalidURI := errors.New("invalid URI")
	// ss://base64(method:password)@host:port
	// ss://base64(method:password@host:port)
	u = strings.TrimLeft(u, "ss://")
	i := strings.IndexRune(u, '@')
	var headParts, tailParts [][]byte
	if i == -1 {
		dat, err := base64.StdEncoding.DecodeString(u)
		if err != nil {
			return "", err
		}
		parts := bytes.Split(dat, []byte("@"))
		if len(parts) != 2 {
			return "", invalidURI
		}
		headParts = bytes.SplitN(parts[0], []byte(":"), 2)
		tailParts = bytes.SplitN(parts[1], []byte(":"), 2)

	} else {
		if i+1 >= len(u) {
			return "", invalidURI
		}
		tailParts = bytes.SplitN([]byte(u[i+1:]), []byte(":"), 2)
		dat, err := base64.StdEncoding.DecodeString(u[:i])
		if err != nil {
			return "", err
		}
		headParts = bytes.SplitN(dat, []byte(":"), 2)
	}
	if len(headParts) != 2 {
		return "", invalidURI
	}

	if len(tailParts) != 2 {
		return "", invalidURI
	}
	cfg.Method = string(headParts[0])

	cfg.Password = string(headParts[1])
	p, e := strconv.Atoi(string(tailParts[1]))
	if e != nil {
		return "", e
	}
	cfg.Server = string(tailParts[0])
	cfg.ServerPort = p
	return string(tailParts[0]), nil

}
