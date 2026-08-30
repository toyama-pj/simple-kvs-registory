package semtech

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/toyama-pj/simple-kvs-registory/lib"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxUDPPacketSize = 65535

type Server struct {
	db     *gorm.DB
	config lib.Config
	now    func() time.Time
}

func NewServer(db *gorm.DB, config lib.Config) *Server {
	return &Server{db: db, config: config, now: time.Now}
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	address := net.JoinHostPort(s.config.SEMTECH_UDP_BIND_HOST, strconv.Itoa(s.config.SEMTECH_UDP_BIND_PORT))
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return fmt.Errorf("resolve Semtech UDP address: %w", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listen for Semtech UDP packets: %w", err)
	}
	defer conn.Close()

	log.Printf("Listening for Semtech UDP packets on %s", address)
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	var handlers sync.WaitGroup
	concurrency := make(chan struct{}, 64)
	defer handlers.Wait()
	for {
		buffer := make([]byte, maxUDPPacketSize)
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("read Semtech UDP packet: %w", err)
		}
		packet := append([]byte(nil), buffer[:n]...)
		concurrency <- struct{}{}
		handlers.Add(1)
		go func(packet []byte, remoteAddr *net.UDPAddr) {
			defer handlers.Done()
			defer func() { <-concurrency }()
			s.handleDatagram(packet, remoteAddr, func(ack []byte) error {
				_, writeErr := conn.WriteToUDP(ack, remoteAddr)
				return writeErr
			})
		}(packet, remoteAddr)
	}
}

func (s *Server) handleDatagram(packet []byte, addr *net.UDPAddr, send func([]byte) error) {
	receivedAt := s.now().UTC()
	entry := lib.SemtechUDPLog{
		ReceivedAt: receivedAt,
		SourceIP:   addr.IP.String(),
		PacketType: packetTypeName(packet),
		Payload:    packetLogPayload(packet),
	}
	defer func() {
		if err := s.db.Create(&entry).Error; err != nil {
			log.Printf("Failed to save Semtech UDP log from %s: %v", addr, err)
		}
	}()

	if len(packet) < 4 {
		entry.Error = "Semtech UDP packet is shorter than its 4-byte header"
		return
	}
	if packet[0] != 1 && packet[0] != 2 {
		entry.Error = fmt.Sprintf("unsupported Semtech UDP protocol version %d", packet[0])
		return
	}
	token := packet[1:3]
	switch packet[3] {
	case IdentifierPushData:
		if len(packet) < 12 {
			entry.Error = "PUSH_DATA packet is shorter than its 12-byte header"
			return
		}
		entry.GatewayEUI = strings.ToUpper(hex.EncodeToString(packet[4:12]))
		if err := send([]byte{packet[0], token[0], token[1], IdentifierPushAck}); err != nil {
			entry.Error = fmt.Sprintf("send PUSH_ACK: %v", err)
			return
		}
		committed, err := s.processPushPayload(packet[12:], entry.GatewayEUI, receivedAt)
		entry.DatabaseCommitted = committed
		if err != nil {
			entry.Error = err.Error()
		}
	case IdentifierPullData:
		if len(packet) < 12 {
			entry.Error = "PULL_DATA packet is shorter than its 12-byte header"
			return
		}
		entry.GatewayEUI = strings.ToUpper(hex.EncodeToString(packet[4:12]))
		if err := send([]byte{packet[0], token[0], token[1], IdentifierPullAck}); err != nil {
			entry.Error = fmt.Sprintf("send PULL_ACK: %v", err)
		}
	case IdentifierTxAck:
		if len(packet) < 12 {
			entry.Error = "TX_ACK packet is shorter than its 12-byte header"
			return
		}
		entry.GatewayEUI = strings.ToUpper(hex.EncodeToString(packet[4:12]))
	case IdentifierPushAck, IdentifierPullResp, IdentifierPullAck:
		// These are normally server-to-gateway messages. They are intentionally
		// retained in the audit log when received on the server socket.
	default:
		entry.Error = fmt.Sprintf("unknown Semtech UDP packet identifier 0x%02x", packet[3])
	}
}

func (s *Server) processPushPayload(raw []byte, gatewayEUI string, receivedAt time.Time) (bool, error) {
	var payload PushDataPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false, fmt.Errorf("parse PUSH_DATA JSON: %w", err)
	}
	if len(payload.RxPackets) == 0 {
		return false, nil
	}

	committed := false
	errorsFound := make([]error, 0)
	for index, rxPacket := range payload.RxPackets {
		if rxPacket.Stat < 0 {
			errorsFound = append(errorsFound, fmt.Errorf("rxpk[%d]: gateway reported invalid CRC", index))
			continue
		}
		phyPayload, err := decodeBase64(rxPacket.Data)
		if err != nil {
			errorsFound = append(errorsFound, fmt.Errorf("rxpk[%d]: decode data: %w", index, err))
			continue
		}
		if rxPacket.Size > 0 && rxPacket.Size != len(phyPayload) {
			errorsFound = append(errorsFound, fmt.Errorf("rxpk[%d]: size is %d but data contains %d bytes", index, rxPacket.Size, len(phyPayload)))
			continue
		}
		if err := s.storeUplink(phyPayload, rxPacket, gatewayEUI, receivedAt); err != nil {
			errorsFound = append(errorsFound, fmt.Errorf("rxpk[%d]: %w", index, err))
			continue
		}
		committed = true
	}
	return committed, errors.Join(errorsFound...)
}

