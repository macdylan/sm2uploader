package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type LocalStorage struct {
	Printers []*Printer `yaml:"printers"`
	savePath string
}

func NewLocalStorage(savePath string) *LocalStorage {
	s := &LocalStorage{
		Printers: []*Printer{},
		savePath: savePath,
	}

	if b, err := os.ReadFile(savePath); err == nil {
		yaml.Unmarshal(b, &s)
	}

	return s
}

// Add to add printers to LocalStorage
func (ls *LocalStorage) Add(printers ...*Printer) {
	// Iterate over each printer
	for _, p := range printers {
		// Skip if printer ID is empty
		if p.ID == "" {
			continue
		}
		// Iterate over each printer in LocalStorage
		for idx, x := range ls.Printers {
			// If printer ID matches, update IP and Token if necessary
			if x.ID == p.ID {
				if x.IP != p.IP {
					ls.Printers[idx].IP = p.IP
				}
				if p.Token != "" && x.Token != p.Token {
					ls.Printers[idx].Token = p.Token
				}
				// Go to exists label
				goto exists
			}
		}
		// Append printer to LocalStorage
		ls.Printers = append(ls.Printers, p)
		// Label for when printer already exists
	exists:
	}
}

func (ls *LocalStorage) Save() (err error) {
	if b, err := yaml.Marshal(ls); err == nil {
		return os.WriteFile(ls.savePath, b, 0644)
	}
	return
}

func (ls *LocalStorage) Find(host string) *Printer {
	for _, p := range ls.Printers {
		if p.ID == host || p.IP == host {
			return p
		}
	}
	return nil
}
