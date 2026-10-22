package memory

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// doctor_anchor_coverage_test.go cubre checkAnchorCoverage (A3): el check informativo del doctor que
// reporta la COBERTURA de origin-anchoring sobre las notas que CITAN CÓDIGO. Las invariantes del
// diseño (R1–R7): población = visibles que citan código y no son git-commit; Status SIEMPRE ok
// (nunca warning, para no ser un nag de arranque vía buildHealthContext); ratio + top-N por salience;
// sin anclables → ok + Unmeasured; no reparable y fuera del auto-heal.

// anclar inserta una fila en observation_origins DIRECTAMENTE (como origins_test.go), sin pasar por
// SaveObservation(origins): anclar por la vía real exige que la ruta EXISTA en disco y saque un
// fingerprint, y un test no debe depender del árbol del repo. Acá sólo importa que la observación
// figure en observation_origins, que es lo único que el check mira.
func anclar(t *testing.T, e *DbEngine, obsID string) {
	t.Helper()
	if _, err := e.db.Exec(
		`INSERT INTO observation_origins (observation_id, path, fingerprint) VALUES (?,?,?)`,
		obsID, "internal/memory/doctor.go", "deadbeef"); err != nil {
		t.Fatalf("anclar %q: %v", obsID, err)
	}
}

// R7 / predicado: referencesCodeRE reconoce la FORMA de una cita de código sin tocar el repo. Es el
// corazón de "qué es anclable", así que se prueba aislado: un token <ruta/>nombre.ext (con :línea o
// #Símbolo opcional) matchea; una versión, una IP o prosa sin archivo, no.
func TestAnchorCoveragePredicado(t *testing.T) {
	casos := []struct {
		texto string
		want  bool
	}{
		// Citan código (want true): nombre pelado, ruta con línea, ruta con símbolo, varias ext.
		{"ver doctor.go para el registry", true},
		{"reusa internal/memory/decay.go:66", true},
		{"el predicado vive en scope.go#visibleObsPredicateDe", true},
		{"el .env de Supabase y config.yaml", true},
		{"tabla en config.yml", true},
		{"el worker es worker.py", true},
		{"puente en index.js", true},
		{"componente app.tsx", true},
		{"motor main.rs", true},
		{"consulta query.sql", true},
		{"guion run.sh", true},
		{"módulo mod.ts", true},
		{"vista comp.jsx", true},
		// Mayúsculas: el predicado es (?i), la extensión matchea sin importar el caso.
		{"ver DOCTOR.GO para el registry", true},
		// NO citan código (want false): versiones, IP, prosa, sin extensión de fuente.
		{"liberado en v0.141.0", false},
		{"version 1.2.3 del esquema", false},
		{"el server está en 100.79.126.62", false},
		{"ver el README del repo", false},
		{"una nota de estado sin archivo", false},
		{"por ejemplo e.g. esto", false},
		// El \b importa: una extensión que EMPIEZA como una de código pero sigue con otra letra
		// (".tsv" sobre "ts") NO cuenta. Sin \b en la regex, esto daría true y el test se pondría rojo.
		{"export a datos.tsv", false},
		{"backup en data.bak", false},
	}
	for _, c := range casos {
		if got := referencesCodeRE.MatchString(c.texto); got != c.want {
			t.Errorf("referencesCodeRE.MatchString(%q) = %v, quería %v", c.texto, got, c.want)
		}
	}
}

// S3 + S4 + S5: una nota SIN token de código queda fuera de la población; una git-commit que SÍ cita
// código también queda fuera (hecho histórico, no anclable). Si no queda ninguna anclable, el check
// es ok + Unmeasured —no un falso verde "todo anclado"—.
func TestAnchorCoverageSinAnclablesEsUnmeasured(t *testing.T) {
	e := newTestEngine(t)
	// S3: cita cero código → fuera.
	if err := e.SaveObservation("n1", "estado/plano", "una nota de estado sin ninguna cita", nil); err != nil {
		t.Fatal(err)
	}
	// S4: cita código pero es git-commit → excluida de la población.
	if err := e.SaveObservation("c1", "git-commit", "fix(doctor): toca internal/memory/doctor.go", nil); err != nil {
		t.Fatal(err)
	}

	r := checkAnchorCoverage(e)

	if r.Status != "ok" {
		t.Errorf("sin anclables el Status debe ser ok, obtuve %q (%s)", r.Status, r.Message)
	}
	if !r.Unmeasured {
		t.Errorf("S5: sin población anclable el check debe marcarse Unmeasured, Message=%q", r.Message)
	}
	if strings.Contains(r.Message, "hipótesis") {
		t.Errorf("sin anclables no se afirma ninguna hipótesis, obtuve %q", r.Message)
	}
}

