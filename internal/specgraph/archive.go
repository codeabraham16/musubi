// archive.go cablea el DESTINO de la fase `archive` del SDD (spec-graph F4): fusiona el delta spec de
// un cambio en la `SPEC.md` canónica de su módulo. Es una función PURA (model-free, sin I/O: el caller
// lee el delta y la canónica del disco y escribe el resultado) y SURFACE-NOT-AUTO: donde la prosa de
// dos secciones con el mismo heading diverge, NO la pisa —la devuelve en `conflicts` para que la
// reconcilie una persona (R15/D9)—. Hereda la regla de F2: «marca, nunca auto-escribe».
//
// El merge es determinista en TRES capas:
//   - frontmatter escalar: gana el delta que lo declara (salvo `id`, que es la identidad del módulo y
//     no se renombra; un id del delta que NO coincide con la canónica es un error, no un merge);
//   - frontmatter lista (dependsOn/references/implements/covers/tags): UNIÓN dedup; `anchors`: unión
//     POR REF con el digest del delta ganando (MergeAnchorEntries, D7), no por string entero;
//   - cuerpo: por secciones de heading — las aditivas entran al final; las que chocan se surfacean.
//
// Lossless (R16): el frontmatter se edita con UpdateFrontmatterText (la misma vía lossless de F2, que
// conserva comentarios, orden de claves, BOM y line-ending) y el cuerpo de la canónica queda byte a
// byte intacto; SÓLO se AGREGAN secciones nuevas. Nada de la canónica se reescribe ni se reordena.

package specgraph

import (
	"bytes"
	"fmt"
	"strings"
)

// SectionConflict es una sección de prosa que el merge NO pudo resolver sola: la canónica y el delta
// traen el MISMO heading con cuerpo distinto. Se surfacea para reconciliación manual (R15/D9); la
// prosa de la canónica queda intacta en el `merged`.
type SectionConflict struct {
	Heading string `json:"heading"` // la línea de heading que diverge, tal cual aparece en la canónica
	Canon   string `json:"canon"`   // la prosa de la canónica (la que se conserva)
	Delta   string `json:"delta"`   // la prosa del delta (la que NO se escribió, para que el caller decida)
}

// MergeSpecInto fusiona `delta` en `canonical` (R14). Si `canonical` está vacío, CREA la canónica a
// partir del delta (promoción, E8) y la devuelve verbatim (lossless por identidad). Si existe, fusiona
// frontmatter (determinista) + cuerpo (por secciones), devolviendo el texto fusionado y las secciones
// en conflicto (E9). NO auto-fusiona prosa en conflicto (R15): la canónica no se pisa, el delta en
// conflicto viaja en `conflicts`. Un error sólo lo producen entradas que hacen el merge imposible
// (delta vacío, delta sin id/type al crear, canónica sin frontmatter, id del delta ≠ id de la canónica,
// o tope de anclas excedido).
func MergeSpecInto(canonical, delta []byte) (merged []byte, conflicts []SectionConflict, err error) {
	if len(bytes.TrimSpace(delta)) == 0 {
		return nil, nil, fmt.Errorf("el delta de archive está vacío: no hay nada que fusionar")
	}

	// CREAR (promoción, E8): sin canónica, el delta ES la canónica. Se exige que el delta sea una spec
	// válida (id+type, R13) antes de promoverlo: promover un no-spec dejaría una canónica inválida que
	// el grafo no podría navegar. Se devuelve el delta VERBATIM (lossless por identidad).
	if len(bytes.TrimSpace(canonical)) == 0 {
		dfm := ParseFile(string(delta)).Frontmatter
		if _, ok := Scalar(dfm, FieldID); !ok {
			return nil, nil, fmt.Errorf("el delta no tiene %q: no se puede promover a canónica (R13)", FieldID)
		}
		if _, ok := Scalar(dfm, FieldType); !ok {
			return nil, nil, fmt.Errorf("el delta no tiene %q: no se puede promover a canónica (R13)", FieldType)
		}
		return delta, nil, nil
	}

	canonParsed := ParseFile(string(canonical))
	deltaParsed := ParseFile(string(delta))

	// Guarda de identidad: archive fusiona la MISMA spec de módulo en su canónica. Un delta cuyo id
	// difiere del de la canónica no es un merge —son dos specs distintas— y la vía lossless ni siquiera
	// permite renombrar el id. Se rechaza en vez de fusionar a ciegas (protege R13 y la navegación).
	if dID, ok := Scalar(deltaParsed.Frontmatter, FieldID); ok {
		cID, _ := Scalar(canonParsed.Frontmatter, FieldID)
		if dID != cID {
			return nil, nil, fmt.Errorf(
				"el id del delta (%q) no coincide con el de la canónica (%q): archive fusiona la misma spec de módulo, no dos distintas",
				dID, cID)
		}
	}

	// --- Capa 1 y 2: frontmatter (vía lossless de F2) ---
	edit := frontmatterMergeEdit(canonParsed.Frontmatter, deltaParsed.Frontmatter)
	// Las anclas se mergean POR REF (el digest del delta gana), no por string entero: un union-dedup
	// crudo dejaría `ruta#S=viejo` y `ruta#S=nuevo` como dos entradas. Se resuelve el conjunto final
	// acá y se pasa por SetList (la misma vía anchor-aware que usa update, D7).
	if deltaAnchors := List(deltaParsed.Frontmatter, FieldAnchors); len(deltaAnchors) > 0 {
		finalAnchors, aErr := MergeAnchorEntries(List(canonParsed.Frontmatter, FieldAnchors), deltaAnchors)
		if aErr != nil {
			return nil, nil, aErr
		}
		if edit.SetList == nil {
			edit.SetList = map[string][]string{}
		}
		edit.SetList[FieldAnchors] = finalAnchors
	}
	mergedText, uErr := UpdateFrontmatterText(string(canonical), edit)
	if uErr != nil {
		return nil, nil, fmt.Errorf("no se pudo fusionar el frontmatter de la canónica: %w", uErr)
	}

	// --- Capa 3: cuerpo por secciones de heading ---
	// El cuerpo de la canónica (dentro de mergedText) NO se toca; sólo se AGREGAN las secciones del
	// delta cuyo heading no existe en la canónica. Las que chocan (mismo heading, cuerpo distinto) se
	// surfacean en `conflicts` y la prosa de la canónica queda intacta (R15).
	additive, conflicts := mergeBodySections(canonParsed.Body, deltaParsed.Body)
	mergedText = appendSections(mergedText, additive, detectEOL(canonical))

	return []byte(mergedText), conflicts, nil
}

