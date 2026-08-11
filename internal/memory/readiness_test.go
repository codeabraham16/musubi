package memory

import (
	"context"
	"strings"
	"testing"
	"time"
)

// readiness_test.go defiende las dos propiedades que separan un puntaje medido de una medalla:
// que NO MEDIR no suba el puntaje, y que el puntaje PUEDA BAJAR cuando el comportamiento empeora.
// Un test que se conformara con "devuelve un número entre 0 y 1" pasaría con la función devolviendo
// 1 siempre.

func dimensionDe(t *testing.T, rep ReadinessReport, key string) ReadinessDimension {
	t.Helper()
	for _, d := range rep.Dimensions {
		if d.Key == key {
			return d
		}
	}
	t.Fatalf("no existe la dimensión %q en %+v", key, rep.Dimensions)
	return ReadinessDimension{}
}

// invocar mete n llamadas al ledger con un outcome dado.
func invocar(t *testing.T, e *DbEngine, tool, outcome string, n int) {
	t.Helper()
	lote := make([]ToolInvocation, 0, n)
	for i := 0; i < n; i++ {
		lote = append(lote, ToolInvocation{Tool: tool, Outcome: outcome, Duration: time.Millisecond})
	}
	if err := e.RecordToolInvocations(context.Background(), lote); err != nil {
		t.Fatalf("RecordToolInvocations: %v", err)
	}
}

// R1: UNA INSTALACIÓN VIRGEN PUNTÚA CERO, y dice de qué se trata cada cero.
//
// Es el caso que ningún chequeo de integridad sabe ver: base sana, migrada, todo verde... y valor
// entregado nulo porque nadie la usa. Si el puntaje arrancara alto y bajara con el uso, mediría lo
// contrario de lo que dice medir.
func TestReadinessVirgenPuntuaCero(t *testing.T) {
	e := newTestEngine(t)
	rep, err := e.Readiness(context.Background(), 30)
	if err != nil {
		t.Fatalf("Readiness: %v", err)
	}
	if rep.Score != 0 {
		t.Errorf("una instalación sin uso, sin memoria y sin grafo debe puntuar 0, dio %v", rep.Score)
	}
	if len(rep.Dimensions) != 5 {
		t.Fatalf("esperaba 5 dimensiones, hay %d", len(rep.Dimensions))
	}
	if len(rep.Unobserved) != 5 {
		t.Errorf("las 5 dimensiones deberían estar como no observadas: %v", rep.Unobserved)
	}
	for _, d := range rep.Dimensions {
		if d.Observed || d.Score != 0 {
			t.Errorf("dimensión %q: observed=%v score=%v, esperaba no observada en 0", d.Key, d.Observed, d.Score)
		}
		// Un cero sin motivo es indistinguible de un bug. Cada dimensión tiene que decir por qué.
		if _, ok := d.Evidence["por_que_cero"]; !ok {
			t.Errorf("dimensión %q puntúa 0 sin explicar por qué: %+v", d.Key, d.Evidence)
		}
	}
}

// R2: NO MEDIR NO PREMIA. La dimensión sin dato entra en el promedio con 0, no se saltea.
//
// Es la regla entera del diseño. Si las no observadas se excluyeran del promedio, apagar la
// instrumentación subiría el puntaje — el incentivo exactamente al revés.
func TestNoObservadoNoSeSaltea(t *testing.T) {
	e := newTestEngine(t)
	// Una sola dimensión perfecta, cuatro sin dato.
	invocar(t, e, "musubi_recall", OutcomeOK, 10)
	invocar(t, e, "musubi_save_observation", OutcomeOK, 5)
	invocar(t, e, "musubi_doctor", OutcomeOK, 5)

	rep, err := e.Readiness(context.Background(), 30)
	if err != nil {
		t.Fatalf("Readiness: %v", err)
	}
	uso := dimensionDe(t, rep, "uso")
	if uso.Score != 1 {
		t.Fatalf("la dimensión de uso debería estar perfecta: %+v", uso)
	}
	// 5 dimensiones, una en 1 y una en 1 (confiabilidad, que también se observa): el global tiene
	// que reflejar las tres que faltan, no ignorarlas.
	if rep.Score >= 0.5 {
		t.Errorf("con 3 de 5 dimensiones sin dato el global no puede pasar de la mitad: %v", rep.Score)
	}
	esperado := redondear((uso.Score + dimensionDe(t, rep, "confiabilidad").Score) / 5)
	if rep.Score != esperado {
		t.Errorf("el global = %v, pero promediando las 5 (con los ceros) da %v", rep.Score, esperado)
	}
	if len(rep.Unobserved) != 3 {
		t.Errorf("esperaba 3 dimensiones no observadas, hay %v", rep.Unobserved)
	}
}

