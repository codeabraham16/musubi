package memory

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// UN LOTE QUE CRECE EN RONDAS SE RECLAMA EN EL ORDEN EN QUE SE POSTEÓ.
//
// El reclamo va por `seq`. Con CreateWorkBatch cada ronda empezaba en 0, y la unidad de la segunda
// ronda se colaba delante de la segunda de la primera.
//
// Sabotaje que la hace fallar: numerar cada ronda desde cero.
// arnes: archivo="internal/memory/work.go"
// arnes: de="\t\t\tuuid.NewString(), batchID, desde+i, s.Title, s.Spec, WorkOpen, nivel,\n"
// arnes: a="\t\t\tuuid.NewString(), batchID, i, s.Title, s.Spec, WorkOpen, nivel,\n"
func TestSumarAlLoteContinuaLaSecuencia(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.SumarAlLote(LoteCuarentena, []WorkUnitSpec{{Title: "a"}, {Title: "b"}}); err != nil {
		t.Fatalf("primera ronda: %v", err)
	}
	b, err := e.SumarAlLote(LoteCuarentena, []WorkUnitSpec{{Title: "c"}})
	if err != nil {
		t.Fatalf("segunda ronda: %v", err)
	}
	if b.Total != 3 {
		t.Fatalf("el lote tiene %d unidades; quería 3", b.Total)
	}
	for _, quiere := range []string{"a", "b", "c"} {
		u, ok, err := e.ClaimWorkUnit(LoteCuarentena, "musubi-tareas", 300, 5)
		if err != nil || !ok {
			t.Fatalf("reclamar %s: ok=%v err=%v", quiere, ok, err)
		}
		if u.Title != quiere {
			t.Fatalf("se reclamó %q cuando tocaba %q: la ronda nueva se coló delante de la vieja", u.Title, quiere)
		}
	}
}

// UN LOTE DEL SISTEMA LO TOMA SÓLO QUIEN LO NOMBRA.
//
// El reclamo «de cualquier lote» es el de un sub-agente de una orquestación propia, y el lote activo
// es lo que el hook del turno le recuerda al agente principal que consolide. Ninguno de los dos es
// para el trabajo que postea Musubi: uno se llevaría una unidad de la cuarentena sin saber qué es, y
// el otro mandaría al principal a consolidar algo que no pidió.
//
// Sabotaje que la hace fallar: que el reclamo sin lote tome también los del sistema.
// arnes: archivo="internal/memory/work.go"
// arnes: de="` AND batch_id NOT LIKE ? ORDER BY created_at, seq, rowid LIMIT 1)"
// arnes: a="` AND (batch_id NOT LIKE ? OR 1) ORDER BY created_at, seq, rowid LIMIT 1)"
//
// Sabotaje que la hace fallar: que el lote activo pueda ser uno del sistema.
// arnes: archivo="internal/memory/work.go"
// arnes: de="\t\tWHERE status IN (?, ?) AND batch_id NOT LIKE ?\n"
// arnes: a="\t\tWHERE status IN (?, ?) AND (batch_id NOT LIKE ? OR 1)\n"
func TestLosLotesDelSistemaSoloLosTomaQuienLosNombra(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.SumarAlLote(LoteCuarentena, []WorkUnitSpec{{Title: "del-sistema"}}); err != nil {
		t.Fatal(err)
	}
	if b, ok, err := e.ActiveBatch(); err != nil || ok {
		t.Errorf("con sólo el lote del sistema, el lote activo es %q (ok=%v, err=%v): el principal no lo orquestó", b.BatchID, ok, err)
	}
	if u, ok, err := e.ClaimWorkUnit("", "orquestado", 300, 5); err != nil || ok {
		t.Errorf("el reclamo sin lote tomó %q (ok=%v, err=%v): el trabajo del sistema lo hace quien lo nombra", u.Title, ok, err)
	}

	// CONTROL: con un lote propio, el lote activo y el reclamo sin lote son ése.
	if _, err := e.CreateWorkBatch("mio", []WorkUnitSpec{{Title: "propia"}}); err != nil {
		t.Fatal(err)
	}
	if b, ok, _ := e.ActiveBatch(); !ok || b.BatchID != "mio" {
		t.Errorf("control: el lote activo es %q (ok=%v); quería «mio»", b.BatchID, ok)
	}
	if u, ok, _ := e.ClaimWorkUnit("", "orquestado", 300, 5); !ok || u.Title != "propia" {
		t.Errorf("control: el reclamo sin lote tomó %q (ok=%v); quería «propia»", u.Title, ok)
	}
	// Nombrándolo, sí.
	if u, ok, _ := e.ClaimWorkUnit(LoteCuarentena, "musubi-tareas", 300, 5); !ok || u.Title != "del-sistema" {
		t.Errorf("nombrando el lote se reclamó %q (ok=%v); quería «del-sistema»", u.Title, ok)
	}
}

