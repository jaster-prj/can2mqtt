package main

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/brutella/can"
)

func TestUDPTransportExchangesNativeSimFrame(t *testing.T) {
	simulator, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer simulator.Close()

	transport := NewUDPTransport("127.0.0.1:0")
	if err := transport.Open(); err != nil {
		t.Fatal(err)
	}
	defer transport.Close()

	gateway := transport.conn.LocalAddr().(*net.UDPAddr)
	wire := make([]byte, udpCANFrameSize)
	binary.LittleEndian.PutUint32(wire[0:4], 0x123)
	wire[4] = 2
	wire[8], wire[9] = 0xaa, 0xbb
	if _, err := simulator.WriteToUDP(wire, gateway); err != nil {
		t.Fatal(err)
	}

	select {
	case frame := <-transport.ReadChan():
		if frame.ID != 0x123 || frame.Length != 2 || frame.Data[0] != 0xaa || frame.Data[1] != 0xbb {
			t.Fatalf("unexpected received frame: %+v", frame)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for received CAN frame")
	}

	outgoing := &can.Frame{ID: 0x456, Length: 1, Data: [can.MaxFrameDataLength]uint8{0x5a}}
	if err := transport.Write(outgoing); err != nil {
		t.Fatal(err)
	}
	_ = simulator.SetReadDeadline(time.Now().Add(time.Second))
	n, _, err := simulator.ReadFromUDP(wire)
	if err != nil {
		t.Fatal(err)
	}
	if n != udpCANFrameSize || binary.LittleEndian.Uint32(wire[:4]) != outgoing.ID || wire[4] != 1 || wire[8] != 0x5a {
		t.Fatalf("unexpected transmitted UDP CAN frame: size=%d bytes=%x", n, wire[:n])
	}
}

func TestUDPTransportRelaysFramesBetweenPeers(t *testing.T) {
	first, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	transport := NewUDPTransport("127.0.0.1:0")
	if err := transport.Open(); err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	gateway := transport.conn.LocalAddr().(*net.UDPAddr)

	frame := make([]byte, udpCANFrameSize)
	frame[4] = 1
	if _, err := first.WriteToUDP(frame, gateway); err != nil {
		t.Fatal(err)
	}
	select {
	case <-transport.ReadChan():
	case <-time.After(time.Second):
		t.Fatal("timed out learning first peer")
	}
	if _, err := second.WriteToUDP(frame, gateway); err != nil {
		t.Fatal(err)
	}
	select {
	case <-transport.ReadChan():
	case <-time.After(time.Second):
		t.Fatal("timed out learning second peer")
	}

	frame[0] = 0x34
	if _, err := first.WriteToUDP(frame, gateway); err != nil {
		t.Fatal(err)
	}
	_ = second.SetReadDeadline(time.Now().Add(time.Second))
	received := make([]byte, udpCANFrameSize)
	n, _, err := second.ReadFromUDP(received)
	if err != nil {
		t.Fatal(err)
	}
	if n != udpCANFrameSize || received[0] != 0x34 {
		t.Fatalf("unexpected relayed datagram: size=%d bytes=%x", n, received[:n])
	}
}
