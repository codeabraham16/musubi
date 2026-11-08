package memory

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Invariantes del A/B del decididor de skills del lado de la base (spec skills-decididor, R10/E9).

func filaABDe(t *testing.T, e *DbEngine, ctx context.Context, pid, arm string) DecididorABRow {
	t.Helper()
	rows, err := e.DecididorAB(ctx)
	if err != nil {
		t.Fatalf("DecididorAB: %v", err)
	}
	for _, r := range rows {
		if r.ProjectID == pid && r.Arm == arm {
			return r
		}
	}
	return DecididorABRow{}
}

func filasABDe(t *testing.T, e *DbEngine, ctx context.Context, pid string) []DecididorABRow {
	t.Helper()
	rows, err := e.DecididorAB(ctx)
	if err != nil {
		t.Fatalf("DecididorAB: %v", err)
	}
	out := make([]DecididorABRow, 0)
	for _, r := range rows {
		if r.ProjectID == pid {
			out = append(out, r)
		}
	}
	return out
}

// EL LIFT ES LA DIFERENCIA DE TASAS, agregando por brazo todos los proyectos de la lectura. Fixture
// SEMBRADO: las filas se construyen acá, no se leen del corpus vivo —si CalcularLift leyera una
// magnitud auto-consumible, el test se mediría a sí mismo (R10/E9)—. Los valores son fracciones
// exactas en binario (3/4, 1/4, 1/2) para poder comparar sin tolerancia.
// Sabotaje: cambiar `TasaOn - TasaOff` por `TasaOn + TasaOff` en CalcularLift ⇒ lift 1.0 ⇒ rojo.
// arnes: archivo="internal/memory/decididor_ab.go"
// arnes: de="l.Lift = l.TasaOn - l.TasaOff"
// arnes: a="l.Lift = l.TasaOn + l.TasaOff"
func TestCalcularLiftMideLaDiferenciaDeTasas(t *testing.T) {
	rows := []DecididorABRow{
		{ProjectID: "a", Arm: BrazoOn, Oportunidades: 2, Cargadas: 2},
		{ProjectID: "b", Arm: BrazoOn, Oportunidades: 2, Cargadas: 1}, // on: oport 4, carg 3 ⇒ 0.75
		{ProjectID: "a", Arm: BrazoOff, Oportunidades: 2, Cargadas: 1},
		{ProjectID: "b", Arm: BrazoOff, Oportunidades: 2, Cargadas: 0}, // off: oport 4, carg 1 ⇒ 0.25
	}
	l, ok := CalcularLift(rows)
	if !ok {
		t.Fatal("con oportunidades en los dos brazos el lift debe estar definido")
	}
	if l.OportunidadesOn != 4 || l.CargadasOn != 3 || l.OportunidadesOff != 4 || l.CargadasOff != 1 {
		t.Fatalf("la agregación por brazo salió mal: %+v", l)
	}
	if l.TasaOn != 0.75 || l.TasaOff != 0.25 {
		t.Fatalf("tasas: on=%v off=%v (esperaba 0.75 / 0.25)", l.TasaOn, l.TasaOff)
	}
	if l.Lift != 0.5 {
		t.Fatalf("lift=%v, esperaba 0.5 (= 0.75 − 0.25)", l.Lift)
	}
}

// R10/E9 — SIN DENOMINADOR EN ALGÚN BRAZO EL LIFT ES INDEFINIDO (ok=false), NO CERO. Es el estado
// natural de un A/B default-off recién encendido: un brazo todavía no vio ocasiones. Reportar lift=0
// ahí sería afirmar «no hubo efecto» cuando la verdad es «no se puede medir todavía».
// Sabotaje: borrar el guard `OportunidadesOn==0 || OportunidadesOff==0` ⇒ división por cero devuelve
// ok=true (con +Inf/NaN) ⇒ rojo.
// arnes: archivo="internal/memory/decididor_ab.go"
// arnes: de="if l.OportunidadesOn == 0 || l.OportunidadesOff == 0 {"
// arnes: a="if false {"
func TestCalcularLiftSinDenominadorEsIndefinido(t *testing.T) {
	soloOn := []DecididorABRow{{Arm: BrazoOn, Oportunidades: 5, Cargadas: 3}}
	if _, ok := CalcularLift(soloOn); ok {
		t.Fatal("sin oportunidades en el brazo off el lift es INDEFINIDO")
	}
	soloOff := []DecididorABRow{{Arm: BrazoOff, Oportunidades: 5, Cargadas: 3}}
	if _, ok := CalcularLift(soloOff); ok {
		t.Fatal("sin oportunidades en el brazo on el lift es INDEFINIDO")
	}
	if _, ok := CalcularLift(nil); ok {
		t.Fatal("sin datos el lift es indefinido")
	}
}

