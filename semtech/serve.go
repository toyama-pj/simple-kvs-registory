package semtech

import (
	"encoding/json"
	"fmt"
	"log"
	"net"

	"github.com/toyama-pj/simple-kvs-registory/lib"
)

func Serve(config lib.Config) {
	address := fmt.Sprintf("%s:%d", config.SEMTECH_UDP_BIND_HOST, config.SEMTECH_UDP_BIND_PORT)
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		log.Fatalf("Failed to resolve UDP address: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on UDP address: %v", err)
	}
	defer conn.Close()

	log.Printf("Listening for Semtech UDP packets on %s", address)

	buffer := make([]byte, 4096)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading from UDP: %v", err)
			continue
		}

		packetData := make([]byte, n)
		copy(packetData, buffer[:n])

		go handlePacket(conn, packetData, remoteAddr)
	}
}

func handlePacket(conn *net.UDPConn, packet []byte, addr *net.UDPAddr) {
	if len(packet) < 4 {
		log.Printf("Packet too short from %s", addr)
		return
	}

	identifier := packet[3]

	switch identifier {
	case 0x00: // PUSH_DATA
		if err := handlePushData(conn, packet, addr); err != nil {
			log.Printf("Error handling PUSH_DATA: %v", err)
		}
	case 0x01: // PUSH_ACK
		// handlePushAck(packet, addr)
	case 0x02: // PULL_DATA
		// handlePullData(conn, packet, addr)
	case 0x03: // PULL_RESP
		// handlePullResp(packet, addr)
	case 0x04: // PULL_ACK
		// handlePullAck(packet, addr)
	case 0x05: // TX_ACK
		// handleTxAck(packet, addr)
	default:
		log.Printf("Unknown packet type: %x", identifier)
	}
}

func handlePushData(conn *net.UDPConn, packet []byte, addr *net.UDPAddr) error {
	if len(packet) < 12 {
		return fmt.Errorf("packet too short for PUSH_DATA")
	}

	version := packet[0]
	randomToken := [2]byte{packet[1], packet[2]}
	pushDataId := packet[3] // 0x00

	// GatewayEUI を安全にコピー
	var gatewayEUI GatewayEUI // GatewayEUI型は [8]byte として定義されている前提
	copy(gatewayEUI[:], packet[4:12])

	payload := packet[12:]

	semtechPacket := SemtechPushDataPacket{
		Version:     version,
		RandomToken: randomToken,
		Identifier:  pushDataId,
		GatewayEUI:  gatewayEUI,
		Payload:     make(json.RawMessage, len(payload)),
	}
	copy(semtechPacket.Payload, payload)

	log.Printf("Received PUSH_DATA from gateway %x with payload: %s", gatewayEUI, string(payload))

	err := pushAck(conn, addr, version, randomToken)
	if err != nil {
		return fmt.Errorf("failed to send PUSH_ACK: %w", err)
	}

	return nil
}

func pushAck(conn *net.UDPConn, addr *net.UDPAddr, version byte, randomToken [2]byte) error {
	// PUSH_ACKパケットの生成
	// [Protocol Version, Random Token LSB, Random Token MSB, PUSH_ACK Identifier (0x01)]
	ackPacket := []byte{
		version,
		randomToken[0],
		randomToken[1],
		0x01, // PUSH_ACK は 0x01 固定
	}

	_, err := conn.WriteToUDP(ackPacket, addr)
	if err != nil {
		return fmt.Errorf("failed to send PUSH_ACK packet: %w", err)
	}

	log.Printf("Sent PUSH_ACK to %s with token %x", addr, randomToken)
	return nil
}
