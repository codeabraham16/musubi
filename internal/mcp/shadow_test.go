package mcp

import (
	"context"
	"errors"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// shadow_test.go defiende la única propiedad que hace aceptable meter un LLM al lado del detector:
// que su lectura NO PUEDA cambiar nada. Todo lo demás del modo sombra es contabilidad.

// servidorConSombra arma un servidor con el modo sombra encendido y dos observaciones que el
// detector va a relacionar. Devuelve el engine para poder mirar las dos tablas.
func servidorConSombra(t *testing.T, motor *motorEspia) (*McpServer, *memory.DbEngine) {
	t.Helper()
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })

	cfg := config.Default().Conflicts
	cfg.Shadow = config.ShadowConfig{Enabled: true, Queue: 8}
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{},
		WithCognition(motor),
		WithConflicts(cfg))
	if s.shadow == nil {
		t.Fatal("la sombra debería estar encendida con motor y config en true")
	}
	return s, engine
}

// parPendiente guarda dos observaciones casi idénticas y corre el detector en modo detectOnly, que
// fuerza el veredicto a `pending`: el par queda ESPERANDO que lo juzgue el agente.
//
// Es el escenario más filoso para probar la sombra. Si la lectura del motor pudiera escribir, lo
// primero que haría es resolver esa pendiente —tiene una opinión y el par está pidiendo una— y el
// agente nunca se enteraría de que alguien contestó por él.
func parPendiente(t *testing.T, s *McpServer, e *memory.DbEngine) memory.ObsRelation {
	t.Helper()
	texto := "el candado de despacho no cruza la red y se toma por proyecto"
	if err := e.SaveObservation("vieja", "tema/candado", texto, nil); err != nil {
		t.Fatalf("SaveObservation vieja: %v", err)
	}
	if err := e.SaveObservation("nueva", "tema/candado", texto+" (verificado)", nil); err != nil {
		t.Fatalf("SaveObservation nueva: %v", err)
	}
	rels, err := e.DetectRelations("nueva", s.conflictOpts(true))
	if err != nil {
		t.Fatalf("DetectRelations: %v", err)
	}
	if len(rels) == 0 {
		t.Fatal("el detector no relacionó dos observaciones casi idénticas: el test no probaría nada")
	}
	return rels[0]
}

// pendientes lista la cola de conflictos por la API pública, que es la que ve el agente.
func pendientes(t *testing.T, e *memory.DbEngine) []memory.ObsRelation {
	t.Helper()
	rels, err := e.PendingObsRelations()
	if err != nil {
		t.Fatalf("PendingObsRelations: %v", err)
	}
	return rels
}

// S1 — EL TEST QUE JUSTIFICA TODO EL MÓDULO. El motor contesta algo distinto de lo que el detector
// dejó pendiente, y el par SIGUE PENDIENTE. Si esto falla, el modo sombra dejó de ser una medición
// y pasó a ser un LLM resolviendo la cola del agente por la puerta de atrás.
func TestSombraNoPuedeResolverLaPendiente(t *testing.T) {
	motor := &motorEspia{respuesta: memory.RelNotConflict}
	s, e := servidorConSombra(t, motor)
	rel := parPendiente(t, s, e)
	if len(pendientes(t, e)) != 1 {
		t.Fatalf("el par debería estar pendiente antes de la sombra")
	}

	// Se procesa en la misma goroutine para no depender de tiempos: se prueba el EFECTO del
	// procesamiento, no que el worker despache rápido.
	s.shadow.procesar(context.Background(), trabajoSombra{rel: rel})

	if motor.llamadas.Load() != 1 {
		t.Fatalf("el motor debería haber sido consultado una vez, fueron %d", motor.llamadas.Load())
	}
	despues := pendientes(t, e)
	if len(despues) != 1 {
		t.Fatalf("la sombra vació la cola de conflictos: quedan %d pendientes, debía quedar 1", len(despues))
	}
	if despues[0].ID != rel.ID || despues[0].Status != memory.RelStatusPending {
		t.Errorf("la pendiente cambió: %+v", despues[0])
	}

	// Y sí quedó la evidencia, con el desacuerdo registrado.
	res, err := e.ShadowAgreementByRelation()
	if err != nil {
		t.Fatalf("ShadowAgreementByRelation: %v", err)
	}
	if len(res) != 1 || res[0].Total != 1 {
		t.Fatalf("debería haber exactamente un veredicto sombra: %+v", res)
	}
	if res[0].Agreed != 0 {
		t.Errorf("el motor dijo %q y el detector %q: eso es desacuerdo, no acuerdo", memory.RelNotConflict, rel.Relation)
	}
}

