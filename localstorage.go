package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type LocalStorage struct {
	Printers []*Printer `yaml:"printers"`
	savePath string
	byID     map[string]*Printer
	byIP     map[string]*Printer
}

func NewLocalStorage(savePath string) *LocalStorage {
	s := &LocalStorage{
		Printers: []*Printer{},
		savePath: savePath,
	}

	if b, err := os.ReadFile(savePath); err == nil {
		yaml.Unmarshal(b, &s)
	}

	s.rebuildIndex()
	return s
}

func (ls *LocalStorage) rebuildIndex() {
	ls.byID = make(map[string]*Printer, len(ls.Printers))
	ls.byIP = make(map[string]*Printer, len(ls.Printers))
	for _, p := range ls.Printers {
		if p.ID != "" {
			ls.byID[p.ID] = p
		}
		if p.IP != "" {
			ls.byIP[p.IP] = p
		}
	}
}

// Add to add printers to LocalStorage
func (ls *LocalStorage) Add(printers ...*Printer) {
	for _, p := range printers {
		if p.ID == "" {
			continue
		}
		if existing, ok := ls.byID[p.ID]; ok {
			// Printer already exists, update fields
			if existing.IP != p.IP {
				if existing.IP != "" {
					delete(ls.byIP, existing.IP)
				}
				existing.IP = p.IP
				if p.IP != "" {
					ls.byIP[p.IP] = existing
				}
			}
			if p.Token != "" && existing.Token != p.Token {
				existing.Token = p.Token
			}
		} else {
			// New printer
			ls.Printers = append(ls.Printers, p)
			ls.byID[p.ID] = p
			if p.IP != "" {
				ls.byIP[p.IP] = p
			}
		}
	}
}

func (ls *LocalStorage) Save() (err error) {
	if b, err := yaml.Marshal(ls); err == nil {
		return os.WriteFile(ls.savePath, b, 0644)
	}
	return
}

func (ls *LocalStorage) Find(host string) *Printer {
	if p, ok := ls.byID[host]; ok {
		return p
	}
	if p, ok := ls.byIP[host]; ok {
		return p
	}
	return nil
}
