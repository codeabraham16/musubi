package specgraph

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/codeintel"
)

// anchors.go ata una spec al CÓDIGO del que habla, y detecta cuándo ese código se movió bajo sus
// pies. Es el espejo, para el grafo de specs, del mecanismo de origin_paths de las observaciones
// (internal/memory/origins.go): un ancla guarda (ref, digest del contenido al anclar); al leer la
// spec se vuelve a derivar del disco y, si difiere, el ancla se MARCA.
//
// Marcar, nunca ocultar ni invalidar: que el código cambie no prueba que la spec esté mal. La
// señal es una advertencia —surface-only— para que una persona o musubi_judge decida; nunca
// vuelve inválida la spec ni la reordena (R5/R8/D8).
//
// Comparabilidad (D3): el núcleo del hash es codeintel.FingerprintSymbols, EL MISMO que usa una
// observación, así que un ancla de spec y un ancla de observación sobre el mismo símbolo producen
// el mismo digest y son comparables byte a byte. A diferencia de la memoria, acá NO hay fallback
// de símbolos declarados ni base de code_memory (D4): el digest siempre se deriva del extractor,
// así que nunca lleva el prefijo "d:" y el re-cálculo siempre usa la misma fuente. El costo
// asumido: anclar a un símbolo en un lenguaje sin extractor es un ERROR al guardar (R2), no un
// ancla que nacería marcada como rancia — y la salida es anclar al archivo entero.

// maxAnchors acota cuántas anclas puede declarar una spec. Excederlo es un error, no un truncado
// silencioso: una spec anclada a medias es la clase de media-verdad que este mecanismo viene a
// eliminar, y algo que necesita más de diez anclas no habla de una pieza concreta de código.
// Mismo tope que maxOriginPaths en la memoria, a propósito.
const maxAnchors = 10

// Razones de ranciedad de un ancla. Mismos valores de cadena que memory.StaleChanged /
// memory.StaleMissing (en el JSON de salida son sólo strings): se redefinen acá en vez de
// importarlos porque specgraph es FS-nativo y no puede depender del motor de memoria.
const (
	StaleChanged = "changed"
	StaleMissing = "missing"
)

// Anchor es una entrada de `anchors` ya parseada: la ref (`ruta` o `ruta#símbolo`, sin el
// "=digest"), sus dos mitades, y el digest de línea base capturado al anclar (vacío si se escribió
// a mano sin "=").
type Anchor struct {
	Ref      string // `ruta` o `ruta#símbolo`, sin el "=digest"
	Path     string // sólo la ruta, relativa a la raíz del repo con "/"
	Symbol   string // el símbolo, o "" para un ancla de archivo entero
	Baseline string // digest guardado, o "" (ancla a mano / todavía sin vouch por tool)
}

// parseAnchorEntry parte una entrada `ruta#símbolo=digest` en Anchor. El digest se separa por el
// ÚLTIMO "=": ni las rutas ni los símbolos de Go/TS/Py contienen "=", así que cuando lo hay el
// último "=" siempre separa el digest sin ambigüedad (D2). Sin "=", el baseline queda vacío y la
// ref es la entrada entera.
func parseAnchorEntry(entrada string) Anchor {
	ref, baseline := entrada, ""
	if i := strings.LastIndex(entrada, "="); i >= 0 {
		ref, baseline = entrada[:i], entrada[i+1:]
	}
	path, symbol := codeintel.SplitOriginRef(ref)
	return Anchor{Ref: ref, Path: path, Symbol: symbol, Baseline: baseline}
}

// anchorFingerprint deriva el digest ACTUAL de un ancla desde el disco: el del archivo entero si
// no hay símbolo, o el de las líneas del símbolo si lo hay. Siempre derivado del extractor (D4):
// sin fallback de símbolos declarados, nunca con prefijo. Un archivo que no existe devuelve un
// error os.IsNotExist; un símbolo que ya no está, codeintel.ErrSymbolNotFound.
func anchorFingerprint(root, path, symbol string) (string, error) {
	if symbol == "" {
		return codeintel.FileFingerprint(root, path)
	}
	return codeintel.SymbolFingerprint(root, path, symbol)
}

