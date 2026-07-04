package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/detector"
	"musubi/internal/memory"
)

// footprint_test.go SIMULA una sesión realista contra el código real de Track 9
// (T9.1 ledger holístico + T9.2/T9.3 gobernador) y reporta el footprint de tokens
// de Musubi POR SUPERFICIE, para decidir las fases siguientes sobre datos y no
// intuición. Mide dos escenarios: una PRIMERA sesión (proyecto nuevo: dispara el
// bloque cognitivo y la generación de skills) y una sesión ESTABLECIDA (proyecto ya
// perfilado: solo priming + turnos + PreToolUse). Correr con -v para ver el reporte:
//
//	go test ./cmd/musubi -run TestTokenFootprintRealisticSession -v

// seedCorpus carga un corpus realista (prosa de varios tamaños, código, JSON) en el
// engine, además de hechos, memorias de código y un error de telemetría, para que las
// superficies del ledger se disparen como en un proyecto real.
func seedCorpus(t *testing.T, eng *memory.DbEngine, root string) {
	t.Helper()
	obs := []struct{ id, topic, content string }{
		{"o1", "arch/auth", "Decidimos usar JWT con refresh tokens para la autenticación; el access token vive 15 minutos y el refresh 7 días, rotando el anterior en cada login."},
		{"o2", "arch/db", "La base de datos es PostgreSQL con migraciones versionadas e idempotentes; las conexiones usan un pool de 20 y timeout de 5s."},
		{"o3", "arch/cache", "Cacheamos sesiones de autenticación en Redis con TTL alineado al access token para no pegar a la base en cada request."},
		{"o4", "decision/deploy", "El despliegue es por contenedores con health checks; un rolling update reemplaza de a una réplica para no cortar servicio."},
		{"o5", "pattern/errors", "Convención: los errores de dominio se envuelven con %w y se loguean a stderr; nunca a stdout (ahí va el canal JSON-RPC)."},
		{"o6", "feature/ratelimit", "Rate limiting por IP con ventana deslizante en Redis para frenar fuerza bruta en el login; 5 intentos por minuto."},
		{"o7", "arch/api", "La API es REST con versionado por path (/api/v1); las respuestas de error siguen el formato problem+json."},
		{"o8", "decision/testing", "Tests con la librería estándar; los de integración corren contra una base efímera en CI con -race. Sin mocks salvo en los bordes de red."},
		{"o9", "bugfix/auth", "Arreglado: el refresh token no rotaba y permitía replay; ahora se invalida el anterior al emitir uno nuevo."},
		{"o10", "perf/query", "La consulta de timeline era O(n) por usuario; se agregó un índice parcial sobre (user_id, created_at) y bajó de 800ms a 40ms."},
		// Memorias GRANDES (donde el gisting más ahorra).
		{"o11", "doc/auth-flow", "Flujo completo de autenticación. " + strings.Repeat("El cliente envía credenciales al login, el servidor valida contra la base, emite un JWT firmado y un refresh opaco guardado hasheado; en cada request el middleware valida firma y expiración, y ante un 401 el cliente usa el refresh para un nuevo par. ", 6)},
		{"o12", "doc/db-schema", "Esquema de la base y decisiones de modelado. " + strings.Repeat("Las tablas usan PK UUID, timestamps en UTC, soft-delete por columna archived e índices parciales para las consultas calientes; las migraciones son idempotentes y reversibles. ", 6)},
		{"o13", "decision/observability", "Métricas con OpenTelemetry exportadas a Prometheus; trazas distribuidas con sampling al 10% salvo en errores, que se samplean al 100%."},
		{"o14", "arch/frontend", "El frontend es React con Vite; el estado de servidor se maneja con React Query y el de UI con Zustand. SSR no, es una SPA."},
		{"o15", "decision/secrets", "Los secretos viven en variables de entorno inyectadas por el orquestador; nunca en el repo ni en el config. Rotación trimestral."},
	}
	for _, o := range obs {
		if err := eng.SaveObservation(o.id, o.topic, o.content, nil); err != nil {
			t.Fatalf("SaveObservation %s: %v", o.id, err)
		}
	}

	// Hechos del grafo (no inflan el ledger directo pero hacen al proyecto realista).
	for _, f := range [][3]string{{"auth", "usa", "JWT"}, {"api", "usa", "Redis"}, {"db", "es", "PostgreSQL"}} {
		if _, err := eng.SaveFact(f[0], f[1], f[2], "", nil); err != nil {
			t.Fatalf("SaveFact: %v", err)
		}
	}

	// Memoria de código + un archivo real, para que el PreToolUse inyecte el gist.
	writeFile(t, root, "internal/auth/middleware.go", "package auth\n\nfunc Middleware() {}\n")
	fp, _ := memory.FileFingerprint(root, "internal/auth/middleware.go")
	if err := eng.SaveCodeMemory(memory.CodeMemory{
		Path:        memory.NormalizeCodePath(root, "internal/auth/middleware.go"),
		Gist:        "Middleware de autenticación: valida el JWT del header Authorization y corta con 401 si es inválido.",
		Symbols:     "Middleware() L3",
		Fingerprint: fp,
		Tokens:      memory.EstimateTokens("Middleware de autenticación: valida el JWT del header Authorization y corta con 401 si es inválido."),
	}); err != nil {
		t.Fatalf("SaveCodeMemory: %v", err)
	}

	// Un error conocido sin resolver sobre ese archivo (superficie precheck_telemetry).
	if err := eng.SaveTelemetryLog("internal/auth/middleware.go", "nil pointer al leer el claim 'sub' cuando el token no trae subject", "chequear claims[\"sub\"] != nil antes de castear"); err != nil {
		t.Fatalf("SaveTelemetryLog: %v", err)
	}
}

