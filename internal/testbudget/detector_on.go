//go:build race

package testbudget

// BajoDetector le dice al guard si el binario está instrumentado con `-race`.
//
// Se hace con build tags porque Go no expone «estoy instrumentado» en runtime. Es el mismo
// mecanismo que ya usa internal/mcp (corriendoBajoDetector, A53), acá elevado a un lugar que
// puedan mirar todos los paquetes caros y no uno solo — que es exactamente el defecto que este
// repo repite: la guarda presente en N-1 de N caminos.
const BajoDetector = true
