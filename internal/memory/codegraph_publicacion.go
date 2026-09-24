package memory

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// PublicacionDelGrafo dice DE QUÉ ÁRBOL es un grafo empujado al central: el commit que se indexó y
// su fecha de commit. Vacía = el emisor no lo dijo (un binario anterior a esta guarda, o un índice
// que no llegó a sellar su commit).
type PublicacionDelGrafo struct {
	Head string
	En   time.Time
}

// Vacia dice si el emisor no declaró de qué árbol es el grafo.
func (p PublicacionDelGrafo) Vacia() bool { return strings.TrimSpace(p.Head) == "" }

// ErrGrafoMasViejo lo devuelve ReplaceProjectGraphPublicado cuando el grafo que llega describe un
// árbol MÁS VIEJO que el publicado. El grafo del central queda intacto.
var ErrGrafoMasViejo = fmt.Errorf("el grafo empujado es de un árbol más viejo que el publicado")

// ErrGrafoIgnorado lo devuelve ReplaceProjectGraphPublicado cuando el grafo que llega no dice de qué
// commit es y el publicado sí: un binario anterior a la guarda. Tampoco toca nada, pero es un error
// DISTINTO porque la respuesta al emisor tiene que ser otra (ver toolCodegraphPush).
var ErrGrafoIgnorado = fmt.Errorf("el grafo empujado no dice de qué commit es y el publicado sí")

// decisionPublicacion es lo que la guarda hace con un push.
type decisionPublicacion int

const (
	aceptarPublicacion decisionPublicacion = iota
	rechazarPublicacion
	ignorarPublicacion
)

// metaPublicacionDelGrafo es la clave de meta, POR PROYECTO, con la publicación vigente en el
// central: "head|fecha RFC3339". Por proyecto porque la base del central guarda el grafo de todos.
func metaPublicacionDelGrafo(projectID string) string {
	return "codegraph_publicado:" + projectID
}

// decidirPublicacion es la regla de la guarda, pura para poder probarla caso por caso.
//
// ⚠️ POR QUÉ EXISTE. El protocolo del push es de REEMPLAZO: gana el último que llega. Medido el
// 2026-09-24 en el central: la laptop tenía checkouteada una rama del 2026-09-12, 174 commits
// detrás de main, y su terminal la indexó y la publicó a las 17:02 encima del grafo de esta PC,
// que era de un día antes. El central quedó describiendo el código de doce días atrás, sin 74
// archivos, y lo contestaba como el mapa vigente. No era la primera vez: el registro de pushes
// muestra el grafo yendo y viniendo entre las dos máquinas según cuál empujó última.
//
// La fecha que se compara es la de COMMIT, no la del push: la del push dice cuándo se mandó, no de
// cuándo es lo que se mandó, y es justo la que un árbol viejo tiene nueva.
//
//   - Nada publicado con commit → se acepta: es el estado de antes de esta guarda, y rechazar
//     dejaría al proyecto sin grafo.
//   - Llega sin commit y lo publicado lo tiene → se IGNORA. Es un binario viejo, y el caso medido
//     fue exactamente ése; aceptarlo dejaría la guarda en manos de quien no la conoce. Se ignora y no
//     se rechaza porque un binario viejo no conoce el rechazo: lo tomaría como falla transitoria y
//     re-empujaría el grafo entero en cada tick, logueando que el central falló (lo encontró la
//     revisión antes del merge). Ignorado, lo toma como éxito, marca su generación y se calla.
//   - Mismo commit → se acepta: es el re-empuje de higiene del mismo árbol.
//   - Commit más viejo → se rechaza. Esto sólo lo manda un cliente nuevo, que sí entiende el rechazo.
func decidirPublicacion(vigente, nueva PublicacionDelGrafo) (decisionPublicacion, string) {
	if vigente.Vacia() {
		return aceptarPublicacion, ""
	}
	if nueva.Vacia() {
		return ignorarPublicacion, fmt.Sprintf("el central tiene publicado el grafo de %s (%s) y este push no dice de qué commit es: "+
			"lo mandó un binario anterior a esta guarda, que hay que actualizar; no se tocó nada", vigente.Head, vigente.En.UTC().Format(time.RFC3339))
	}
	if nueva.Head == vigente.Head {
		return aceptarPublicacion, ""
	}
	if nueva.En.Before(vigente.En) {
		return rechazarPublicacion, fmt.Sprintf("el grafo empujado es de %s (%s) y el publicado es de %s (%s), más nuevo: "+
			"se conserva el publicado", nueva.Head, nueva.En.UTC().Format(time.RFC3339), vigente.Head, vigente.En.UTC().Format(time.RFC3339))
	}
	return aceptarPublicacion, ""
}

