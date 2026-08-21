package memory

import (
	"fmt"
	"sort"
	"strings"
)

// semdedup.go expone el DEDUP SEMÁNTICO del acervo (pilar 'Musubi Renaissance', el "afilador"): hallar
// tarjetas que dicen la MISMA lección con palabras distintas —gemelas por COSENO de embeddings, no por
// trigramas (de eso ya se ocupa Consolidate para lo casi-textual)— y, cuando un juez externo confirma
// que son redundantes, archivar la más débil conservando el conocimiento en la más fuerte.
//
// La capa de memoria se queda MODEL-FREE y agnóstica del dominio: HALLA pares cercanos y ARCHIVA un
// duplicado; el JUICIO de si dos tarjetas son "la misma" lo hace el caller (mcp, con un LLM offline). El
// coseno lo da el mismo embedder ya presente; sin embedder no hay candidatos (degrada blando). El
// archivado calca el de Consolidate (soft-delete reversible + superseded_by + herencia de accesos), así
// una fusión por falso positivo no pierde datos y el recall deja de ver la tarjeta al instante.

// SemDupCandidate es un par de observaciones cuyos vectores superan el piso de coseno: un POSIBLE
// duplicado semántico a juzgar. A es el canónico tentativo (el "más fuerte": más accesos, luego más
// importante, luego más nuevo) y B el candidato a archivar. El orden lo fija esta capa para que el
// caller no tenga que re-derivarlo y para que coincida con la elección de canónico de Consolidate.
type SemDupCandidate struct {
	A        string  `json:"a"`
	B        string  `json:"b"`
	TopicA   string  `json:"topic_a"`
	TopicB   string  `json:"topic_b"`
	ContentA string  `json:"content_a"`
	ContentB string  `json:"content_b"`
	Cosine   float64 `json:"cosine"`
}

// semObs es la proyección que necesita el dedup: id + topic + contenido + las señales de "fuerza".
type semObs struct {
	id, topic, content, createdAt string
	access                        int
	importance                    float64
}

// masFuerte devuelve true si o es "más fuerte" que p (mismo criterio de canónico que Consolidate: más
// accesos, luego más importante, luego más nuevo). Determinista: desempata por id para que el orden del
// par sea estable entre corridas.
func (o semObs) masFuerte(p semObs) bool {
	if o.access != p.access {
		return o.access > p.access
	}
	if o.importance != p.importance {
		return o.importance > p.importance
	}
	if o.createdAt != p.createdAt {
		return o.createdAt > p.createdAt
	}
	return o.id > p.id
}

// SemanticDuplicateCandidates devuelve hasta maxPairs pares de observaciones VISIBLES de projectID cuyo
// topic_key empieza con topicPrefix y cuyo coseno de embeddings supera floor, ORDENADOS de más parecido
// a menos. Excluye los pares que YA tienen una relación entre sí (en cualquier dirección): un par ya
// juzgado —marcado `not_duplicate` (KEEP) o ya fusionado— no se vuelve a proponer. Un par sin vector en
// alguno de los dos lados no puede compararse y se omite. Model-free salvo por el coseno (que sale del
// embedder ya presente); si no hay embedder no hay vectores y devuelve vacío.
func (e *DbEngine) SemanticDuplicateCandidates(projectID, topicPrefix string, floor float64, maxPairs int) ([]SemDupCandidate, error) {
	if maxPairs <= 0 {
		maxPairs = 8
	}
	rows, err := e.db.Query(
		`SELECT id, topic_key, content, access_count, importance, COALESCE(created_at,'')
		 FROM observations WHERE project_id = ? AND topic_key LIKE ? AND `+visibleObsPredicate,
		projectID, topicPrefix+"%")
	if err != nil {
		return nil, fmt.Errorf("listar tarjetas para dedup (prefijo %q): %w", topicPrefix, err)
	}
	var all []semObs
	for rows.Next() {
		var o semObs
		if err := rows.Scan(&o.id, &o.topic, &o.content, &o.access, &o.importance, &o.createdAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("escanear tarjeta para dedup: %w", err)
		}
		all = append(all, o)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterar tarjetas para dedup: %w", err)
	}
	rows.Close()
	if len(all) < 2 {
		return nil, nil
	}

	ids := make([]string, len(all))
	idset := make(map[string]bool, len(all))
	for i, o := range all {
		ids[i] = o.id
		idset[o.id] = true
	}
	vecs, err := e.vectorsFor(ids)
	if err != nil {
		return nil, fmt.Errorf("cargar vectores para dedup: %w", err)
	}
	judged, err := e.judgedPairs(ids, idset)
	if err != nil {
		return nil, err
	}

	var cands []SemDupCandidate
	for i := 0; i < len(all); i++ {
		va, ok := vecs[all[i].id]
		if !ok {
			continue
		}
		for j := i + 1; j < len(all); j++ {
			vb, ok := vecs[all[j].id]
			if !ok {
				continue
			}
			if judged[pairKey(all[i].id, all[j].id)] {
				continue
			}
			c, cerr := CosineSimilarity(va, vb)
			if cerr != nil {
				continue // dimensión incompatible ⇒ no comparable
			}
			if float64(c) < floor {
				continue
			}
			strong, weak := all[i], all[j]
			if weak.masFuerte(strong) {
				strong, weak = all[j], all[i]
			}
			cands = append(cands, SemDupCandidate{
				A: strong.id, B: weak.id,
				TopicA: strong.topic, TopicB: weak.topic,
				ContentA: strong.content, ContentB: weak.content,
				Cosine: float64(c),
			})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].Cosine > cands[j].Cosine })
	if len(cands) > maxPairs {
		cands = cands[:maxPairs]
	}
	return cands, nil
}