// LA CARGA CUENTA SÓLO SI YA HAY UNA OPORTUNIDAD para ese (proyecto, brazo). Es la decisión UPDATE-only
// de bumpCargadasDecididorTx: una carga de un período sin A/B no debe inflar el numerador ni nacer una
// fila con oportunidades=0 que rompería el lift.
// Sabotaje: convertir el UPDATE de bumpCargadasDecididorTx en un upsert (INSERT ... ON CONFLICT) ⇒ la
// carga del paso 1 crea una fila ⇒ `len != 0` ⇒ rojo.
// arnes: archivo="internal/memory/decididor_ab.go"
// arnes: de="UPDATE decididor_ab SET cargadas = cargadas + ?, last_at = CURRENT_TIMESTAMP\n\t\t\tWHERE project_id = ? AND arm = ?"
// arnes: a="INSERT INTO decididor_ab (cargadas, project_id, arm) VALUES (?, ?, ?)\n\t\t\tON CONFLICT(project_id, arm) DO UPDATE SET cargadas = cargadas + excluded.cargadas"
func TestCargaCuentaSoloConOportunidad(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	pid := "proj-decididor"
	arm := BrazoAB(pid, time.Now())

	// 1) Carga sin oportunidad previa: UPDATE-only ⇒ no crea fila, no cuenta.
	contarSkill(t, e, ctx, SkillEvent{Skill: "go-hygiene", ProjectID: pid, Kind: UsoCargada})
	if filas := len(filasABDe(t, e, ctx, pid)); filas != 0 {
		t.Fatalf("una carga sin oportunidad no debe crear fila; hay %d", filas)
	}

	// 2) Nace la oportunidad (la fila existe) y se vuelve a cargar: ahora sí suma.
	if err := e.RegistrarOportunidadDecididor(ctx, pid, arm, true); err != nil {
		t.Fatalf("RegistrarOportunidadDecididor: %v", err)
	}
	contarSkill(t, e, ctx, SkillEvent{Skill: "go-hygiene", ProjectID: pid, Kind: UsoCargada})

	fila := filaABDe(t, e, ctx, pid, arm)
	if fila.Oportunidades != 1 || fila.Exigencias != 1 || fila.Cargadas != 1 {
		t.Fatalf("esperaba oport=1 exig=1 carg=1, obtuve %+v", fila)
	}
}

// EXIGENCIAS SUMA SÓLO CUANDO EL MOTOR EXIGIÓ; oportunidades suma siempre. exigencias es telemetría
// para leer el lift, no el denominador.
// Sabotaje: en RegistrarOportunidadDecididor arrancar `inc` en 1 (inc := 1) ⇒ exigencias suma aun sin
// exigir ⇒ exig=2 ⇒ rojo.
// arnes: archivo="internal/memory/decididor_ab.go"
// arnes: de="inc := 0"
// arnes: a="inc := 1"
func TestRegistrarOportunidadSumaExigenciasSoloSiExigio(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	pid := "proj-x"
	// Brazo fijo: este camino no pasa por el hook de carga, así que no depende del día.
	e.RegistrarOportunidadDecididor(ctx, pid, BrazoOn, true)  // exigió
	e.RegistrarOportunidadDecididor(ctx, pid, BrazoOn, false) // no exigió

	fila := filaABDe(t, e, ctx, pid, BrazoOn)
	if fila.Oportunidades != 2 || fila.Exigencias != 1 {
		t.Fatalf("2 ocasiones, 1 con exigencia: esperaba oport=2 exig=1, obtuve %+v", fila)
	}
}

// Un brazo que no es on/off es un error del caller, no una fila basura en la tabla.
func TestRegistrarOportunidadRechazaBrazoInvalido(t *testing.T) {
	e := newTestEngine(t)
	if err := e.RegistrarOportunidadDecididor(context.Background(), "p", "medio", true); err == nil {
		t.Fatal("un brazo distinto de on/off debe ser error")
	}
}

