package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"musubi/internal/cognition"
	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"

	_ "modernc.org/sqlite" // para leer el ledger crudo y verificar el outcome exacto (M9)
)

// Invariantes del spec «El motor tiene freno propio» (specs/motor-con-freno/).

// preguntaSembrada matchea el contenido que siembra `sembrar`. Tiene que matchear: musubi_ask corta
// ANTES de llamar al motor si el recall del grounding vuelve vacío, así que una pregunta que no
// recupera nada haría pasar las pruebas del freno sin haber ejercitado el freno.
const preguntaSembrada = "candado del despacho"

// servidorConFreno arma un servidor con motor, el juez read-time encendido, el presupuesto en
// maxPorHora Y MEMORIA SEMBRADA. Devuelve también el engine para poder mirar el ledger.
func servidorConFreno(t *testing.T, motor cognition.Provider, maxPorHora int) (*McpServer, *memory.DbEngine) {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	si := true
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{},
		WithCognition(motor),
		WithCognitionConfig(config.CognitionConfig{Provider: "fake", ReadTimeRerank: &si}),
		WithConflicts(config.ConflictConfig{}), // ver la guarda de sembrar(): la siembra son casi-duplicados
		WithMotorQuota(maxPorHora))
	sembrar(t, s, 3)
	exigeSembradas(t, engine, 3)
	return s, engine
}

// ctxPrincipal arma un contexto con un principal writer del nombre dado.
func ctxPrincipal(nombre string) context.Context {
	return withPrincipal(context.Background(), &Principal{Name: nombre, Role: RoleWriter})
}

// resultadoDePrueba: tres items, que es lo mínimo para que el juez actúe (necesita ≥2).
func resultadoDePrueba() memory.RecallResult {
	return memory.RecallResult{Items: []memory.RecallItem{
		{ID: "a", Gist: "el candado no cruza la red"},
		{ID: "b", Gist: "el bump de accesos es atómico"},
		{ID: "c", Gist: "el juez ordena, no descarta"},
	}}
}

// M1 — musubi_ask SIN PRESUPUESTO SE RECHAZA.
//
// Quien la llamó pidió una respuesta razonada del motor. Devolverle otra cosa —o una respuesta vacía
// con cara de éxito— sería mentirle. Por eso acá se exige un error explícito y con código propio.
func TestM1AskSinPresupuestoSeRechaza(t *testing.T) {
	motor := &motorEspia{respuesta: "una respuesta"}
	s, _ := servidorConFreno(t, motor, 1)
	ctx := ctxPrincipal("p1")
	params := json.RawMessage(`{"name":"musubi_ask","arguments":{"question":"` + preguntaSembrada + `"}}`)

	if _, rpcErr := s.handleToolsCall(ctx, params); rpcErr != nil {
		t.Fatalf("la 1ra llamada está dentro del presupuesto y no debería fallar: %v", rpcErr)
	}
	_, rpcErr := s.handleToolsCall(ctx, params)
	if rpcErr == nil {
		t.Fatal("la 2da llamada excede el presupuesto: tiene que RECHAZARSE, no responder igual")
	}
	if rpcErr.Code != codeMotorQuota {
		t.Errorf("código %d, esperaba codeMotorQuota (%d): el remedio es distinto al de la cuota general",
			rpcErr.Code, codeMotorQuota)
	}
	if n := motor.llamadas.Load(); n != 1 {
		t.Errorf("el motor recibió %d llamadas, esperaba 1: rechazar NO puede gastar", n)
	}
}

// M2 — musubi_recall SIN PRESUPUESTO DEGRADA, NO FALLA.
//
// Quien llamó pidió memoria, no un juez. Es la misma regla que ya rige cuando el motor se cae: un
// límite de gasto es, para el recall, otra forma de que el juez no esté disponible.
func TestM2RecallSinPresupuestoDegrada(t *testing.T) {
	// El juez INVIERTE, así se distingue «ordenó» de «devolvió el orden model-free».
	motor := &motorEspia{respuesta: `["c","b","a"]`}
	s, _ := servidorConFreno(t, motor, 1)
	ctx := ctxPrincipal("p1")

	primera := s.rerankIfEnabled(ctx, "consulta m2 uno", resultadoDePrueba())
	if primera.Items[0].ID != "c" {
		t.Fatalf("con presupuesto el juez tiene que ordenar; quedó %s primero", primera.Items[0].ID)
	}

	segunda := s.rerankIfEnabled(ctx, "consulta m2 dos", resultadoDePrueba())
	if got := []string{segunda.Items[0].ID, segunda.Items[1].ID, segunda.Items[2].ID}; got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("sin presupuesto el recall debe devolver el orden MODEL-FREE intacto, quedó %v", got)
	}
	if len(segunda.Items) != 3 {
		t.Errorf("degradar no puede perder memorias: %d items de 3", len(segunda.Items))
	}
	if n := motor.llamadas.Load(); n != 1 {
		t.Errorf("el motor recibió %d llamadas, esperaba 1: la degradada no debe gastar", n)
	}
}