// R3: EL PUNTAJE BAJA cuando el comportamiento empeora. Un indicador que sólo sube es una medalla.
func TestReadinessBajaConLosErrores(t *testing.T) {
	e := newTestEngine(t)
	invocar(t, e, "musubi_recall", OutcomeOK, 100)
	invocar(t, e, "musubi_save_observation", OutcomeOK, 50)
	invocar(t, e, "musubi_doctor", OutcomeOK, 50)
	antes, err := e.Readiness(context.Background(), 30)
	if err != nil {
		t.Fatalf("Readiness: %v", err)
	}
	confAntes := dimensionDe(t, antes, "confiabilidad")
	if confAntes.Score != 1 {
		t.Fatalf("sin errores la confiabilidad debe ser 1: %+v", confAntes)
	}

	// 50 fallas sobre 250: 20 %, muy por encima del tope.
	invocar(t, e, "musubi_recall", OutcomeError, 50)
	despues, err := e.Readiness(context.Background(), 30)
	if err != nil {
		t.Fatalf("Readiness: %v", err)
	}
	confDespues := dimensionDe(t, despues, "confiabilidad")
	if confDespues.Score >= confAntes.Score {
		t.Errorf("la confiabilidad no bajó con un 20 %% de errores: %v → %v", confAntes.Score, confDespues.Score)
	}
	if despues.Score >= antes.Score {
		t.Errorf("el puntaje global no bajó: %v → %v", antes.Score, despues.Score)
	}
	// Y la evidencia tiene que traer los números crudos, no sólo el veredicto.
	if confDespues.Evidence["errores"] != 50 || confDespues.Evidence["invocaciones"] != 250 {
		t.Errorf("la evidencia no trae los números con los que se juzgó: %+v", confDespues.Evidence)
	}
}

// R4: los rechazos son una señal APARTE de los errores. Una instalación donde todo funciona pero
// la mitad de las llamadas rebota contra los permisos no está lista, y sin este eje el puntaje
// no lo vería: los rechazos no son errores.
func TestRechazosCuentanAparteDeLosErrores(t *testing.T) {
	e := newTestEngine(t)
	invocar(t, e, "musubi_recall", OutcomeOK, 40)
	invocar(t, e, "musubi_save_observation", OutcomeOK, 30)
	invocar(t, e, "musubi_maintain", OutcomeDeniedRole, 30)

	rep, err := e.Readiness(context.Background(), 30)
	if err != nil {
		t.Fatalf("Readiness: %v", err)
	}
	conf := dimensionDe(t, rep, "confiabilidad")
	if conf.Evidence["errores"] != 0 {
		t.Fatalf("no hubo errores: %+v", conf.Evidence)
	}
	if conf.Score == 1 {
		t.Errorf("un 30 %% de rechazos no puede dar confiabilidad perfecta: %+v", conf)
	}
}

// R5: la cola de contradicciones tiene que poder GANAR. Una memoria donde el detector encuentra y
// nadie arbitra es la forma en que un cerebro «empieza a fallar» sin que ningún chequeo de
// integridad se ponga en rojo — las dos memorias en conflicto son individualmente válidas.
func TestCoherenciaCaeCuandoLaColaGana(t *testing.T) {
	e := newTestEngine(t)
	obs := func(id string) string {
		if err := e.SaveObservation(id, "tema", "contenido de "+id, nil); err != nil {
			t.Fatalf("SaveObservation: %v", err)
		}
		return id
	}
	// Una resuelta, una pendiente: la cola no gana.
	a, b, c, d := obs("o1"), obs("o2"), obs("o3"), obs("o4")
	if _, err := e.UpsertObsRelation(ObsRelation{SourceID: a, TargetID: b,
		Relation: "supersedes", Status: RelStatusResolved, ResolvedBy: "humano"}); err != nil {
		t.Fatalf("UpsertObsRelation: %v", err)
	}
	if _, err := e.UpsertObsRelation(ObsRelation{SourceID: c, TargetID: d, Status: RelStatusPending}); err != nil {
		t.Fatalf("UpsertObsRelation: %v", err)
	}
	rep, err := e.Readiness(context.Background(), 30)
	if err != nil {
		t.Fatalf("Readiness: %v", err)
	}
	coh := dimensionDe(t, rep, "coherencia")
	if !coh.Observed || coh.Score != 1 {
		t.Fatalf("1 resuelta y 1 pendiente debería dar coherencia perfecta: %+v", coh)
	}

	// Ahora se acumulan pendientes: la cola gana y la dimensión cae.
	for i := 0; i < 5; i++ {
		s, tg := obs("p"+string(rune('a'+i))), obs("q"+string(rune('a'+i)))
		if _, err := e.UpsertObsRelation(ObsRelation{SourceID: s, TargetID: tg, Status: RelStatusPending}); err != nil {
			t.Fatalf("UpsertObsRelation: %v", err)
		}
	}
	rep2, err := e.Readiness(context.Background(), 30)
	if err != nil {
		t.Fatalf("Readiness: %v", err)
	}
	coh2 := dimensionDe(t, rep2, "coherencia")
	if coh2.Score >= coh.Score {
		t.Errorf("con 6 pendientes contra 1 resuelta la coherencia no bajó: %v → %v", coh.Score, coh2.Score)
	}
	if coh2.Evidence["pendientes"] != 6 || coh2.Evidence["resueltas"] != 1 {
		t.Errorf("la evidencia no cuenta bien la cola: %+v", coh2.Evidence)
	}
}

