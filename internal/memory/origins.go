package memory

// origins.go ata una observación al ESTADO DEL PROYECTO del que habla.
//
// El detector de conflictos compara observaciones ENTRE SÍ, así que nunca puede ver
// una nota por lo demás válida con una línea vencida adentro ("PENDIENTE: X" cuando X
// ya se hizo). Eso no se arregla comparando notas: hay que comparar la nota contra el
// mundo. Un ancla guarda (ruta, fingerprint del contenido al guardar); en el recall se
// vuelve a leer el disco y, si difiere, la observación se MARCA.
//
// Marcar, nunca ocultar: que un archivo cambie no prueba que la nota sea falsa. La
// señal es una advertencia para que un humano —o musubi_judge— decida.

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/codeintel"
)

// projectRoot devuelve la raíz del proyecto: e.path es <root>/.musubi/memory.db, así que
// subir dos niveles da la raíz. Es la base contra la que se resuelven las anclas.
func (e *DbEngine) projectRoot() string {
	return filepath.Dir(filepath.Dir(e.path))
}

// errSymbolNotFound distingue "el símbolo ya no está en el archivo" de un fallo de E/S. Lo
// primero es deriva legítima (alguien borró o renombró la función); lo segundo no es evidencia
// de nada y no debe marcar.
var errSymbolNotFound = errors.New("símbolo no encontrado")

// splitOriginRef parte un ancla `ruta#símbolo` en sus dos mitades. Sin '#', el símbolo queda
// vacío y el ancla es del archivo entero.
func splitOriginRef(ref string) (path, symbol string) {
	if i := strings.LastIndex(ref, "#"); i > 0 {
		return ref[:i], ref[i+1:]
	}
	return ref, ""
}

// symbolFingerprint calcula la identidad de UN SÍMBOLO: sha256 de las líneas que ocupa hoy en
// el archivo. Se extrae del contenido ACTUAL con codeintel (model-free, tolerante a archivos
// que no compilan), nunca del grafo de código persistido: los rangos del grafo valen para el
// snapshot en que se indexó, y justo cuando el archivo cambia —el caso que nos importa— serían
// los rangos equivocados.
//
// Anclar a un símbolo en vez de al archivo entero es lo que hace la marca UTIL: un archivo
// grande cambia todo el tiempo por motivos ajenos a la nota, y una marca que salta siempre se
// aprende a ignorar. El símbolo cambia cuando cambia lo que la nota describe.
func symbolFingerprint(root, path, symbol string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return "", err
	}
	content := string(data)
	syms := codeintel.ExtractSymbols(path, content)
	lineas := strings.Split(content, "\n")

	h := sha256.New()
	encontrado := false
	for _, s := range syms {
		if s.Name != symbol {
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
		return "", errSymbolNotFound
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// originFingerprint resuelve la identidad de un ancla, sea de archivo entero o de símbolo.
func originFingerprint(root, ref string) (string, error) {
	path, symbol := splitOriginRef(ref)
	if symbol == "" {
		return FileFingerprint(root, path)
	}
	return symbolFingerprint(root, path, symbol)
}

// maxOriginPaths acota cuántos archivos puede anclar una observación. Excederlo es un
// error y no un truncado silencioso: una nota anclada a medias es justo la clase de
// media-verdad que este mecanismo viene a eliminar. Además, si algo necesita más de
// diez anclas no habla de un estado concreto y la marca no le serviría a nadie.
const maxOriginPaths = 10

// StaleOrigin describe un ancla que dejó de coincidir con el disco.
type StaleOrigin struct {
	Path string `json:"path"`
	// Reason es "changed" (el contenido difiere) o "missing" (la ruta ya no existe).
	// Se distinguen porque significan cosas distintas: un archivo borrado es la señal
	// más fuerte de deriva que existe.
	Reason string `json:"reason"`
}

const (
	StaleChanged = "changed"
	StaleMissing = "missing"
)

// saveObservationOrigins persiste las anclas de una observación DENTRO de la tx del
// caller: si algo falla, no queda la observación guardada sin la protección que se pidió.
// Valida que cada ruta exista (anclar a lo inexistente nacería marcado como rancio) y
// respeta el tope. paths vacío es un no-op.
func saveObservationOrigins(tx *sql.Tx, obsID, root string, paths []string) error {
	limpias, err := normalizeOriginPaths(root, paths)
	if err != nil {
		return err
	}
	for _, rel := range limpias {
		fp, ferr := originFingerprint(root, rel)
		if errors.Is(ferr, errSymbolNotFound) {
			p, sym := splitOriginRef(rel)
			return fmt.Errorf(
				"no se puede anclar a %q: %q no tiene un símbolo top-level llamado %q (¿está bien escrito, o el archivo es de un lenguaje sin extractor?). Sin el símbolo, anclá al archivo entero: %q",
				rel, p, sym, p)
		}
		if ferr != nil {
			return fmt.Errorf("no se pudo calcular el fingerprint de %q: %w", rel, ferr)
		}
		if _, err := tx.Exec(
			`INSERT INTO observation_origins (observation_id, path, fingerprint) VALUES (?,?,?)
			 ON CONFLICT(observation_id, path) DO UPDATE SET fingerprint=excluded.fingerprint, captured_at=CURRENT_TIMESTAMP`,
			obsID, rel, fp,
		); err != nil {
			return fmt.Errorf("no se pudo anclar %q: %w", rel, err)
		}
	}
	return nil
}

// normalizeOriginPaths limpia, deduplica y valida las rutas declaradas. Devuelve rutas
// relativas a root con '/', en el orden en que llegaron.
func normalizeOriginPaths(root string, paths []string) ([]string, error) {
	out := make([]string, 0, len(paths))
	visto := map[string]bool{}
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		// El ancla puede ser `ruta` o `ruta#símbolo`: sólo la ruta se normaliza y se valida
		// contra el disco; el símbolo se resuelve después, al calcular el fingerprint.
		crudo, simbolo := splitOriginRef(strings.TrimSpace(p))
		rel := NormalizeCodePath(root, crudo)
		if strings.HasPrefix(rel, "../") || rel == ".." {
			return nil, fmt.Errorf("la ruta %q queda fuera del proyecto: las anclas son relativas a la raíz", crudo)
		}
		if simbolo != "" {
			rel += "#" + simbolo
		}
		if visto[rel] {
			continue
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(NormalizeCodePath(root, crudo))))
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf(
					"no se puede anclar a %q: no existe. Una observación anclada a un archivo inexistente nacería marcada como rancia", rel)
			}
			return nil, fmt.Errorf("no se pudo leer %q: %w", rel, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("no se puede anclar a %q: es un directorio, no un archivo", rel)
		}
		visto[rel] = true
		out = append(out, rel)
	}
	if len(out) > maxOriginPaths {
		return nil, fmt.Errorf(
			"demasiadas anclas (%d, el tope es %d): si una observación necesita anclar a tantos archivos, probablemente no habla de un estado concreto",
			len(out), maxOriginPaths)
	}
	return out, nil
}

