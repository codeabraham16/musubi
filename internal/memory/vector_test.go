package memory

import (
	"math"
	"testing"
)

func TestFloat32BytesRoundTrip(t *testing.T) {
	original := []float32{0, 1.5, -2.25, 3.0e10, math.MaxFloat32}

	b, err := Float32ToBytes(original)
	if err != nil {
		t.Fatalf("Float32ToBytes error: %v", err)
	}
	if len(b) != len(original)*4 {
		t.Fatalf("esperaba %d bytes, obtuve %d", len(original)*4, len(b))
	}

	back, err := BytesToFloat32(b)
	if err != nil {
		t.Fatalf("BytesToFloat32 error: %v", err)
	}
	if len(back) != len(original) {
		t.Fatalf("esperaba %d floats, obtuve %d", len(original), len(back))
	}
	for i := range original {
		if back[i] != original[i] {
			t.Errorf("índice %d: esperaba %v, obtuve %v", i, original[i], back[i])
		}
	}
}

func TestBytesToFloat32RejectsInvalidLength(t *testing.T) {
	if _, err := BytesToFloat32([]byte{0x00, 0x01, 0x02}); err == nil {
		t.Fatal("esperaba error por longitud no múltiplo de 4")
	}
}

func TestFloat32ToBytesLittleEndianLayout(t *testing.T) {
	// 1.0 en IEEE-754 float32 = 0x3F800000; little-endian => 00 00 80 3F
	b, err := Float32ToBytes([]float32{1.0})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	want := []byte{0x00, 0x00, 0x80, 0x3F}
	if len(b) != 4 {
		t.Fatalf("esperaba 4 bytes, obtuve %d", len(b))
	}
	for i := range want {
		if b[i] != want[i] {
			t.Errorf("byte %d: esperaba 0x%02X, obtuve 0x%02X", i, want[i], b[i])
		}
	}
}

func almostEqual(a, b, tol float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func TestCosineSimilarity(t *testing.T) {
	const tol = 1e-6

	tests := []struct {
		name    string
		a, b    []float32
		want    float32
		wantErr bool
	}{
		{"idénticos", []float32{1, 2, 3}, []float32{1, 2, 3}, 1.0, false},
		{"ortogonales", []float32{1, 0}, []float32{0, 1}, 0.0, false},
		{"opuestos", []float32{1, 2, 3}, []float32{-1, -2, -3}, -1.0, false},
		{"vector cero", []float32{0, 0}, []float32{1, 1}, 0.0, false},
		{"longitudes distintas", []float32{1, 2}, []float32{1, 2, 3}, 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CosineSimilarity(tc.a, tc.b)
			if tc.wantErr {
				if err == nil {
					t.Fatal("esperaba error, no hubo")
				}
				return
			}
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if !almostEqual(got, tc.want, tol) {
				t.Errorf("esperaba %v, obtuve %v", tc.want, got)
			}
		})
	}
}
