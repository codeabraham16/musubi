package memory

import (
	"strings"
	"testing"
)

// work_autonomia_test.go cubre la escalera de autonomía de la pizarra (F2).
//
// Lo que estos tests defienden no es "existe una columna": es que el techo declarado en la unidad
// FRENE de verdad un cierre que se pasa de la raya, y que la firma que lo destraba valga sólo para
// el intento que se revisó. Un test que se conformara con leer el campo de vuelta pasaría igual
// con el gate borrado — que es exactamente la mutación vacía que documenta docs/failure-modes.md.

// unidadReclamada postea una unidad con el nivel dado y la deja reclamada por `agente`.
func unidadReclamada(t *testing.T, e *DbEngine, nivel, agente string) WorkUnit {
	t.Helper()
	b, err := e.CreateWorkBatch("", []WorkUnitSpec{{Title: "u", Spec: "hacer algo", Autonomy: nivel}})
	if err != nil {
		t.Fatalf("CreateWorkBatch(%s): %v", nivel, err)
	}
	u, ok, err := e.ClaimWorkUnit(b.BatchID, agente, 300, 5)
	if err != nil || !ok {
		t.Fatalf("ClaimWorkUnit(%s): ok=%v err=%v", nivel, ok, err)
	}
	return u
}

// A1: el default no cambia nada. Una unidad posteada sin declarar nivel es L3 y cierra sola,
// declarando o no el efecto — es el contrato de compatibilidad con todo lo que ya usa la pizarra.
func TestAutonomiaAusenteEsDesatendida(t *testing.T) {
	e := newTestEngine(t)
	u := unidadReclamada(t, e, "", "agente-1")
	if u.Autonomy != AutonomyUnattended {
		t.Fatalf("autonomía por defecto = %q, esperaba %q", u.Autonomy, AutonomyUnattended)
	}
	if err := e.CompleteWorkUnit(u.ID, "listo", WorkDone, "agente-1", u.FencingToken); err != nil {
		t.Errorf("una unidad sin nivel declarado debe cerrar como siempre: %v", err)
	}
}

// A2: el nivel se valida al POSTEAR, no al cerrar. Un nivel mal escrito que se descubriera recién
// al final dejaría al agente trabajando horas bajo un techo que nadie fijó.
func TestAutonomiaInvalidaSeRechazaAlPostear(t *testing.T) {
	e := newTestEngine(t)
	for _, malo := range []string{"L4", "L0", "alta", "1", "L1;--"} {
		if _, err := e.CreateWorkBatch("", []WorkUnitSpec{{Title: "u", Autonomy: malo}}); err == nil {
			t.Errorf("autonomy=%q debería rechazarse al postear", malo)
		}
	}
	// Las variantes legítimas de tipeo sí entran: el nivel se normaliza.
	for _, bueno := range []string{"l2", " L2 ", "L2"} {
		b, err := e.CreateWorkBatch("", []WorkUnitSpec{{Title: "u", Autonomy: bueno}})
		if err != nil {
			t.Fatalf("autonomy=%q debería aceptarse: %v", bueno, err)
		}
		if got := b.Units[0].Autonomy; got != AutonomyAssisted {
			t.Errorf("autonomy=%q se guardó como %q, esperaba %q", bueno, got, AutonomyAssisted)
		}
	}
	// Y un batch con UNA unidad inválida no deja las otras posteadas a medias.
	if _, err := e.CreateWorkBatch("mixto", []WorkUnitSpec{{Title: "ok"}, {Title: "mala", Autonomy: "L9"}}); err == nil {
		t.Fatal("un batch con una unidad inválida debe fallar entero")
	}
	if b, _ := e.WorkBatchStatus("mixto"); b.Total != 0 {
		t.Errorf("el batch rechazado dejó %d unidades posteadas; debía ser transaccional", b.Total)
	}
}

