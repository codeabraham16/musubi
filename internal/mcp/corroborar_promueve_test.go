package mcp

// CORROBORAR TAMBIÉN PROMUEVE (team mode) — "Automático, todo" (decisión del dueño, 2026-10-22).
//
// LO QUE SE PRUEBA, Y POR QUÉ. Hasta este cambio el camino propose→corroborate clavaba 'local': una
// inferencia de un LLM corroborada quedaba visible SÓLO en la máquina donde se corroboró y nunca
// viajaba al cerebro central ni a las otras máquinas del dueño — "mucha info se queda corta". El
// camino de *save* ya comparte solo en team mode (defaultScope, server.go); éste era el hueco.
//
// Los cuatro tests recorren la decisión completa: en team mode corroborar promueve y conserva el
// sello (T1), fuera de team mode se comporta como siempre (T2), la muralla multi-tenant sigue de
// pie (T3), y una promoción que la guarda del sobre bloquea NO des-corrobora la nota (T4). T1 es el
// invariante nuevo: el sabotaje de la fase verify quita PromoteObservationCtx del handler y T1 debe
// ponerse ROJO (scope sigue 'local', sin fila en el outbox).

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// servidorCorroborar arma un server sobre una base real sembrada, con el team mode pedido. Devuelve
// el engine (para sembrar la propuesta por fuera del borde MCP) y el dir (para leer la fila cruda).
func servidorCorroborar(t *testing.T, teamMode bool) (*McpServer, *memory.DbEngine, string) {
	t.Helper()
	dir := memtest.DirSembrado(t)
	engine, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	s := NewMcpServer(engine, dir, embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: teamMode}))
	return s, engine, dir
}

// duenoOwn es la credencial de la sala de mando: acotada a su propio proyecto (read=own/write=own).
// NO es federada a propósito — con background ctx (principal nil) el scope queda vacío y la muralla
// NO filtra (recallScopeFor), así que T3 no probaría nada. La muralla sólo se ejerce con un scope.
func duenoOwn() *Principal {
	return &Principal{Name: "davantis", ProjectID: "musubi", Read: ReadOwn, Write: WriteOwn}
}

// corroborar llama a musubi_corroborate con el principal dado y devuelve el texto del resultado (o
// el RpcError). Usa handleToolsCall con un ctx con principal, que es lo que `call` no permite: `call`
// corre con background ctx y scope vacío, y acá el scope es justo lo que se está probando.
func corroborar(t *testing.T, s *McpServer, p *Principal, id string) (string, *RpcError) {
	t.Helper()
	argBytes, _ := json.Marshal(map[string]interface{}{"id": id})
	params, _ := json.Marshal(CallToolRequest{Name: "musubi_corroborate", Arguments: argBytes})
	res, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), p), params)
	if rpcErr != nil {
		return "", rpcErr
	}
	resp, ok := res.(CallToolResponse)
	if !ok || len(resp.Content) == 0 {
		t.Fatalf("respuesta inesperada de corroborate: %#v", res)
	}
	return resp.Content[0].Text, nil
}

// filaObs lee scope/provenance/quarantined de una observación por SQL crudo, por fuera del engine
// (misma técnica que envenenar): afirma el ESTADO PERSISTIDO, no lo que el engine dice que hizo.
func filaObs(t *testing.T, dir, id string) (scope, provenance string, quarantined int) {
	t.Helper()
	ruta := filepath.Join(dir, config.DirName, config.DBFile)
	db, err := sql.Open("sqlite", ruta+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("abrir la base cruda: %v", err)
	}
	defer db.Close()
	row := db.QueryRow(`SELECT scope, COALESCE(provenance,''), quarantined FROM observations WHERE id = ?`, id)
	if err := row.Scan(&scope, &provenance, &quarantined); err != nil {
		t.Fatalf("leer la fila %q: %v", id, err)
	}
	return
}