// leerPublicacionDelGrafo lee la publicación vigente de un proyecto. Un valor ilegible cuenta como
// «nada publicado»: preferible aceptar un push a dejar un proyecto trabado para siempre por una
// fila corrupta.
func leerPublicacionDelGrafo(q interface {
	QueryRow(string, ...any) *sql.Row
}, projectID string) (PublicacionDelGrafo, error) {
	var v string
	err := q.QueryRow(`SELECT value FROM meta WHERE key = ?`, metaPublicacionDelGrafo(projectID)).Scan(&v)
	if err == sql.ErrNoRows {
		return PublicacionDelGrafo{}, nil
	}
	if err != nil {
		return PublicacionDelGrafo{}, fmt.Errorf("error al leer la publicación del grafo: %w", err)
	}
	head, fecha, ok := strings.Cut(v, "|")
	if !ok {
		return PublicacionDelGrafo{}, nil
	}
	en, err := time.Parse(time.RFC3339, fecha)
	if err != nil {
		return PublicacionDelGrafo{}, nil
	}
	return PublicacionDelGrafo{Head: head, En: en}, nil
}

// PublicacionDelGrafoDe devuelve lo publicado para un proyecto (vacía si nada).
func (e *DbEngine) PublicacionDelGrafoDe(projectID string) (PublicacionDelGrafo, error) {
	if projectID == "" {
		projectID = e.projectID
	}
	return leerPublicacionDelGrafo(e.db, projectID)
}

// ReplaceProjectGraphPublicado es ReplaceProjectGraphFrom con la guarda de antigüedad: en UNA
// transacción lee lo publicado, decide, y sólo si acepta reemplaza el grafo y registra la nueva
// publicación. Si no acepta devuelve un error que envuelve ErrGrafoMasViejo o ErrGrafoIgnorado, y no
// toca nada.
//
// La decisión va DENTRO de la transacción del reemplazo, no antes: dos pushes que llegan juntos no
// pueden pasar los dos la pregunta con la misma publicación vieja y aterrizar en el orden malo.
func (e *DbEngine) ReplaceProjectGraphPublicado(originProjectID string, nueva PublicacionDelGrafo, nodes []GraphNode, edges []GraphEdge) error {
	projectID := originProjectID
	if projectID == "" {
		projectID = e.projectID
	}
	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("error al iniciar transacción de reemplazo del grafo: %w", err)
	}
	defer tx.Rollback()

	vigente, err := leerPublicacionDelGrafo(tx, projectID)
	if err != nil {
		return err
	}
	switch decision, motivo := decidirPublicacion(vigente, nueva); decision {
	case rechazarPublicacion:
		return fmt.Errorf("%w: %s", ErrGrafoMasViejo, motivo)
	case ignorarPublicacion:
		return fmt.Errorf("%w: %s", ErrGrafoIgnorado, motivo)
	}
	if err := reemplazarGrafoTx(tx, projectID, nodes, edges); err != nil {
		return err
	}
	if !nueva.Vacia() {
		if _, err := tx.Exec(
			`INSERT INTO meta (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
			metaPublicacionDelGrafo(projectID), nueva.Head+"|"+nueva.En.UTC().Format(time.RFC3339),
		); err != nil {
			return fmt.Errorf("error al registrar la publicación del grafo: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error al commitear el reemplazo del grafo: %w", err)
	}
	return nil
}
