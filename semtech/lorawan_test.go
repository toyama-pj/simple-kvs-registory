package semtech

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/toyama-pj/simple-kvs-registory/lib"
	database "github.com/toyama-pj/simple-kvs-registory/lib/db"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const testMasterKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestAESCMACRFC4493EmptyMessage(t *testing.T) {
	key, _ := hex.DecodeString("2b7e151628aed2a6abf7158809cf4f3c")
	mac, err := aesCMAC(key, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(mac[:]); got != "bb1d6929e95937287fa37d129b756746" {
		t.Fatalf("CMAC = %s", got)
	}
}

func TestReconstructFrameCounter(t *testing.T) {
	if got, err := ReconstructFrameCounter(0, false, 5); err != nil || got != 5 {
		t.Fatalf("initial counter = %d, %v", got, err)
	}
	if got, err := ReconstructFrameCounter(0xffff, true, 0); err != nil || got != 0x10000 {
		t.Fatalf("rollover counter = %d, %v", got, err)
	}
	if _, err := ReconstructFrameCounter(7, true, 7); err != ErrDuplicateUplink {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestPushDataDecryptsCayenneAndCreatesAuditLog(t *testing.T) {
	db := semtechTestDB(t)
	appKeyHex := "00112233445566778899aabbccddeeff"
	nwkKeyHex := "ffeeddccbbaa99887766554433221100"
	appEncrypted, err := lib.EncryptSessionKey(testMasterKey, appKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	nwkEncrypted, err := lib.EncryptSessionKey(testMasterKey, nwkKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	organization := lib.Organization{Name: "test organization"}
	if err := db.Create(&organization).Error; err != nil {
		t.Fatal(err)
	}
	namespace := lib.Namespace{OrganizationID: organization.ID, Name: "test namespace"}
	if err := db.Create(&namespace).Error; err != nil {
		t.Fatal(err)
	}
	device := lib.Device{
		NamespaceID:      namespace.ID,
		Name:             "field sensor",
		DevEUI:           "0102030405060708",
		DevAddr:          "26011BDA",
		AppSKeyEncrypted: appEncrypted,
		NwkSKeyEncrypted: nwkEncrypted,
		Enabled:          true,
	}
	if err := db.Create(&device).Error; err != nil {
		t.Fatal(err)
	}

	phyPayload := buildUplink(t, device.DevAddr, 1, 1, []byte{1, 103, 0x00, 0xeb, 2, 104, 0x85}, appKeyHex, nwkKeyHex)
	gatewayTime := time.Date(2026, 8, 31, 1, 2, 3, 0, time.UTC)
	jsonPayload, err := json.Marshal(PushDataPayload{RxPackets: []RxPacket{{
		Time: &gatewayTime,
		Stat: 1,
		Size: len(phyPayload),
		Data: base64.StdEncoding.EncodeToString(phyPayload),
	}}})
	if err != nil {
		t.Fatal(err)
	}
	packet := append([]byte{2, 0xaa, 0xbb, IdentifierPushData, 1, 2, 3, 4, 5, 6, 7, 8}, jsonPayload...)

	receivedAt := time.Date(2026, 8, 31, 2, 3, 4, 0, time.UTC)
	server := NewServer(db, lib.Config{DEVICE_SESSION_KEY_ENCRYPTION_KEY: testMasterKey})
	server.now = func() time.Time { return receivedAt }
	var ack []byte
	server.handleDatagram(packet, &net.UDPAddr{IP: net.ParseIP("192.0.2.10"), Port: 1700}, func(value []byte) error {
		ack = append([]byte(nil), value...)
		return nil
	})
	if !bytes.Equal(ack, []byte{2, 0xaa, 0xbb, IdentifierPushAck}) {
		t.Fatalf("ACK = %x", ack)
	}

	var measurements []lib.Measurement
	if err := db.Order("channel").Find(&measurements).Error; err != nil {
		t.Fatal(err)
	}
	if len(measurements) != 2 {
		t.Fatalf("measurements = %d, want 2", len(measurements))
	}
	if measurements[0].DeviceID != device.ID || measurements[0].Name != "temperature" || string(measurements[0].Value) != "23.5" {
		t.Fatalf("temperature measurement = %#v", measurements[0])
	}
	if measurements[1].Name != "relative_humidity" || string(measurements[1].Value) != "66.5" {
		t.Fatalf("humidity measurement = %#v", measurements[1])
	}

	var audit lib.SemtechUDPLog
	if err := db.First(&audit).Error; err != nil {
		t.Fatal(err)
	}
	if !audit.DatabaseCommitted || audit.SourceIP != "192.0.2.10" || audit.GatewayEUI != "0102030405060708" || audit.Error != "" {
		t.Fatalf("audit log = %#v", audit)
	}

	// A repeated frame is acknowledged and audited, but not inserted twice.
	server.handleDatagram(packet, &net.UDPAddr{IP: net.ParseIP("192.0.2.10"), Port: 1700}, func([]byte) error { return nil })
	var measurementCount, auditCount int64
	if err := db.Model(&lib.Measurement{}).Count(&measurementCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&lib.SemtechUDPLog{}).Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if measurementCount != 2 || auditCount != 2 {
		t.Fatalf("after duplicate measurements=%d logs=%d", measurementCount, auditCount)
	}
	var duplicateLog lib.SemtechUDPLog
	if err := db.Where("database_committed = ?", false).Where("error <> ?", "").First(&duplicateLog).Error; err != nil {
		t.Fatal(err)
	}
	if duplicateLog.DatabaseCommitted || duplicateLog.Error == "" {
		t.Fatalf("duplicate log = %#v", duplicateLog)
	}
}

func TestNonPushPacketIsAcknowledgedAndLogged(t *testing.T) {
	db := semtechTestDB(t)
	server := NewServer(db, lib.Config{})
	packet := []byte{2, 1, 2, IdentifierPullData, 1, 2, 3, 4, 5, 6, 7, 8}
	var ack []byte
	server.handleDatagram(packet, &net.UDPAddr{IP: net.ParseIP("2001:db8::1")}, func(value []byte) error {
		ack = append([]byte(nil), value...)
		return nil
	})
	if !bytes.Equal(ack, []byte{2, 1, 2, IdentifierPullAck}) {
		t.Fatalf("ACK = %x", ack)
	}
	var audit lib.SemtechUDPLog
	if err := db.First(&audit).Error; err != nil {
		t.Fatal(err)
	}
	if audit.PacketType != "PULL_DATA" || audit.DatabaseCommitted || audit.SourceIP != "2001:db8::1" {
		t.Fatalf("audit log = %#v", audit)
	}
}

func semtechTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(database.GetDatabaseDialector("duckdb", ":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := lib.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func buildUplink(t *testing.T, devAddr string, frameCounter uint32, fport uint8, plaintext []byte, appKeyHex, nwkKeyHex string) []byte {
	t.Helper()
	parsedAddr, err := strconvParseHex32(devAddr)
	if err != nil {
		t.Fatal(err)
	}
	var devAddrWire [4]byte
	binary.LittleEndian.PutUint32(devAddrWire[:], parsedAddr)
	appKey, err := lib.ParseSessionKey(appKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	nwkKey, err := lib.ParseSessionKey(nwkKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := DecryptUplinkPayload(plaintext, devAddrWire, frameCounter, appKey)
	if err != nil {
		t.Fatal(err)
	}
	message := []byte{0x40}
	message = append(message, devAddrWire[:]...)
	message = append(message, 0)
	message = binary.LittleEndian.AppendUint16(message, uint16(frameCounter))
	message = append(message, fport)
	message = append(message, encrypted...)
	mic, err := calculateUplinkMIC(message, devAddrWire, frameCounter, nwkKey)
	if err != nil {
		t.Fatal(err)
	}
	return append(message, mic[:]...)
}

func strconvParseHex32(value string) (uint32, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return 0, err
	}
	if len(decoded) != 4 {
		return 0, errors.New("DevAddr must contain four bytes")
	}
	return binary.BigEndian.Uint32(decoded), nil
}
