package main

import (
	"errors"
	"net"
	"strings"
)

type Machine struct {
	Addr     *net.UDPAddr `yaml:"-"`
	Response []byte       `yaml:"-"`
	IP       string       `yaml:"ip"`
	ID       string       `yaml:"id"`
	Token    string       `yaml:"token"`
}

func NewMachine(addr *net.UDPAddr, resp []byte) (*Machine, error) {
	identity := string(resp)
	if !strings.Contains(identity, "|model:") || !strings.Contains(identity, "@") {
		return nil, errors.New("invalid response")
	}
	identity = strings.Split(identity, "@")[0]
	return &Machine{
		Addr:     addr,
		Response: resp,
		IP:       addr.IP.String(),
		ID:       identity,
		Token:    "",
	}, nil
}

/*
URL to make url with host and path */
func (m *Machine) URL(path string) string {
	return "http://" + m.IP + ":8080/api/v1" + path
}