// T1 (R1/R2) — EN TEAM MODE, CORROBORAR PROMUEVE Y CONSERVA EL SELLO.
//
// Es el invariante nuevo y el objetivo del sabotaje: quitar PromoteObservationCtx del handler deja
// el scope en 'local' y el outbox vacío, y este test cae.
func TestCorroborarEnTeamModePromueveYConservaElSello(t *testing.T) {
	s, engine, dir := servidorCorroborar(t, true)
	id, err := engine.ProposeObservation("musubi", "davantis", "t/promo", "una inferencia del modelo que vale la pena compartir", "groq/llama-3.3", 0.8, "semantic", nil)
	if err != nil {
		t.Fatalf("ProposeObservation: %v", err)
	}
	// Precondición: una propuesta en cuarentena es 'local' y NO encola — si ya hubiera algo en el
	// outbox, el conteo de abajo no probaría que lo encoló la promoción.
	if p, _, _ := statsOf(t, s); p != 0 {
		t.Fatalf("precondición: una propuesta no debe encolar, hay %d pending", p)
	}

	txt, rpcErr := corroborar(t, s, duenoOwn(), id)
	if rpcErr != nil {
		t.Fatalf("corroborate devolvió error: %+v", rpcErr)
	}

	scope, prov, q := filaObs(t, dir, id)
	if q != 0 {
		t.Errorf("tras corroborar, quarantined=%d (esperaba 0: la nota debe ser visible)", q)
	}
	if scope != memory.ScopeShared {
		t.Errorf("scope=%q (esperaba %q: en team mode corroborar debe promover)", scope, memory.ScopeShared)
	}
	if prov != "llm:groq/llama-3.3" {
		t.Errorf("provenance=%q: la promoción NO debe reescribir el sello (esperaba \"llm:groq/llama-3.3\")", prov)
	}
	if p, _, _ := statsOf(t, s); p != 1 {
		t.Errorf("tras promover esperaba 1 fila pending en el outbox, hay %d", p)
	}
	if !strings.Contains(txt, "Promovida a compartida") {
		t.Errorf("el texto debe avisar que viaja al central, obtuve: %q", txt)
	}
}

// T2 (R3) — FUERA DE TEAM MODE, CORROBORAR NO PROMUEVE (comportamiento histórico preservado).
func TestCorroborarFueraDeTeamModeNoPromueve(t *testing.T) {
	s, engine, dir := servidorCorroborar(t, false)
	id, err := engine.ProposeObservation("musubi", "davantis", "t/promo", "otra inferencia del modelo", "groq/llama-3.3", 0.8, "semantic", nil)
	if err != nil {
		t.Fatalf("ProposeObservation: %v", err)
	}

	if _, rpcErr := corroborar(t, s, duenoOwn(), id); rpcErr != nil {
		t.Fatalf("corroborate devolvió error: %+v", rpcErr)
	}

	scope, _, q := filaObs(t, dir, id)
	if q != 0 {
		t.Errorf("quarantined=%d (esperaba 0: corroborar sí la hace visible)", q)
	}
	if scope != memory.ScopeLocal {
		t.Errorf("scope=%q (esperaba %q: sin team mode NO se promueve)", scope, memory.ScopeLocal)
	}
	if p, _, _ := statsOf(t, s); p != 0 {
		t.Errorf("sin team mode no debe encolar nada, hay %d pending", p)
	}
}

// T3 (R6) — LA MURALLA MULTI-TENANT SIGUE DE PIE: corroborar la nota de otro proyecto se rechaza.
//
// La propuesta nace atribuida a 'otro-proyecto'; la credencial está acotada a 'musubi'. El rechazo
// lo produce CorroborateObservationCtx (su propia guarda obsInScope), ANTES de llegar a promover: la
// nota ajena no se vuelve visible ni se promueve con sólo conocer su id.
func TestCorroborarRechazaObservacionDeOtroProyecto(t *testing.T) {
	s, engine, dir := servidorCorroborar(t, true)
	id, err := engine.ProposeObservation("otro-proyecto", "forastero", "t/promo", "inferencia de otro tenant", "groq/llama-3.3", 0.8, "semantic", nil)
	if err != nil {
		t.Fatalf("ProposeObservation: %v", err)
	}

	p := &Principal{Name: "davantis", ProjectID: "musubi", Read: ReadOwn, Write: WriteOwn}
	_, rpcErr := corroborar(t, s, p, id)
	if rpcErr == nil {
		t.Fatal("corroborar una observación de otro proyecto debía rechazarse")
	}
	if rpcErr.Code != codeUnauthorized {
		t.Errorf("código %d (esperaba %d, cross-tenant)", rpcErr.Code, codeUnauthorized)
	}

	// Intacta: sigue en cuarentena y local — ni visible ni promovida.
	scope, _, q := filaObs(t, dir, id)
	if q != 1 || scope != memory.ScopeLocal {
		t.Errorf("la observación ajena no debe cambiar: quarantined=%d scope=%q (esperaba 1/local)", q, scope)
	}
}