// BuildAnchorEntries normaliza y valida una lista de refs crudos (`ruta` o `ruta#símbolo`, SIN
// digest) y devuelve las entradas `ref=digest` listas para escribir en el frontmatter. Es el
// equivalente, para specgraph, de saveObservationOrigins+normalizeOriginPaths de la memoria:
// limpia y deduplica POR REF (la última gana, orden de 1ª aparición), valida que cada ruta exista
// y no sea un directorio, exige que el símbolo exista, computa el digest de línea base, y respeta
// el tope. Un ref irresoluble es un ERROR (R2), no un ancla que nacería marcada como rancia.
//
// El vouch (calcular el digest) lo hace ESTA función, no el llamador: por eso un ref que ya trae
// "=digest" es un error de uso, no una entrada a respetar.
func BuildAnchorEntries(root string, refs []string) ([]string, error) {
	orden := make([]string, 0, len(refs)) // refs en orden de 1ª aparición
	porRef := map[string]string{}         // ref → digest (la última gana; D7)
	for _, crudo := range refs {
		crudo = strings.TrimSpace(crudo)
		if crudo == "" {
			continue
		}
		// El digest lo pone BuildAnchorEntries; un "=" en la entrada es un ref mal formado (o un
		// intento de pre-suministrar la huella, que no se acepta: el vouch se hace acá).
		if strings.Contains(crudo, "=") {
			return nil, fmt.Errorf("el ancla %q no debe incluir un digest: pasá sólo `ruta` o `ruta#símbolo`", crudo)
		}
		path, symbol := codeintel.SplitOriginRef(crudo)
		rel := codeintel.NormalizeCodePath(root, path)
		if strings.HasPrefix(rel, "../") || rel == ".." {
			return nil, fmt.Errorf("la ruta %q queda fuera del repo: las anclas son relativas a la raíz", path)
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf(
					"no se puede anclar a %q: no existe. Un ancla a un archivo inexistente nacería marcada como rancia", crudo)
			}
			return nil, fmt.Errorf("no se pudo leer %q: %w", crudo, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("no se puede anclar a %q: es un directorio, no un archivo", crudo)
		}
		ref := rel
		if symbol != "" {
			ref += "#" + symbol
		}
		digest, derr := anchorFingerprint(root, rel, symbol)
		if errors.Is(derr, codeintel.ErrSymbolNotFound) {
			return nil, fmt.Errorf(
				"no se puede anclar a %q: %q no tiene un símbolo top-level llamado %q (¿está bien escrito, o el archivo es de un lenguaje sin extractor?). Sin el símbolo, anclá al archivo entero: %q",
				crudo, rel, symbol, rel)
		}
		if derr != nil {
			return nil, fmt.Errorf("no se pudo calcular el digest de %q: %w", crudo, derr)
		}
		if _, visto := porRef[ref]; !visto {
			orden = append(orden, ref)
		}
		porRef[ref] = digest
	}
	if len(orden) > maxAnchors {
		return nil, fmt.Errorf(
			"demasiadas anclas (%d, el tope es %d): si una spec necesita anclar a tantos lugares, probablemente no habla de una pieza concreta de código",
			len(orden), maxAnchors)
	}
	out := make([]string, 0, len(orden))
	for _, ref := range orden {
		out = append(out, ref+"="+porRef[ref])
	}
	return out, nil
}

// MergeAnchorEntries combina las entradas `ref=digest` que ya tiene la spec con las recién
// vencidas (de BuildAnchorEntries), keyed POR REF: una ref que ya estaba conserva su posición y
// toma el digest nuevo (re-vouch explícito); una ref nueva se agrega al final. Es la resolución
// del "conjunto final" del update (D7): dedup por ref —NO por string entero— para que `ruta#S` (a
// mano, sin digest) y `ruta#S=hash` (por tool) no coexistan como dos anclas. Respeta el tope
// maxAnchors sobre la unión; excederlo es error, no truncado.
//
// Una ref que YA estaba y NO viene en `fresh` se conserva TAL CUAL (no re-vouch): un update que
// toca una ref no puede borrar en silencio el `changed` legítimo de otra. El re-vouch es siempre
// explícito: sólo las refs que el llamador pasa se re-calculan.
func MergeAnchorEntries(existing, fresh []string) ([]string, error) {
	freshByRef := make(map[string]string, len(fresh)) // ref → entrada nueva (ya con digest)
	freshOrder := make([]string, 0, len(fresh))       // refs nuevas en orden de 1ª aparición
	for _, e := range fresh {
		ref := parseAnchorEntry(e).Ref
		if _, ok := freshByRef[ref]; !ok {
			freshOrder = append(freshOrder, ref)
		}
		freshByRef[ref] = e
	}
	out := make([]string, 0, len(existing)+len(fresh))
	usados := make(map[string]bool, len(fresh))    // refs nuevas ya emitidas (al re-vouchear una existente)
	vistos := make(map[string]bool, len(existing)) // refs ya emitidas (colapsa un existing duplicado)
	for _, e := range existing {
		ref := parseAnchorEntry(e).Ref
		if vistos[ref] {
			continue
		}
		vistos[ref] = true
		if nueva, ok := freshByRef[ref]; ok {
			out = append(out, nueva) // re-vouch: el digest nuevo gana, misma posición
			usados[ref] = true
		} else {
			out = append(out, e) // no se tocó esta ref: se conserva tal cual
		}
	}
	for _, ref := range freshOrder {
		if !usados[ref] {
			out = append(out, freshByRef[ref]) // ref nueva: al final
		}
	}
	if len(out) > maxAnchors {
		return nil, fmt.Errorf(
			"demasiadas anclas (%d, el tope es %d): quitá alguna con removeList antes de sumar más",
			len(out), maxAnchors)
	}
	return out, nil
}