// Track 19 — LA LECTURA SE ACOTA AL PROYECTO DE LA CREDENCIAL: el experimento de otro tenant no se ve.
// Sabotaje: quitar scopeSQL de la query de DecididorAB ⇒ proj-b se filtra en la lectura de proj-a ⇒ rojo.
// arnes: archivo="internal/memory/decididor_ab.go"
// arnes: de=`scopeSQL, scopeArgs := projectScopeFrom(ctx).scopeClause("")`
// arnes: a=`scopeSQL, scopeArgs := "", []any(nil)`
func TestDecididorABAcotaAlProyecto(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	e.RegistrarOportunidadDecididor(ctx, "proj-a", BrazoOn, true)
	e.RegistrarOportunidadDecididor(ctx, "proj-b", BrazoOn, true)

	scoped := WithProjectScope(ctx, ProjectScope{ProjectID: "proj-a"})
	rows, err := e.DecididorAB(scoped)
	if err != nil {
		t.Fatalf("DecididorAB: %v", err)
	}
	vioA := false
	for _, r := range rows {
		if r.ProjectID == "proj-b" {
			t.Fatalf("la lectura acotada a proj-a vio a proj-b: %+v", r)
		}
		if r.ProjectID == "proj-a" {
			vioA = true
		}
	}
	if !vioA {
		t.Fatal("proj-a debería estar en su propia lectura")
	}
}

// R10/E9 EN LA PRESENTACIÓN — EL LIFT INDEFINIDO SE DICE «INDEFINIDO», NUNCA 0. Es el mismo invariante
// que CalcularLift custodia en el cálculo, pero en la capa que una persona lee en musubi_skill_usage:
// con un brazo sin ocasiones, imprimir «0» afirmaría «no hubo efecto» cuando la verdad es «todavía no
// se puede medir». Fixture SEMBRADO (no lee el corpus vivo).
// Sabotaje: en la rama !ok de FormatLiftDecididor imprimir el número `l.Lift` (que vale 0) en vez de
// la palabra INDEFINIDO ⇒ aparece «+0.0» y desaparece «INDEFINIDO» ⇒ rojo.
// arnes: archivo="internal/memory/decididor_ab.go"
// arnes: de="b.WriteString(\"  lift = INDEFINIDO: un brazo todavía no acumuló ocasiones; no es 0, es que aún no se puede medir.\\n\")"
// arnes: a="fmt.Fprintf(&b, \"  lift = %+.1f\\n\", l.Lift*100)"
func TestFormatLiftIndefinidoDiceIndefinidoNoCero(t *testing.T) {
	// Un solo brazo con ocasiones ⇒ CalcularLift devuelve ok=false (el otro brazo no tiene denominador).
	soloOn := []DecididorABRow{{Arm: BrazoOn, Oportunidades: 5, Cargadas: 3}}
	txt := FormatLiftDecididor(CalcularLift(soloOn))
	if !strings.Contains(txt, "INDEFINIDO") {
		t.Fatalf("un brazo sin ocasiones hace el lift indefinido y hay que decirlo:\n%s", txt)
	}
	if strings.Contains(txt, "+0.0") || strings.Contains(txt, "lift = 0") {
		t.Fatalf("el lift indefinido NO es 0: no puede imprimir un número de lift:\n%s", txt)
	}
	// Pero los crudos del brazo que SÍ tiene ocasiones se muestran: el dato honesto es «faltan ocasiones».
	if !strings.Contains(txt, "3 cargas / 5 ocasiones") {
		t.Fatalf("los crudos del brazo con ocasiones tienen que verse:\n%s", txt)
	}
}