// S1 + S2 + S6: con una anclada y una sin anclar, el check reporta el RATIO como hipótesis, nombra la
// sin ancla como candidata y NO nombra la anclada; y el Status es ok (nunca warning). Esta aserción de
// Status es el GUARDA DE SABOTAJE del invariante R2: si alguien hiciera el check 'warning' cuando hay
// notas sin ancla, este test se pone rojo.
func TestAnchorCoverageRatioYCandidatas(t *testing.T) {
	e := newTestEngine(t)
	// S2: cita código y está anclada → cuenta en el numerador, NO es candidata.
	if err := e.SaveObservation("A", "anclada/ya", "esto reusa internal/memory/decay.go:66", nil); err != nil {
		t.Fatal(err)
	}
	anclar(t, e, "A")
	// S1: cita código y NO está anclada → candidata.
	if err := e.SaveObservation("B", "suelta/viva", "el predicado vive en scope.go hoy", nil); err != nil {
		t.Fatal(err)
	}

	r := checkAnchorCoverage(e)

	if r.Status != "ok" { // R2 / S6: NUNCA warning
		t.Errorf("S6: el Status debe ser ok aunque haya notas sin ancla, obtuve %q (%s)", r.Status, r.Message)
	}
	if r.Unmeasured {
		t.Errorf("con población anclable el check NO es Unmeasured, Message=%q", r.Message)
	}
	if !strings.Contains(r.Message, "hipótesis") {
		t.Errorf("con cobertura parcial el Message debe enmarcar una hipótesis, obtuve %q", r.Message)
	}
	if !strings.Contains(r.Message, "1 de 2") || !strings.Contains(r.Message, "50.0%") {
		t.Errorf("el Message debe reportar el ratio '1 de 2 (50.0%%)', obtuve %q", r.Message)
	}
	// S1: la candidata va en el Detail (nombres de filas), NO en el Message agregado.
	if !strings.Contains(r.Detail, "suelta/viva") {
		t.Errorf("S1: la nota sin ancla debe aparecer como candidata en Detail, obtuve Detail=%q", r.Detail)
	}
	// B1 (unidad): el Message es el agregado y NO debe nombrar filas concretas. Si alguien volviera a
	// volcar las candidatas al Message, el borde MCP ya no podría redactarlas para otro tenant.
	if strings.Contains(r.Message, "suelta/viva") {
		t.Errorf("B1: las candidatas NO van en el Message (agregado), van en Detail, obtuve Message=%q", r.Message)
	}
	if strings.Contains(r.Detail, "anclada/ya") {
		t.Errorf("S2: una nota YA anclada no debe listarse como candidata, obtuve Detail=%q", r.Detail)
	}
	if !strings.Contains(r.Message, "origins.go") {
		t.Errorf("el Message debe nombrar el mecanismo (origins.go), obtuve %q", r.Message)
	}
}

// Contracara: con TODAS las notas que citan código ancladas, el check reporta limpio —sin hipótesis—
// y no es Unmeasured. Guarda contra un futuro bug donde el check liste algo siempre.
func TestAnchorCoverageTodasAncladasSinHipotesis(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("A", "temas/uno", "arranca en cmd/musubi/turn.go", nil); err != nil {
		t.Fatal(err)
	}
	anclar(t, e, "A")
	if err := e.SaveObservation("B", "temas/dos", "el olvido en decay.go", nil); err != nil {
		t.Fatal(err)
	}
	anclar(t, e, "B")

	r := checkAnchorCoverage(e)

	if r.Status != "ok" || r.Unmeasured {
		t.Errorf("con todo anclado esperaba ok medido, obtuve status=%q unmeasured=%v", r.Status, r.Unmeasured)
	}
	if strings.Contains(r.Message, "hipótesis") {
		t.Errorf("con todo anclado no debe haber hipótesis, obtuve %q", r.Message)
	}
	if !strings.Contains(r.Message, "tienen ancla") {
		t.Errorf("esperaba el Message de cobertura completa, obtuve %q", r.Message)
	}
	if r.Detail != "" {
		t.Errorf("con todo anclado no hay candidatas: Detail debe quedar vacío, obtuve %q", r.Detail)
	}
}