// estadoDe lee lo que importa de una observación para estas pruebas.
func estadoDe(t *testing.T, e *DbEngine, id string) (archivada, enCuarentena bool, reemplazadaPor string) {
	t.Helper()
	var a, q int
	var sup *string
	if err := e.db.QueryRow(`SELECT archived, quarantined, superseded_by FROM observations WHERE id=?`, id).Scan(&a, &q, &sup); err != nil {
		t.Fatalf("leer %s: %v", id, err)
	}
	if sup != nil {
		reemplazadaPor = *sup
	}
	return a == 1, q == 1, reemplazadaPor
}

func idsDe(ps []PropuestaEnCuarentena) []string {
	out := []string{}
	for _, p := range ps {
		out = append(out, p.ID)
	}
	return out
}

// DESCARTAR ARCHIVA SÓLO UNA PROPUESTA VIVA, Y SE LLEVA A QUIÉN LA REEMPLAZA.
//
// Es el veredicto que faltaba: una propuesta equivocada o superada no tenía cómo salir sin volverse
// visible. Tiene que archivar sin corroborar, anotar la que la reemplaza, sacarla del listado, y no
// servir de puerta lateral para ocultar una nota visible.
//
// Sabotaje que la hace fallar: que descartar alcance a una nota que no está en cuarentena.
// arnes: archivo="internal/memory/tareas_del_sistema.go"
// arnes: de="\t\t WHERE id = ? AND quarantined = 1 AND archived = 0`, reemplazadaPor, id)\n"
// arnes: a="\t\t WHERE id = ?`, reemplazadaPor, id)\n"
// arnes: colision_ok="TestDescartarArchivaSoloUnaPropuestaViva"
//
// Sabotaje que la hace fallar: no anotar la que la reemplaza.
// arnes: archivo="internal/memory/tareas_del_sistema.go"
// arnes: de="archived = 0`, reemplazadaPor, id)\n"
// arnes: a="archived = 0`, \"\", id)\n"
// arnes: colision_ok="TestDescartarArchivaSoloUnaPropuestaViva"
//
// Sabotaje que la hace fallar: aceptar que se reemplace a sí misma.
// arnes: archivo="internal/memory/tareas_del_sistema.go"
// arnes: de="\t\tif reemplazadaPor == id {\n"
// arnes: a="\t\tif false {\n"
//
// Sabotaje que la hace fallar: aceptar un reemplazo que no existe.
// arnes: archivo="internal/memory/tareas_del_sistema.go"
// arnes: de="\t\tif existe == 0 {\n\t\t\treturn fmt.Errorf(\"%w: la que la reemplaza"
// arnes: a="\t\tif false {\n\t\t\treturn fmt.Errorf(\"%w: la que la reemplaza"
func TestDescartarArchivaSoloUnaPropuestaViva(t *testing.T) {
	e := newTestEngine(t)
	vieja := proponerObs(t, e, "t/estado", "estado viejo del trabajo")
	nueva := proponerObs(t, e, "t/estado", "estado nuevo del trabajo")
	otra := proponerObs(t, e, "t/otro", "otra cosa")

	ps, err := e.PropuestasEnCuarentena()
	if err != nil {
		t.Fatal(err)
	}
	if got := idsDe(ps); !reflect.DeepEqual(got, []string{vieja, nueva, otra}) {
		t.Fatalf("el listado es %v; quería las tres, de la más vieja a la más nueva", got)
	}

	if err := e.DescartarPropuesta(vieja, nueva); err != nil {
		t.Fatalf("descartar la vieja por la nueva: %v", err)
	}
	if a, q, sup := estadoDe(t, e, vieja); !a || !q || sup != nueva {
		t.Errorf("la descartada quedó archivada=%v, en cuarentena=%v, reemplazada por %q; quería archivada, "+
			"todavía en cuarentena (descartar no la verifica) y reemplazada por %s", a, q, sup, nueva)
	}
	if ps, _ := e.PropuestasEnCuarentena(); !reflect.DeepEqual(idsDe(ps), []string{nueva, otra}) {
		t.Errorf("después de descartar, el listado es %v; quería sólo las vivas", idsDe(ps))
	}

	// Una nota visible no se descarta: sería ocultar memoria que ya salió de la cuarentena.
	if err := e.CorroborateObservation(otra); err != nil {
		t.Fatal(err)
	}
	if err := e.DescartarPropuesta(otra, ""); !errors.Is(err, ErrNotQuarantined) {
		t.Errorf("descartar una nota visible dio %v; quería ErrNotQuarantined", err)
	}
	if a, _, _ := estadoDe(t, e, otra); a {
		t.Error("descartar archivó una nota VISIBLE")
	}
	// Dos veces, tampoco.
	if err := e.DescartarPropuesta(vieja, ""); !errors.Is(err, ErrNotQuarantined) {
		t.Errorf("descartar dos veces dio %v; quería ErrNotQuarantined", err)
	}

	// Un reemplazo que no existe o que es ella misma se rechaza, y no se archiva nada.
	if err := e.DescartarPropuesta(nueva, "no-existe"); !errors.Is(err, ErrObservationNotFound) {
		t.Errorf("reemplazada por una que no existe dio %v; quería ErrObservationNotFound", err)
	}
	if err := e.DescartarPropuesta(nueva, nueva); err == nil {
		t.Error("una propuesta se reemplazó a sí misma")
	}
	if a, _, _ := estadoDe(t, e, nueva); a {
		t.Fatal("un descarte rechazado igual archivó la propuesta")
	}

	// Sin reemplazo (repite una nota visible): archivada, sin superseded_by.
	if err := e.DescartarPropuesta(nueva, ""); err != nil {
		t.Fatalf("descartar sin reemplazo: %v", err)
	}
	if a, _, sup := estadoDe(t, e, nueva); !a || sup != "" {
		t.Errorf("descartada sin reemplazo: archivada=%v, reemplazada por %q", a, sup)
	}
	if err := e.DescartarPropuesta("no-existe", ""); !errors.Is(err, ErrObservationNotFound) {
		t.Errorf("descartar una que no existe dio %v; quería ErrObservationNotFound", err)
	}
}

