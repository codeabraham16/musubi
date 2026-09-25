package memory

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// PublicacionDelGrafo dice DE QUÉ ÁRBOL es un grafo empujado al central: el commit que se indexó y
// su fecha de commit. Vacía = el emisor no lo dijo (un binario anterior a esta guarda, o un índice
// que no llegó a sellar su commit).
type PublicacionDelGrafo struct {
	Head string
	En   time.Time
	// Por es la CREDENCIAL que empujó la publicación: sale del principal, nunca del payload. Vacío
	// si no se sabe (loopback, una credencial legacy, o un push anterior a este campo).
	Por string
	// Huella es el sha256 del CONTENIDO del grafo (HuellaDelGrafo), calculado por el emisor. Con el
	// mismo Head y otra huella, dos máquinas describen el mismo commit distinto. Vacía = el emisor
	// no la mandó (un binario anterior a este campo).
	Huella string
}

// HuellaDelGrafo es el sha256 (hex) del CONTENIDO de un grafo: sus node_key y sus aristas
// (from, to, kind), ordenadas y sin repetir. No depende del orden en que se leyeron ni de las
// claves repetidas que el central colapsa al guardar, así que dos máquinas con el mismo árbol
// indexado dan la misma huella.
//
// ⚠️ POR QUÉ EXISTE. Medido el 2026-09-24: la laptop y esta PC tenían sellado el MISMO commit
// (eb2cdb7) y describían cosas distintas —12.053 nodos contra 13.329, porque esta PC indexa un
// archivo que git ignora—. La guarda de antigüedad acepta los dos (mismo head), así que el mapa
// del central iba y venía según cuál empujó última, siempre con la misma etiqueta. La huella es lo
// que deja verlo.
func HuellaDelGrafo(nodes []GraphNode, edges []GraphEdge) string {
	claves := make([]string, 0, len(nodes))
	for _, n := range nodes {
		claves = append(claves, n.Key)
	}
	aristas := make([]string, 0, len(edges))
	for _, ed := range edges {
		aristas = append(aristas, ed.FromKey+"\x1f"+ed.ToKey+"\x1f"+ed.Kind)
	}
	sort.Strings(claves)
	sort.Strings(aristas)
	h := sha256.New()
	for _, lista := range [][]string{claves, aristas} {
		for i, s := range lista {
			if i > 0 && s == lista[i-1] {
				continue
			}
			h.Write([]byte(s))
			h.Write([]byte{'\n'})
		}
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ConteoDelGrafo es cuánto hay GUARDADO de un proyecto: lo que el central contesta como recibo del
// push, en vez del largo de lo que recibió.
type ConteoDelGrafo struct {
	Nodes, Edges, Gists int
}

// ConteoDelGrafoDe cuenta nodos, aristas y gists de un proyecto ("" = el del engine) en UNA
// sentencia, así los tres números son del mismo instante.
func (e *DbEngine) ConteoDelGrafoDe(projectID string) (ConteoDelGrafo, error) {
	if projectID == "" {
		projectID = e.projectID
	}
	var c ConteoDelGrafo
	err := e.db.QueryRow(`SELECT
		(SELECT COUNT(*) FROM code_graph_nodes WHERE project_id=?),
		(SELECT COUNT(*) FROM code_graph_edges WHERE project_id=?),
		(SELECT COUNT(*) FROM code_memory WHERE project_id=?)`,
		projectID, projectID, projectID).Scan(&c.Nodes, &c.Edges, &c.Gists)
	if err != nil {
		return ConteoDelGrafo{}, fmt.Errorf("error al contar el grafo de %q: %w", projectID, err)
	}
	return c, nil
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

// metaFirmaDeLaPublicacion es la clave APARTE, por proyecto, con QUIÉN publicó y la HUELLA de lo
// publicado: "head|por|huella". Va aparte a propósito: si se sumara un campo a "head|fecha" y el
// central volviera a un binario anterior, ése leería "fecha|por" como fecha, time.Parse fallaría,
// la publicación se leería vacía y la guarda aceptaría cualquier árbol viejo. El head adentro de
// la firma es para no atribuirle a alguien una publicación que no hizo: un binario anterior
// reescribe la publicación sin tocar esta clave, y sólo vale si su head es el vigente.
func metaFirmaDeLaPublicacion(projectID string) string {
	return "codegraph_publicado_por:" + projectID
}

// firmaDeLaPublicacion desarma "head|por|huella" y devuelve por y huella sólo si la firma es del
// head vigente. Ni el head ni la huella llevan '|', así que el por es todo lo del medio.
func firmaDeLaPublicacion(firma, headVigente string) (por, huella string) {
	head, resto, ok := strings.Cut(firma, "|")
	if !ok || head != headVigente {
		return "", ""
	}
	i := strings.LastIndex(resto, "|")
	if i < 0 {
		return "", ""
	}
	return resto[:i], resto[i+1:]
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
//
// Cuando se sabe quién publicó lo vigente, los motivos lo dicen: el emisor rechazado sabe así qué
// máquina tiene el árbol que ganó. Y el mismo commit con OTRA huella se acepta —no hay forma de
// saber cuál de las dos describe bien el commit— pero el motivo vuelve como AVISO (ver
// AvisoDePublicacion): con la decisión «aceptar», el motivo no vacío es un aviso, no un rechazo.
func decidirPublicacion(vigente, nueva PublicacionDelGrafo) (decisionPublicacion, string) {
	if vigente.Vacia() {
		return aceptarPublicacion, ""
	}
	quien := ""
	if vigente.Por != "" {
		quien = ", lo publicó " + vigente.Por
	}
	if nueva.Vacia() {
		return ignorarPublicacion, fmt.Sprintf("el central tiene publicado el grafo de %s (%s%s) y este push no dice de qué commit es: "+
			"lo mandó un binario anterior a esta guarda, que hay que actualizar; no se tocó nada", vigente.Head, vigente.En.UTC().Format(time.RFC3339), quien)
	}
	if nueva.Head == vigente.Head {
		if nueva.Huella != "" && vigente.Huella != "" && nueva.Huella != vigente.Huella {
			return aceptarPublicacion, fmt.Sprintf("el central ya tenía publicado %s con OTRO contenido (%s%s): dos índices del mismo "+
				"commit no coinciden y gana el último que empuja. Lo típico son dos máquinas y un archivo que una indexa y la "+
				"otra no (ignorado por git o sin commitear)", vigente.Head, vigente.En.UTC().Format(time.RFC3339), quien)
		}
		return aceptarPublicacion, ""
	}
	if nueva.En.Before(vigente.En) {
		return rechazarPublicacion, fmt.Sprintf("el grafo empujado es de %s (%s) y el publicado es de %s (%s%s), más nuevo: "+
			"se conserva el publicado", nueva.Head, nueva.En.UTC().Format(time.RFC3339), vigente.Head, vigente.En.UTC().Format(time.RFC3339), quien)
	}
	return aceptarPublicacion, ""
}

// AvisoDePublicacion dice si un push que la guarda ACEPTA merece un aviso: hoy, el mismo commit que
// el publicado con otra huella. "" si no hay nada que avisar o si el push no se aceptaría.
func AvisoDePublicacion(vigente, nueva PublicacionDelGrafo) string {
	if decision, motivo := decidirPublicacion(vigente, nueva); decision == aceptarPublicacion {
		return motivo
	}
	return ""
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
	pub := PublicacionDelGrafo{Head: head, En: en}
	var firma string
	switch err := q.QueryRow(`SELECT value FROM meta WHERE key = ?`, metaFirmaDeLaPublicacion(projectID)).Scan(&firma); {
	case err == sql.ErrNoRows:
	case err != nil:
		return PublicacionDelGrafo{}, fmt.Errorf("error al leer la firma de la publicación del grafo: %w", err)
	default:
		pub.Por, pub.Huella = firmaDeLaPublicacion(firma, head)
	}
	return pub, nil
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
	// La firma se escribe SIEMPRE que se acepta, aunque Por venga vacío (loopback, legacy): si sólo
	// se escribiera con Por, quedaría la del publicador anterior atribuyéndole este push.
	if _, err := tx.Exec(
		`INSERT INTO meta (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		metaFirmaDeLaPublicacion(projectID), nueva.Head+"|"+nueva.Por+"|"+nueva.Huella,
	); err != nil {
		return fmt.Errorf("error al registrar la firma de la publicación del grafo: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error al commitear el reemplazo del grafo: %w", err)
	}
	return nil
}