// detalleEsperado arma la cadena Detail EXACTA que checkAnchorCoverage produce para las candidatas
// dadas, EN EL ORDEN dado: "candidatas a anclar, por salience: <topic> (<token>); …". Afirmar la
// cadena entera —y no "contiene u01 pero no u09"— es lo que ata la EXISTENCIA del ranking: sin el sort
// el Detail saldría en otro orden y la comparación falla. (B3 en review/doctor-rancidez-silenciosa: un
// contains/not-contains sólo mordía la INVERSIÓN del comparador, no su ausencia.)
func detalleEsperado(ids ...string) string {
	partes := make([]string, 0, len(ids))
	for _, id := range ids {
		partes = append(partes, fmt.Sprintf("anchless/%s (%s.go)", id, id))
	}
	return "candidatas a anclar, por salience: " + strings.Join(partes, "; ")
}

// Tope + DIRECCIÓN del ranking: con más de anchorCandidateTop notas sin ancla, el Detail lista
// EXACTAMENTE anchorCandidateTop candidatas y en orden de salience descendente. CLAVE (B3): las notas
// se insertan en orden INVERSO a su salience (u09 primero … u01 último) para DESACOPLAR el orden de
// inserción del de salience. `observations` es tabla rowid y la query no tiene ORDER BY, así que sin el
// sort las filas saldrían en orden de inserción; si insertáramos u01..u09 en orden, ese orden
// coincidiría con la salience y el top-8 sin ordenar sería idéntico al ordenado —el verde NO mediría la
// existencia del sort, sólo su inversión—. Cada nota lleva una importancia DISTINTA por id (u01 la
// mayor, u09 la menor) y un created_at/last_accessed FIJO e idéntico: el único discriminador es la
// importancia, sin depender del segundo en que SQLite selló el save (CURRENT_TIMESTAMP tiene resolución
// de 1 s; sin fijarlo este test era flaky ~1,4%). Afirmar el Detail completo pone en ROJO tanto quitar
// el sort (M1) como invertir el comparador (M2). Lo que este fixture NO distingue —porque la importancia
// está CORRELACIONADA con el id (u01 la mayor en el id menor) y todo lo demás es igual, así que
// salience-DESC == importance-DESC == id-ASC— es un ranking por id crudo, por importancia cruda o sin
// edad: eso lo ata TestAnchorCoverageRankingEsPorSalienceNoPorIdNiImportancia, con un fixture que
// DESACOPLA la salience de esos tres confusores.
func TestAnchorCoverageTopeYDireccionDelRanking(t *testing.T) {
	e := newTestEngine(t)
	// Insertadas al revés de la salience: u09 primero (rowid más bajo), u01 último.
	ids := []string{"u09", "u08", "u07", "u06", "u05", "u04", "u03", "u02", "u01"}
	if anchorCandidateTop >= len(ids) {
		t.Fatalf("este test necesita MÁS candidatas (%d) que el tope (%d) para probar el truncado", len(ids), anchorCandidateTop)
	}
	for _, id := range ids {
		// topic único "anchless/<id>" y una cita de código única "<id>.go".
		if err := e.SaveObservation(id, "anchless/"+id, "mirar "+id+".go", nil); err != nil {
			t.Fatal(err)
		}
		// Importancia por el NÚMERO del id (u01→9 … u09→1), NO por el índice de inserción: la salience
		// queda al revés del orden en que se insertaron, y timestamp FIJO idéntico.
		var n int
		fmt.Sscanf(id, "u%d", &n)
		imp := float64(10 - n) // u01=9 … u09=1
		if _, err := e.db.Exec(
			`UPDATE observations SET importance=?, created_at=?, last_accessed=? WHERE id=?`,
			imp, "2026-01-01 00:00:00", "2026-01-01 00:00:00", id); err != nil {
			t.Fatalf("fijar importance/timestamp de %q: %v", id, err)
		}
	}

	r := checkAnchorCoverage(e)

	if r.Status != "ok" {
		t.Fatalf("Status debe ser ok, obtuve %q (%s)", r.Status, r.Message)
	}
	// El Detail debe ser EXACTAMENTE el top-8 por salience descendente (u01..u08). La cadena entera
	// —no un contains— ata el orden: sin el sort saldría en orden de inserción (u09..u02) y esto falla.
	if want := detalleEsperado("u01", "u02", "u03", "u04", "u05", "u06", "u07", "u08"); r.Detail != want {
		t.Errorf("orden/top-N del Detail equivocado:\n got  %q\n want %q", r.Detail, want)
	}
	// El ratio del Message cuenta las 9 en el denominador aunque el Detail liste sólo el tope.
	if !strings.Contains(r.Message, "0 de 9") {
		t.Errorf("el denominador debe contar las 9 anclables, obtuve %q", r.Message)
	}
}