// judgedPairs arma el conjunto de pares (no ordenados) que YA tienen una relación entre sí dentro del
// set de ids, en cualquiera de las dos puntas. Es lo que hace que un par marcado `not_duplicate` (o ya
// superseded) no se vuelva a proponer para juicio. Trocea el IN por el tope de parámetros de SQLite.
func (e *DbEngine) judgedPairs(ids []string, idset map[string]bool) (map[string]bool, error) {
	out := map[string]bool{}
	const chunk = 400
	for start := 0; start < len(ids); start += chunk {
		end := min(start+chunk, len(ids))
		batch := ids[start:end]
		ph := strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")
		args := make([]interface{}, 0, len(batch))
		for _, id := range batch {
			args = append(args, id)
		}
		rows, err := e.db.Query(
			`SELECT source_id, target_id FROM observation_relations WHERE source_id IN (`+ph+`)`, args...)
		if err != nil {
			return nil, fmt.Errorf("listar pares ya juzgados para dedup: %w", err)
		}
		for rows.Next() {
			var src, tgt string
			if err := rows.Scan(&src, &tgt); err != nil {
				rows.Close()
				return nil, fmt.Errorf("escanear par ya juzgado: %w", err)
			}
			// Sólo cuentan los pares con AMBAS puntas dentro del set (una arista derived_from tarjeta→blob
			// tiene el blob afuera y no marca a ningún par de tarjetas).
			if idset[src] && idset[tgt] {
				out[pairKey(src, tgt)] = true
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("iterar pares ya juzgados: %w", err)
		}
		rows.Close()
	}
	return out, nil
}

// pairKey es la clave NO ORDENADA de un par de ids (min|max): la relación entre A y B es la misma sin
// importar en qué dirección se guardó la arista.
func pairKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "\x00" + b
}

// NearestVisibleByVector devuelve la observación VISIBLE de projectID+topicPrefix cuyo vector es el más
// parecido a vec (coseno máximo), excluyendo excludeID. Es el chequeo ANTI-GEMELO del destilador (causa
// raíz del loop de afilado): antes de escribir una tarjeta nueva, ver si ya existe una casi idéntica. Con
// vec nil (sin embedder) devuelve vacío sin error: el chequeo se saltea y degrada blando.
func (e *DbEngine) NearestVisibleByVector(projectID, topicPrefix string, vec []float32, excludeID string) (id, topic string, cosine float64, err error) {
	if len(vec) == 0 {
		return "", "", 0, nil
	}
	rows, err := e.db.Query(
		`SELECT id, topic_key FROM observations WHERE project_id = ? AND topic_key LIKE ? AND `+visibleObsPredicate,
		projectID, topicPrefix+"%")
	if err != nil {
		return "", "", 0, fmt.Errorf("listar tarjetas para anti-gemelo (prefijo %q): %w", topicPrefix, err)
	}
	ids := []string{}
	topics := map[string]string{}
	for rows.Next() {
		var oid, tk string
		if err := rows.Scan(&oid, &tk); err != nil {
			rows.Close()
			return "", "", 0, fmt.Errorf("escanear tarjeta para anti-gemelo: %w", err)
		}
		if oid == excludeID {
			continue
		}
		ids = append(ids, oid)
		topics[oid] = tk
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", "", 0, fmt.Errorf("iterar tarjetas para anti-gemelo: %w", err)
	}
	rows.Close()
	if len(ids) == 0 {
		return "", "", 0, nil
	}
	vecs, verr := e.vectorsFor(ids)
	if verr != nil {
		return "", "", 0, fmt.Errorf("cargar vectores para anti-gemelo: %w", verr)
	}
	best, bestID := -1.0, ""
	for _, oid := range ids {
		v, ok := vecs[oid]
		if !ok {
			continue
		}
		c, cerr := CosineSimilarity(vec, v)
		if cerr != nil {
			continue
		}
		if float64(c) > best {
			best, bestID = float64(c), oid
		}
	}
	if bestID == "" {
		return "", "", 0, nil
	}
	return bestID, topics[bestID], best, nil
}

