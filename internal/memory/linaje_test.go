package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// Pruebas del LINAJE del acervo (memory/linaje.go): la vuelta ficha ↔ fuente sobre las aristas
// derived_from que escribe el destilador, y la herencia que la hace durable cuando el afilador o la
// consolidación funden una observación en otra.

const linajeProj = "musubi-design"

// sembrarLinaje guarda observaciones del acervo en el tenant de diseño: id → topic. El contenido es
// distinto por id y no contiene ningún otro id, así que ninguna consolidación las toma por gemelas.
func sembrarLinaje(t *testing.T, e *DbEngine, obs map[string]string) {
	t.Helper()
	for id, topic := range obs {
		if err := e.SaveObservationTypedFrom(linajeProj, "seed", id, topic, "contenido propio de "+id, 1.0, "semantic", "shared", nil); err != nil {
			t.Fatalf("sembrar %s: %v", id, err)
		}
	}
}

// derivar escribe la arista que escribe el destilador: ficha → blob, derived_from, resuelta.
func derivar(t *testing.T, e *DbEngine, ficha, blob string) {
	t.Helper()
	if _, err := e.UpsertObsRelation(ObsRelation{SourceID: ficha, TargetID: blob, Relation: RelDerivedFrom,
		Status: RelStatusResolved, ResolvedBy: "destilador", Confidence: 1.0}); err != nil {
		t.Fatalf("arista %s → %s: %v", ficha, blob, err)
	}
}

// fundirComoAntes deja una fusión como la dejaba ArchiveAsDuplicate ANTES del linaje durable:
// perdedor archivado y apuntando al canónico, y sus aristas donde estaban. Así están hoy en el central
// las 58 aristas que cuelgan de fichas y blobs fundidos.
func fundirComoAntes(t *testing.T, e *DbEngine, perdedor, canonico string) {
	t.Helper()
	if _, err := e.db.Exec(`UPDATE observations SET archived=1, archived_at=CURRENT_TIMESTAMP, superseded_by=? WHERE id=?`,
		canonico, perdedor); err != nil {
		t.Fatalf("fundir %s en %s: %v", perdedor, canonico, err)
	}
}

func idsLinaje(refs []LinajeRef) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.ID)
	}
	sort.Strings(out)
	return out
}

func linajeDe(t *testing.T, e *DbEngine, ids ...string) map[string]Linaje {
	t.Helper()
	lin, err := e.LinajeCtx(context.Background(), ids)
	if err != nil {
		t.Fatalf("LinajeCtx(%v): %v", ids, err)
	}
	return lin
}

// TestLinajeIdaYVuelta: de la ficha se llega al blob y del blob a sus fichas, con el topic de cada
// punta; una nota sin aristas no trae nada.
//
// Sabotaje que la hace fallar: la ida arranca de las aristas que ENTRAN a la raíz.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="JOIN observation_relations r ON r.source_id = a.id"
// arnes: a="JOIN observation_relations r ON r.target_id = a.id"
//
// Sabotaje que la hace fallar: la vuelta arranca de las aristas que SALEN de la raíz.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="JOIN observation_relations r ON r.target_id = b.id"
// arnes: a="JOIN observation_relations r ON r.source_id = b.id"
func TestLinajeIdaYVuelta(t *testing.T) {
	e := newTestEngine(t)
	sembrarLinaje(t, e, map[string]string{
		"b1": "ingested/web/x",
		"c1": "design-corpus/contraste",
		"c2": "design-corpus/espaciado",
		"n1": "notas/suelta",
	})
	derivar(t, e, "c1", "b1")
	derivar(t, e, "c2", "b1")

	lin := linajeDe(t, e, "c1", "b1", "n1")

	if got := idsLinaje(lin["c1"].SalioDe); strings.Join(got, ",") != "b1" {
		t.Errorf("la ficha c1 tiene que decir que salió de b1; salio_de=%v", got)
	}
	if len(lin["c1"].SalioDe) == 1 && lin["c1"].SalioDe[0].TopicKey != "ingested/web/x" {
		t.Errorf("la punta tiene que traer el topic del blob; trajo %q", lin["c1"].SalioDe[0].TopicKey)
	}
	if len(lin["c1"].DestiladoEn) != 0 {
		t.Errorf("de una ficha no se destiló nada; destilado_en=%v", idsLinaje(lin["c1"].DestiladoEn))
	}
	if got := idsLinaje(lin["b1"].DestiladoEn); strings.Join(got, ",") != "c1,c2" {
		t.Errorf("del blob b1 salieron c1 y c2; destilado_en=%v", got)
	}
	if len(lin["b1"].SalioDe) != 0 {
		t.Errorf("un blob crudo no salió de nada; salio_de=%v", idsLinaje(lin["b1"].SalioDe))
	}
	if l, ok := lin["n1"]; ok && !l.Vacio() {
		t.Errorf("una nota sin aristas no tiene linaje; trajo %+v", l)
	}
}