// Desempate determinista por id: con la salience IDÉNTICA en las 9 candidatas (misma importancia y
// timestamps), el único discriminador del ranking es el desempate por id ascendente de
// checkAnchorCoverage. Insertadas en orden inverso de id (t09 primero … t01 último), el Detail debe dar
// t01..t08. Quitar el desempate (M3: sort.Slice queda inestable y conserva el orden de inserción
// t09..t02) o invertirlo (M4: id desc → t09..t02) pone esta aserción en ROJO.
func TestAnchorCoverageDesempateDeterministaPorID(t *testing.T) {
	e := newTestEngine(t)
	ids := []string{"t09", "t08", "t07", "t06", "t05", "t04", "t03", "t02", "t01"}
	for _, id := range ids {
		if err := e.SaveObservation(id, "anchless/"+id, "mirar "+id+".go", nil); err != nil {
			t.Fatal(err)
		}
		// Salience IDÉNTICA para las 9: misma importancia y timestamps → el orden lo decide SÓLO el
		// desempate por id.
		if _, err := e.db.Exec(
			`UPDATE observations SET importance=?, created_at=?, last_accessed=? WHERE id=?`,
			5.0, "2026-01-01 00:00:00", "2026-01-01 00:00:00", id); err != nil {
			t.Fatalf("fijar importance/timestamp de %q: %v", id, err)
		}
	}

	r := checkAnchorCoverage(e)

	if r.Status != "ok" {
		t.Fatalf("Status debe ser ok, obtuve %q (%s)", r.Status, r.Message)
	}
	if want := detalleEsperado("t01", "t02", "t03", "t04", "t05", "t06", "t07", "t08"); r.Detail != want {
		t.Errorf("el desempate por id (asc, determinista) está roto:\n got  %q\n want %q", r.Detail, want)
	}
}

// R6 / S7 — el ranking es por SALIENCE, no por id crudo, ni por importancia cruda, ni por edad sola, ni
// por orden de inserción. Éste es el test que CIERRA el hueco que dejaba abierto
// TestAnchorCoverageTopeYDireccionDelRanking: aquel fixture da a cada nota una importancia que decrece con
// el id y comparte todo lo demás, así que salience-DESC coincide con importance-DESC y con id-ASC, y un
// ranking que ordenara sólo por id, sólo por importancia cruda o ignorando la edad quedaría VERDE. Acá las
// tres notas se eligen para que el orden por salience (c,b,a) DIFIERA de todos los confusores: del de id
// (a,b,c), del de importancia cruda (a primero, imp 9), del de "sin edad" (a primero) y del de inserción
// (b,c,a). La premisa sal(c)>sal(b)>sal(a) se VERIFICA con salience() antes de afirmar, para no depender de
// un half-life concreto. Este fixture deja la FRECUENCIA (access=0) y el TIPO (mem_type="") en su valor
// identidad, así que por sí solo ata una proyección de salience() a importancia×recencia; los otros dos
// ejes los atan TestAnchorCoverageFrecuenciaReordenaElRanking y TestAnchorCoverageTipoReordenaElRanking,
// de modo que los tres juntos fijan el contrato completo "orden por salience() DESC".
// (Portado de la vuelta 4 del debate B3 en review/doctor-rancidez-silenciosa.)
func TestAnchorCoverageRankingEsPorSalienceNoPorIdNiImportancia(t *testing.T) {
	e := newTestEngine(t)
	type nota struct {
		id        string
		imp, edad float64 // edad en días
	}
	a, b, c := nota{"a", 9, 365}, nota{"b", 5, 100}, nota{"c", 5, 1}
	sal := func(n nota) float64 { return salience(n.imp, 0, n.edad, defaultHalfLifeDays, 1.0, 0) }
	if !(sal(c) > sal(b) && sal(b) > sal(a)) {
		t.Fatalf("premisa del fixture rota (half-life=%v): sal(c)=%v sal(b)=%v sal(a)=%v",
			defaultHalfLifeDays, sal(c), sal(b), sal(a))
	}
	now := time.Now().UTC()
	for _, n := range []nota{b, c, a} { // inserción b,c,a: ni el esperado (c,b,a) ni el de id (a,b,c)
		if err := e.SaveObservation(n.id, "anchless/"+n.id, "mirar "+n.id+".go", nil); err != nil {
			t.Fatal(err)
		}
		ts := now.Add(-time.Duration(n.edad * 24 * float64(time.Hour))).Format(sqliteTimeLayout)
		if _, err := e.db.Exec(
			`UPDATE observations SET importance=?, created_at=?, last_accessed=? WHERE id=?`,
			n.imp, ts, ts, n.id); err != nil {
			t.Fatalf("fijar importance/timestamps de %q: %v", n.id, err)
		}
	}

	r := checkAnchorCoverage(e)

	if want := detalleEsperado("c", "b", "a"); r.Detail != want {
		t.Errorf("el ranking debe ser por salience (c,b,a), no por id/importancia/inserción:\n got  %q\n want %q", r.Detail, want)
	}
}