// reportLedger imprime el reporte del gobernador (T9.2) sobre el ledger acumulado:
// total vs presupuesto, estado y desglose por superficie ordenado por gasto.
func reportLedger(t *testing.T, eng *memory.DbEngine, escenario string, budget int) {
	t.Helper()
	led, err := eng.LedgerStatus()
	if err != nil {
		t.Fatalf("LedgerStatus: %v", err)
	}
	b := led.Budget(budget)
	t.Logf("\n================ FOOTPRINT — %s ================", escenario)
	t.Logf("total=%d tokens | presupuesto=%d | restante=%d | usado=%d%% | estado=%s",
		b.Total, b.Budget, b.Remaining, b.PctUsed, b.Status)
	t.Logf("%-22s %8s %6s", "superficie", "tokens", "%")
	t.Logf("%s", strings.Repeat("-", 38))
	for _, s := range b.Surfaces {
		t.Logf("%-22s %8d %5d%%", s.Surface, s.Tokens, s.Pct)
	}
}

// driveTurns corre nTurns turnos realistas (prompts variados) y algunos PreToolUse
// (lecturas del archivo con gist + telemetría) contra el engine, como en una sesión.
func driveTurns(t *testing.T, eng *memory.DbEngine, root, sessionID string, budget int) {
	t.Helper()
	cfg := config.Default()
	prompts := []string{
		"cómo está implementada la autenticación con jwt y el refresh",
		"revisemos el rate limiting del login y la base de datos",
		"qué decisiones tomamos sobre el deploy y la observabilidad",
		"necesito tocar el middleware de auth, hay algún bug conocido",
	}
	for _, p := range prompts {
		in := fmt.Sprintf(`{"prompt":%q,"session_id":%q}`, p, sessionID)
		turnOutput(eng, cfg.Loop, cfg.Pipeline, cfg.MultiAgent, config.MemoryConfig{SessionTokenBudget: budget}, strings.NewReader(in))
	}
	// Dos lecturas del archivo con memoria de código + error conocido.
	abs := filepath.Join(root, "internal/auth/middleware.go")
	for i := 0; i < 2; i++ {
		ev := fmt.Sprintf(`{"tool_name":"Read","tool_input":{"file_path":%q},"session_id":%q}`, filepath.ToSlash(abs), sessionID)
		precheckOutput(eng, root, strings.NewReader(ev))
	}
}

