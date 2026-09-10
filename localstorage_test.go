package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewLocalStorageMissingFile(t *testing.T) {
	ls := NewLocalStorage(filepath.Join(t.TempDir(), "missing.yaml"))
	if len(ls.Printers) != 0 {
		t.Errorf("Printers = %v, want empty", ls.Printers)
	}
}

func TestLocalStorageAddAndFind(t *testing.T) {
	ls := NewLocalStorage(filepath.Join(t.TempDir(), "hosts.yaml"))

	// skip ID-less printers
	ls.Add(&Printer{IP: "1.1.1.1"})
	if len(ls.Printers) != 0 {
		t.Errorf("ID-less printer was added: %v", ls.Printers)
	}

	a := &Printer{IP: "192.168.1.10", ID: "J1V19", Model: "Snapmaker J1"}
	ls.Add(a)
	if len(ls.Printers) != 1 {
		t.Fatalf("Printers = %v, want 1", ls.Printers)
	}

	// find by ID
	if got := ls.Find("J1V19"); got == nil || got.IP != "192.168.1.10" {
		t.Errorf("Find(ID) = %v", got)
	}
	// find by IP
	if got := ls.Find("192.168.1.10"); got == nil || got.ID != "J1V19" {
		t.Errorf("Find(IP) = %v", got)
	}
	// miss
	if got := ls.Find("unknown"); got != nil {
		t.Errorf("Find(unknown) = %v, want nil", got)
	}
}

func TestLocalStorageUpdateIP(t *testing.T) {
	ls := NewLocalStorage(filepath.Join(t.TempDir(), "hosts.yaml"))
	ls.Add(&Printer{IP: "192.168.1.10", ID: "J1V19"})

	// same printer shows up on a new address
	ls.Add(&Printer{IP: "192.168.1.99", ID: "J1V19"})
	if len(ls.Printers) != 1 {
		t.Fatalf("duplicate added: %v", ls.Printers)
	}
	if got := ls.Find("J1V19"); got.IP != "192.168.1.99" {
		t.Errorf("IP = %q, want updated address", got.IP)
	}
	// old IP index entry must be gone
	if got := ls.Find("192.168.1.10"); got != nil {
		t.Errorf("stale IP still resolves: %v", got)
	}
	if got := ls.Find("192.168.1.99"); got == nil {
		t.Error("new IP not indexed")
	}
}

func TestLocalStorageTokenRefresh(t *testing.T) {
	ls := NewLocalStorage(filepath.Join(t.TempDir(), "hosts.yaml"))
	ls.Add(&Printer{IP: "192.168.1.10", ID: "A350", Token: "old-token"})

	// empty token must not overwrite
	ls.Add(&Printer{IP: "192.168.1.10", ID: "A350", Token: ""})
	if got := ls.Find("A350"); got.Token != "old-token" {
		t.Errorf("Token = %q, want old-token preserved", got.Token)
	}

	ls.Add(&Printer{IP: "192.168.1.10", ID: "A350", Token: "new-token"})
	if got := ls.Find("A350"); got.Token != "new-token" {
		t.Errorf("Token = %q, want new-token", got.Token)
	}
}

func TestLocalStorageSaveLoadRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts.yaml")
	ls := NewLocalStorage(path)
	ls.Add(&Printer{IP: "192.168.1.10", ID: "J1V19", Model: "Snapmaker J1", Token: "tok"})
	ls.Add(&Printer{IP: "192.168.1.20", ID: "A350", Model: "Snapmaker A350", Token: "", Sacp: true})

	if err := ls.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}

	ls2 := NewLocalStorage(path)
	if len(ls2.Printers) != 2 {
		t.Fatalf("reloaded %d printers, want 2", len(ls2.Printers))
	}
	if got := ls2.Find("J1V19"); got == nil || got.Token != "tok" || got.Model != "Snapmaker J1" {
		t.Errorf("reloaded printer = %+v", got)
	}
	if got := ls2.Find("A350"); got == nil || !got.Sacp {
		t.Errorf("reloaded printer = %+v", got)
	}
}

func TestLocalStorageOverwritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts.yaml")
	ls := NewLocalStorage(path)
	ls.Add(&Printer{IP: "10.0.0.1", ID: "ONE"})
	if err := ls.Save(); err != nil {
		t.Fatal(err)
	}

	ls2 := NewLocalStorage(path)
	ls2.Printers = nil // simulate a wipe then re-save
	if err := ls2.Save(); err != nil {
		t.Fatal(err)
	}
	ls3 := NewLocalStorage(path)
	if len(ls3.Printers) != 0 {
		t.Errorf("stale printers survived overwrite: %v", ls3.Printers)
	}
}