// Con ocasiones en los DOS brazos el lift es medible y se imprime el número con signo.
// Sabotaje: forzar la rama !ok (p. ej. no imprimir nunca el número) ⇒ falta «+50.0» ⇒ rojo.
// arnes: archivo="internal/memory/decididor_ab.go"
// arnes: de="\tif ok {"
// arnes: a="\tif false {"
func TestFormatLiftMedibleMuestraElNumero(t *testing.T) {
	rows := []DecididorABRow{
		{Arm: BrazoOn, Oportunidades: 4, Cargadas: 3},  // 75%
		{Arm: BrazoOff, Oportunidades: 4, Cargadas: 1}, // 25%
	}
	txt := FormatLiftDecididor(CalcularLift(rows))
	if strings.Contains(txt, "INDEFINIDO") {
		t.Fatalf("con los dos brazos poblados el lift es medible, no indefinido:\n%s", txt)
	}
	if !strings.Contains(txt, "+50.0") {
		t.Fatalf("lift = 75%% − 25%% = +50.0 puntos; no aparece:\n%s", txt)
	}
}

// Sin NINGUNA fila (el default: decididor apagado) el bloque lo dice en una línea, sin hablar de un
// lift que no existe. Distinguir «apagado» de «un brazo sin datos» es lo que evita leer ceros como
// medición.
func TestFormatLiftSinFilasLoDice(t *testing.T) {
	txt := FormatLiftDecididor(CalcularLift(nil))
	if !strings.Contains(txt, "sin ocasiones todavía") {
		t.Fatalf("sin filas el bloque tiene que decir que no hubo ocasiones:\n%s", txt)
	}
	if strings.Contains(txt, "INDEFINIDO") || strings.Contains(txt, "lift =") {
		t.Fatalf("sin filas no se habla de lift: no hubo experimento:\n%s", txt)
	}
}

// ─── MergeDecididorABDesdeCentral: el cable que cierra el límite deferido (spec ab-lift-bajada) ───

// I1 (R3) — EL MERGE PUEBLA LA BASE LOCAL CON EL DENOMINADOR DEL CENTRAL. oportunidades/exigencias
// nacen sólo en el central (RegistrarOportunidadDecididor corre con motor real); sin este canal nunca
// llegaban a la base local y el lift era inmedible. Fixture SEMBRADO: el snapshot se construye acá.
// Sabotaje: bindear 0 en vez del oportunidades del central (se conserva el conteo de placeholders,
// así el rojo es por el VALOR observado y no por un mismatch de args) ⇒ oport=0 ⇒ rojo.
// arnes: archivo="internal/memory/decididor_ab.go"
// arnes: de="r.ProjectID, r.Arm, r.Oportunidades, r.Exigencias, r.FirstAt, r.LastAt); err != nil {"
// arnes: a="r.ProjectID, r.Arm, 0, r.Exigencias, r.FirstAt, r.LastAt); err != nil {"
func TestMergePueblaElDenominadorLocal(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	pid := "proj-merge-puebla"

	if err := e.MergeDecididorABDesdeCentral([]DecididorABRow{
		{ProjectID: pid, Arm: BrazoOn, Oportunidades: 5, Exigencias: 2},
	}); err != nil {
		t.Fatalf("MergeDecididorABDesdeCentral: %v", err)
	}

	fila := filaABDe(t, e, ctx, pid, BrazoOn)
	if fila.Oportunidades != 5 || fila.Exigencias != 2 {
		t.Fatalf("el merge tiene que traer el denominador del central; esperaba oport=5 exig=2, obtuve %+v", fila)
	}
}

// D3 — UNA FILA NUEVA NACE CON cargadas=0, NO HEREDA EL cargadas DEL CENTRAL. Las cargas del central
// son de OTRA máquina; heredarlas contaminaría el numerador local y fabricaría un lift falso. El
// snapshot trae cargadas=777 a propósito: tiene que ser IGNORADO.
// Sabotaje: insertar cargadas con un literal ≠0 ⇒ la fila nueva nace con ese número ⇒ rojo.
// arnes: archivo="internal/memory/decididor_ab.go"
// arnes: de="?, ?, ?, ?, 0, COALESCE"
// arnes: a="?, ?, ?, ?, 999, COALESCE"
func TestMergeNoHeredaCargadasDelCentral(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	pid := "proj-merge-nohereda"

	if err := e.MergeDecididorABDesdeCentral([]DecididorABRow{
		{ProjectID: pid, Arm: BrazoOn, Oportunidades: 5, Exigencias: 2, Cargadas: 777},
	}); err != nil {
		t.Fatalf("MergeDecididorABDesdeCentral: %v", err)
	}

	if fila := filaABDe(t, e, ctx, pid, BrazoOn); fila.Cargadas != 0 {
		t.Fatalf("una fila nueva nace con cargadas=0 y NO hereda el 777 del central; obtuve cargadas=%d", fila.Cargadas)
	}
}

