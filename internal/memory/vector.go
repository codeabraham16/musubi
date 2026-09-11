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

// VectoresDePruebas devuelve todos los vectores guardados, por id. Es para el BANCO de evaluación
// (internal/recalleval), que necesita comparar candidatos entre sí para medir redundancia y tiene
// que hacerlo con LOS MISMOS vectores que el ranker compara, no con unos recalculados.
//
// Está exportada y no es de test porque recalleval es otro paquete. Read-only.
func (e *DbEngine) VectoresDePruebas() (map[string][]float32, error) {
	rows, err := e.db.Query(`SELECT observation_id, vector FROM embeddings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]float32{}
	for rows.Next() {
		var id string
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			return nil, err
		}
		v, err := BytesToFloat32(blob)
		if err != nil {
			continue
		}
		out[id] = v
	}
	return out, rows.Err()
}
