package main

import (
	"errors"
	"fmt"
	"strings"
)

type Printer struct {
	IP    string `yaml:"ip"`
	ID    string `yaml:"id"`
	Model string `yaml:"model"`
	Token string `yaml:"token"`
	Sacp  bool   `yaml:"sacp"`
}

/*
NewPrinter create new printer from response
Snapmaker J1X123P@192.168.1.201|model:Snapmaker J1|status:IDLE|SACP:1
*/
func NewPrinter(resp []byte) (*Printer, error) {
	msg := string(resp)
	if !strings.Contains(msg, "|model:") || !strings.Contains(msg, "@") {
		return nil, errors.New("invalid response")
	}
	var (
		parts = strings.Split(msg, "|")
		id    = parts[0][:strings.LastIndex(parts[0], "@")]
		ip    = parts[0][strings.LastIndex(parts[0], "@")+1:]
		model = parts[1][strings.Index(parts[1], ":")+1:]
		sacp  = strings.Contains(msg, "SACP:1")
	)

	return &Printer{
		IP:    ip,
		ID:    id,
		Model: model,
		Token: "",
		Sacp:  sacp,
	}, nil
}

/* Name for promptui */
func (p *Printer) String() string {
	return fmt.Sprintf("%s@%s - %s", p.ID, p.IP, p.Model)
}