// I2 (R3) — EL MERGE ES IDEMPOTENTE: bajar el MISMO snapshot dos veces deja los MISMOS números.
// oportunidades/exigencias se REEMPLAZAN con el valor del central, no se suman; si se sumaran, cada
// bajada duplicaría el denominador y fabricaría un lift falso.
// Sabotaje: que el DO UPDATE SUME en vez de reemplazar oportunidades ⇒ la 2ª bajada da oport=10 ⇒ rojo.
// arnes: archivo="internal/memory/decididor_ab.go"
// arnes: de="oportunidades = excluded.oportunidades,"
// arnes: a="oportunidades = decididor_ab.oportunidades + excluded.oportunidades,"
func TestMergeEsIdempotente(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	pid := "proj-merge-idem"
	snap := []DecididorABRow{{ProjectID: pid, Arm: BrazoOn, Oportunidades: 5, Exigencias: 2}}

	for i := 0; i < 2; i++ {
		if err := e.MergeDecididorABDesdeCentral(snap); err != nil {
			t.Fatalf("merge #%d: %v", i+1, err)
		}
	}

	if fila := filaABDe(t, e, ctx, pid, BrazoOn); fila.Oportunidades != 5 || fila.Exigencias != 2 {
		t.Fatalf("bajar el mismo snapshot dos veces no debe sumar; esperaba oport=5 exig=2, obtuve %+v", fila)
	}
}

// I3 (R4) — EL MERGE PRESERVA EL cargadas LOCAL (el numerador es del dueño LOCAL) mientras REEMPLAZA
// el denominador con el del central. Es la autoridad por columna: el DO UPDATE no nombra cargadas, así
// que una bajada no pisa lo que esta máquina contó, aunque el central mande otro valor.
// Sabotaje: agregar `cargadas = excluded.cargadas` al DO UPDATE ⇒ la bajada pisa el cargadas local
// (lo lleva a 0, el literal del INSERT) ⇒ rojo.
// arnes: archivo="internal/memory/decididor_ab.go"
// arnes: de="\t\t\t\texigencias    = excluded.exigencias,\n"
// arnes: a="\t\t\t\texigencias    = excluded.exigencias,\n\t\t\t\tcargadas = excluded.cargadas,\n"
func TestMergePreservaCargadasLocal(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	pid := "proj-merge-preserva"
	arm := BrazoAB(pid, time.Now())

	// Estado local previo: una oportunidad (la fila existe) y una carga contada acá ⇒ cargadas=1.
	if err := e.RegistrarOportunidadDecididor(ctx, pid, arm, true); err != nil {
		t.Fatalf("RegistrarOportunidadDecididor: %v", err)
	}
	contarSkill(t, e, ctx, SkillEvent{Skill: "go-hygiene", ProjectID: pid, Kind: UsoCargada})
	if fila := filaABDe(t, e, ctx, pid, arm); fila.Cargadas != 1 {
		t.Fatalf("precondición: esperaba cargadas=1 local, obtuve %+v", fila)
	}

	// El central manda otro oportunidades/exigencias para ESE mismo (pid, brazo), y un cargadas ajeno.
	if err := e.MergeDecididorABDesdeCentral([]DecididorABRow{
		{ProjectID: pid, Arm: arm, Oportunidades: 50, Exigencias: 20, Cargadas: 99},
	}); err != nil {
		t.Fatalf("MergeDecididorABDesdeCentral: %v", err)
	}

	fila := filaABDe(t, e, ctx, pid, arm)
	if fila.Cargadas != 1 {
		t.Fatalf("el merge NO debe pisar el cargadas local; esperaba cargadas=1, obtuve %d", fila.Cargadas)
	}
	if fila.Oportunidades != 50 || fila.Exigencias != 20 {
		t.Fatalf("el denominador SÍ se reemplaza con el del central; esperaba oport=50 exig=20, obtuve %+v", fila)
	}
}