// DESCARTAR NO CRUZA PROYECTOS, NI POR LA PROPUESTA NI POR LA QUE LA REEMPLAZA.
//
// Sabotaje que la hace fallar: no mirar el proyecto de la que la reemplaza.
// arnes: archivo="internal/memory/tareas_del_sistema.go"
// arnes: de="\tfor _, cual := range []string{id, reemplazadaPor} {\n"
// arnes: a="\tfor _, cual := range []string{id} {\n"
//
// Sabotaje que la hace fallar: no mirar el proyecto.
// arnes: archivo="internal/memory/tareas_del_sistema.go"
// arnes: de="\t\tif !ok {\n\t\t\treturn fmt.Errorf(\"%w: no se puede descartar"
// arnes: a="\t\tif !ok && false {\n\t\t\treturn fmt.Errorf(\"%w: no se puede descartar"
func TestDescartarNoCruzaProyectos(t *testing.T) {
	e := newTestEngine(t)
	deB, err := e.ProposeObservation("proj-B", "", "t/x", "propuesta de B", "m", 0.7, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	deA, err := e.ProposeObservation("proj-A", "", "t/x", "propuesta de A", "m", 0.7, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctxA := WithProjectScope(context.Background(), ProjectScope{ProjectID: "proj-A"})
	if err := e.DescartarPropuestaCtx(ctxA, deB, ""); !errors.Is(err, ErrCrossTenant) {
		t.Errorf("A descartó una propuesta de B: %v", err)
	}
	if err := e.DescartarPropuestaCtx(ctxA, deA, deB); !errors.Is(err, ErrCrossTenant) {
		t.Errorf("A descartó la suya como reemplazada por una de B: %v", err)
	}
	if a, _, _ := estadoDe(t, e, deB); a {
		t.Fatal("la propuesta de B quedó archivada")
	}
	// El dueño sí.
	ctxB := WithProjectScope(context.Background(), ProjectScope{ProjectID: "proj-B"})
	if err := e.DescartarPropuestaCtx(ctxB, deB, ""); err != nil {
		t.Errorf("el dueño no pudo descartar la suya: %v", err)
	}
}
