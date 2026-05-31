package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

/* Discover discovers printers on the network. It returns a slice of
 * pointers to Printer objects. If no printers are found, it returns
 * an empty slice. If an error occurs, it returns nil.
 */
func Discover(timeout time.Duration) ([]*Printer, error) {
	var (
		mu       = sync.Mutex{}
		printers = []*Printer{}
		wg       sync.WaitGroup
	)

	// UDP broadcast discovery (Artisan/J1)
	addrs, err := getBroadcastAddresses()
	if err != nil {
		log.Printf("Error getting broadcast addresses: %v", err)
	}

	for _, addr := range addrs {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			results, err := discoverUDP(addr, timeout)
			if err != nil {
				if Debug {
					log.Printf("Error discovering UDP on %s: %v", addr, err)
				}
				return
			}
			mu.Lock()
			printers = append(printers, results...)
			mu.Unlock()
		}(addr)
	}

	// mDNS discovery (U1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		results := discoverMDNS(timeout)
		mu.Lock()
		printers = append(printers, results...)
		mu.Unlock()
	}()

	wg.Wait()
	return printers, nil
}

func discoverUDP(addr string, timeout time.Duration) ([]*Printer, error) {
	printers := []*Printer{}

	broadcastAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", addr, 20054))
	if err != nil {
		return printers, err
	}

	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return printers, err
	}
	defer conn.Close()

	if Debug {
		log.Printf("-- Discovering UDP on %s", broadcastAddr)
	}

	conn.SetDeadline(time.Now().Add(timeout))

	_, err = conn.WriteTo([]byte("discover"), broadcastAddr)
	if err != nil {
		return printers, err
	}

	buf := make([]byte, 1500)

	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Timeout() {
				break
			}
			return printers, err
		}

		if Debug {
			log.Printf("-- Discover UDP got %d bytes %s", n, buf[:n])
		}

		printer, err := NewPrinter(buf[:n])
		if err != nil {
			continue
		}
		printers = append(printers, printer)
	}
	return printers, nil
}

// discoverMDNS listens on the mDNS multicast address for Snapmaker devices.
// It parses both standard and non-standard (e.g. empty NSEC block) responses.
func discoverMDNS(timeout time.Duration) []*Printer {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ch := make(chan *Printer, 8)
	go sniffRawMDNS(ctx, ch)

	printers := []*Printer{}
	seen := map[string]bool{}

	for printer := range ch {
		key := printer.IP + "/" + printer.ID
		if seen[key] {
			continue
		}
		seen[key] = true

		if Debug {
			log.Printf("-- Discover mDNS: %s @ %s (%s)", printer.ID, printer.IP, printer.Model)
		}

		printers = append(printers, printer)
	}

	return printers
}

func getBroadcastAddresses() ([]string, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	addrMap := map[string]bool{}
	for _, iface := range ifs {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if n, ok := addr.(*net.IPNet); ok && !n.IP.IsLoopback() {
				if v4addr := n.IP.To4(); v4addr != nil {
					// convert all parts of the masked bits to its maximum value
					// by converting the address into a 32 bit integer and then
					// ORing it with the inverted mask
					baddr := make(net.IP, len(v4addr))
					binary.BigEndian.PutUint32(baddr, binary.BigEndian.Uint32(v4addr)|^binary.BigEndian.Uint32(n.IP.DefaultMask()))
					if s := baddr.String(); !addrMap[s] {
						addrMap[s] = true
					}
				}
			}
		}
	}

	addrs := make([]string, 0, len(addrMap))
	for addr := range addrMap {
		addrs = append(addrs, addr)
	}
	return addrs, nil
}

// sniffRawMDNS listens on the mDNS multicast address and extracts
// Snapmaker device info from raw mDNS response packets.
// Handles both well-formed responses and non-standard ones (e.g. empty NSEC block).
func sniffRawMDNS(ctx context.Context, ch chan<- *Printer) {
	defer close(ch)

	addr, err := net.ResolveUDPAddr("udp", "224.0.0.251:5353")
	if err != nil {
		return
	}

	conn, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		return
	}
	defer conn.Close()

	buf := make([]byte, 65536)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Timeout() {
				continue
			}
			return
		}

		msg := new(dns.Msg)
		if err := msg.Unpack(buf[:n]); err != nil {
			if strings.Contains(err.Error(), "empty NSEC(3) block") {
				if Debug {
					log.Printf("[DEBUG] mdns: non-standard NSEC record from %s, %d bytes:\n%s",
						src, n, hex.Dump(buf[:n]))
				}
				if printer := parseSnapmakerRawMDNS(buf[:n], src.IP.String()); printer != nil {
					select {
					case ch <- printer:
					case <-ctx.Done():
						return
					}
				}
			}
			continue
		}

		// Well-formed packet: extract Snapmaker devices from TXT records
		if printer := extractPrinterFromMDNS(msg, src.IP.String()); printer != nil {
			select {
			case ch <- printer:
			case <-ctx.Done():
				return
			}
		}
	}
}