// TestLinajeSigueLaFusionVieja cubre lo que ya está fundido SIN la herencia: el estado de hoy en el
// central (38 aristas desde fichas fundidas, 20 hacia blobs fundidos). La lectura tiene que seguir
// superseded_by en las dos puntas: del blob se llega a la ficha VIVA y no a la archivada, la
// canónica hereda las fuentes de la que absorbió, y lo mismo del lado del blob.
//
// Sabotaje que la hace fallar: no seguir ningún superseded_by.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="const linajeSaltos = 8"
// arnes: a="const linajeSaltos = 0"
//
// Sabotaje que la hace fallar: no resolver el vecino a su versión viva.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="WHERE o.superseded_by IS NOT NULL AND v.salto < ?"
// arnes: a="WHERE o.superseded_by IS NOT NULL AND v.salto < ? AND 0"
//
// Sabotaje que la hace fallar: la raíz no junta lo que se fundió en ella.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="JOIN fundida f ON f.en = x.id"
// arnes: a="JOIN fundida f ON f.en = x.id AND 0"
//
// Sabotaje que la hace fallar: sacar el filtro de visibilidad de la punta.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="visibleObsPredicateDe(\"t\") + sc"
// arnes: a="\"1=1\" + sc"
func TestLinajeSigueLaFusionVieja(t *testing.T) {
	e := newTestEngine(t)
	sembrarLinaje(t, e, map[string]string{
		"b1": "ingested/web/uno", "b2": "ingested/web/dos",
		"c2": "design-corpus/gemela-debil", "c3": "design-corpus/gemela-fuerte",
		"bX": "ingested/web/repetido", "bY": "ingested/web/original",
		"cX": "design-corpus/del-repetido",
	})
	// Lado ficha: c2 salió de b1, c3 de b2, y el afilador fundió c2 en c3.
	derivar(t, e, "c2", "b1")
	derivar(t, e, "c3", "b2")
	fundirComoAntes(t, e, "c2", "c3")
	// Lado blob: cX salió de bX, y bX se fundió en bY.
	derivar(t, e, "cX", "bX")
	fundirComoAntes(t, e, "bX", "bY")

	lin := linajeDe(t, e, "b1", "c3", "bY", "cX")

	if got := idsLinaje(lin["b1"].DestiladoEn); strings.Join(got, ",") != "c3" {
		t.Errorf("del blob b1 se tiene que llegar a la ficha VIVA c3, no a la archivada c2; destilado_en=%v", got)
	}
	if got := idsLinaje(lin["c3"].SalioDe); strings.Join(got, ",") != "b1,b2" {
		t.Errorf("c3 absorbió a c2 y tiene que heredar su fuente b1 además de la suya b2; salio_de=%v", got)
	}
	if got := idsLinaje(lin["bY"].DestiladoEn); strings.Join(got, ",") != "cX" {
		t.Errorf("bY absorbió a bX y tiene que traer la ficha que salió de bX; destilado_en=%v", got)
	}
	if got := idsLinaje(lin["cX"].SalioDe); strings.Join(got, ",") != "bY" {
		t.Errorf("cX salió de bX, que se fundió en bY: la fuente viva es bY; salio_de=%v", got)
	}
}

// TestLinajeTieneTope: un blob del que salieron muchas fichas no desborda la respuesta. Las
// referencias no pasan por el max_tokens de la expansión, así que el tope es lo único que las acota.
//
// Sabotaje que la hace fallar: subir el tope por encima de lo sembrado.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="const linajeTope = 12"
// arnes: a="const linajeTope = 100"
func TestLinajeTieneTope(t *testing.T) {
	e := newTestEngine(t)
	obs := map[string]string{"b1": "ingested/web/prolifico"}
	for i := 0; i < 15; i++ {
		obs[fmt.Sprintf("c%02d", i)] = fmt.Sprintf("design-corpus/ficha-%02d", i)
	}
	sembrarLinaje(t, e, obs)
	for i := 0; i < 15; i++ {
		derivar(t, e, fmt.Sprintf("c%02d", i), "b1")
	}
	// Las dos condiciones: que corte (menos que lo sembrado) y que corte en el tope declarado. Sólo la
	// segunda dejaría pasar un tope subido a 15, justo lo sembrado.
	if n := len(linajeDe(t, e, "b1")["b1"].DestiladoEn); n >= 15 || n != linajeTope {
		t.Errorf("del blob salieron 15 fichas y el linaje tiene que cortar en %d; devolvió %d", linajeTope, n)
	}
}