// M3 — CON EL JUEZ APAGADO, EL RECALL NO CONSUME PRESUPUESTO.
//
// Es el modo por default. Un recall model-free no llama al motor: si consumiera cuota, el freno
// estaría estrangulando una tool gratis.
func TestM3RecallModelFreeNoGastaPresupuesto(t *testing.T) {
	motor := &motorEspia{respuesta: `["c","b","a"]`}
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	// SIN ReadTimeRerank: el juez apagado, que es el default.
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{},
		WithCognition(motor),
		WithCognitionConfig(config.CognitionConfig{Provider: "fake"}),
		WithMotorQuota(1))
	ctx := ctxPrincipal("p1")

	for i := 0; i < 5; i++ {
		res := s.rerankIfEnabled(ctx, fmt.Sprintf("consulta m3 %d", i), resultadoDePrueba())
		if res.Items[0].ID != "a" {
			t.Fatalf("con el juez apagado el orden no se toca; quedó %s primero", res.Items[0].ID)
		}
	}
	// El presupuesto es de 1 y hubo 5 recalls: si los hubiera contado, no quedaría nada. Un ask
	// tiene que seguir pasando.
	if !s.hayPresupuestoDeMotor(ctx) {
		t.Error("5 recalls model-free agotaron el presupuesto del motor: no deberían haber contado ninguno")
	}
}

// M4 — UN ACIERTO DE CACHÉ NO CONSUME PRESUPUESTO.
//
// El caché existe para no llamar al motor. Cobrarle contaría gasto que no ocurrió, y agotaría el
// presupuesto justamente cuando el sistema se está portando bien.
func TestM4AciertoDeCacheNoGasta(t *testing.T) {
	motor := &motorEspia{respuesta: `["c","b","a"]`}
	s, _ := servidorConFreno(t, motor, 1)
	ctx := ctxPrincipal("p1")
	// MISMA consulta y mismos ids las dos veces ⇒ la 2da tiene que pegarle al caché.
	const consulta = "consulta m4 identica"

	primera := s.rerankIfEnabled(ctx, consulta, resultadoDePrueba())
	if primera.Items[0].ID != "c" {
		t.Fatalf("la 1ra tenía que ordenar; quedó %s primero", primera.Items[0].ID)
	}
	segunda := s.rerankIfEnabled(ctx, consulta, resultadoDePrueba())
	if segunda.Items[0].ID != "c" {
		t.Errorf("el acierto de caché tiene que ordenar igual; quedó %s primero — si degradó, "+
			"el freno le está cobrando a algo que no llamó al motor", segunda.Items[0].ID)
	}
	if n := motor.llamadas.Load(); n != 1 {
		t.Fatalf("el motor recibió %d llamadas, esperaba 1: la 2da tenía que salir del caché", n)
	}
}

// M5 — RECHAZAR NO GASTA. El presupuesto se consulta ANTES de la llamada al motor, así que una
// llamada rechazada no puede haber costado nada. Se verifica con un motor que CUENTA.
func TestM5RechazarNoGasta(t *testing.T) {
	motor := &motorEspia{respuesta: "una respuesta"}
	s, _ := servidorConFreno(t, motor, 2)
	ctx := ctxPrincipal("p1")
	params := json.RawMessage(`{"name":"musubi_ask","arguments":{"question":"` + preguntaSembrada + `"}}`)

	for i := 0; i < 6; i++ {
		_, _ = s.handleToolsCall(ctx, params)
	}
	if n := motor.llamadas.Load(); n != 2 {
		t.Errorf("el motor recibió %d llamadas con presupuesto 2: los 4 rechazos no debían gastar", n)
	}
}

