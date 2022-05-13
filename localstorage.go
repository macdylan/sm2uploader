package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type LocalStorage struct {
	LastID   string     `yaml:"last_id"`
	Machines []*Machine `yaml:"machines"`
	file     string
}

func NewLocalStorage(file string) *LocalStorage {
	s := &LocalStorage{
		LastID:   "",
		Machines: []*Machine{},
		file:     file,
	}

	if b, err := os.ReadFile(file); err == nil {
		yaml.Unmarshal(b, &s)
	}

	return s
}

func (ls *LocalStorage) Add(m *Machine) *LocalStorage {
	found := false

	// update
	if m.ID != "" {
		for idx, x := range ls.Machines {
			if x.ID == m.ID {
				found = true
				if x.IP != m.IP {
					ls.Machines[idx].IP = m.IP
				}
				if m.Token != "" && x.Token != m.Token {
					ls.Machines[idx].Token = m.Token
				}
				break
			}
		}
	}

	// append
	if !found {
		ls.Machines = append(ls.Machines, m)
	}
	return ls
}

func (ls *LocalStorage) Save() (err error) {
	if b, err := yaml.Marshal(ls); err == nil {
		err = os.WriteFile(ls.file, b, 0644)
	}
	return
}

func (ls *LocalStorage) FindByID(identity string) *Machine {
	for _, m := range ls.Machines {
		if m.ID != "" && m.ID == identity {
			return m
		}
	}
	return nil
}

func (ls *LocalStorage) FindByIP(ip string) *Machine {
	for _, m := range ls.Machines {
		if m.IP != "" && m.IP == ip {
			return m
		}
	}
	return nil
}

func (ls *LocalStorage) SetLast(m *Machine) *LocalStorage {
	ls.Add(m)
	ls.LastID = m.ID
	return ls
}

func (ls *LocalStorage) GetLast() *Machine {
	if ls.LastID != "" {
		return ls.FindByID(ls.LastID)
	}
	return nil
}