// TestLaFusionDelAfiladorDejaElLinajeDurable: después de ArchiveAsDuplicate y de la PURGA, que borra
// la fila archivada con sus aristas y sus punteros, el canónico sigue teniendo el linaje. Sin la
// herencia, lo único que lo sostenía era superseded_by, y la purga se lo lleva: a los 90 días en el
// central, sin ningún error. Y ANTES de la purga, con la arista original y la copiada conviviendo,
// cada punta sale una sola vez.
//
// Sabotaje que la hace fallar: ArchiveAsDuplicate no hereda el linaje.
// arnes: archivo="internal/memory/semdedup.go"
// arnes: de="if err := heredarLinaje(tx, loserID, canonicalID); err != nil {"
// arnes: a="if err := heredarLinaje(tx, loserID, loserID); err != nil {"
//
// Sabotaje que la hace fallar: no se copian las fuentes de una ficha fundida.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="WHERE r.source_id = ? AND r.target_id <> ?"
// arnes: a="WHERE r.source_id = ? AND r.target_id <> ? AND 0"
//
// Sabotaje que la hace fallar: no se re-apuntan las fichas de un blob fundido.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="WHERE r.target_id = ? AND r.source_id <> ?"
// arnes: a="WHERE r.target_id = ? AND r.source_id <> ? AND 0"
//
// Sabotaje que la hace fallar: heredar pisa la relación que el canónico ya tenía con esa punta.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="const heredarSinPisar = ` ON CONFLICT(source_id, target_id) DO NOTHING`"
// arnes: a="const heredarSinPisar = ` ON CONFLICT(source_id, target_id) DO UPDATE SET relation = excluded.relation`"
//
// Sabotaje que la hace fallar: sin el DISTINCT final, que es lo único que evita la punta repetida en
// los 90 días entre la fusión y la purga.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="SELECT DISTINCT v.dir"
// arnes: a="SELECT v.dir"
//
// Sabotaje que la hace fallar: la herencia copia todas las relaciones del perdedor, no sólo el linaje.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="const heredarSoloDerivedFrom = ` AND r.relation = ?`"
// arnes: a="const heredarSoloDerivedFrom = ` AND (r.relation = ? OR 1)`"
func TestLaFusionDelAfiladorDejaElLinajeDurable(t *testing.T) {
	e := newTestEngine(t)
	sembrarLinaje(t, e, map[string]string{
		"b1": "ingested/web/uno", "b2": "ingested/web/dos", "b9": "ingested/web/nueve",
		"c2": "design-corpus/gemela-debil", "c3": "design-corpus/gemela-fuerte",
		"bX": "ingested/web/repetido", "bY": "ingested/web/original",
		"cX": "design-corpus/del-repetido",
		"nR": "notas/relacionada-con-c2", "nZ": "notas/que-cita-a-bX",
	})
	derivar(t, e, "c2", "b1")
	derivar(t, e, "c3", "b2")
	// Dos relaciones que NO son linaje, una en cada dirección: c2 → nR y nZ → bX.
	for _, par := range [][2]string{{"c2", "nR"}, {"nZ", "bX"}} {
		if _, err := e.UpsertObsRelation(ObsRelation{SourceID: par[0], TargetID: par[1], Relation: RelRelated, Status: RelStatusResolved}); err != nil {
			t.Fatal(err)
		}
	}
	// c2 también salió de b9, pero c3 ya tenía con b9 un veredicto propio. Ese par no se pisa.
	derivar(t, e, "c2", "b9")
	if _, err := e.UpsertObsRelation(ObsRelation{SourceID: "c3", TargetID: "b9", Relation: RelRelated, Status: RelStatusResolved}); err != nil {
		t.Fatal(err)
	}
	derivar(t, e, "cX", "bX")

	for _, par := range [][2]string{{"c2", "c3"}, {"bX", "bY"}} {
		if ok, err := e.ArchiveAsDuplicate(linajeProj, par[0], par[1]); err != nil || !ok {
			t.Fatalf("fundir %s en %s: archived=%v err=%v", par[0], par[1], ok, err)
		}
	}
	// Se hereda el linaje y nada más: el `related` de c2 y el que le entraba a bX hablan de ellos, no
	// de c3 ni de bY, y nadie los juzgó para el canónico.
	var ajenas int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observation_relations
		WHERE (source_id = 'c3' AND target_id = 'nR') OR (source_id = 'nZ' AND target_id = 'bY')`).Scan(&ajenas); err != nil || ajenas != 0 {
		t.Errorf("la fusión sólo hereda derived_from; el canónico heredó %d relaciones que no son linaje (err %v)", ajenas, err)
	}

	// ANTES DE LA PURGA, que es como pasa sus primeros 90 días toda fusión nueva: conviven la arista
	// original del perdedor y la copiada en el canónico, así que la raíz llega al mismo vecino por dos
	// caminos con distinto salto (b1 ← c2 → c3 y b1 ← c3; cX → bX → bY y cX → bY). Cada punta sale UNA
	// vez. idsLinaje ordena pero no deduplica: una repetida se ve.
	antes := linajeDe(t, e, "b1", "cX", "c3", "bY")
	if got := idsLinaje(antes["b1"].DestiladoEn); strings.Join(got, ",") != "c3" {
		t.Errorf("antes de la purga, del blob b1 se llega a c3 una sola vez, aunque haya dos caminos; destilado_en=%v", got)
	}
	if got := idsLinaje(antes["cX"].SalioDe); strings.Join(got, ",") != "bY" {
		t.Errorf("antes de la purga, cX llega a bY una sola vez (por bX fundido y por la arista copiada); salio_de=%v", got)
	}
	if got := idsLinaje(antes["bY"].DestiladoEn); strings.Join(got, ",") != "cX" {
		t.Errorf("antes de la purga, bY trae a cX una sola vez; destilado_en=%v", got)
	}
	// b9 todavía está: la copia no pisó el `related` que c3 ya tenía con b9, y a esa punta la sostiene
	// sólo la arista de c2 por superseded_by. Es el costo de DO NOTHING, y se va con la purga (abajo).
	if got := idsLinaje(antes["c3"].SalioDe); strings.Join(got, ",") != "b1,b2,b9" {
		t.Errorf("antes de la purga, c3 trae sus fuentes y las de c2, cada una una vez; salio_de=%v", got)
	}

	// La purga cuenta desde archived_at: se lo corre al pasado para que venza ya, sin depender de
	// que el reloj de la prueba avance.
	if _, err := e.db.Exec(`UPDATE observations SET archived_at='2020-01-01 00:00:00' WHERE id IN ('c2','bX')`); err != nil {
		t.Fatal(err)
	}
	if n, err := e.PurgeArchived(1); err != nil || n != 2 {
		t.Fatalf("la purga tenía que borrar c2 y bX: borró %d (err %v). Sin purga, esta prueba no mide la herencia", n, err)
	}

	lin := linajeDe(t, e, "c3", "b1", "cX", "bY")
	if got := idsLinaje(lin["c3"].SalioDe); strings.Join(got, ",") != "b1,b2" {
		t.Errorf("tras la purga, c3 tiene que conservar la fuente b1 que heredó de c2; salio_de=%v", got)
	}
	if got := idsLinaje(lin["b1"].DestiladoEn); strings.Join(got, ",") != "c3" {
		t.Errorf("tras la purga, del blob b1 se tiene que seguir llegando a c3; destilado_en=%v", got)
	}
	if got := idsLinaje(lin["cX"].SalioDe); strings.Join(got, ",") != "bY" {
		t.Errorf("tras la purga de bX, cX tiene que apuntar a bY; salio_de=%v", got)
	}
	if got := idsLinaje(lin["bY"].DestiladoEn); strings.Join(got, ",") != "cX" {
		t.Errorf("tras la purga de bX, bY tiene que traer a cX; destilado_en=%v", got)
	}
	var rel string
	if err := e.db.QueryRow(`SELECT relation FROM observation_relations WHERE source_id='c3' AND target_id='b9'`).Scan(&rel); err != nil || rel != RelRelated {
		t.Errorf("el par c3→b9 era %q y tiene que seguir siéndolo; quedó %q (err %v)", RelRelated, rel, err)
	}
}

// TestLaConsolidacionDejaElLinajeDurable: la otra vía que funde, Consolidate, también hereda. Es la
// fusión por trigramas del mantenimiento, que corre sobre todos los tenants.
//
// Sabotaje que la hace fallar: Consolidate no hereda el linaje.
// arnes: archivo="internal/memory/consolidate.go"
// arnes: de="if err := heredarLinaje(tx, f.dupID, f.canonID); err != nil {"
// arnes: a="if err := heredarLinaje(tx, f.dupID, f.dupID); err != nil {"
func TestLaConsolidacionDejaElLinajeDurable(t *testing.T) {
	e := newTestEngine(t)
	saveAt(t, e, "a", "design-corpus/db", "Usamos PostgreSQL para la base de datos del sistema.", "2026-01-01 10:00:00")
	saveAt(t, e, "b", "design-corpus/db", "Usamos PostgreSQL para la base de datos del sistema productivo.", "2026-01-02 10:00:00")
	saveAt(t, e, "z", "ingested/web/fuente", "Un artículo crudo que habla de cualquier otra cosa: tipografía, márgenes y ritmo.", "2026-01-01 09:00:00")
	if _, err := e.db.Exec(`UPDATE observations SET access_count=10 WHERE id='a'`); err != nil {
		t.Fatal(err)
	}
	derivar(t, e, "b", "z")

	res, err := e.Consolidate(0.3)
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	var sup string
	if err := e.db.QueryRow(`SELECT COALESCE(superseded_by,'') FROM observations WHERE id='b'`).Scan(&sup); err != nil || sup != "a" {
		t.Fatalf("la consolidación tenía que fundir b en a (merged=%d, superseded_by=%q, err %v): sin fusión no hay nada que heredar", res.Merged, sup, err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET archived_at='2020-01-01 00:00:00' WHERE id='b'`); err != nil {
		t.Fatal(err)
	}
	if n, err := e.PurgeArchived(1); err != nil || n != 1 {
		t.Fatalf("la purga tenía que borrar b: borró %d (err %v)", n, err)
	}

	if got := idsLinaje(linajeDe(t, e, "a")["a"].SalioDe); strings.Join(got, ",") != "z" {
		t.Errorf("tras la purga de b, a tiene que conservar la fuente z que heredó; salio_de=%v", got)
	}
}

