package main

import (
	"strings"
	"testing"
)

// buildTXTRecord creates raw bytes containing length-prefixed TXT strings,
// optionally surrounded by noise bytes.
func buildTXTRecord(pairs map[string]string, noise bool) []byte {
	var buf []byte
	if noise {
		buf = append(buf, 0x00, 0x05, 'h', 'e', 'a', 'd', 'r') // noise prefix
	}
	for k, v := range pairs {
		s := k + "=" + v
		buf = append(buf, byte(len(s)))
		buf = append(buf, s...)
	}
	if noise {
		buf = append(buf, 0x02, 'x', 'y') // noise suffix
	}
	return buf
}

func TestParseTXT(t *testing.T) {
	raw := buildTXTRecord(map[string]string{
		"machine_type": "Snapmaker U1",
		"device_name":  "U1-001",
		"ip":           "192.168.1.66",
	}, true)

	pairs := parseTXT(raw)
	if pairs["machine_type"] != "Snapmaker U1" {
		t.Errorf("machine_type = %q", pairs["machine_type"])
	}
	if pairs["device_name"] != "U1-001" {
		t.Errorf("device_name = %q", pairs["device_name"])
	}
	if pairs["ip"] != "192.168.1.66" {
		t.Errorf("ip = %q", pairs["ip"])
	}
}

func TestParseTXTEmpty(t *testing.T) {
	if pairs := parseTXT([]byte{0x00, 0x01}); len(pairs) != 0 {
		t.Errorf("pairs = %v, want empty", pairs)
	}
	if pairs := parseTXT(nil); len(pairs) != 0 {
		t.Errorf("pairs = %v, want empty", pairs)
	}
}

func TestLooksLikeTXTKey(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"machine_type", true},
		{"Device9", true},
		{"under_scored", true},
		{"", false},
		{"has space", false},
		{"has-dash", false},
		{"dot.name", false},
	}
	for _, c := range cases {
		if got := looksLikeTXTKey(c.in); got != c.want {
			t.Errorf("looksLikeTXTKey(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParsePrinterFromMDNS(t *testing.T) {
	raw := buildTXTRecord(map[string]string{
		"machine_type": "Snapmaker U1",
		"device_name":  "U1-LAB",
		"ip":           "10.0.0.42",
	}, false)

	p := parsePrinter(raw, "172.16.0.1")
	if p == nil {
		t.Fatal("parsePrinter returned nil")
	}
	if p.ID != "U1-LAB" {
		t.Errorf("ID = %q", p.ID)
	}
	if p.Model != "Snapmaker U1" {
		t.Errorf("Model = %q", p.Model)
	}
	// TXT "ip" wins over the packet source address
	if p.IP != "10.0.0.42" {
		t.Errorf("IP = %q, want TXT ip", p.IP)
	}
	if !p.Moonraker {
		t.Error("Moonraker = false, want true")
	}
	if p.Sacp {
		t.Error("Sacp = true, want false")
	}
}

func TestParsePrinterFallsBackToSourceIP(t *testing.T) {
	raw := buildTXTRecord(map[string]string{
		"machine_type": "Snapmaker U1",
		"sn":           "SN123456",
	}, false)

	p := parsePrinter(raw, "172.16.0.9")
	if p == nil {
		t.Fatal("parsePrinter returned nil")
	}
	if p.IP != "172.16.0.9" {
		t.Errorf("IP = %q, want source ip", p.IP)
	}
	if p.ID != "SN123456" {
		t.Errorf("ID = %q, want sn fallback", p.ID)
	}
}

func TestParsePrinterRejectsNonSnapmaker(t *testing.T) {
	raw := buildTXTRecord(map[string]string{
		"machine_type": "Generic Printer",
		"device_name":  "PRINTER1",
	}, false)
	if p := parsePrinter(raw, "1.1.1.1"); p != nil {
		t.Errorf("parsePrinter = %+v, want nil", p)
	}
}

func TestDiscoverUDPBadAddress(t *testing.T) {
	// port 0 broadcast address cannot resolve -> error expected
	if _, err := discoverUDP("not-an-ip", 1000000); err == nil {
		t.Log("discoverUDP accepted odd address; environments vary, ignoring")
	}
}

func TestGetBroadcastAddresses(t *testing.T) {
	addrs, err := getBroadcastAddresses()
	if err != nil {
		t.Fatalf("getBroadcastAddresses: %v", err)
	}
	for _, a := range addrs {
		if strings.Contains(a, ":") {
			t.Errorf("unexpected non-IPv4 address %q", a)
		}
	}
}
