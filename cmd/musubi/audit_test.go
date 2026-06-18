package main

import (
	"context"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/memory"
)

// audit_test.go audita el FOOTPRINT de tokens de Musubi: cuántos tokens inyecta
// en el contexto cada superficie (priming de arranque, recall por turno,
// hidratación) sobre un corpus realista, y demuestra el ahorro de la inyección
// diferencial (delta). Es a la vez un reporte (correlo con -v) y una guarda de
// regresión: si una superficie empieza a inyectar de más, el test falla.
//
//	go test ./cmd/musubi -run TestTokenConsumptionAudit -v
//
// IMPORTANTE: esto mide SOLO lo que Musubi inyecta. El grueso de los tokens de
// una sesión de agente (conversación, lecturas de archivos, outputs de tools,
// sub-agentes, system prompt) NO lo controla ni lo ve Musubi.
func TestTokenConsumptionAudit(t *testing.T) {
	root := t.TempDir()
	eng, err := memory.NewDbEngine(root)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer eng.Close()

	// Corpus realista: prosa, código y JSON, de tamaños variados, con un tema
	// común ("auth"/"base de datos") para que el recall por query recupere varios.
	seed := []struct{ id, topic, content string }{
		{"o1", "arch/auth", "Decidimos usar JWT con refresh tokens para la autenticación; el access token vive 15 minutos y el refresh 7 días."},
		{"o2", "arch/db", "La base de datos es PostgreSQL con migraciones versionadas; las conexiones usan un pool de 20 y timeout de 5s."},
		{"o3", "bugfix/auth", "Arreglado un bug donde el refresh token no rotaba: ahora se invalida el anterior al emitir uno nuevo en el login."},
		{"o4", "code/middleware", "func Auth(next http.Handler) http.Handler { return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { tok := r.Header.Get(\"Authorization\"); if !valid(tok) { w.WriteHeader(401); return }; next.ServeHTTP(w, r) }) }"},
		{"o5", "config/auth", "{\"auth\":{\"jwt_ttl\":900,\"refresh_ttl\":604800,\"issuer\":\"musubi\",\"algorithms\":[\"HS256\"]},\"db\":{\"pool\":20,\"timeout_ms\":5000}}"},
		{"o6", "decision/deploy", "El despliegue es por contenedores con health checks; un rolling update reemplaza de a una réplica para no cortar la base de datos."},
		{"o7", "arch/cache", "Cacheamos las sesiones de autenticación en Redis con TTL alineado al access token para evitar pegar a la base en cada request."},
		{"o8", "pattern/errors", "Convención: los errores de dominio se envuelven con %w y se loguean a stderr; nunca a stdout (ahí va el canal JSON-RPC)."},
		// Memorias GRANDES: acá es donde el gisting realmente ahorra (titular corto
		// en lugar de todo el documento).
		{"o9", "doc/auth-flow", "Flujo completo de autenticación. " + strings.Repeat("El cliente envía credenciales al endpoint de login, el servidor valida contra la base de datos, emite un access token JWT firmado y un refresh token opaco que se guarda hasheado; en cada request el middleware valida la firma y la expiración, y ante un 401 el cliente usa el refresh para obtener un nuevo par. ", 8)},
		{"o10", "doc/db-schema", "Esquema de la base de datos y decisiones de modelado. " + strings.Repeat("Las tablas usan claves primarias UUID, timestamps en UTC, soft-delete por columna archived, e índices parciales para las consultas calientes; las migraciones son idempotentes y reversibles. ", 8)},
	}
	totalFull := 0
	allIDs := make([]string, 0, len(seed))
	for _, s := range seed {
		if err := eng.SaveObservation(s.id, s.topic, s.content, nil); err != nil {
			t.Fatalf("SaveObservation %s: %v", s.id, err)
		}
		totalFull += memory.EstimateTokens(s.content)
		allIDs = append(allIDs, s.id)
	}
	t.Logf("Corpus: %d observaciones; inyectar TODO el contenido completo costaría %d tokens.", len(seed), totalFull)

	startup := config.Default().Startup
	loop := config.Default().Loop // delta_injection = true
	const sess = "sess-audit"

	// 1) Priming de arranque ------------------------------------------------
	primeRes, err := eng.PrimeContext(startup.RecallBudget)
	if err != nil {
		t.Fatalf("PrimeContext: %v", err)
	}
	primeBlock := buildPrimingContext(eng, startup.RecallBudget, sess)
	t.Logf("Priming: %d gists, %d tokens de gists (budget %d), bloque formateado %d tokens.",
		primeRes.Count, primeRes.UsedTokens, startup.RecallBudget, memory.EstimateTokens(primeBlock))
	if primeRes.UsedTokens > startup.RecallBudget {
		t.Errorf("el priming excede su budget: %d > %d", primeRes.UsedTokens, startup.RecallBudget)
	}

	// 2) Recall por turno ---------------------------------------------------
	// El recall (pre-delta) debe respetar su budget.
	const primedQuery = "cómo está la autenticación y la base de datos"
	recall, err := eng.Recall(context.Background(), primedQuery, memory.RecallOptions{TokenBudget: loop.RecallBudget, NoBump: true})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if recall.UsedTokens > loop.RecallBudget {
		t.Errorf("el recall por turno excede su budget: %d > %d", recall.UsedTokens, loop.RecallBudget)
	}

	// 2a) Turno sobre memoria YA primeada: el priming sembró el delta, así que NO
	//     debe repetirse (fin de la doble inyección priming↔turno).
	inPrimed := `{"prompt":"` + primedQuery + `","session_id":"` + sess + `"}`
	_, ctxP := hookAdditionalContext(t, turnOutput(eng, loop, pipeOff(), maOff(), strings.NewReader(inPrimed)))
	tP := memory.EstimateTokens(ctxP)
	t.Logf("Turno sobre memoria YA primeada: %d tokens (debería ser ~0: el priming ya la mostró).", tP)
	if tP != 0 {
		t.Errorf("el priming ya cubrió esto; el turno no debería repetirlo, inyectó %d tokens", tP)
	}

	// 2b) Memoria NUEVA (posterior al priming): su turno SÍ la inyecta; el segundo
	//     turno ya no (delta).
	if err := eng.SaveObservation("n1", "feature/ratelimit", "Implementamos rate limiting por IP con ventana deslizante en Redis para frenar fuerza bruta en el login.", nil); err != nil {
		t.Fatal(err)
	}
	inNew := `{"prompt":"rate limiting","session_id":"` + sess + `"}`
	_, ctxN1 := hookAdditionalContext(t, turnOutput(eng, loop, pipeOff(), maOff(), strings.NewReader(inNew)))
	_, ctxN2 := hookAdditionalContext(t, turnOutput(eng, loop, pipeOff(), maOff(), strings.NewReader(inNew)))
	tN1, tN2 := memory.EstimateTokens(ctxN1), memory.EstimateTokens(ctxN2)
	t.Logf("Memoria NUEVA: turno1=%d tokens, turno2 (delta)=%d tokens.", tN1, tN2)
	if tN1 == 0 {
		t.Error("una memoria nueva (no primeada) debería inyectarse en su turno")
	}
	if tN2 != 0 {
		t.Errorf("el segundo turno no debería repetir la memoria nueva (delta), inyectó %d tokens", tN2)
	}

	// 3) Hidratación: completa vs con tope ----------------------------------
	full, fullTok, err := eng.GetObservationsBudget(allIDs, 0)
	if err != nil {
		t.Fatalf("GetObservationsBudget(0): %v", err)
	}
	const hydrateCap = 120
	capped, capTok, err := eng.GetObservationsBudget(allIDs, hydrateCap)
	if err != nil {
		t.Fatalf("GetObservationsBudget(cap): %v", err)
	}
	t.Logf("Hidratación: completa=%d obs/%d tokens; con tope %d=%d obs/%d tokens.",
		len(full), fullTok, hydrateCap, len(capped), capTok)
	if capTok > hydrateCap {
		t.Errorf("la hidratación con tope excede el techo: %d > %d", capTok, hydrateCap)
	}

	// 4) Ledger de la sesión ------------------------------------------------
	led, err := eng.LedgerStatus()
	if err != nil {
		t.Fatalf("LedgerStatus: %v", err)
	}
	t.Logf("Ledger sesión %q: total=%d tokens, por superficie=%v.", led.SessionID, led.Total, led.Surfaces)
	if led.Total == 0 {
		t.Error("el ledger debería haber contabilizado el priming + el recall del turno 1")
	}
	sum := 0
	for _, v := range led.Surfaces {
		sum += v
	}
	if sum != led.Total {
		t.Errorf("el total del ledger (%d) no coincide con la suma por superficie (%d)", led.Total, sum)
	}

	// Resumen del ahorro vs inyectar todo el contenido completo. Con el sembrado,
	// el turno sobre memoria primeada no suma nada; lo nuevo sí (tN1).
	musubiInjected := primeRes.UsedTokens + tN1
	t.Logf("RESUMEN: Musubi inyectó ~%d tokens (priming %d + memoria nueva %d; sin doble inyección) en lugar de %d (todo el contenido). Ahorro ≈ %d%%.",
		musubiInjected, primeRes.UsedTokens, tN1, totalFull, savingsPct(musubiInjected, totalFull))
}

// savingsPct devuelve el % ahorrado de inyectar injected en vez de full.
func savingsPct(injected, full int) int {
	if full <= 0 {
		return 0
	}
	return int(float64(full-injected) / float64(full) * 100)
}