// frontmatterMergeEdit arma el FrontmatterEdit determinista para fusionar los escalares y listas
// (menos anchors, que es anchor-aware y lo resuelve el caller) del delta sobre la canónica. `id` se
// EXCLUYE: es la identidad del módulo (ya validada igual arriba) y la vía lossless no la renombra.
func frontmatterMergeEdit(canon, delta Frontmatter) FrontmatterEdit {
	edit := FrontmatterEdit{Set: map[string]string{}, AddList: map[string][]string{}}
	for key := range delta {
		if key == FieldID {
			continue // identidad del módulo: no se toca (ya se validó que coincide)
		}
		if contains(ListFields, key) {
			if key == FieldAnchors {
				continue // anchors: unión por ref, la resuelve el caller con MergeAnchorEntries
			}
			// Resto de listas (dependsOn/references/implements/covers/tags): unión dedup por string.
			if items := List(delta, key); len(items) > 0 {
				edit.AddList[key] = items
			}
			continue
		}
		// Escalar (type/title/status/parent/shared/…): gana el delta que lo declara (D9).
		if v, ok := Scalar(delta, key); ok {
			edit.Set[key] = v
		}
	}
	return edit
}

// bodySection es una sección del cuerpo: su línea de heading tal cual, una clave de match (nivel +
// título normalizado, case-insensitive) y el texto que cuelga de ella. La sección de preámbulo (lo que
// va antes del primer heading, normalmente vacío porque la spec arranca con `# Título`) lleva key "".
type bodySection struct {
	headingLine string
	key         string
	content     string
}

// splitBodySections parte un cuerpo markdown en secciones por heading ATX (`#`..`######` seguido de
// espacio). El contenido de cada sección es lo que va DESPUÉS de su línea de heading hasta el próximo
// heading, conservado tal cual (el merge sólo compara en trim; el append normaliza el EOL).
func splitBodySections(body string) []bodySection {
	lines := strings.Split(body, "\n")
	secs := make([]bodySection, 0, 8)
	cur := bodySection{} // preámbulo (key "")
	buf := make([]string, 0, len(lines))
	flush := func() {
		cur.content = strings.Join(buf, "\n")
		secs = append(secs, cur)
		buf = buf[:0:0]
	}
	for _, ln := range lines {
		if lvl, title, ok := atxHeading(ln); ok {
			flush()
			cur = bodySection{
				headingLine: strings.TrimRight(ln, "\r"),
				key:         fmt.Sprintf("%d\x00%s", lvl, strings.ToLower(strings.Join(strings.Fields(title), " "))),
			}
			continue
		}
		buf = append(buf, ln)
	}
	flush()
	return secs
}