// Un brazo que no es on/off aborta el merge ENTERO (una sola transacción). El central sólo guarda
// 'on'/'off' (CHECK de v59), así que recibir otra cosa es una anomalía, no un borde que valga tragarse.
func TestMergeRechazaBrazoInvalido(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	pid := "proj-merge-malbrazo"

	err := e.MergeDecididorABDesdeCentral([]DecididorABRow{
		{ProjectID: pid, Arm: BrazoOn, Oportunidades: 3},
		{ProjectID: pid, Arm: "medio", Oportunidades: 9}, // brazo inválido: aborta el merge entero
	})
	if err == nil {
		t.Fatal("un brazo distinto de on/off debe ser error")
	}
	// Atómico: la fila válida del mismo lote NO quedó a medias.
	if filas := len(filasABDe(t, e, ctx, pid)); filas != 0 {
		t.Fatalf("un brazo inválido aborta la transacción entera; no debe quedar ninguna fila, hay %d", filas)
	}
}

// T8 (R6) — EL FLUJO CRUZANDO EL SYNC: el merge DESBLOQUEA el lift. Es el end-to-end del límite
// deferido. Antes del merge la base local no tiene filas (oportunidades nace en el central y el bump
// es UPDATE-only): el lift es INMEDIBLE (ok=false). El merge baja el denominador de los DOS brazos y
// el lift se vuelve MEDIBLE (ok=true). Y una carga local AHORA SÍ aterriza sobre la fila que bajó el
// merge —antes la carga se perdía porque la fila no existía—.
// Sabotaje: volver el merge un no-op (early return) ⇒ el lift NUNCA se vuelve medible ⇒ rojo.
// arnes: archivo="internal/memory/decididor_ab.go"
// arnes: de="\tif len(rows) == 0 {\n\t\treturn nil\n\t}"
// arnes: a="\tif true {\n\t\treturn nil\n\t}"
func TestMergeDesbloqueaElLiftCruzandoElSync(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	pid := "proj-e2e-lift"

	// 1) Sin filas locales no hay denominador: el lift es INDEFINIDO. Es el límite que este PR cierra.
	if _, ok := CalcularLift(filasABDe(t, e, ctx, pid)); ok {
		t.Fatal("sin filas locales el lift tiene que ser indefinido (el límite deferido)")
	}

	// 2) El sync entrante baja el denominador de los DOS brazos (cargadas=777 a propósito: se ignora).
	if err := e.MergeDecididorABDesdeCentral([]DecididorABRow{
		{ProjectID: pid, Arm: BrazoOn, Oportunidades: 10, Exigencias: 4, Cargadas: 777},
		{ProjectID: pid, Arm: BrazoOff, Oportunidades: 8, Exigencias: 3, Cargadas: 777},
	}); err != nil {
		t.Fatalf("MergeDecididorABDesdeCentral: %v", err)
	}

	// 3) Con los dos denominadores presentes el lift YA es medible (ok=true), aunque valga 0: el flujo
	//    cruzó el sync. cargadas nace en 0 pese al 777 del central.
	l, ok := CalcularLift(filasABDe(t, e, ctx, pid))
	if !ok {
		t.Fatal("tras bajar los dos brazos el lift tiene que ser medible")
	}
	if l.CargadasOn != 0 || l.CargadasOff != 0 {
		t.Fatalf("cargadas del central no se hereda; esperaba 0/0, obtuve on=%d off=%d", l.CargadasOn, l.CargadasOff)
	}

	// 4) Una carga local aterriza sobre la fila bajada por el merge (el bump UPDATE-only ya la encuentra).
	//    Bajamos los dos brazos, así que cae sea cual sea el brazo del día.
	contarSkill(t, e, ctx, SkillEvent{Skill: "go-hygiene", ProjectID: pid, Kind: UsoCargada})
	l2, ok := CalcularLift(filasABDe(t, e, ctx, pid))
	if !ok {
		t.Fatal("el lift sigue medible tras la carga")
	}
	if l2.CargadasOn+l2.CargadasOff != 1 {
		t.Fatalf("la carga tenía que aterrizar sobre una fila bajada del central; cargadas totales on=%d off=%d", l2.CargadasOn, l2.CargadasOff)
	}
}

