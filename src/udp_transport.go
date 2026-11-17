package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/brutella/can"
)

const (
	udpCANFrameSize = 72
	udpCANDataSize  = 64
)

// UDPTransport bridges native_sim's fixed-size UDP CAN frames to can2mqtt.
// A frame is encoded as little-endian ID, DLC, flags, two reserved bytes, and
// 64 data bytes, matching zephyr-native-linux-udp-can's can_udp_frame.
type UDPTransport struct {
	listen string
	conn   *net.UDPConn
	frames chan *can.Frame
	mu     sync.RWMutex
	peers  map[string]*net.UDPAddr
}

func NewUDPTransport(listen string) *UDPTransport {
	return &UDPTransport{
		listen: listen,
		frames: make(chan *can.Frame, 64),
		peers:  make(map[string]*net.UDPAddr),
	}
}

func (t *UDPTransport) Open() error {
	addr, err := net.ResolveUDPAddr("udp", t.listen)
	if err != nil {
		return fmt.Errorf("resolve UDP listen address: %w", err)
	}
	t.conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listen for UDP CAN frames: %w", err)
	}
	go t.receive()
	return nil
}

func (t *UDPTransport) Close() error {
	if t.conn == nil {
		return nil
	}
	return t.conn.Close()
}

func (t *UDPTransport) ReadChan() chan *can.Frame {
	return t.frames
}

func (t *UDPTransport) Write(frame *can.Frame) error {
	if t.conn == nil {
		return errors.New("UDP CAN transport is not open")
	}
	wire := make([]byte, udpCANFrameSize)
	binary.LittleEndian.PutUint32(wire[0:4], frame.ID)
	wire[4] = frame.Length
	wire[5] = frame.Flags
	copy(wire[8:8+len(frame.Data)], frame.Data[:])

	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.peers) == 0 {
		return errors.New("no UDP CAN peers have been discovered")
	}
	for _, peer := range t.peers {
		if _, err := t.conn.WriteToUDP(wire, peer); err != nil {
			return fmt.Errorf("send UDP CAN frame to %s: %w", peer, err)
		}
	}
	return nil
}

func (t *UDPTransport) receive() {
	defer close(t.frames)
	wire := make([]byte, 2048)
	for {
		n, peer, err := t.conn.ReadFromUDP(wire)
		if err != nil {
			return
		}
		if n != udpCANFrameSize || wire[4] > 8 {
			continue
		}
		t.mu.Lock()
		t.peers[peer.String()] = peer
		peers := make([]*net.UDPAddr, 0, len(t.peers))
		for key, address := range t.peers {
			if key != peer.String() {
				peers = append(peers, address)
			}
		}
		t.mu.Unlock()
		for _, destination := range peers {
			if _, err := t.conn.WriteToUDP(wire[:n], destination); err != nil {
				slog.Warn("canbus: failed to relay UDP CAN frame", "peer", destination, "error", err)
				continue
			}
		}

		frame := &can.Frame{
			ID:     binary.LittleEndian.Uint32(wire[0:4]),
			Length: wire[4],
			Flags:  wire[5],
		}
		copy(frame.Data[:], wire[8:16])
		t.frames <- frame
	}
}
