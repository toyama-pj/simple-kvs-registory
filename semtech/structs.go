package semtech

import (
	"encoding/json"
	"time"
)

const (
	IdentifierPushData = 0x00
	IdentifierPushAck  = 0x01
	IdentifierPullData = 0x02
	IdentifierPullResp = 0x03
	IdentifierPullAck  = 0x04
	IdentifierTxAck    = 0x05
)

type PayloadData map[string]any
type GatewayEUI [8]byte

type SemtechPushDataPacket struct {
	Version     byte
	RandomToken [2]byte
	Identifier  byte // IdentifierPushData (0x00)
	GatewayEUI  GatewayEUI
	Payload     json.RawMessage
}

type SemtechPushAckPacket struct {
	Version     byte
	RandomToken [2]byte
	Identifier  byte // IdentifierPushAck (0x01)
}

type SemtechPullDataPacket struct {
	Version     byte
	RandomToken [2]byte
	Identifier  byte // IdentifierPullData (0x02)
	GatewayEUI  GatewayEUI
}

type SemtechPullAckPacket struct {
	Version     byte
	RandomToken [2]byte
	Identifier  byte // IdentifierPullAck (0x04)
}

type SemtechPullRespPacket struct {
	Version     byte
	RandomToken [2]byte
	Identifier  byte // IdentifierPullResp (0x03)
	Payload     json.RawMessage
}

type SemtechTxAckPacket struct {
	Version     byte
	RandomToken [2]byte
	Identifier  byte // IdentifierTxAck (0x05)
	GatewayEUI  GatewayEUI
	Payload     json.RawMessage
}

type PushDataPayload struct {
	RxPackets []RxPacket      `json:"rxpk"`
	Stat      json.RawMessage `json:"stat,omitempty"`
}

type RxPacket struct {
	Time *time.Time `json:"time,omitempty"`
	Stat int        `json:"stat"`
	Size int        `json:"size"`
	Data string     `json:"data"`
}
