package main

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"net"
	"runtime"
	"testing"
	"time"
)

func encodePacket(receiver, sender, attribute byte, sequence uint16, cset, cid byte, data []byte) []byte {
	return SACP_pack{
		ReceiverID: receiver,
		SenderID:   sender,
		Attribute:  attribute,
		Sequence:   sequence,
		CommandSet: cset,
		CommandID:  cid,
		Data:       data,
	}.Encode()
}

func TestSACPPackEncodeDecodeRoundtrip(t *testing.T) {
	src := SACP_pack{
		ReceiverID: 2,
		SenderID:   0,
		Attribute:  1,
		Sequence:   1234,
		CommandSet: 0xb0,
		CommandID:  0x01,
		Data:       []byte("hello sacp packet data"),
	}
	encoded := src.Encode()

	var dst SACP_pack
	if err := dst.Decode(encoded); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if dst.ReceiverID != src.ReceiverID || dst.SenderID != src.SenderID ||
		dst.Attribute != src.Attribute || dst.Sequence != src.Sequence ||
		dst.CommandSet != src.CommandSet || dst.CommandID != src.CommandID {
		t.Errorf("header mismatch: %+v vs %+v", dst, src)
	}
	if !bytes.Equal(dst.Data, src.Data) {
		t.Errorf("Data = %q, want %q", dst.Data, src.Data)
	}
}

func TestSACPPackDecodeRejectsCorruption(t *testing.T) {
	src := SACP_pack{
		ReceiverID: 1, SenderID: 0, Sequence: 7,
		CommandSet: 0x01, CommandID: 0x02,
		Data: []byte{1, 2, 3, 4, 5},
	}
	encoded := src.Encode()

	// too short
	var p SACP_pack
	if err := p.Decode(encoded[:5]); err == nil {
		t.Error("short packet accepted")
	}

	// bad magic
	bad := append([]byte(nil), encoded...)
	bad[0] = 0x00
	if err := p.Decode(bad); err == nil {
		t.Error("bad magic accepted")
	}

	// bad version
	bad = append([]byte(nil), encoded...)
	bad[4] = 0x02
	if err := p.Decode(bad); err != nil {
		if err != errInvalidSACPVer {
			t.Errorf("want errInvalidSACPVer, got %v", err)
		}
	} else {
		t.Error("bad version accepted")
	}

	// corrupted header (breaks head checksum)
	bad = append([]byte(nil), encoded...)
	bad[5] ^= 0xff
	if err := p.Decode(bad); err != errInvalidChksum {
		t.Errorf("corrupt header: want errInvalidChksum, got %v", err)
	}

	// corrupted body (breaks data checksum)
	bad = append([]byte(nil), encoded...)
	bad[13] ^= 0xff
	if err := p.Decode(bad); err != errInvalidChksum {
		t.Errorf("corrupt body: want errInvalidChksum, got %v", err)
	}
}

// TestSACPReadFragmented is the regression test for the io.ReadFull fix:
// a packet delivered in small TCP-style fragments must still parse.
func TestSACPReadFragmented(t *testing.T) {
	client, printer := net.Pipe()
	defer client.Close()
	defer printer.Close()

	packet := encodePacket(0, 2, 1, 99, 0xb0, 0x01, bytes.Repeat([]byte{0xA5}, 300))

	go func() {
		for i := 0; i < len(packet); i += 7 {
			end := i + 7
			if end > len(packet) {
				end = len(packet)
			}
			printer.Write(packet[i:end])
			time.Sleep(2 * time.Millisecond)
		}
	}()

	p, err := SACP_read(client, 5*time.Second)
	if err != nil {
		t.Fatalf("SACP_read on fragmented packet: %v", err)
	}
	if p.CommandSet != 0xb0 || p.CommandID != 0x01 || p.Sequence != 99 {
		t.Errorf("parsed packet mismatch: cset=%x cid=%x seq=%d", p.CommandSet, p.CommandID, p.Sequence)
	}
	if len(p.Data) != 300 {
		t.Errorf("Data length = %d, want 300", len(p.Data))
	}
}

func TestSACPReadTimeout(t *testing.T) {
	client, printer := net.Pipe()
	defer client.Close()
	defer printer.Close()

	start := time.Now()
	if _, err := SACP_read(client, 200*time.Millisecond); err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed < 150*time.Millisecond {
		t.Errorf("returned too early: %v", elapsed)
	}
}

