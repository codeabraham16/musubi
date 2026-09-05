package selfupdate

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ UN SHA-256 NO ALCANZA, Y NUNCA ALCANZÓ
//
// `musubi update` baja el binario y su `.sha256` DEL MISMO RELEASE. O sea que el hash lo publica
// exactamente quien publica el binario: quien controle el release —una credencial de GitHub
// filtrada, un token de CI, alguien con permiso de escribir en el repo— controla los dos, y el
// checksum verifica que el archivo llegó entero, no que lo hayamos hecho nosotros.
//
// Es la definición del ataque de cadena de suministro, y en este sistema pesa más que en la
// mayoría: el mismo binario es el CEREBRO y el AGENTE, así que un release comprometido no
// entrega una máquina, entrega la flota — con exec y shell sobre todas.
//
// La firma cambia la pregunta. El manifiesto lo firma una clave PRIVADA que no vive en el CI ni
// en el repo, y el binario lleva embebida la pública. Quien controle el release puede reemplazar
// el binario y el hash, y no puede producir una firma válida.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL ORDEN: PRIMERO LA FIRMA, DESPUÉS EL HASH
//
// Verificar el hash antes de la firma no es sólo redundante: es verificar la integridad de un
// dato cuya procedencia todavía no se estableció. La firma dice de quién es el manifiesto; el
// hash dice si el binario es el que ese manifiesto nombra. En el otro orden, el segundo paso no
// agrega nada al primero.
// ════════════════════════════════════════════════════════════════════════════════════════════

// Manifiesto es lo que se firma: la versión y el sha256 de cada asset de ese release.
//
// Se firma el MANIFIESTO ENTERO y no cada binario por separado a propósito: firmando uno por uno,
// nada impide combinar el binario de Linux de un release con el de Windows de otro, ni servir un
// release viejo con firmas perfectamente válidas. El manifiesto ata los assets a UNA versión, y
// la firma ata el manifiesto a nosotros.
type Manifiesto struct {
	Version string `json:"version"`
	// Assets es nombre de archivo -> sha256 en hex.
	Assets map[string]string `json:"assets"`
}

// BytesFirmables devuelve la forma canónica que se firma y se verifica.
//
// TIENE QUE SER LA MISMA EN LOS DOS LADOS Y NO PUEDE DEPENDER DEL ORDEN DE UN MAPA. `json.Marshal`
// de un map ordena las claves alfabéticamente, así que es determinista — y eso está clavado por
// una prueba, porque si un día dejara de serlo la firma empezaría a fallar en algunos releases y
// no en otros, que es el peor síntoma posible: intermitente y con pinta de problema de red.
func (m Manifiesto) BytesFirmables() ([]byte, error) {
	return json.Marshal(m)
}

// VerificarFirma comprueba que el manifiesto lo firmó quien tiene la clave privada.
//
// `pub` es la clave pública embebida en el binario. VACÍA ⇒ ERROR, nunca «pasa igual»: un binario
// compilado sin clave no puede verificar nada, y dejarlo actualizar sería exactamente el estado
// anterior con una función de más que da falsa tranquilidad.
func VerificarFirma(pub ed25519.PublicKey, manifiesto []byte, firmaHex string) error {
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("este binario no trae una clave pública de release válida (%d bytes): no puede verificar la firma de una actualización, y actualizar sin verificar es peor que no actualizar", len(pub))
	}
	firma, err := hex.DecodeString(strings.TrimSpace(firmaHex))
	if err != nil {
		return fmt.Errorf("la firma del manifiesto no es hex válido: %w", err)
	}
	if len(firma) != ed25519.SignatureSize {
		return fmt.Errorf("la firma mide %d bytes y una ed25519 mide %d", len(firma), ed25519.SignatureSize)
	}
	if !ed25519.Verify(pub, manifiesto, firma) {
		return fmt.Errorf("la firma del manifiesto NO verifica contra la clave de este binario: el release no lo publicamos nosotros, o el manifiesto viene alterado. No se actualiza")
	}
	return nil
}

// ShaDeAsset saca de un manifiesto ya verificado el sha256 que le toca a un asset.
//
// Un asset que no está en el manifiesto es un ERROR y no un «seguí sin verificar»: el manifiesto
// firmado es la lista de lo que publicamos, y bajar algo que no figura ahí es bajar algo que no
// firmamos.
func (m Manifiesto) ShaDeAsset(nombre string) (string, error) {
	sha, hay := m.Assets[nombre]
	if !hay || strings.TrimSpace(sha) == "" {
		return "", fmt.Errorf("el manifiesto firmado de %s no incluye %q: ese archivo no es parte de este release", m.Version, nombre)
	}
	return strings.ToLower(strings.TrimSpace(sha)), nil
}

// ParsearManifiesto lee el JSON del manifiesto.
func ParsearManifiesto(data []byte) (Manifiesto, error) {
	var m Manifiesto
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifiesto{}, fmt.Errorf("manifiesto ilegible: %w", err)
	}
	if strings.TrimSpace(m.Version) == "" || len(m.Assets) == 0 {
		return Manifiesto{}, fmt.Errorf("manifiesto sin versión o sin assets: no dice qué release describe")
	}
	return m, nil
}