// sembrarTextos guarda observaciones del acervo con un texto elegido: id → {topic, texto}. Las pruebas
// de Consolidate lo necesitan porque la fusión la decide el texto.
func sembrarTextos(t *testing.T, e *DbEngine, obs map[string][2]string) {
	t.Helper()
	for id, tt := range obs {
		if err := e.SaveObservationTypedFrom(linajeProj, "seed", id, tt[0], tt[1], 1.0, "semantic", "shared", nil); err != nil {
			t.Fatalf("sembrar %s: %v", id, err)
		}
	}
}

// fundidaEn exige que Consolidate haya fundido perdedor en canonico: sin esa fusión, lo que sigue en
// la prueba no mide nada.
func fundidaEn(t *testing.T, e *DbEngine, perdedor, canonico string) {
	t.Helper()
	var sup string
	if err := e.db.QueryRow(`SELECT COALESCE(superseded_by,'') FROM observations WHERE id=?`, perdedor).Scan(&sup); err != nil || sup != canonico {
		t.Fatalf("control: Consolidate tenía que fundir %s en %s; superseded_by=%q (err %v)", perdedor, canonico, sup, err)
	}
}

// TestLaFichaFundidaConSuPropioBlob: Consolidate funde por trigramas, y una ficha cuyo texto quedó
// casi igual al de su blob se funde con él, en cualquiera de los dos sentidos. La herencia no escribe
// una arista de una observación hacia sí misma, y el linaje del sobreviviente no lo nombra a él como
// su propia fuente ni como su propia ficha.
//
// Sabotaje que la hace fallar: la copia de ida no saltea la punta igual al canónico.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="AND r.target_id <> ?"
// arnes: a="AND r.target_id <> ? || 'x'"
//
// Sabotaje que la hace fallar: la copia de vuelta no saltea la punta igual al canónico.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="AND r.source_id <> ?"
// arnes: a="AND r.source_id <> ? || 'x'"
//
// Sabotaje que la hace fallar: la raíz se lista como su propia punta.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="AND t.id <> v.raiz"
// arnes: a="AND t.id <> v.raiz || 'x'"
func TestLaFichaFundidaConSuPropioBlob(t *testing.T) {
	e := newTestEngine(t)
	sembrarTextos(t, e, map[string][2]string{
		// A: la ficha se funde en su blob.
		"zA": {"ingested/web/grilla", "La grilla de ocho puntos ordena el espaciado de toda la interfaz, sin excepciones."},
		"fA": {"design-corpus/grilla", "La grilla de ocho puntos ordena el espaciado de toda la interfaz, sin excepciones."},
		// B: el blob se funde en su ficha.
		"zB": {"ingested/web/acento", "Un solo acento de color dominante guía la mirada hacia la acción principal."},
		"fB": {"design-corpus/acento", "Un solo acento de color dominante guía la mirada hacia la acción principal."},
	})
	derivar(t, e, "fA", "zA")
	derivar(t, e, "fB", "zB")
	// Consolidate deja vivo al más fuerte: los accesos deciden el sentido de cada fusión.
	if _, err := e.db.Exec(`UPDATE observations SET access_count=10 WHERE id IN ('zA','fB')`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Consolidate(0); err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	fundidaEn(t, e, "fA", "zA")
	fundidaEn(t, e, "zB", "fB")

	var auto int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observation_relations WHERE source_id = target_id`).Scan(&auto); err != nil || auto != 0 {
		t.Errorf("la herencia escribió %d aristas de una observación hacia sí misma (err %v)", auto, err)
	}
	lin := linajeDe(t, e, "zA", "fB")
	for _, id := range []string{"zA", "fB"} {
		if l := lin[id]; !l.Vacio() {
			t.Errorf("%s absorbió a su par del linaje: no le queda fuente ni ficha que no sea él mismo; trajo salio_de=%v destilado_en=%v",
				id, idsLinaje(l.SalioDe), idsLinaje(l.DestiladoEn))
		}
	}
}

// TestElBlobQueAbsorbeAUnoDestiladoSaleDeLaCola: cuando un blob que nunca se destiló absorbe a uno
// que sí, hereda sus fichas y sale de la cola del destilador. Es una decisión, no un accidente (ver
// heredarLinaje): Consolidate fundió los dos porque sus textos son casi iguales, y destilar el
// canónico daría fichas gemelas de las que ya salieron del perdedor.
//
// Sabotaje que la hace fallar: la herencia no re-apunta las fichas del blob fundido.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="WHERE r.target_id = ? AND r.source_id <> ?"
// arnes: a="WHERE r.target_id = ? AND r.source_id <> ? AND 0"
func TestElBlobQueAbsorbeAUnoDestiladoSaleDeLaCola(t *testing.T) {
	e := newTestEngine(t)
	const texto = "El contraste mínimo del texto normal es de 4,5 a 1 contra su fondo, y de 3 a 1 para el texto grande."
	sembrarTextos(t, e, map[string][2]string{
		"bA": {"ingested/web/original", texto},
		"bB": {"ingested/web/repetido", texto},
		"cB": {"design-corpus/contraste", "Tarjeta: cuatro coma cinco para el cuerpo; tres para los titulares."},
	})
	derivar(t, e, "cB", "bB")
	// bA es el canónico (más accesos) y nunca se destiló.
	if _, err := e.db.Exec(`UPDATE observations SET access_count=10 WHERE id='bA'`); err != nil {
		t.Fatal(err)
	}
	cola := func() string {
		pend, err := e.ObservationsMissingRelation(linajeProj, "ingested/", RelDerivedFrom, 10)
		if err != nil {
			t.Fatalf("cola del destilador: %v", err)
		}
		ids := make([]string, 0, len(pend))
		for _, o := range pend {
			ids = append(ids, o.ID)
		}
		sort.Strings(ids)
		return strings.Join(ids, ",")
	}
	if got := cola(); got != "bA" {
		t.Fatalf("control: antes de fundir, bA es el único blob sin destilar; cola=%q", got)
	}

	if _, err := e.Consolidate(0); err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	fundidaEn(t, e, "bB", "bA")

	if got := cola(); got != "" {
		t.Errorf("bA absorbió a bB, que ya estaba destilado: hereda su ficha y sale de la cola; cola=%q", got)
	}
	if got := idsLinaje(linajeDe(t, e, "bA")["bA"].DestiladoEn); strings.Join(got, ",") != "cB" {
		t.Errorf("bA tiene que traer la ficha que salió de bB; destilado_en=%v", got)
	}
}