// fakeSACPPrinter emulates the printer side of an upload session over a
// net.Pipe: it acknowledges the begin packet, requests every chunk
// (out of order), verifies the reassembled content against the advertised
// MD5 and finally sends the finish packet.
func fakeSACPPrinter(t *testing.T, conn net.Conn, wantSize int64) {
	t.Helper()
	defer conn.Close()

	readString := func(data []byte, off int) (string, int) {
		n := binary.LittleEndian.Uint16(data[off : off+2])
		return string(data[off+2 : off+2+int(n)]), off + 2 + int(n)
	}

	// 1. begin packet
	p, err := SACP_read(conn, 10*time.Second)
	if err != nil {
		t.Errorf("printer read begin: %v", err)
		return
	}
	if p.CommandSet != 0xb0 || p.CommandID != 0x00 {
		t.Errorf("begin packet mismatch: cset=%x cid=%x", p.CommandSet, p.CommandID)
		return
	}
	filename, off := readString(p.Data, 0)
	size := int64(binary.LittleEndian.Uint32(p.Data[off : off+4]))
	count := binary.LittleEndian.Uint16(p.Data[off+4 : off+6])
	md5hex, _ := readString(p.Data, off+6)
	if size != wantSize {
		t.Errorf("advertised size = %d, want %d", size, wantSize)
	}
	_ = filename

	// 2. request every chunk, last package first to exercise random access
	h := md5.New()
	var assembled bytes.Buffer
	for i := int(count) - 1; i >= 0; i-- {
		data := bytes.Buffer{}
		md5len := make([]byte, 2)
		binary.LittleEndian.PutUint16(md5len, uint16(len(md5hex)))
		data.Write(md5len)
		data.WriteString(md5hex)
		pkg := make([]byte, 2)
		binary.LittleEndian.PutUint16(pkg, uint16(i))
		data.Write(pkg)

		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if _, err := conn.Write(encodePacket(0, 2, 1, uint16(i+100), 0xb0, 0x01, data.Bytes())); err != nil {
			t.Errorf("printer write request: %v", err)
			return
		}

		resp, err := SACP_read(conn, 10*time.Second)
		if err != nil {
			t.Errorf("printer read chunk %d: %v", i, err)
			return
		}
		if resp.CommandSet != 0xb0 || resp.CommandID != 0x01 {
			t.Errorf("chunk response mismatch: cset=%x cid=%x", resp.CommandSet, resp.CommandID)
			return
		}
		if resp.Data[0] != 0 {
			t.Errorf("chunk %d status = %d, want 0", i, resp.Data[0])
			return
		}
		chunkMd5, off := readString(resp.Data, 1)
		if chunkMd5 != md5hex {
			t.Errorf("chunk %d md5 = %q, want %q", i, chunkMd5, md5hex)
			return
		}
		pkgNo := binary.LittleEndian.Uint16(resp.Data[off : off+2])
		if int(pkgNo) != i {
			t.Errorf("chunk %d answered with pkg %d", i, pkgNo)
			return
		}
		chunk, _ := readString(resp.Data, off+2)
		if i == int(count)-1 {
			// last chunk goes first in this loop; length must fit
			if int64(len(chunk)) > wantSize-int64(SACP_data_len)*int64(count-1) {
				t.Errorf("last chunk too large: %d", len(chunk))
				return
			}
		}
		h.Write([]byte(chunk))
		assembled.WriteString(chunk)
	}

	// 3. verify content
	if got := hex.EncodeToString(h.Sum(nil)); got != md5hex {
		t.Errorf("md5 mismatch: got %s want %s", got, md5hex)
	}
	if int64(assembled.Len()) != wantSize {
		t.Errorf("assembled size = %d, want %d", assembled.Len(), wantSize)
	}

	// 4. finish packet
	if _, err := conn.Write(encodePacket(0, 2, 1, 999, 0xb0, 0x02, []byte{0})); err != nil {
		t.Errorf("printer write finish: %v", err)
		return
	}

	// 5. drain the disconnect packet so the uploader's write never blocks
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	buf := make([]byte, 128)
	for {
		if _, err := conn.Read(buf); err != nil {
			return
		}
	}
}

func TestSACPUploadReaderSpool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping upload test in short mode")
	}

	// 2.5 chunks: exercises full chunks and the shorter last chunk
	content := make([]byte, SACP_data_len*2+30720)
	for i := range content {
		content[i] = byte(i * 7)
	}

	client, printer := net.Pipe()
	defer client.Close()
	go fakeSACPPrinter(t, printer, int64(len(content)))

	// bounded-memory assertion: the spool path must not allocate anywhere
	// near the size of the transferred content
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	err := SACP_start_upload_reader(client, "test.gcode", bytes.NewReader(content), int64(len(content)), 20*time.Second)
	if err != nil {
		t.Fatalf("SACP_start_upload_reader: %v", err)
	}

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	alloc := after.TotalAlloc - before.TotalAlloc
	if limit := uint64(64 << 20); alloc > limit {
		t.Errorf("upload allocated %d bytes, limit %d (memory must stay bounded)",
			alloc, limit)
	}
}

func TestSACPUploadReaderEmptyFile(t *testing.T) {
	client, printer := net.Pipe()
	defer client.Close()
	go fakeSACPPrinter(t, printer, 0)

	if err := SACP_start_upload_reader(client, "empty.gcode", bytes.NewReader(nil), 0, 20*time.Second); err != nil {
		t.Fatalf("empty file upload: %v", err)
	}
}

func TestSACPUploadReaderSizeMismatch(t *testing.T) {
	client, printer := net.Pipe()
	defer client.Close()
	// uploader warns about the stale size hint (99) but transfers the
	// actual 6-byte content; the fake printer validates against 6
	go fakeSACPPrinter(t, printer, 6)

	content := []byte("abcdef")
	if err := SACP_start_upload_reader(client, "x.gcode", bytes.NewReader(content), 99, 20*time.Second); err != nil {
		t.Fatalf("upload with stale size hint should still succeed: %v", err)
	}
}