// atxHeading reconoce una línea de heading ATX: 1..6 `#` seguidos de un espacio. Devuelve el nivel y
// el título (sin los `#` ni espacios de los bordes). El `\r` de un CRLF se recorta antes de mirar.
func atxHeading(line string) (level int, title string, ok bool) {
	s := strings.TrimRight(line, "\r")
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || n >= len(s) || s[n] != ' ' {
		return 0, "", false
	}
	return n, strings.TrimSpace(s[n:]), true
}

// mergeBodySections compara las secciones del delta contra las de la canónica: una sección cuyo heading
// no existe en la canónica es ADITIVA (entra); una con el mismo heading y cuerpo IDÉNTICO (en trim) ya
// está (no-op); una con el mismo heading y cuerpo DISTINTO es un CONFLICTO que se surfacea sin pisar la
// canónica (R15). El preámbulo del delta, si difiere y no está vacío, también se surfacea (nunca se
// descarta en silencio). Las secciones aditivas salen en el orden del delta, deduplicadas por heading.
func mergeBodySections(canonBody, deltaBody string) (additive []bodySection, conflicts []SectionConflict) {
	canonSecs := splitBodySections(canonBody)
	deltaSecs := splitBodySections(deltaBody)

	canonByKey := make(map[string]bodySection, len(canonSecs))
	var canonPreamble string
	for _, s := range canonSecs {
		if s.key == "" {
			canonPreamble = s.content
			continue
		}
		if _, dup := canonByKey[s.key]; !dup {
			canonByKey[s.key] = s // ante headings duplicados en la canónica, gana el primero (orden de walk)
		}
	}

	seen := make(map[string]bool, len(deltaSecs))
	for _, d := range deltaSecs {
		if d.key == "" {
			// Preámbulo del delta: prosa suelta antes del primer heading. No cuelga de un heading, así
			// que no se puede ubicar sola; si difiere y trae texto, se surfacea en vez de descartarla.
			if strings.TrimSpace(d.content) != "" && strings.TrimSpace(d.content) != strings.TrimSpace(canonPreamble) {
				conflicts = append(conflicts, SectionConflict{
					Heading: "(preámbulo)",
					Canon:   strings.TrimSpace(canonPreamble),
					Delta:   strings.TrimSpace(d.content),
				})
			}
			continue
		}
		if seen[d.key] {
			continue // el delta repitió un heading: ya se procesó la primera aparición
		}
		seen[d.key] = true
		c, ok := canonByKey[d.key]
		if !ok {
			additive = append(additive, d)
			continue
		}
		if strings.TrimSpace(c.content) == strings.TrimSpace(d.content) {
			continue // misma sección, mismo cuerpo: ya presente
		}
		conflicts = append(conflicts, SectionConflict{
			Heading: c.headingLine,
			Canon:   strings.TrimSpace(c.content),
			Delta:   strings.TrimSpace(d.content),
		})
	}
	return additive, conflicts
}

// appendSections agrega las secciones aditivas al final de `text`, separadas por una línea en blanco y
// con el EOL de la canónica, sin tocar un solo byte de lo que ya estaba (lossless, R16). Una sección
// sin cuerpo entra sólo con su heading. Si no hay aditivas, devuelve `text` tal cual.
func appendSections(text string, sections []bodySection, eol string) string {
	if len(sections) == 0 {
		return text
	}
	var b strings.Builder
	b.WriteString(text)
	if !strings.HasSuffix(text, "\n") {
		b.WriteString(eol) // cerrar la última línea de la canónica si no venía terminada
	}
	for _, s := range sections {
		b.WriteString(eol) // línea en blanco antes de la sección nueva
		b.WriteString(normalizeEOL(s.headingLine, eol))
		b.WriteString(eol)
		if body := strings.Trim(s.content, "\r\n"); strings.TrimSpace(body) != "" {
			b.WriteString(normalizeEOL(body, eol))
			b.WriteString(eol)
		}
	}
	return b.String()
}

// detectEOL devuelve el line-ending dominante de la canónica: CRLF si aparece alguno, LF si no. Lo que
// se AGREGA usa ese EOL para no mezclar estilos dentro de un archivo que llegó con CRLF (el repo fija
// eol=lf por .gitattributes, pero un SPEC.md editado en Windows puede traer CRLF).
func detectEOL(content []byte) string {
	if bytes.Contains(content, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

// normalizeEOL lleva todos los saltos de `s` al EOL pedido (primero colapsa CRLF→LF para no duplicar
// el \r). Se usa sólo sobre el texto AGREGADO, nunca sobre lo que ya estaba en la canónica.
func normalizeEOL(s, eol string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if eol != "\n" {
		s = strings.ReplaceAll(s, "\n", eol)
	}
	return s
}