// staleOriginsFor devuelve, por observación, las anclas que ya no coinciden con el disco.
// Una sola consulta para todos los ids y un memo ruta→fingerprint por llamada: varias
// observaciones suelen anclar al mismo archivo.
//
// Un error de E/S que NO sea "no existe" se ignora a propósito: un disco ocupado o un
// permiso momentáneo no son evidencia de que la memoria envejeció, y marcar por eso
// entrenaría a ignorar la marca.
func (e *DbEngine) staleOriginsFor(ids []string, root string) (map[string][]StaleOrigin, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := e.db.Query(
		`SELECT observation_id, path, fingerprint FROM observation_origins WHERE observation_id IN (`+ph+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	actual := map[string]string{} // ruta → fingerprint vigente ("" = no existe)
	out := map[string][]StaleOrigin{}
	for rows.Next() {
		var obsID, path, guardado string
		if err := rows.Scan(&obsID, &path, &guardado); err != nil {
			return nil, err
		}
		vigente, calculado := actual[path]
		if !calculado {
			fp, ferr := originFingerprint(root, path)
			switch {
			case ferr == nil:
				vigente = fp
			case os.IsNotExist(ferr), errors.Is(ferr, errSymbolNotFound):
				// El archivo desapareció, o el símbolo anclado ya no está en él (lo
				// borraron o renombraron). Las dos son deriva, y de la fuerte.
				vigente = ""
			default:
				vigente = guardado // E/S ajena al contenido: no marcar
			}
			actual[path] = vigente
		}
		switch {
		case vigente == "":
			out[obsID] = append(out[obsID], StaleOrigin{Path: path, Reason: StaleMissing})
		case vigente != guardado:
			out[obsID] = append(out[obsID], StaleOrigin{Path: path, Reason: StaleChanged})
		}
	}
	return out, rows.Err()
}

// markStaleOrigins anota los items cuyas anclas dejaron de coincidir con el disco: llena
// el campo Stale y antepone la advertencia al gist. NO reordena, NO descarta y NO toca el
// presupuesto ya consumido — el resultado entra y sale con los mismos items, en el mismo
// orden. Cualquier error se traga a propósito: la ranciedad es información ADICIONAL, y
// un fallo derivándola no puede romper un recall que ya está armado.
func (e *DbEngine) markStaleOrigins(result RecallResult) RecallResult {
	if len(result.Items) == 0 {
		return result
	}
	ids := make([]string, 0, len(result.Items))
	for _, it := range result.Items {
		ids = append(ids, it.ID)
	}
	stale, err := e.staleOriginsFor(ids, e.projectRoot())
	if err != nil || len(stale) == 0 {
		return result
	}
	for i := range result.Items {
		s, hay := stale[result.Items[i].ID]
		if !hay || len(s) == 0 {
			continue
		}
		result.Items[i].Stale = s
		result.Items[i].Gist = staleWarning(s) + result.Items[i].Gist
	}
	return result
}

// staleWarning arma el prefijo que se antepone al gist. Es lo que el agente LEE: el campo
// estructurado sirve al dashboard y a los tests, pero si la advertencia no viaja en el
// gist el modelo no la ve sin hidratar la observación entera.
func staleWarning(stale []StaleOrigin) string {
	if len(stale) == 0 {
		return ""
	}
	cambiados, faltantes := []string{}, []string{}
	for _, s := range stale {
		if s.Reason == StaleMissing {
			faltantes = append(faltantes, s.Path)
		} else {
			cambiados = append(cambiados, s.Path)
		}
	}
	partes := []string{}
	if len(cambiados) > 0 {
		partes = append(partes, "cambió: "+strings.Join(cambiados, ", "))
	}
	if len(faltantes) > 0 {
		partes = append(partes, "ya no existe: "+strings.Join(faltantes, ", "))
	}
	return "⚠ posiblemente rancia (" + strings.Join(partes, "; ") + ") — "
}
