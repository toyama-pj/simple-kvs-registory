package semtech

import "encoding/json"

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
