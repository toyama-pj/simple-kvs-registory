package semtech

import (
	"reflect"
	"testing"
)

func TestParseCayenneLPP(t *testing.T) {
	payload := []byte{
		1, 103, 0x00, 0xeb, // 23.5 C
		2, 104, 0x85, // 66.5 %
		3, 113, 0x03, 0xe8, 0xfc, 0x18, 0x00, 0x00, // 1, -1, 0 g
	}
	values, err := ParseCayenneLPP(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 3 {
		t.Fatalf("values = %d, want 3", len(values))
	}
	if values[0].Name != "temperature" || values[0].Value != 23.5 {
		t.Fatalf("temperature = %#v", values[0])
	}
	if values[1].Name != "relative_humidity" || values[1].Value != 66.5 {
		t.Fatalf("humidity = %#v", values[1])
	}
	wantVector := map[string]float64{"x": 1, "y": -1, "z": 0}
	if !reflect.DeepEqual(values[2].Value, wantVector) {
		t.Fatalf("accelerometer = %#v, want %#v", values[2].Value, wantVector)
	}
}

func TestParseCayenneLPPRejectsMalformedPayload(t *testing.T) {
	for _, payload := range [][]byte{{}, {1}, {1, 255, 0}, {1, 103, 0}} {
		if _, err := ParseCayenneLPP(payload); err == nil {
			t.Fatalf("payload %x was accepted", payload)
		}
	}
}
