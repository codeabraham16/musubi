package memory

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
)

// Float32ToBytes convierte un slice de float32 a su representación en bytes binarios.
func Float32ToBytes(slice []float32) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, slice)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// BytesToFloat32 convierte un slice de bytes binarios de vuelta a []float32.
func BytesToFloat32(b []byte) ([]float32, error) {
	if len(b)%4 != 0 {
		return nil, errors.New("longitud de bytes inválida para float32")
	}
	length := len(b) / 4
	slice := make([]float32, length)
	reader := bytes.NewReader(b)
	err := binary.Read(reader, binary.LittleEndian, &slice)
	if err != nil {
		return nil, err
	}
	return slice, nil
}

// CosineSimilarity calcula la similitud de coseno entre dos vectores A y B.
func CosineSimilarity(a, b []float32) (float32, error) {
	if len(a) != len(b) {
		return 0, errors.New("los vectores deben tener la misma longitud")
	}
	var dotProduct, normA, normB float64
	for i := range a {
		valA := float64(a[i])
		valB := float64(b[i])
		dotProduct += valA * valB
		normA += valA * valA
		normB += valB * valB
	}
	if normA == 0 || normB == 0 {
		return 0, nil
	}
	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))), nil
}
