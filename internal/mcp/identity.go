package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"musubi/internal/buildid"
)

// huellaDelCatalogo cuenta y hashea las tools VISIBLES de un registro, con la misma regla de
// visibilidad que aplica handleToolsList: una tool dormida no entra salvo que MUSUBI_TOOLS_ALL
// esté puesta. Tiene que ser la misma regla, o el número que publica la identidad describiría un
// catálogo que nadie sirve.
//
// POR QUÉ UNA SOLA FUNCIÓN Y NO DOS. Los dos llamadores —el servidor vivo, que hashea su propio
// registro, y la CLI, que arma uno al vuelo— pasan por acá. Este repo ya pagó el precio de la
// otra opción: dos structs de cable calculadas por separado en internal/fleet mataron dos
// features en silencio hasta que se unificaron.
func huellaDelCatalogo(entries []toolEntry) (int, string) {
	todas := toolsAllEnabled()
	h := sha256.New()
	n := 0
	for i := range entries {
		if entries[i].dormant && !todas {
			continue
		}
		// Tool es una struct de tipos planos (strings, bools y mapas de lo mismo): json.Marshal
		// no tiene forma de fallar acá, y el orden es determinista porque el encoder ordena las
		// claves de los mapas.
		b, _ := json.Marshal(entries[i].Tool)
		h.Write(b)
		n++
	}
	return n, hex.EncodeToString(h.Sum(nil))
}

// Identity devuelve la identidad de este binario, con el catálogo contado sobre el registro YA
// construido de ESTE servidor y no sobre uno nuevo: así el número que publica el handshake es
// exactamente el que sirve tools/list.
func (s *McpServer) Identity() buildid.Identity {
	n, sha := huellaDelCatalogo(s.tools)
	return buildid.Derive(s.version, n, sha)
}

// CatalogFingerprint arma un registro al vuelo y lo mide, para los llamadores que no tienen un
// servidor vivo —hoy, `musubi version --json`—.
//
// Es seguro sobre un McpServer sin inicializar: buildRegistry sólo construye literales y toma
// method values (s.toolX), que no dereferencian nada. Verificado sobre las 84 entradas, incluidas
// las cuatro que se appendean al final. Si algún día una entrada empieza a leer estado del
// servidor, este camino explota en el acto y no en silencio.
func CatalogFingerprint() (int, string) {
	s := &McpServer{}
	return huellaDelCatalogo(s.buildRegistry())
}