// extractPrinterFromMDNS extracts a Printer from a well-formed mDNS response
// by looking for PTR answers pointing to _snapmaker._tcp.local and their
// associated TXT records.
func extractPrinterFromMDNS(msg *dns.Msg, srcIP string) *Printer {
	// Collect TXT records keyed by name (lowercased, trailing dot stripped)
	txtRecords := map[string][]string{}
	for _, rr := range msg.Extra {
		if txt, ok := rr.(*dns.TXT); ok {
			name := strings.TrimSuffix(strings.ToLower(txt.Hdr.Name), ".")
			txtRecords[name] = txt.Txt
		}
	}
	if len(txtRecords) == 0 {
		return nil
	}

	// Look for PTR answers pointing to _snapmaker._tcp.local
	for _, rr := range msg.Answer {
		ptr, ok := rr.(*dns.PTR)
		if !ok {
			continue
		}
		if strings.ToLower(ptr.Hdr.Name) != "_snapmaker._tcp.local." {
			continue
		}
		target := strings.TrimSuffix(strings.ToLower(ptr.Ptr), ".")
		if txts, ok := txtRecords[target]; ok {
			if printer := buildPrinterFromTXT(txts, srcIP); printer != nil {
				return printer
			}
		}
	}

	return nil
}

// parseSnapmakerRawMDNS attempts to manually extract Snapmaker device info
// from a raw mDNS response packet. It looks for TXT records containing
// key=value pairs like machine_type=, device_name=, sn=, ip= etc.
// Returns nil if insufficient info is found.
func parseSnapmakerRawMDNS(raw []byte, srcIP string) *Printer {
	if len(raw) < 12 {
		return nil
	}

	// Skip DNS header (12 bytes)
	off := 12

	// Skip Question section — count from header
	qdcount := int(raw[4])<<8 | int(raw[5])
	for i := 0; i < qdcount; i++ {
		off = skipDNSName(raw, off)
		off += 4 // QTYPE + QCLASS
	}

	// Skip Answer section
	ancount := int(raw[6])<<8 | int(raw[7])
	for i := 0; i < ancount; i++ {
		off = skipDNSName(raw, off)
		off += 10 // TYPE+CLASS+TTL+RDLENGTH
		rdlen := int(raw[off-2])<<8 | int(raw[off-1])
		off += rdlen
	}

	// Skip Authority (NSEC) section — these are the broken records
	nscount := int(raw[8])<<8 | int(raw[9])
	for i := 0; i < nscount; i++ {
		off = skipDNSName(raw, off)
		if off+10 > len(raw) {
			return nil
		}
		off += 8 // TYPE+CLASS+TTL
		rdlen := int(raw[off])<<8 | int(raw[off+1])
		off += 2
		off += rdlen
	}

	// Parse Additional section (contains TXT records)
	arcount := int(raw[10])<<8 | int(raw[11])
	for i := 0; i < arcount; i++ {
		off = skipDNSName(raw, off)
		if off+10 > len(raw) {
			return nil
		}
		rtype := int(raw[off])<<8 | int(raw[off+1])
		off += 8 // TYPE+CLASS+TTL
		rdlen := int(raw[off])<<8 | int(raw[off+1])
		off += 2
		if off+rdlen > len(raw) {
			return nil
		}

		if rtype == int(dns.TypeTXT) {
			txts := parseRawTXT(raw[off : off+rdlen])
			printer := buildPrinterFromTXT(txts, srcIP)
			off += rdlen
			if printer != nil {
				return printer
			}
		} else {
			off += rdlen
		}
	}

	return nil
}

// parseRawTXT parses raw DNS TXT RDATA into key=value strings.
// TXT RDATA is a sequence of length-prefixed strings.
func parseRawTXT(rdata []byte) []string {
	var txts []string
	pos := 0
	for pos < len(rdata) {
		length := int(rdata[pos])
		pos++
		if pos+length > len(rdata) {
			break
		}
		txts = append(txts, string(rdata[pos:pos+length]))
		pos += length
	}
	return txts
}

// buildPrinterFromTXT extracts device info from TXT key=value pairs.
func buildPrinterFromTXT(txts []string, srcIP string) *Printer {
	var (
		hostname     string
		model        string
		deviceIP     string
		hasSnapmaker bool
	)

	for _, t := range txts {
		kv := strings.SplitN(t, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := kv[0], kv[1]
		switch key {
		case "device_name":
			hostname = val
		case "machine_type":
			model = val
			if strings.Contains(strings.ToLower(val), "snapmaker") {
				hasSnapmaker = true
			}
		case "ip":
			deviceIP = val
		case "sn":
			// Serial number can serve as fallback ID
			if hostname == "" {
				hostname = val
			}
		case "version":
			/***
			version=1.3.0
			machine_type=Snapmaker U1
			device_name=U1
			sn=811XXXXXXXXXXXXXXXX
			link_mode=wan
			userid=
			ip=192.168.1.88
			region=cn
			*/

		}
	}

	// Must at least identify as a Snapmaker device
	if !hasSnapmaker {
		return nil
	}

	// Use device's self-reported IP; fall back to source address
	ip := deviceIP
	if ip == "" {
		ip = srcIP
	}

	// Fallback hostname
	if hostname == "" {
		hostname = "unknown"
	}

	// Fallback model
	if model == "" {
		model = "Snapmaker"
	}

	return &Printer{
		IP:        ip,
		ID:        hostname,
		Model:     model,
		Token:     "",
		Sacp:      false,
		Moonraker: true,
	}
}

// skipDNSName advances past a DNS-encoded name in raw bytes.
// Handles both label sequences and compression pointers.
func skipDNSName(raw []byte, off int) int {
	for off < len(raw) {
		length := int(raw[off])
		if length == 0 {
			return off + 1 // root label
		}
		if length&0xC0 == 0xC0 {
			return off + 2 // compression pointer
		}
		off += 1 + length
	}
	return off
}
