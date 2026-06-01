package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

/* Discover discovers printers on the network. It returns a slice of
 * pointers to Printer objects. If no printers are found, it returns
 * an empty slice. If an error occurs, it returns nil.
 */
func Discover(timeout time.Duration) ([]*Printer, error) {
	var (
		mu       sync.Mutex
		printers []*Printer
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
	var printers []*Printer

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

	if _, err = conn.WriteTo([]byte("discover"), broadcastAddr); err != nil {
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

// discoverMDNS discovers Snapmaker devices via passive mDNS sniffing.
// A zeroconf Browse probe is started to trigger device responses.
func discoverMDNS(timeout time.Duration) []*Printer {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Probe: send active mDNS queries to trigger device responses.
	// We don't read the results — the passive sniffer catches them.
	go probeZeroconf(ctx)

	ch := sniffer(ctx)

	var printers []*Printer
	seen := map[string]bool{}
	for printer := range ch {
		key := printer.IP + "/" + printer.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		if Debug {
			log.Printf("-- Discover mDNS got: %s @ %s (%s)", printer.ID, printer.IP, printer.Model)
		}
		printers = append(printers, printer)
	}
	return printers
}

// probeZeroconf sends a zeroconf Browse query to trigger mDNS responses.
// The returned entries are discarded — the passive sniffer handles parsing.
func probeZeroconf(ctx context.Context) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		defer func() { recover() }()
		_ = resolver.Browse(ctx, "_snapmaker._tcp", "local.", entries)
	}()

	// Drain entries so the Browse goroutine doesn't block.
	for range entries {
	}
}

// sniffer starts a passive mDNS listener and returns a channel of discovered printers.
func sniffer(ctx context.Context) <-chan *Printer {
	ch := make(chan *Printer, 512)

	conn, err := net.ListenMulticastUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353})
	if err != nil {
		close(ch)
		return ch
	}

	go func() {
		defer conn.Close()
		defer close(ch)

		localIPs := localIPSet()
		buf := make([]byte, 1500)

		for {
			if err := conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
				return
			}

			n, src, err := conn.ReadFromUDP(buf)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}

			// Ignore own packets
			if localIPs[src.IP.String()] {
				continue
			}

			if !strings.Contains(strings.ToLower(string(buf[:n])), "snapmaker") {
				continue
			}

			if Debug {
				log.Printf("-- mDNS raw packet from %s (%d bytes): %q", src.IP, n, buf[:n])
			}

			p := parsePrinter(buf[:n], src.IP.String())
			if p == nil {
				continue
			}

			select {
			case ch <- p:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch
}

// parsePrinter extracts a Printer from a raw mDNS packet by parsing
// length-prefixed DNS TXT records.
func parsePrinter(raw []byte, srcIP string) *Printer {
	pairs := parseTXT(raw)
	if Debug {
		// log.Printf("-- mDNS TXT pairs: %v", pairs)
	}
	if len(pairs) == 0 {
		return nil
	}

	model := pairs["machine_type"]
	hostname := pairs["device_name"]
	deviceIP := pairs["ip"]

	if hostname == "" {
		hostname = pairs["sn"]
	}
	if !strings.Contains(strings.ToLower(model), "snapmaker") {
		return nil
	}
	if ip := deviceIP; ip != "" {
		srcIP = ip
	}
	if hostname == "" {
		hostname = "unknown"
	}

	return &Printer{
		IP:        srcIP,
		ID:        hostname,
		Model:     model,
		Token:     "",
		Sacp:      false,
		Moonraker: true,
	}
}

// parseTXT extracts key=value pairs from DNS TXT records in raw bytes.
// DNS TXT format: each string is a 1-byte length followed by the text.
func parseTXT(raw []byte) map[string]string {
	pairs := map[string]string{}
	for i := 0; i < len(raw)-2; i++ {
		length := int(raw[i])
		// Bounds: min 3 (e.g. "a=b"), max 120 to skip DNS header/RDLENGTH bytes.
		if length < 3 || length > 120 || i+1+length > len(raw) {
			continue
		}
		text := string(raw[i+1 : i+1+length])
		idx := strings.IndexByte(text, '=')
		if idx < 1 {
			continue
		}
		key := text[:idx]
		if looksLikeTXTKey(key) {
			pairs[key] = text[idx+1:]
		}
	}
	return pairs
}

// looksLikeTXTKey checks if a string looks like a TXT record key.
func looksLikeTXTKey(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// localIPSet returns all local IP addresses for filtering own mDNS packets.
func localIPSet() map[string]bool {
	ips := map[string]bool{}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			ips[ipnet.IP.String()] = true
		}
	}
	return ips
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
					baddr := make(net.IP, len(v4addr))
					binary.BigEndian.PutUint32(baddr, binary.BigEndian.Uint32(v4addr)|^binary.BigEndian.Uint32(n.IP.DefaultMask()))
					addrMap[baddr.String()] = true
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