// R1 / R3 — la población anclable son las notas VISIBLES (archived=0 AND superseded_by IS NULL AND
// quarantined=0, vía visibleObsPredicateDe). Una nota en cuarentena, archivada o reemplazada que cita
// código NO cuenta en el denominador y NO se nombra en el Detail. Sin este test, quitar el predicado de
// visibilidad de la query (mandar las cuatro a la población) quedaba VERDE: ningún otro test tiene una nota
// invisible que cite código. Y toca B1: una nota en CUARENTENA que se colara al Detail sería contenido en
// cuarentena surfaceado por el doctor. (Portado de la vuelta 4 del debate B3 en review/doctor-rancidez-silenciosa.)
func TestAnchorCoverageInvisiblesFueraDelConteoYDelDetail(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"vis", "quar", "arch", "supe"} {
		if err := e.SaveObservation(id, "anchless/"+id, "mirar "+id+".go", nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []struct{ sql, id string }{
		{`UPDATE observations SET quarantined=1 WHERE id=?`, "quar"},
		{`UPDATE observations SET archived=1 WHERE id=?`, "arch"},
		{`UPDATE observations SET superseded_by='vis' WHERE id=?`, "supe"},
	} {
		if _, err := e.db.Exec(q.sql, q.id); err != nil {
			t.Fatalf("invisibilizar %q: %v", q.id, err)
		}
	}

	r := checkAnchorCoverage(e)

	if !strings.Contains(r.Message, "0 de 1") {
		t.Errorf("sólo la nota visible debe contar (0 de 1), obtuve Message=%q", r.Message)
	}
	if want := detalleEsperado("vis"); r.Detail != want {
		t.Errorf("el Detail debe nombrar SÓLO la visible:\n got  %q\n want %q", r.Detail, want)
	}
}

// R1: anchor_coverage está registrado como deep, NO es reparable (sin count/apply) y NO está en el
// auto-heal. Un check que listara candidatas y después alguien "arreglara" en automático sería un
// cambio invisible en la memoria; este check sólo informa.
func TestAnchorCoverageNoSeAutoReparaNiEsReparable(t *testing.T) {
	e := newTestEngine(t)
	var encontrado bool
	for _, c := range e.doctorChecks() {
		if c.code != "anchor_coverage" {
			continue
		}
		encontrado = true
		if !c.deep {
			t.Error("anchor_coverage debe registrarse como deep (recorre content con regexp + salience en Go)")
		}
		if c.count != nil || c.apply != nil {
			t.Error("anchor_coverage NO es reparable: count y apply deben ser nil")
		}
	}
	if !encontrado {
		t.Fatal("anchor_coverage no está registrado en doctorChecks()")
	}
	if autoHealCodes["anchor_coverage"] {
		t.Error("anchor_coverage NO debe estar en autoHealCodes: es informativo, no se auto-repara")
	}
}

// R6 (eje FRECUENCIA) — el ranking responde al acceso: salience() incluye freq = 1 + ln(1+access_count)
// (decay.go), que NO depende del refuerzo K, y checkAnchorCoverage pasa access_count real. El fixture de
// TestAnchorCoverageRankingEsPorSalienceNoPorIdNiImportancia deja este eje en su identidad (access=0 → freq=1
// en todas), así que un ranking que pasara 0 en vez del acceso quedaría VERDE allí. Acá p2 tiene 20 accesos y
// p1 ninguno, con importancia y edad idénticas: el único discriminador es la frecuencia, y el orden debe ser
// (p2, p1). La premisa se VERIFICA con salience(). Pasar 0 en vez de access al salience del check (mutante N1
// del debate B3) pone esto en ROJO.
func TestAnchorCoverageFrecuenciaReordenaElRanking(t *testing.T) {
	e := newTestEngine(t)
	const imp, edad = 5.0, 100.0
	sal := func(acc int) float64 { return salience(imp, acc, edad, defaultHalfLifeDays, 1.0, 0) }
	if !(sal(20) > sal(0)) {
		t.Fatalf("premisa rota: salience con 20 accesos (%v) no supera a la de 0 accesos (%v)", sal(20), sal(0))
	}
	for _, id := range []string{"p1", "p2"} {
		if err := e.SaveObservation(id, "anchless/"+id, "mirar "+id+".go", nil); err != nil {
			t.Fatal(err)
		}
	}
	ts := time.Now().UTC().Add(-time.Duration(edad * 24 * float64(time.Hour))).Format(sqliteTimeLayout)
	for _, q := range []struct {
		id  string
		acc int
	}{{"p1", 0}, {"p2", 20}} {
		if _, err := e.db.Exec(
			`UPDATE observations SET importance=?, access_count=?, created_at=?, last_accessed=? WHERE id=?`,
			imp, q.acc, ts, ts, q.id); err != nil {
			t.Fatalf("fijar importance/access/timestamps de %q: %v", q.id, err)
		}
	}

	r := checkAnchorCoverage(e)

	if want := detalleEsperado("p2", "p1"); r.Detail != want {
		t.Errorf("el ranking debe responder a la frecuencia de acceso (p2, con más accesos, primero):\n got  %q\n want %q", r.Detail, want)
	}
}

// R6 (eje TIPO) — el ranking responde al peso por tipo: salience() multiplica por
// memTypeSalienceWeight(mem_type) (memtype.go), y checkAnchorCoverage pasa el peso real. El fixture de
// RankingEsPorSalience deja mem_type="" (peso 1.0) en todas, así que un ranking que pasara 1.0 fijo quedaría
// VERDE allí. Acá las dos notas comparten importancia, acceso y edad; sólo difieren en el tipo, y la de MAYOR
// peso debe ir primero. Pasar 1.0 en vez del peso real al salience del check (mutante N2 del debate B3) pone
// esto en ROJO.
func TestAnchorCoverageTipoReordenaElRanking(t *testing.T) {
	e := newTestEngine(t)
	wE, wP := memTypeSalienceWeight("episodic"), memTypeSalienceWeight("procedural")
	if wE == wP {
		t.Fatalf("premisa rota: episodic y procedural pesan igual (%v): la sonda no discrimina", wE)
	}
	lo, hi := "episodic", "procedural"
	if wE > wP {
		lo, hi = "procedural", "episodic"
	}
	for _, id := range []string{"p1", "p2"} {
		if err := e.SaveObservation(id, "anchless/"+id, "mirar "+id+".go", nil); err != nil {
			t.Fatal(err)
		}
	}
	ts := time.Now().UTC().Add(-100 * 24 * time.Hour).Format(sqliteTimeLayout)
	for _, q := range []struct{ id, tipo string }{{"p1", lo}, {"p2", hi}} {
		if _, err := e.db.Exec(
			`UPDATE observations SET importance=?, mem_type=?, created_at=?, last_accessed=? WHERE id=?`,
			5.0, q.tipo, ts, ts, q.id); err != nil {
			t.Fatalf("fijar importance/mem_type/timestamps de %q: %v", q.id, err)
		}
	}

	r := checkAnchorCoverage(e)

	if want := detalleEsperado("p2", "p1"); r.Detail != want {
		t.Errorf("el ranking debe responder al peso por tipo (p2=%s, de mayor peso, primero):\n got  %q\n want %q (p1=%s)", hi, r.Detail, want, lo)
	}
}