// A3: EL GATE. L1 sólo reporta — cerrar diciendo que aplicó cambios se rechaza, y el rechazo
// explica qué hacer. Cerrar como report sí se puede: reportar ES el entregable de una L1.
func TestL1FrenaElCierreQueAplica(t *testing.T) {
	e := newTestEngine(t)
	u := unidadReclamada(t, e, AutonomyReport, "agente-1")

	err := e.CompleteWorkUnitConEfecto(u.ID, "toqué el disco", WorkDone, "agente-1", u.FencingToken, EffectApply)
	if err == nil {
		t.Fatal("una unidad L1 no puede cerrarse declarando effect=apply")
	}
	if !strings.Contains(err.Error(), "L1") || !strings.Contains(err.Error(), "report") {
		t.Errorf("el rechazo debe decir el nivel y la salida; dijo: %v", err)
	}
	// El rechazo no dejó la unidad cerrada por las dudas.
	if b, _ := e.WorkBatchStatus(u.BatchID); b.Done != 0 {
		t.Fatalf("la unidad quedó cerrada pese al rechazo: %+v", b)
	}
	// NO DECLARAR EFECTO ES DECLARAR apply. Las dos formas de no declararlo tienen que frenar:
	// el atajo CompleteWorkUnit (que lo fija) y el efecto vacío que llega desde MCP cuando el
	// cliente omite el campo. La segunda es la que importa —es el camino real de un cliente que
	// no conoce la escalera— y un saboteo mostró que sin este caso el default quedaba sin probar:
	// invertirlo a "report" dejaba la suite entera en verde.
	if err := e.CompleteWorkUnit(u.ID, "sin declarar", WorkDone, "agente-1", u.FencingToken); err == nil {
		t.Error("el atajo CompleteWorkUnit debe frenar en L1 (fija effect=apply)")
	}
	if err := e.CompleteWorkUnitConEfecto(u.ID, "sin declarar", WorkDone, "agente-1", u.FencingToken, ""); err == nil {
		t.Error("un effect vacío debe tratarse como apply (fail-closed) y frenar en L1")
	}
	// Reportar sí cierra.
	if err := e.CompleteWorkUnitConEfecto(u.ID, "hallazgos", WorkDone, "agente-1", u.FencingToken, EffectReport); err != nil {
		t.Errorf("una L1 debe poder cerrar reportando: %v", err)
	}
}

// A4: L2 exige la firma de OTRO. Sin firma no cierra; firmada por el propio dueño tampoco —
// el maker/checker se vuelve teatro si el maker puede ser su propio checker.
func TestL2ExigeFirmaDeOtro(t *testing.T) {
	e := newTestEngine(t)
	u := unidadReclamada(t, e, AutonomyAssisted, "agente-1")

	if err := e.CompleteWorkUnitConEfecto(u.ID, "arreglado", WorkDone, "agente-1", u.FencingToken, EffectApply); err == nil {
		t.Fatal("una L2 sin firma no puede cerrarse con effect=apply")
	} else if !strings.Contains(err.Error(), "firma") {
		t.Errorf("el rechazo debe hablar de la firma que falta; dijo: %v", err)
	}

	if err := e.ApproveWorkUnit(u.ID, "agente-1"); err == nil {
		t.Error("el dueño no puede firmarse a sí mismo")
	}
	if err := e.ApproveWorkUnit(u.ID, "revisor"); err != nil {
		t.Fatalf("ApproveWorkUnit: %v", err)
	}
	if err := e.CompleteWorkUnitConEfecto(u.ID, "arreglado", WorkDone, "agente-1", u.FencingToken, EffectApply); err != nil {
		t.Errorf("con firma de otro, la L2 debe cerrar: %v", err)
	}
}

// A5: la firma aprueba UN INTENTO, no la unidad para siempre.
//
// Es el agujero que no se ve venir: la unidad se firma, al dueño se le vence el lease, otro agente
// la retoma y hace un trabajo que nadie miró — y cerraría amparado por la revisión del trabajo
// viejo. El fencing_token es lo que ata la firma a lo que se revisó.
func TestFirmaVenceAlCambiarDeManos(t *testing.T) {
	e := newTestEngine(t)
	u := unidadReclamada(t, e, AutonomyAssisted, "agente-1")
	if err := e.ApproveWorkUnit(u.ID, "revisor"); err != nil {
		t.Fatalf("ApproveWorkUnit: %v", err)
	}

	// Al dueño le vence el lease y otro agente retoma la unidad.
	ageLease(t, e, u.ID)
	u2, ok, err := e.ClaimWorkUnit(u.BatchID, "agente-2", 300, 5)
	if err != nil || !ok {
		t.Fatalf("re-claim: ok=%v err=%v", ok, err)
	}
	if u2.FencingToken == u.FencingToken {
		t.Fatal("el re-claim debía avanzar el fencing token")
	}

	err = e.CompleteWorkUnitConEfecto(u.ID, "trabajo nuevo", WorkDone, "agente-2", u2.FencingToken, EffectApply)
	if err == nil {
		t.Fatal("el trabajo del agente nuevo no puede cerrar con la firma del intento viejo")
	}
	if !strings.Contains(err.Error(), "VENCIDA") {
		t.Errorf("el rechazo debe decir que la firma venció; dijo: %v", err)
	}
	// Una firma nueva sobre el intento en curso sí lo destraba.
	if err := e.ApproveWorkUnit(u.ID, "revisor"); err != nil {
		t.Fatalf("re-firma: %v", err)
	}
	if err := e.CompleteWorkUnitConEfecto(u.ID, "trabajo nuevo", WorkDone, "agente-2", u2.FencingToken, EffectApply); err != nil {
		t.Errorf("con la firma renovada debe cerrar: %v", err)
	}
}

