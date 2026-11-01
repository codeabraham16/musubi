package codeintel

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// fingerprint.go deriva la IDENTIDAD de una porción de código del estado ACTUAL del archivo: el
// sha256 de las líneas que ocupa un símbolo, o del archivo entero. Es el mecanismo que comparten
// la memoria de observaciones (anclas de origin_path) y el grafo de specs (anclas de SPEC.md).
//
// Vive acá, en Go puro y model-free, por DOS razones que son la misma: que la regla de matcheo
// calificado (#326) exista en UN solo lugar y no derive entre subsistemas, y que el núcleo del
// hash sea único, de modo que un ancla de spec y un ancla de observación sobre el mismo símbolo
// produzcan el MISMO fingerprint y sean comparables byte a byte. codeintel no depende del motor de
// memoria ni de la base: recibe rutas y contenido y devuelve identidad, así que specgraph —que es
// FS-nativo y no puede depender de memory— lo usa directo.

// ErrSymbolNotFound distingue "el símbolo ya no está en el archivo" de un fallo de E/S. Lo primero
// es deriva legítima (alguien borró o renombró la función); lo segundo no es evidencia de nada y
// no debe marcar. El llamador decide con errors.Is.
var ErrSymbolNotFound = errors.New("símbolo no encontrado")

// NormalizeCodePath devuelve una clave de path estable: relativa a root cuando el path cae dentro
// de root, limpia, y con separadores "/" (consistente en todo SO).
func NormalizeCodePath(root, path string) string {
	p := path
	if filepath.IsAbs(p) {
		if rel, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(rel, "..") {
			p = rel
		}
	}
	return filepath.ToSlash(filepath.Clean(p))
}

// FileFingerprint devuelve el sha256 (hex) del contenido actual del archivo, resolviendo un path
// relativo contra root. Es la señal de frescura del archivo entero: si cambia el contenido, cambia
// el fingerprint.
func FileFingerprint(root, path string) (string, error) {
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(root, path)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// SplitOriginRef parte un ancla `ruta#símbolo` en sus dos mitades. Sin '#', el símbolo queda vacío
// y el ancla es del archivo entero.
func SplitOriginRef(ref string) (path, symbol string) {
	if i := strings.LastIndex(ref, "#"); i > 0 {
		return ref[:i], ref[i+1:]
	}
	return ref, ""
}

// SymbolMatches decide si un símbolo extraído del archivo es el que pide el ancla, y acepta DOS
// formas a propósito:
//
//	`Tipo.Metodo`  -> exacta. Es la clave que ya usa el grafo de código, así que la que devuelve
//	                  `musubi_code_graph` se puede pegar tal cual en un origin_path. Y al exigir
//	                  el receptor, NO funde métodos homónimos.
//	`Nombre`       -> pelada, el comportamiento HISTÓRICO. Sigue matcheando funciones, tipos y
//	                  también métodos de cualquier receptor.
//
// LA FORMA PELADA NO SE QUITA, y no es por comodidad: las anclas ya guardadas en la base están
// escritas así. Angostarla haría que observaciones que hoy resuelven bien pasaran a `missing` de
// un día para otro —una marca de rancio masiva y falsa—, que es justo el ruido que este mecanismo
// existe para no producir. Se AGREGA precisión para quien la pida; no se le cambia el piso a nadie.
//
// El costo asumido: un ancla pelada a un nombre repetido sigue cubriendo a todos sus homónimos.
// Quien quiera precisión ahora tiene cómo pedirla, y la lista de símbolos se la muestra calificada.
func SymbolMatches(s Symbol, pedido string) bool {
	if tipo, metodo, esCalificado := strings.Cut(pedido, "."); esCalificado {
		return s.Recv == tipo && s.Name == metodo
	}
	return s.Name == pedido
}

// FingerprintSymbols es el NÚCLEO del hash: sha256 de las líneas que ocupan, dentro de `content`,
// los símbolos que matchean `pedido` (con SymbolMatches). Devuelve ErrSymbolNotFound si ninguno
// matchea. El formato de byte vive acá y en ningún otro lado: por eso un ancla de spec y un ancla
// de observación sobre el mismo símbolo producen el mismo fingerprint.
func FingerprintSymbols(content string, syms []Symbol, pedido string) (string, error) {
	lineas := strings.Split(content, "\n")
	h := sha256.New()
	encontrado := false
	for _, s := range syms {
		if !SymbolMatches(s, pedido) {
			continue
		}
		encontrado = true
		// Rangos 1-based inclusivos; se acotan por si el extractor estimó de más.
		ini, fin := s.StartLine, s.EndLine
		if ini < 1 {
			ini = 1
		}
		if fin > len(lineas) {
			fin = len(lineas)
		}
		fmt.Fprintf(h, "%s\x00%s\x00", s.Kind, strings.Join(lineas[ini-1:fin], "\n"))
	}
	if !encontrado {
		return "", ErrSymbolNotFound
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// SymbolFingerprint es la forma DERIVADA: lee el archivo, extrae sus símbolos top-level con el
// extractor (model-free, tolerante a archivos que no compilan) y hashea las líneas del símbolo
// pedido. NUNCA usa los rangos del grafo persistido: valen para el snapshot en que se indexó, y
// justo cuando el archivo cambia —el caso que importa— serían los rangos equivocados.
//
// Anclar a un símbolo en vez de al archivo entero es lo que hace la marca ÚTIL: un archivo grande
// cambia todo el tiempo por motivos ajenos a la nota, y una marca que salta siempre se aprende a
// ignorar. El símbolo cambia cuando cambia lo que la nota describe.
func SymbolFingerprint(root, path, symbol string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return "", err
	}
	content := string(data)
	return FingerprintSymbols(content, ExtractSymbols(path, content), symbol)
}