// I4 (hallazgo de la revisión adversarial, reforzado en la 2ª) — LAS MARCAS SE COMPARAN
// CRONOLÓGICAMENTE, NO COMO TEXTO, EN LAS DOS ORIENTACIONES. first_at/last_at llegan en DOS layouts: el
// central en RFC3339 ('…T…Z', porque allá la columna DATETIME se escanea a un string de Go) y una punta
// en CURRENT_TIMESTAMP ('… …', con espacio). Como el espacio (0x20) ordena ANTES que la 'T' (0x54), un
// MIN/MAX de TEXTO elige mal el borde de la ventana cuando una fila mezcla los dos; datetime() normaliza
// LOS DOS operandos antes de comparar.
//
// Dos fixtures, porque cada orientación fija un operando distinto del MIN/MAX:
//   - directa: local en ESPACIO tardío (23:00/23:30), entrante RFC3339 temprano (08:00/09:00). Fija el
//     datetime() del operando ENTRANTE (excluded.*).
//   - inversa: local en RFC3339 temprano (08:00/09:00), entrante ESPACIO tardío (23:00/23:30). Fija el
//     datetime() del operando LOCAL (decididor_ab.*). Sin ésta, un fix one-sided que normalice sólo el
//     lado entrante quedaba VERDE (lo midió la 2ª revisión).
//
// En las dos, first_at tiene que quedar en las 08:00 y last_at en las 23:30. Se asserta por la HORA, que
// sobrevive a la normalización de la lectura: importa QUÉ marca ganó.
//
// Sabotaje: sacarle el datetime() al operando LOCAL del MIN de first_at (el one-sided que la orientación
// directa NO cazaba) ⇒ la INVERSA vuelve al orden de texto ⇒ su first_at se queda con la marca ESPACIO
// tardía (23:00) en vez de la temprana (08:00) ⇒ rojo.
// arnes: archivo="internal/memory/decididor_ab.go"
// arnes: de="first_at = MIN(datetime(decididor_ab.first_at), datetime(excluded.first_at)),"
// arnes: a="first_at = MIN(decididor_ab.first_at, datetime(excluded.first_at)),"
func TestMergeComparaLasMarcasCronologicamenteNoComoTexto(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	pid := "proj-merge-crono"

	// Merge #1: siembra la fila local con marcas en formato ESPACIO (como las de CURRENT_TIMESTAMP) y
	// TARDÍAS en el día. El VALUES no normaliza, así que quedan guardadas tal cual (texto con espacio).
	if err := e.MergeDecididorABDesdeCentral([]DecididorABRow{
		{ProjectID: pid, Arm: BrazoOn, Oportunidades: 5, Exigencias: 2, FirstAt: "2026-11-07 23:00:00", LastAt: "2026-11-07 23:30:00"},
	}); err != nil {
		t.Fatalf("merge #1 (siembra formato espacio): %v", err)
	}

	// Merge #2: el central manda marcas RFC3339 y TEMPRANAS. Al mezclarse con las de espacio del merge
	// #1, un MIN/MAX de texto elegiría mal; datetime() tiene que rescatar el orden cronológico.
	if err := e.MergeDecididorABDesdeCentral([]DecididorABRow{
		{ProjectID: pid, Arm: BrazoOn, Oportunidades: 9, Exigencias: 3, FirstAt: "2026-11-07T08:00:00Z", LastAt: "2026-11-07T09:00:00Z"},
	}); err != nil {
		t.Fatalf("merge #2 (mezcla de formatos): %v", err)
	}

	fila := filaABDe(t, e, ctx, pid, BrazoOn)
	if !strings.Contains(fila.FirstAt, "08:00:00") {
		t.Fatalf("first_at tiene que ser la marca cronológicamente MÁS TEMPRANA (08:00:00), no la que gana por texto; obtuve %q", fila.FirstAt)
	}
	if !strings.Contains(fila.LastAt, "23:30:00") {
		t.Fatalf("last_at tiene que ser la marca cronológicamente MÁS TARDÍA (23:30:00), no la que gana por texto; obtuve %q", fila.LastAt)
	}

	// ORIENTACIÓN INVERSA (hallazgo de la 2ª revisión). El caso de arriba deja el formato ESPACIO del
	// lado LOCAL y el RFC3339 del ENTRANTE, así que sólo fija el datetime() del operando `excluded.*`.
	// Acá se da vuelta: la fila local nace en RFC3339 TEMPRANO y el entrante trae ESPACIO TARDÍO, de
	// modo que el borde correcto sale del operando `decididor_ab.*`. Un fix que normalice sólo el lado
	// entrante pasaría el caso directo y dejaría ÉSTE en rojo — por eso los dos.
	pidInv := "proj-merge-crono-inv"
	if err := e.MergeDecididorABDesdeCentral([]DecididorABRow{
		{ProjectID: pidInv, Arm: BrazoOn, Oportunidades: 5, Exigencias: 2, FirstAt: "2026-11-07T08:00:00Z", LastAt: "2026-11-07T09:00:00Z"},
	}); err != nil {
		t.Fatalf("merge #1 inverso (siembra RFC3339 temprano): %v", err)
	}
	if err := e.MergeDecididorABDesdeCentral([]DecididorABRow{
		{ProjectID: pidInv, Arm: BrazoOn, Oportunidades: 9, Exigencias: 3, FirstAt: "2026-11-07 23:00:00", LastAt: "2026-11-07 23:30:00"},
	}); err != nil {
		t.Fatalf("merge #2 inverso (mezcla de formatos): %v", err)
	}
	filaInv := filaABDe(t, e, ctx, pidInv, BrazoOn)
	if !strings.Contains(filaInv.FirstAt, "08:00:00") {
		t.Fatalf("inverso: first_at tiene que quedar en la marca MÁS TEMPRANA (08:00:00, el lado LOCAL), no la que gana por texto; obtuve %q", filaInv.FirstAt)
	}
	if !strings.Contains(filaInv.LastAt, "23:30:00") {
		t.Fatalf("inverso: last_at tiene que quedar en la marca MÁS TARDÍA (23:30:00, el lado ENTRANTE), no la que gana por texto; obtuve %q", filaInv.LastAt)
	}
}