// M6 — EL PRESUPUESTO ES POR PRINCIPAL. Si fuera global, un cliente desbocado dejaría sin motor a
// todos — que es el fallo que este spec viene a evitar, no a mover de lugar.
func TestM6PresupuestoPorPrincipal(t *testing.T) {
	motor := &motorEspia{respuesta: "una respuesta"}
	s, _ := servidorConFreno(t, motor, 1)
	params := json.RawMessage(`{"name":"musubi_ask","arguments":{"question":"` + preguntaSembrada + `"}}`)

	ctx1 := ctxPrincipal("p1")
	if _, rpcErr := s.handleToolsCall(ctx1, params); rpcErr != nil {
		t.Fatalf("p1 dentro del presupuesto: %v", rpcErr)
	}
	if _, rpcErr := s.handleToolsCall(ctx1, params); rpcErr == nil {
		t.Fatal("p1 ya agotó lo suyo y debería estar frenado")
	}
	ctx2 := ctxPrincipal("p2")
	if _, rpcErr := s.handleToolsCall(ctx2, params); rpcErr != nil {
		t.Errorf("p2 tiene su propio presupuesto y no debería pagar el gasto de p1: %v", rpcErr)
	}
}

// M7 — SIN PRINCIPAL NO HAY FRENO.
//
// En stdio local no hay identidad contra la cual contar, exactamente como la cuota general. Un
// límite sobre la clave vacía sería uno solo compartido por todos los usos anónimos.
func TestM7SinPrincipalNoHayFreno(t *testing.T) {
	motor := &motorEspia{respuesta: "una respuesta"}
	s, _ := servidorConFreno(t, motor, 1)
	ctx := context.Background() // sin principal
	params := json.RawMessage(`{"name":"musubi_ask","arguments":{"question":"` + preguntaSembrada + `"}}`)

	for i := 0; i < 4; i++ {
		if _, rpcErr := s.handleToolsCall(ctx, params); rpcErr != nil {
			t.Fatalf("llamada %d sin principal no debería estar frenada: %v", i+1, rpcErr)
		}
	}
	if n := motor.llamadas.Load(); n != 4 {
		t.Errorf("el motor recibió %d llamadas, esperaba 4", n)
	}
}

// M8 — CERO ⇒ DEFAULT; NEGATIVO ⇒ SIN LÍMITE. La misma semántica que quota_per_minute, y por eso
// mismo se prueba: dos números que se parecen y significan cosas distintas es la forma más barata de
// que alguien se apague el freno creyendo que lo apretaba.
func TestM8SemanticaDelCeroYDelNegativo(t *testing.T) {
	casos := []struct {
		nombre string
		valor  int
		quiero int
	}{
		// El 60 va HARDCODEADO y no como referencia a la constante: así el test fija el default
		// documentado. Si alguien lo cambia, esta prueba se pone en rojo y obliga a decirlo — que es
		// lo que uno quiere de un número que decide cuánta plata se puede gastar.
		{"cero da el default (protección ON)", 0, 60},
		{"negativo desactiva el freno", -1, 0},
		{"positivo se respeta tal cual", 25, 25},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := config.CognitionConfig{MotorQuotaPerHour: c.valor}.EffectiveMotorQuotaPerHour()
			if got != c.quiero {
				t.Errorf("EffectiveMotorQuotaPerHour(%d) = %d, quería %d", c.valor, got, c.quiero)
			}
		})
	}
	// Y la semántica tiene que ser IDÉNTICA a la de la cuota general: si una divergiera, el YAML
	// se volvería una trampa.
	if (config.ServiceConfig{}).EffectiveQuotaPerMinute() == 0 {
		t.Error("la cuota general con 0 debería dar su default, no 0: las dos semánticas divergieron")
	}
	if (config.ServiceConfig{QuotaPerMinute: -1}).EffectiveQuotaPerMinute() != 0 {
		t.Error("la cuota general con negativo debería dar 0 (sin límite): las dos semánticas divergieron")
	}
}

