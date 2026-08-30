package semtech

import (
	"crypto/aes"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	ErrDuplicateUplink = errors.New("duplicate uplink frame")
	ErrFrameCounter    = errors.New("invalid uplink frame counter")
)

type LoRaWANUplink struct {
	DevAddr          string
	DevAddrWire      [4]byte
	FrameCounterLow  uint16
	FPort            uint8
	EncryptedPayload []byte
	Message          []byte
	MIC              [4]byte
}

func ParseLoRaWANUplink(packet []byte) (LoRaWANUplink, error) {
	if len(packet) < 13 {
		return LoRaWANUplink{}, errors.New("LoRaWAN PHYPayload is too short")
	}
	mtype := packet[0] >> 5
	if mtype != 2 && mtype != 4 {
		return LoRaWANUplink{}, fmt.Errorf("LoRaWAN message type %d is not a data uplink", mtype)
	}
	message := packet[:len(packet)-4]
	macPayload := message[1:]
	foptsLen := int(macPayload[4] & 0x0f)
	fportOffset := 7 + foptsLen
	if len(macPayload) <= fportOffset {
		return LoRaWANUplink{}, errors.New("LoRaWAN data uplink has no FPort or application payload")
	}
	var devAddrWire [4]byte
	copy(devAddrWire[:], macPayload[:4])
	var mic [4]byte
	copy(mic[:], packet[len(packet)-4:])
	return LoRaWANUplink{
		DevAddr:          fmt.Sprintf("%08X", binary.LittleEndian.Uint32(devAddrWire[:])),
		DevAddrWire:      devAddrWire,
		FrameCounterLow:  binary.LittleEndian.Uint16(macPayload[5:7]),
		FPort:            macPayload[fportOffset],
		EncryptedPayload: append([]byte(nil), macPayload[fportOffset+1:]...),
		Message:          append([]byte(nil), message...),
		MIC:              mic,
	}, nil
}

func ReconstructFrameCounter(last uint32, hasLast bool, low uint16) (uint32, error) {
	candidate := uint32(low)
	if !hasLast {
		return candidate, nil
	}
	candidate |= last & 0xffff0000
	if candidate+0x8000 < last {
		candidate += 0x10000
	}
	if candidate == last {
		return 0, ErrDuplicateUplink
	}
	if candidate < last || candidate-last > 16384 {
		return 0, ErrFrameCounter
	}
	return candidate, nil
}

func VerifyUplinkMIC(uplink LoRaWANUplink, frameCounter uint32, nwkSKey [16]byte) error {
	calculated, err := calculateUplinkMIC(uplink.Message, uplink.DevAddrWire, frameCounter, nwkSKey)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(calculated[:], uplink.MIC[:]) != 1 {
		return errors.New("LoRaWAN MIC verification failed")
	}
	return nil
}

func calculateUplinkMIC(message []byte, devAddr [4]byte, frameCounter uint32, nwkSKey [16]byte) ([4]byte, error) {
	var result [4]byte
	if len(message) > 255 {
		return result, errors.New("LoRaWAN message is too long")
	}
	b0 := make([]byte, 16)
	b0[0] = 0x49
	b0[5] = 0 // uplink direction
	copy(b0[6:10], devAddr[:])
	binary.LittleEndian.PutUint32(b0[10:14], frameCounter)
	b0[15] = byte(len(message))
	mac, err := aesCMAC(nwkSKey[:], append(b0, message...))
	if err != nil {
		return result, err
	}
	copy(result[:], mac[:4])
	return result, nil
}

func DecryptUplinkPayload(payload []byte, devAddr [4]byte, frameCounter uint32, key [16]byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	result := make([]byte, len(payload))
	for offset, blockNumber := 0, byte(1); offset < len(payload); offset, blockNumber = offset+aes.BlockSize, blockNumber+1 {
		a := make([]byte, aes.BlockSize)
		a[0] = 0x01
		a[5] = 0 // uplink direction
		copy(a[6:10], devAddr[:])
		binary.LittleEndian.PutUint32(a[10:14], frameCounter)
		a[15] = blockNumber
		s := make([]byte, aes.BlockSize)
		block.Encrypt(s, a)
		end := min(offset+aes.BlockSize, len(payload))
		for i := offset; i < end; i++ {
			result[i] = payload[i] ^ s[i-offset]
		}
	}
	return result, nil
}

func aesCMAC(key, message []byte) ([16]byte, error) {
	var result [16]byte
	block, err := aes.NewCipher(key)
	if err != nil {
		return result, err
	}
	zero := make([]byte, 16)
	l := make([]byte, 16)
	block.Encrypt(l, zero)
	k1 := cmacSubkey(l)
	k2 := cmacSubkey(k1)

	blockCount := (len(message) + 15) / 16
	complete := len(message) > 0 && len(message)%16 == 0
	if blockCount == 0 {
		blockCount = 1
	}
	last := make([]byte, 16)
	lastOffset := (blockCount - 1) * 16
	if complete {
		copy(last, message[lastOffset:lastOffset+16])
		xorBlock(last, k1)
	} else {
		remaining := message[lastOffset:]
		copy(last, remaining)
		last[len(remaining)] = 0x80
		xorBlock(last, k2)
	}

	x := make([]byte, 16)
	y := make([]byte, 16)
	for i := 0; i < blockCount-1; i++ {
		copy(y, message[i*16:i*16+16])
		xorBlock(y, x)
		block.Encrypt(x, y)
	}
	copy(y, last)
	xorBlock(y, x)
	block.Encrypt(result[:], y)
	return result, nil
}

func cmacSubkey(input []byte) []byte {
	result := make([]byte, 16)
	carry := byte(0)
	for i := 15; i >= 0; i-- {
		result[i] = input[i]<<1 | carry
		carry = input[i] >> 7
	}
	if input[0]&0x80 != 0 {
		result[15] ^= 0x87
	}
	return result
}

func xorBlock(target, value []byte) {
	for i := range target {
		target[i] ^= value[i]
	}
}