// T4 (R5) — EL ORDEN ES CORROBORAR→PROMOVER, Y LA PROMOCIÓN ES BEST-EFFORT.
//
// Nace sana (ProposeObservation rechazaría el sobre), queda en cuarentena, y recién ahí se envenena
// el contenido por fuera del engine — igual que culpa_del_llamador_test. Al corroborar, la guarda
// del sobre (dentro de PromoteObservation) bloquea el cruce a 'shared', PERO la corroboración ya
// hecha QUEDA: estado válido "visible local, no compartida", reintentable. No se des-corrobora lo
// que el usuario pidió.
func TestCorroborarConPromocionBloqueadaPorElSobreQuedaVisibleLocal(t *testing.T) {
	s, engine, dir := servidorCorroborar(t, true)
	id, err := engine.ProposeObservation("musubi", "davantis", "t/promo", "nace sana y después se envenena", "groq/llama-3.3", 0.8, "semantic", nil)
	if err != nil {
		t.Fatalf("ProposeObservation: %v", err)
	}
	envenenar(t, dir, id, contenidoConSobre)

	txt, rpcErr := corroborar(t, s, duenoOwn(), id)
	if rpcErr != nil {
		t.Fatalf("corroborate NO debe fallar: la corroboración es válida aunque la promoción se bloquee (%+v)", rpcErr)
	}

	scope, _, q := filaObs(t, dir, id)
	if q != 0 {
		t.Errorf("quarantined=%d (esperaba 0: la corroboración queda hecha)", q)
	}
	if scope != memory.ScopeLocal {
		t.Errorf("scope=%q: la guarda del sobre debe impedir el cruce a 'shared' (esperaba %q)", scope, memory.ScopeLocal)
	}
	if p, _, _ := statsOf(t, s); p != 0 {
		t.Errorf("una promoción bloqueada no debe encolar, hay %d pending", p)
	}
	if !strings.Contains(txt, "NO se promovió") {
		t.Errorf("el texto debe informar que se corroboró pero no se promovió, obtuve: %q", txt)
	}
}

// T5 — UNA PROPUESTA DESCARTADA NO SE RESCATA NI VIAJA.
//
// DescartarPropuesta deja archived=1 pero quarantined=1; antes del cambio, corroborar igual la
// volvía visible (quarantined=0) y el daño quedaba LOCAL. Desde que corroborar PROMUEVE en team
// mode, sin la guarda archived=0 esa nota descartada viajaría al cerebro central como nota viva. La
// guarda vive en CorroborateObservation (el WHERE), así que el rechazo pasa ANTES de promover:
// sigue local y no encola. Sabotaje: quitar `AND archived = 0 AND superseded_by IS NULL` del WHERE
// pone este test ROJO (scope='shared', pending=1).
func TestCorroborarNoRescataUnaPropuestaDescartada(t *testing.T) {
	s, engine, dir := servidorCorroborar(t, true)
	id, err := engine.ProposeObservation("musubi", "davantis", "t/promo", "propuesta que alguien descarta antes de corroborar", "groq/llama-3.3", 0.8, "semantic", nil)
	if err != nil {
		t.Fatalf("ProposeObservation: %v", err)
	}
	if err := engine.DescartarPropuesta(id, ""); err != nil {
		t.Fatalf("DescartarPropuesta: %v", err)
	}

	_, rpcErr := corroborar(t, s, duenoOwn(), id)
	if rpcErr == nil {
		t.Fatal("corroborar una propuesta ya descartada debía rechazarse")
	}
	if rpcErr.Code != codeInvalidParams {
		t.Errorf("código %d (esperaba %d: no hay nada corroborable)", rpcErr.Code, codeInvalidParams)
	}

	// Lo que de verdad importa: NO se promovió ni encoló — la nota descartada no viaja a la malla.
	scope, _, _ := filaObs(t, dir, id)
	if scope != memory.ScopeLocal {
		t.Errorf("una propuesta descartada no debe promoverse, scope=%q (esperaba %q)", scope, memory.ScopeLocal)
	}
	if p, _, _ := statsOf(t, s); p != 0 {
		t.Errorf("una propuesta descartada no debe encolar, hay %d pending", p)
	}
}