// M9 — EL RECHAZO DE musubi_ask QUEDA EN EL LEDGER CON OUTCOME PROPIO.
//
// Si compartiera outcome con denied_quota sería imposible responder «¿me frenó el límite general o
// el del motor?», que es la única pregunta que alguien se hace cuando una tool deja de andar.
func TestM9ElRechazoQuedaEnElLedgerConOutcomePropio(t *testing.T) {
	motor := &motorEspia{respuesta: "una respuesta"}
	dir := t.TempDir()
	engine, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	si := true
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{},
		WithCognition(motor),
		WithCognitionConfig(config.CognitionConfig{Provider: "fake", ReadTimeRerank: &si}),
		WithConflicts(config.ConflictConfig{}), // ver la guarda de sembrar(): la siembra son casi-duplicados
		WithMotorQuota(1),
		WithUsageLedger(engine, config.UsageLedgerConfig{}))
	sembrar(t, s, 3)
	exigeSembradas(t, engine, 3)
	ctx := ctxPrincipal("p1")
	params := json.RawMessage(`{"name":"musubi_ask","arguments":{"question":"` + preguntaSembrada + `"}}`)

	_, _ = s.handleToolsCall(ctx, params) // gasta el único
	if _, rpcErr := s.handleToolsCall(ctx, params); rpcErr == nil {
		t.Fatal("la 2da tenía que ser rechazada")
	}
	s.ledger.flush()

	// El outcome EXACTO, leyendo la tabla: es lo que distingue «me frenó el motor» de «me frenó la
	// cuota general» o de un error cualquiera.
	db, err := sql.Open("sqlite", "file:"+filepath.Join(dir, config.DirName, config.DBFile)+"?mode=ro")
	if err != nil {
		t.Fatalf("abrir la base: %v", err)
	}
	defer db.Close()
	filas, err := db.Query(`SELECT outcome, COUNT(*) FROM tool_invocations WHERE tool = 'musubi_ask' GROUP BY outcome`)
	if err != nil {
		t.Fatalf("leer el ledger: %v", err)
	}
	defer filas.Close()
	porOutcome := map[string]int{}
	for filas.Next() {
		var o string
		var n int
		if err := filas.Scan(&o, &n); err != nil {
			t.Fatal(err)
		}
		porOutcome[o] = n
	}
	if porOutcome[memory.OutcomeDeniedMotor] != 1 {
		t.Errorf("esperaba 1 invocación con outcome %q, hay %d (todos: %v)",
			memory.OutcomeDeniedMotor, porOutcome[memory.OutcomeDeniedMotor], porOutcome)
	}
	if porOutcome[memory.OutcomeError] != 0 {
		t.Errorf("el rechazo por presupuesto no puede quedar como %q genérico: se pierde la causa",
			memory.OutcomeError)
	}

	// Y tiene que aparecer en el REPORTE que ya existe, sin haber tocado la consulta: ToolUsage
	// cuenta `outcome LIKE 'denied_%'`, así que un outcome nuevo entra solo. Si no entrara, el freno
	// estaría frenando sin salir en el panel que la gente mira.
	filasUso, err := engine.ToolUsage(ctx, 1)
	if err != nil {
		t.Fatalf("ToolUsage: %v", err)
	}
	var visto bool
	for _, f := range filasUso {
		if f.Tool == "musubi_ask" {
			visto = true
			if f.Denied != 1 {
				t.Errorf("ToolUsage reporta %d rechazos para musubi_ask, esperaba 1", f.Denied)
			}
		}
	}
	if !visto {
		t.Error("musubi_ask no aparece en ToolUsage")
	}
}

// M10 — LA DEGRADACIÓN DEL RECALL QUEDA CONTADA.
//
// El recall devuelve ok y el usuario no ve nada raro: si además no quedara registro, el sistema
// estaría dejando de usar el juez sin que nadie pudiera enterarse.
func TestM10LaDegradacionQuedaContada(t *testing.T) {
	motor := &motorEspia{respuesta: `["c","b","a"]`}
	s, _ := servidorConFreno(t, motor, 1)
	s.metrics = &serverMetrics{}
	ctx := ctxPrincipal("p1")

	s.rerankIfEnabled(ctx, "consulta m10 uno", resultadoDePrueba())  // gasta el único
	s.rerankIfEnabled(ctx, "consulta m10 dos", resultadoDePrueba())  // degrada
	s.rerankIfEnabled(ctx, "consulta m10 tres", resultadoDePrueba()) // degrada
	if n := s.metrics.motorDenied.Load(); n != 2 {
		t.Errorf("motorDenied = %d, esperaba 2: una degradación silenciosa es indistinguible de un juez que no aporta", n)
	}
}