func TestTokenFootprintRealisticSession(t *testing.T) {
	const budget = 8000

	// ---- Escenario A: PRIMERA sesión (proyecto nuevo, sin perfil ni sentinel) ----
	rootA := t.TempDir()
	writeFile(t, rootA, "go.mod", "module ejemplo.com/app\n\ngo 1.26.4\n")
	writeFile(t, rootA, "package.json", `{"name":"app","dependencies":{"react":"^18.0.0"}}`)
	engA, err := memory.NewDbEngine(rootA)
	if err != nil {
		t.Fatalf("NewDbEngine A: %v", err)
	}
	defer engA.Close()
	seedCorpus(t, engA, rootA)

	cfg := config.Default()
	// SessionStart: dispara priming + cognitivo (sin perfil) + generación de skills (sin sentinel).
	if _, err := buildHookOutput(rootA, engA, cfg.Startup, "sessA"); err != nil {
		t.Fatalf("buildHookOutput A: %v", err)
	}
	if _, err := engA.StartPhase("implementar el endpoint de refresh", cfg.Pipeline.Phases); err != nil {
		t.Fatalf("StartPhase: %v", err)
	}
	driveTurns(t, engA, rootA, "sessA", budget)
	reportLedger(t, engA, "PRIMERA SESIÓN (proyecto nuevo)", budget)

	// ---- Escenario B: sesión ESTABLECIDA (proyecto ya perfilado + skills generadas) ----
	rootB := t.TempDir()
	writeFile(t, rootB, "go.mod", "module ejemplo.com/app\n\ngo 1.26.4\n")
	writeFile(t, rootB, "package.json", `{"name":"app","dependencies":{"react":"^18.0.0"}}`)
	engB, err := memory.NewDbEngine(rootB)
	if err != nil {
		t.Fatalf("NewDbEngine B: %v", err)
	}
	defer engB.Close()
	seedCorpus(t, engB, rootB)
	// Perfilar el proyecto (silencia el cognitivo) y marcar skills generadas (silencia skillgen).
	if err := engB.SaveObservation("prof", profileTopicKey, "Perfil: API Go + frontend React. Auth con JWT/refresh, datos en PostgreSQL, cache Redis. Deploy por contenedores. Convenciones: errores con %w a stderr, tests con stdlib.", nil); err != nil {
		t.Fatalf("perfil: %v", err)
	}
	crearSentinel(t, rootB)
	stackB, _ := detector.DetectStack(rootB)
	_ = engB.SetMeta(memory.MetaStackFingerprint, detector.StackFingerprint(stackB))

	if _, err := buildHookOutput(rootB, engB, cfg.Startup, "sessB"); err != nil {
		t.Fatalf("buildHookOutput B: %v", err)
	}
	if _, err := engB.StartPhase("agregar métricas al login", cfg.Pipeline.Phases); err != nil {
		t.Fatalf("StartPhase B: %v", err)
	}
	driveTurns(t, engB, rootB, "sessB", budget)
	reportLedger(t, engB, "SESIÓN ESTABLECIDA (proyecto perfilado)", budget)

	// Guarda de cordura: ambos escenarios deben haber contabilizado algo y el desglose
	// debe sumar el total (invariante del ledger holístico).
	for _, eng := range []*memory.DbEngine{engA, engB} {
		led, _ := eng.LedgerStatus()
		if led.Total == 0 {
			t.Error("el ledger no contabilizó ninguna superficie")
		}
		sum := 0
		for _, v := range led.Surfaces {
			sum += v
		}
		if sum != led.Total {
			t.Errorf("el total (%d) no coincide con la suma por superficie (%d)", led.Total, sum)
		}
	}

	// Ordena e imprime, para el ojo, cuál superficie domina en la primera sesión.
	ledA, _ := engA.LedgerStatus()
	type kv struct {
		s string
		n int
	}
	var top []kv
	for s, n := range ledA.Surfaces {
		top = append(top, kv{s, n})
	}
	sort.Slice(top, func(i, j int) bool { return top[i].n > top[j].n })
	if len(top) > 0 {
		t.Logf("\nSuperficie dominante en la primera sesión: %s (%d tokens)", top[0].s, top[0].n)
	}
}