// R6: «se usó» no es «se usa». Una instalación con mucho historial pero muerta hace meses tiene
// que perder el chequeo de frescura — si no, el puntaje se queda congelado en la foto de su mejor
// momento y nadie se entera de que el cerebro dejó de recibir llamadas.
func TestFrescuraDistingueSeUsaDeSeUso(t *testing.T) {
	e := newTestEngine(t)
	invocar(t, e, "musubi_recall", OutcomeOK, 10)
	invocar(t, e, "musubi_save_observation", OutcomeOK, 10)
	invocar(t, e, "musubi_doctor", OutcomeOK, 10)
	// Envejece TODO el ledger dentro de la ventana de 30 días pero fuera de la de frescura.
	if _, err := e.db.Exec(`UPDATE tool_invocations SET created_at = datetime('now','-20 days')`); err != nil {
		t.Fatalf("no se pudo envejecer el ledger: %v", err)
	}
	rep, err := e.Readiness(context.Background(), 30)
	if err != nil {
		t.Fatalf("Readiness: %v", err)
	}
	uso := dimensionDe(t, rep, "uso")
	if !uso.Observed {
		t.Fatalf("con 30 invocaciones en la ventana la dimensión está observada: %+v", uso)
	}
	if uso.Score == 1 {
		t.Errorf("un cerebro sin llamadas hace 20 días no puede sacar uso perfecto: %+v", uso.Checks)
	}
	for _, c := range uso.Checks {
		if strings.Contains(c.Name, "ahora") && c.Pass {
			t.Errorf("el chequeo de frescura debería fallar: %+v", c)
		}
	}
}

// R7: el mantenimiento se mide por su marca REAL, y una marca vieja no vale.
func TestMantenimientoViejoNoCuentaComoReciente(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("o1", "tema", "algo que recordar", nil); err != nil {
		t.Fatalf("SaveObservation: %v", err)
	}
	// Nunca corrió: pierde dos de los tres chequeos.
	rep, err := e.Readiness(context.Background(), 30)
	if err != nil {
		t.Fatalf("Readiness: %v", err)
	}
	nunca := dimensionDe(t, rep, "memoria")
	if !nunca.Observed || nunca.Score >= 0.5 {
		t.Errorf("sin mantenimiento la memoria no puede pasar de un tercio: %+v", nunca)
	}

	// Corrió hace 90 días: sigue sin ser reciente.
	if err := e.SetMeta(metaLastMaintenance, time.Now().UTC().Add(-90*24*time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	viejo, _ := e.Readiness(context.Background(), 30)
	dViejo := dimensionDe(t, viejo, "memoria")

	// Corrió recién: perfecta.
	if err := e.SetMeta(metaLastMaintenance, time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	fresco, _ := e.Readiness(context.Background(), 30)
	dFresco := dimensionDe(t, fresco, "memoria")

	if !(nunca.Score < dViejo.Score && dViejo.Score < dFresco.Score) {
		t.Errorf("la memoria debería mejorar de nunca(%v) → viejo(%v) → reciente(%v)",
			nunca.Score, dViejo.Score, dFresco.Score)
	}
	if dFresco.Score != 1 {
		t.Errorf("con memoria viva y mantenimiento recién corrido debería dar 1: %+v", dFresco.Checks)
	}
}

// R8: una marca ilegible NO cuenta como reciente. Un dato que no se puede leer no es evidencia de
// nada, y tratarlo como válido sería regalar puntaje por tener basura en la tabla.
func TestMarcaIlegibleNoCuenta(t *testing.T) {
	if recienteISO("no soy una fecha", 30) {
		t.Error("una marca ilegible no puede contar como reciente")
	}
	if recienteISO("", 30) {
		t.Error("una marca vacía no puede contar como reciente")
	}
	if !recienteISO(time.Now().UTC().Format(time.RFC3339), 1) {
		t.Error("una marca de ahora sí debe contar como reciente")
	}
}
