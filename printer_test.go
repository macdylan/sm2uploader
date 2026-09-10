package main

import "testing"

func TestNewPrinterValid(t *testing.T) {
	p, err := NewPrinter([]byte("Snapmaker J1X123P@192.168.1.201|model:Snapmaker J1|status:IDLE|SACP:1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID != "Snapmaker J1X123P" {
		t.Errorf("ID = %q, want %q", p.ID, "Snapmaker J1X123P")
	}
	if p.IP != "192.168.1.201" {
		t.Errorf("IP = %q, want %q", p.IP, "192.168.1.201")
	}
	if p.Model != "Snapmaker J1" {
		t.Errorf("Model = %q, want %q", p.Model, "Snapmaker J1")
	}
	if !p.Sacp {
		t.Error("Sacp = false, want true")
	}
	if p.Moonraker {
		t.Error("Moonraker = true, want false")
	}
	if p.Token != "" {
		t.Errorf("Token = %q, want empty", p.Token)
	}
}

func TestNewPrinterWithoutSacp(t *testing.T) {
	p, err := NewPrinter([]byte("A350-3DP@192.168.1.20|model:Snapmaker 2 Model A350|status:IDLE"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Sacp {
		t.Error("Sacp = true, want false")
	}
}

func TestNewPrinterInvalid(t *testing.T) {
	cases := []string{
		"",
		"garbage",
		"Snapmaker@192.168.1.201|status:IDLE", // missing model
		"Snapmaker J1|model:Snapmaker J1|status:IDLE", // missing @
	}
	for _, c := range cases {
		if p, err := NewPrinter([]byte(c)); err == nil {
			t.Errorf("NewPrinter(%q) = %+v, want error", c, p)
		}
	}
}

func TestPrinterString(t *testing.T) {
	p := &Printer{ID: "J1V19", IP: "192.168.1.19", Model: "Snapmaker J1"}
	want := "J1V19@192.168.1.19 - Snapmaker J1"
	if got := p.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