func (s *Server) storeUplink(phyPayload []byte, rxPacket RxPacket, gatewayEUI string, receivedAt time.Time) error {
	uplink, err := ParseLoRaWANUplink(phyPayload)
	if err != nil {
		return err
	}
	if uplink.FPort == 0 {
		return errors.New("FPort 0 contains MAC commands, not Cayenne LPP application data")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var device lib.Device
		query := tx.Where("dev_addr = ? AND enabled = ? AND deleted_at IS NULL", uplink.DevAddr, true)
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&device).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("no enabled device is registered for DevAddr %s", uplink.DevAddr)
			}
			return err
		}
		frameCounter, err := ReconstructFrameCounter(device.UplinkFrameCounter, device.HasUplinkFrame, uplink.FrameCounterLow)
		if err != nil {
			return err
		}
		nwkSKey, err := lib.DecryptSessionKey(s.config.DEVICE_SESSION_KEY_ENCRYPTION_KEY, device.NwkSKeyEncrypted)
		if err != nil {
			return fmt.Errorf("load NwkSKey: %w", err)
		}
		if err := VerifyUplinkMIC(uplink, frameCounter, nwkSKey); err != nil {
			return err
		}
		appSKey, err := lib.DecryptSessionKey(s.config.DEVICE_SESSION_KEY_ENCRYPTION_KEY, device.AppSKeyEncrypted)
		if err != nil {
			return fmt.Errorf("load AppSKey: %w", err)
		}
		applicationPayload, err := DecryptUplinkPayload(uplink.EncryptedPayload, uplink.DevAddrWire, frameCounter, appSKey)
		if err != nil {
			return fmt.Errorf("decrypt application payload: %w", err)
		}
		values, err := ParseCayenneLPP(applicationPayload)
		if err != nil {
			return err
		}

		measurements := make([]lib.Measurement, 0, len(values))
		for _, value := range values {
			encoded, err := lib.NewJSONValue(value.Value)
			if err != nil {
				return fmt.Errorf("encode Cayenne LPP value: %w", err)
			}
			measurements = append(measurements, lib.Measurement{
				DeviceID:     device.ID,
				NamespaceID:  device.NamespaceID,
				GatewayEUI:   gatewayEUI,
				ReceivedAt:   receivedAt,
				GatewayTime:  rxPacket.Time,
				FrameCounter: frameCounter,
				Channel:      value.Channel,
				Type:         value.Type,
				Name:         value.Name,
				Value:        encoded,
			})
		}
		if err := tx.CreateInBatches(&measurements, 100).Error; err != nil {
			return err
		}
		return tx.Model(&lib.Device{}).Where("id = ?", device.ID).Updates(map[string]any{
			"uplink_frame_counter": frameCounter,
			"has_uplink_frame":     true,
		}).Error
	})
}

func decodeBase64(value string) ([]byte, error) {
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	return base64.RawStdEncoding.DecodeString(value)
}

func packetTypeName(packet []byte) string {
	if len(packet) < 4 {
		return "MALFORMED"
	}
	switch packet[3] {
	case IdentifierPushData:
		return "PUSH_DATA"
	case IdentifierPushAck:
		return "PUSH_ACK"
	case IdentifierPullData:
		return "PULL_DATA"
	case IdentifierPullResp:
		return "PULL_RESP"
	case IdentifierPullAck:
		return "PULL_ACK"
	case IdentifierTxAck:
		return "TX_ACK"
	default:
		return fmt.Sprintf("UNKNOWN_0x%02X", packet[3])
	}
}

func packetLogPayload(packet []byte) lib.JSONValue {
	offset := len(packet)
	if len(packet) >= 12 && (packet[3] == IdentifierPushData || packet[3] == IdentifierTxAck) {
		offset = 12
	} else if len(packet) >= 4 && packet[3] == IdentifierPullResp {
		offset = 4
	}
	if offset < len(packet) && json.Valid(packet[offset:]) {
		return append(lib.JSONValue(nil), packet[offset:]...)
	}
	wrapped, _ := lib.NewJSONValue(map[string]any{
		"raw_base64": base64.StdEncoding.EncodeToString(packet),
		"size":       len(packet),
	})
	return wrapped
}