// A6: declararse fallida nunca se frena, en ningún nivel. Una L1 que no pudiera ni decir "no pude"
// quedaría colgada hasta que le venza el lease y se escalaría sola como un agente desaparecido:
// la señal equivocada, y encima gastando reintentos.
func TestFallarNoDependeDeLaAutonomia(t *testing.T) {
	e := newTestEngine(t)
	for _, nivel := range []string{AutonomyReport, AutonomyAssisted, AutonomyUnattended} {
		u := unidadReclamada(t, e, nivel, "agente-1")
		if err := e.CompleteWorkUnitConEfecto(u.ID, "no pude", WorkFailed, "agente-1", u.FencingToken, EffectApply); err != nil {
			t.Errorf("%s: cerrar como failed no debe frenarse: %v", nivel, err)
		}
	}
}

// A7: la firma sólo existe donde significa algo. En L3 no destraba nada y en L1 no hay nada que
// aplicar: aceptarla sería registrar una revisión que no gobierna ninguna decisión — verificación
// de adorno. Y sobre una unidad que nadie está haciendo sería una firma en blanco.
func TestFirmaSoloDondeGobierna(t *testing.T) {
	e := newTestEngine(t)
	for _, nivel := range []string{AutonomyReport, AutonomyUnattended} {
		u := unidadReclamada(t, e, nivel, "agente-1")
		if err := e.ApproveWorkUnit(u.ID, "revisor"); err == nil {
			t.Errorf("%s: la firma no debería aceptarse donde no gobierna nada", nivel)
		}
	}
	// Unidad posteada pero no reclamada: no hay intento que revisar.
	b, err := e.CreateWorkBatch("", []WorkUnitSpec{{Title: "u", Autonomy: AutonomyAssisted}})
	if err != nil {
		t.Fatalf("CreateWorkBatch: %v", err)
	}
	if err := e.ApproveWorkUnit(b.Units[0].ID, "revisor"); err == nil {
		t.Error("firmar una unidad que nadie reclamó debería rechazarse")
	}
	if err := e.ApproveWorkUnit("no-existe", "revisor"); err == nil {
		t.Error("firmar una unidad inexistente debería rechazarse")
	}
	if err := e.ApproveWorkUnit(b.Units[0].ID, ""); err == nil {
		t.Error("firmar sin revisor debería rechazarse")
	}
}

// A9: la separación maker/checker también vive en el SQL, no sólo en ApproveWorkUnit.
//
// Este test es WHITE-BOX a propósito: planta la firma directamente en la fila, salteando la única
// puerta que hoy rechaza que el dueño se firme solo. Lo detectó un saboteo — al borrar el término
// `approved_by <> owner_id` del gate, la suite entera seguía verde, porque el chequeo en Go tapaba
// al de SQL. Un invariante que sólo sostiene una capa deja de sostenerse el día que alguien
// escriba `approved_by` por otro camino (una reparación, una migración, un import). Acá se ejerce
// la capa de abajo sola.
func TestGateSqlRechazaLaAutofirmaAunquePlantada(t *testing.T) {
	e := newTestEngine(t)
	u := unidadReclamada(t, e, AutonomyAssisted, "agente-1")

	if _, err := e.db.Exec(
		`UPDATE work_units SET approved_by=owner_id, approved_at=datetime('now'), approved_token=fencing_token WHERE id=?`,
		u.ID); err != nil {
		t.Fatalf("no se pudo plantar la autofirma: %v", err)
	}
	err := e.CompleteWorkUnitConEfecto(u.ID, "me firmé solo", WorkDone, "agente-1", u.FencingToken, EffectApply)
	if err == nil {
		t.Fatal("el gate SQL debe rechazar una firma del propio dueño, aunque no haya pasado por ApproveWorkUnit")
	}
	if !strings.Contains(err.Error(), "el que hace no puede ser el que aprueba") {
		t.Errorf("el rechazo debe nombrar la separación que se violó; dijo: %v", err)
	}
}

// A8: un efecto que no se entiende se rechaza en vez de interpretarse. Si "aplicar" se colara como
// desconocido y el gate lo dejara pasar, el techo se destrabaría con un typo.
func TestEfectoDesconocidoSeRechaza(t *testing.T) {
	e := newTestEngine(t)
	u := unidadReclamada(t, e, AutonomyReport, "agente-1")
	if err := e.CompleteWorkUnitConEfecto(u.ID, "x", WorkDone, "agente-1", u.FencingToken, "aplicar"); err == nil {
		t.Fatal("un effect desconocido debe rechazarse, no interpretarse")
	}
	if b, _ := e.WorkBatchStatus(u.BatchID); b.Done != 0 {
		t.Errorf("la unidad cerró con un effect inválido: %+v", b)
	}
}
