package main

import (
	"log"
	"net"
	"time"
)

/* Discover discovers printers on the network. It returns a slice of
 * pointers to Printer objects. If no printers are found, it returns
 * an empty slice. If an error occurs, it returns nil.
 */
func Discover(timeout time.Duration) ([]*Printer, error) {
	// Create a new UDP broadcast address
	broadcastAddr, err := net.ResolveUDPAddr("udp4", "255.255.255.255:20054")
	if err != nil {
		return nil, err
	}

	// Create a new UDP connection
	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Set a timeout for the connection
	conn.SetDeadline(time.Now().Add(timeout))

	// Send the discover message
	_, err = conn.WriteTo([]byte("discover"), broadcastAddr)
	if err != nil {
		return nil, err
	}

	// Create a buffer to hold the response
	buf := make([]byte, 1500)

	// Create a slice to hold the printers
	printers := []*Printer{}

	// Loop until the timeout is reached
	for {
		// Read the response
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			// If the error is a timeout, break out of the loop
			if err, ok := err.(net.Error); ok && err.Timeout() {
				break
			}
			return nil, err
		}

		if Debug {
			log.Printf("-- Discover got %d bytes %s", n, buf[:n])
		}

		// Parse the response into a Printer object
		printer, err := NewPrinter(buf[:n])
		if err != nil {
			continue
		}

		// Add the printer to the slice
		printers = append(printers, printer)
	}

	// Return the slice of printers
	return printers, nil
}
