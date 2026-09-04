package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"strings"
)

// clavePublicaDeReleaseHex es la clave pública ed25519 con la que se verifica el manifiesto de un
// release. Se inyecta al compilar:
//
//	go build -ldflags "-X main.clavePublicaDeReleaseHex=<64 hex>"
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// VACÍA SIGNIFICA «ESTE BINARIO NO PUEDE ACTUALIZARSE SOLO», Y ESO ES LO CORRECTO
//
// Un binario compilado sin clave —el de un desarrollador, el de una prueba— no puede verificar
// la procedencia de nada. La tentación es dejarlo actualizar «porque total antes tampoco
// verificaba», y eso sería peor que antes: ahora existe una función de verificación que da
// tranquilidad y no verifica.
//
// Así que `musubi update` FALLA con la clave vacía, y lo dice con esas palabras. La clave privada
// vive fuera de línea; no está en el CI ni en el repo, y eso es justamente lo que hace que la
// firma valga algo: quien comprometa el pipeline puede reemplazar el binario y su hash, y no
// puede firmar.
var clavePublicaDeReleaseHex = ""

// nombreDelManifiesto es el asset que lista los sha256 de un release. Se firma ENTERO, no cada
// binario por separado: firmando uno por uno nada impide combinar el Linux de un release con el
// Windows de otro, ni servir un release viejo con firmas perfectamente válidas.
const nombreDelManifiesto = "manifest.json"

// clavePublicaDeRelease decodifica la clave embebida. Devuelve nil si no hay o si es ilegible —
// las dos cosas terminan en el mismo error explícito río abajo, que es lo que corresponde: una
// clave que no se puede leer no es mejor que ninguna.
func clavePublicaDeRelease() ed25519.PublicKey {
	h := strings.TrimSpace(clavePublicaDeReleaseHex)
	if h == "" {
		return nil
	}
	b, err := hex.DecodeString(h)
	if err != nil || len(b) != ed25519.PublicKeySize {
		return nil
	}
	return ed25519.PublicKey(b)
}
