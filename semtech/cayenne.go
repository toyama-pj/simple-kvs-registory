package semtech

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type CayenneValue struct {
	Channel uint8
	Type    uint8
	Name    string
	Value   any
}

type cayenneType struct {
	name   string
	length int
	decode func([]byte) any
}

var cayenneTypes = map[uint8]cayenneType{
	0:   {name: "digital_input", length: 1, decode: func(v []byte) any { return v[0] }},
	1:   {name: "digital_output", length: 1, decode: func(v []byte) any { return v[0] }},
	2:   {name: "analog_input", length: 2, decode: func(v []byte) any { return float64(int16(binary.BigEndian.Uint16(v))) / 100 }},
	3:   {name: "analog_output", length: 2, decode: func(v []byte) any { return float64(int16(binary.BigEndian.Uint16(v))) / 100 }},
	101: {name: "illuminance", length: 2, decode: func(v []byte) any { return binary.BigEndian.Uint16(v) }},
	102: {name: "presence", length: 1, decode: func(v []byte) any { return v[0] != 0 }},
	103: {name: "temperature", length: 2, decode: func(v []byte) any { return float64(int16(binary.BigEndian.Uint16(v))) / 10 }},
	104: {name: "relative_humidity", length: 1, decode: func(v []byte) any { return float64(v[0]) / 2 }},
	113: {name: "accelerometer", length: 6, decode: decodeVector(1000, "x", "y", "z")},
	115: {name: "barometric_pressure", length: 2, decode: func(v []byte) any { return float64(binary.BigEndian.Uint16(v)) / 10 }},
	134: {name: "gyrometer", length: 6, decode: decodeVector(100, "x", "y", "z")},
	136: {name: "gps", length: 9, decode: func(v []byte) any {
		return map[string]float64{
			"latitude":  float64(signed24(v[0:3])) / 10000,
			"longitude": float64(signed24(v[3:6])) / 10000,
			"altitude":  float64(signed24(v[6:9])) / 100,
		}
	}},
}

func decodeVector(divisor float64, names ...string) func([]byte) any {
	return func(value []byte) any {
		result := make(map[string]float64, len(names))
		for i, name := range names {
			result[name] = float64(int16(binary.BigEndian.Uint16(value[i*2:i*2+2]))) / divisor
		}
		return result
	}
}

func signed24(value []byte) int32 {
	result := int32(value[0])<<16 | int32(value[1])<<8 | int32(value[2])
	if result&0x800000 != 0 {
		result |= ^int32(0xffffff)
	}
	return result
}

func ParseCayenneLPP(payload []byte) ([]CayenneValue, error) {
	if len(payload) == 0 {
		return nil, errors.New("Cayenne LPP payload is empty")
	}
	values := make([]CayenneValue, 0, len(payload)/3)
	for offset := 0; offset < len(payload); {
		if len(payload)-offset < 2 {
			return nil, fmt.Errorf("Cayenne LPP entry at byte %d has no type", offset)
		}
		channel, typeID := payload[offset], payload[offset+1]
		definition, ok := cayenneTypes[typeID]
		if !ok {
			return nil, fmt.Errorf("unsupported Cayenne LPP type %d on channel %d", typeID, channel)
		}
		offset += 2
		if len(payload)-offset < definition.length {
			return nil, fmt.Errorf("Cayenne LPP %s on channel %d needs %d value bytes", definition.name, channel, definition.length)
		}
		valueBytes := payload[offset : offset+definition.length]
		values = append(values, CayenneValue{Channel: channel, Type: typeID, Name: definition.name, Value: definition.decode(valueBytes)})
		offset += definition.length
	}
	return values, nil
}