// StaleAnchor describe un ancla que dejó de coincidir con el disco.
type StaleAnchor struct {
	Path string `json:"path"` // la ref del ancla (`ruta` o `ruta#símbolo`)
	// Reason es "changed" (el contenido difiere del digest de base) o "missing" (la ruta o el
	// símbolo ya no existe). Se distinguen porque significan cosas distintas: borrar lo anclado es
	// la señal más fuerte de deriva que existe.
	Reason string `json:"reason"`
}

// StaleWarning arma la advertencia legible para un conjunto de anclas rancias: el canal en PROSA,
// complemento del campo estructurado (StaleAnchor) que sirve al dashboard y a los tests. Espeja a
// memory.StaleWarning —mismo formato exacto— para que una spec rancia y una observación rancia se
// lean igual. Vacío si no hay nada rancio.
func StaleWarning(stale []StaleAnchor) string {
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

// StalenessOf devuelve las anclas de una spec que ya no coinciden con el disco. Espeja
// markStaleOrigins de la memoria: re-deriva cada ancla, compara contra su digest de base, y
// devuelve lo que derivó. Es SURFACE-ONLY (R5/R8/D8): NO reordena, NO descarta, NO vuelve inválida
// la spec; sólo informa. Cómputo fresco por llamada, acotado a ≤maxAnchors lecturas chicas (D6): el
// archivo anclado es otro que el SPEC.md, así que no se cachea en el (mtime,size) del índice.
//
// Un error de E/S ajeno al contenido (disco ocupado, permiso momentáneo) se ignora a propósito: no
// es evidencia de deriva, y marcar por eso entrenaría a ignorar la marca (R6). Devuelve una lista
// (nunca nil) por higiene, aunque el llamador sólo la itere.
func StalenessOf(root string, fm Frontmatter) []StaleAnchor {
	refs := List(fm, FieldAnchors)
	out := make([]StaleAnchor, 0, len(refs))
	for _, entrada := range refs {
		a := parseAnchorEntry(entrada)
		if a.Path == "" {
			continue // entrada degenerada (p. ej. "=hash" a mano): nada que verificar
		}
		vigente, err := anchorFingerprint(root, a.Path, a.Symbol)
		switch {
		case err == nil:
			// El archivo/símbolo está. Sólo se puede hablar de "changed" si había un baseline con
			// qué comparar: un ancla escrita a mano sin "=" (baseline vacío) NO marca changed hasta
			// el primer guardado por tool (D2) — comparar contra "" marcaría todo como cambiado.
			if a.Baseline != "" && vigente != a.Baseline {
				out = append(out, StaleAnchor{Path: a.Ref, Reason: StaleChanged})
			}
		case os.IsNotExist(err), errors.Is(err, codeintel.ErrSymbolNotFound):
			// El archivo desapareció, o el símbolo anclado ya no está en él (lo borraron o
			// renombraron). Las dos son deriva, y de la fuerte. Funciona aun sin baseline.
			out = append(out, StaleAnchor{Path: a.Ref, Reason: StaleMissing})
		default:
			// E/S ajena al contenido: no marcar (R6).
		}
	}
	return out
}