// ArchiveAsDuplicate archiva loserID como duplicado de canonicalID (ambos deben ser de projectID —
// aislamiento por tenant, fail-closed). Calca a Consolidate: el canónico HEREDA los accesos del perdedor
// y la mayor importancia; el perdedor se soft-deletea (archived + superseded_by → canónico, reversible)
// y se re-apuntan los superseded_by que colgaban de él. Devuelve archived=false SIN error cuando el
// perdedor ya estaba archivado (idempotente: dos pares del mismo lote que apuntan al mismo perdedor no
// se pisan). Post-commit saca el perdedor del índice vectorial.
func (e *DbEngine) ArchiveAsDuplicate(projectID, loserID, canonicalID string) (archived bool, err error) {
	if loserID == "" || canonicalID == "" || loserID == canonicalID {
		return false, fmt.Errorf("archivar duplicado: ids inválidos (loser=%q canonical=%q)", loserID, canonicalID)
	}

	read := func(id string) (proj string, access int, importance float64, isArchived bool, superseded string, exists bool, rerr error) {
		var arch int
		var sup interface{}
		row := e.db.QueryRow(
			`SELECT COALESCE(project_id,''), access_count, importance, COALESCE(archived,0), COALESCE(superseded_by,'')
			 FROM observations WHERE id = ?`, id)
		if serr := row.Scan(&proj, &access, &importance, &arch, &sup); serr != nil {
			if serr.Error() == "sql: no rows in result set" {
				return "", 0, 0, false, "", false, nil
			}
			return "", 0, 0, false, "", false, serr
		}
		s, _ := sup.(string)
		return proj, access, importance, arch == 1, s, true, nil
	}

	lProj, lAccess, lImp, lArch, lSup, lOK, rerr := read(loserID)
	if rerr != nil {
		return false, fmt.Errorf("leer perdedor %q: %w", loserID, rerr)
	}
	cProj, _, _, cArch, cSup, cOK, rerr := read(canonicalID)
	if rerr != nil {
		return false, fmt.Errorf("leer canónico %q: %w", canonicalID, rerr)
	}
	if !lOK || !cOK {
		return false, fmt.Errorf("archivar duplicado: alguna observación no existe (loser=%v canonical=%v)", lOK, cOK)
	}
	// Aislamiento por tenant (Track 17): jamás fusionar a través de proyectos, aunque el caller se
	// equivoque. La guarda es fail-closed.
	if lProj != projectID || cProj != projectID {
		return false, fmt.Errorf("%w: dedup entre tenants (loser=%q canonical=%q, esperado %q)", ErrCrossTenant, lProj, cProj, projectID)
	}
	// El perdedor ya archivado/superseded ⇒ no-op idempotente (otro par del lote ya lo fusionó).
	if lArch || lSup != "" {
		return false, nil
	}
	// El canónico NO puede estar muerto: fusionar hacia una tarjeta oculta del recall enterraría el
	// conocimiento. Se corta; el próximo barrido re-emparejará al perdedor con el canónico vivo.
	if cArch || cSup != "" {
		return false, fmt.Errorf("archivar duplicado: el canónico %q está archivado/superseded, no se fusiona hacia una tarjeta muerta", canonicalID)
	}

	tx, err := e.db.Begin()
	if err != nil {
		return false, fmt.Errorf("iniciar transacción de dedup: %w", err)
	}
	defer tx.Rollback()

	// El canónico hereda accesos e importancia máxima (misma acumulación que Consolidate).
	if _, err := tx.Exec(
		`UPDATE observations SET access_count = access_count + ?, importance = MAX(importance, ?) WHERE id = ?`,
		lAccess, lImp, canonicalID); err != nil {
		return false, fmt.Errorf("acumular en el canónico: %w", err)
	}
	// Soft-delete reversible del perdedor: archived + superseded_by → canónico. archived_at arranca la
	// ventana de gracia de la purga (PurgeArchived limpia relaciones/embeddings al vencer).
	if _, err := tx.Exec(
		`UPDATE observations SET archived=1, archived_at=CURRENT_TIMESTAMP, superseded_by=? WHERE id=?`,
		canonicalID, loserID); err != nil {
		return false, fmt.Errorf("archivar el perdedor: %w", err)
	}
	// Re-apuntar los superseded_by que colgaban del perdedor hacia el canónico vivo (aplana la cadena).
	if _, err := tx.Exec(
		`UPDATE observations SET superseded_by=? WHERE superseded_by=?`, canonicalID, loserID); err != nil {
		return false, fmt.Errorf("re-apuntar punteros superseded_by: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commitear dedup: %w", err)
	}

	if e.index != nil {
		e.index.RemoveBatch([]string{loserID})
	}
	return true, nil
}