// I5 (R7) — EL MERGE DE BAJADA NO ENCOLA NADA EN EL OUTBOX (sin eco). El contador del A/B sólo BAJA:
// si el merge encolara algo para subir, cerraría el loop «re-sube lo que baja» que el sello anti-eco
// del sync existe para evitar. Se ARMA el uplink (outbox habilitado) y, aun así, tras el merge el
// outbox tiene que quedar VACÍO.
// Sabotaje: encolar una fila pendiente en el outbox dentro del merge ⇒ queda 1 pendiente ⇒ rojo.
// arnes: archivo="internal/memory/decididor_ab.go"
// arnes: de="if err := tx.Commit(); err != nil {\n\t\treturn fmt.Errorf(\"decididor A/B: commitear el merge de bajada: %w\", err)"
// arnes: a="if _, e2 := tx.Exec(\"INSERT INTO outbox (obs_id, enqueued_hash, status, attempts, next_attempt_at, created_at, updated_at) VALUES ('ab-echo-saboteo', 'h', 'pending', 0, datetime('now'), datetime('now'), datetime('now'))\"); e2 != nil {\n\t\treturn e2\n\t}\n\tif err := tx.Commit(); err != nil {\n\t\treturn fmt.Errorf(\"decididor A/B: commitear el merge de bajada: %w\", err)"
func TestMergeNoEncolaNadaEnElOutbox(t *testing.T) {
	e := newTestEngine(t)
	e.SetOutboxEnabled(true) // el uplink armado: aun así el merge de bajada no debe encolar nada
	pid := "proj-merge-noeco"

	if err := e.MergeDecididorABDesdeCentral([]DecididorABRow{
		{ProjectID: pid, Arm: BrazoOn, Oportunidades: 5, Exigencias: 2},
		{ProjectID: pid, Arm: BrazoOff, Oportunidades: 4, Exigencias: 1},
	}); err != nil {
		t.Fatalf("MergeDecididorABDesdeCentral: %v", err)
	}

	pending, _, _, err := e.OutboxStats()
	if err != nil {
		t.Fatalf("OutboxStats: %v", err)
	}
	if pending != 0 {
		t.Fatalf("el merge de bajada no debe encolar nada para subir; outbox pendiente=%d", pending)
	}
	if hay, err := e.HayOutboxPorSubir(); err != nil {
		t.Fatalf("HayOutboxPorSubir: %v", err)
	} else if hay {
		t.Fatal("el merge de bajada no debe dejar nada por subir en el outbox")
	}
}