// S2 — un fallo del motor NO deja fila. Guardar un veredicto vacío lo contaría como desacuerdo y
// ensuciaría justo la tasa que la tabla existe para medir.
func TestSombraNoInventaVeredictoCuandoElMotorFalla(t *testing.T) {
	motor := &motorEspia{err: errors.New("motor caído")}
	s, e := servidorConSombra(t, motor)
	rel := parPendiente(t, s, e)

	s.shadow.procesar(context.Background(), trabajoSombra{rel: rel})

	res, err := e.ShadowAgreementByRelation()
	if err != nil {
		t.Fatalf("ShadowAgreementByRelation: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("un motor caído no debería dejar evidencia: %+v", res)
	}
}

// S3 — una respuesta fuera del vocabulario tampoco deja fila. Es el mismo riesgo que S2 con otra
// cara: una etiqueta inventada contamina la muestra en vez de faltar de ella.
func TestSombraDescartaLaRespuestaFueraDeVocabulario(t *testing.T) {
	motor := &motorEspia{respuesta: "me parece que se parecen bastante, che"}
	s, e := servidorConSombra(t, motor)
	rel := parPendiente(t, s, e)

	s.shadow.procesar(context.Background(), trabajoSombra{rel: rel})

	res, err := e.ShadowAgreementByRelation()
	if err != nil {
		t.Fatalf("ShadowAgreementByRelation: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("una respuesta ininteligible no debería contarse como veredicto: %+v", res)
	}
}

// S4 — la cola llena DESCARTA y no bloquea. Es la propiedad que mantiene el guardado con la
// latencia de siempre: si encolar pudiera esperar, un motor lento frenaría los saves.
func TestSombraConColaLlenaDescartaSinBloquear(t *testing.T) {
	motor := &motorEspia{respuesta: memory.RelRelated}
	s, _ := servidorConSombra(t, motor) // Queue: 8

	listo := make(chan struct{})
	go func() {
		defer close(listo)
		for i := 0; i < 100; i++ { // muy por encima del tope
			s.shadow.encolar(trabajoSombra{rel: memory.ObsRelation{ID: "r"}})
		}
	}()
	select {
	case <-listo:
	case <-time.After(5 * time.Second):
		t.Fatal("encolar se bloqueó con la cola llena: un motor lento frenaría los guardados")
	}

	s.shadow.mu.Lock()
	tirados := s.shadow.tirados
	s.shadow.mu.Unlock()
	if tirados != 92 { // 100 intentos - 8 de capacidad
		t.Errorf("debería haber descartado 92 y descartó %d: el conteo de descartes es lo que evita leer una muestra sesgada como completa", tirados)
	}
}

// S5 — apagada (el default), la sombra no existe: ni worker, ni motor consultado. Encender un
// tercer pilar por accidente sería gasto invisible.
func TestSombraApagadaNoConsultaAlMotor(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer engine.Close()
	motor := &motorEspia{respuesta: memory.RelRelated}
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{},
		WithCognition(motor),
		WithConflicts(config.Default().Conflicts)) // Shadow.Enabled queda en false

	if s.shadow != nil {
		t.Fatal("la sombra debería nacer apagada")
	}
	s.encolarSombra([]memory.ObsRelation{{ID: "r"}}) // no debe entrar en pánico ni hacer nada
	s.RunShadowWorker(context.Background())          // no-op: si corriera el bucle, colgaría el test
	if motor.llamadas.Load() != 0 {
		t.Errorf("con la sombra apagada el motor no se consulta, y se consultó %d vez/veces", motor.llamadas.Load())
	}
}

// S6 — la normalización acepta lo que el motor realmente contesta, y RECHAZA lo ambiguo. Un motor
// que nombra dos categorías no eligió; tratarlo como elección sería fabricar la etiqueta.
func TestNormalizarVeredicto(t *testing.T) {
	casos := []struct {
		entrada  string
		esperado string
		porque   string
	}{
		{"supersedes", memory.RelSupersedes, "la respuesta ideal"},
		{"  Supersedes  ", memory.RelSupersedes, "mayúsculas y espacios de más"},
		{"conflicts_with.", memory.RelConflictsWith, "puntuación pegada pese al prompt"},
		{"Creo que es related, la verdad", memory.RelRelated, "una coletilla con una sola categoría"},
		{"puede ser related o compatible", "", "nombró dos: no eligió"},
		{"ni idea", "", "no dijo ninguna"},
		{"", "", "vacío"},
	}
	for _, c := range casos {
		if got := normalizarVeredicto(c.entrada); got != c.esperado {
			t.Errorf("normalizarVeredicto(%q) = %q, esperaba %q (%s)", c.entrada, got, c.esperado, c.porque)
		}
	}
}
