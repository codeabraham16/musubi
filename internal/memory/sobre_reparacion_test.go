package memory

// LA IMPORTANCE QUE EL SOBRE SE LLEVÓ (R1–R5).
//
// QUÉ SE REPARA Y QUÉ NO. De las 73 observaciones medidas con el sobre adentro, 11 todavía dicen en
// su sobre qué importance se les pidió y su columna dice 1.0 —el default, y el valor más común de
// toda la memoria—, así que el recall las entierra justo por haber sido marcadas como importantes.
// La reparación les devuelve ESE número y no toca una coma del texto.
//
// El banco existe para fijar dónde está la línea: R1 que devuelve lo declarado, R2 que el texto y
// su hash quedan intactos, R3 que no aparece un falso verde, R4 que el número sale del sobre y no
// de la prosa, y R5 que lo que el plan promete es lo que el apply toca.

import (
	"strings"
	"testing"
)

// sembrarConSobre guarda una observación sana y después le mete el sobre por SQL crudo. El save de
// hoy rechazaría el texto envenenado —para eso está la guarda—, así que la fila tiene que nacer
// sana: es la misma secuencia por la que existen las de producción, guardadas por un binario
// anterior a la guarda.
func sembrarConSobre(t *testing.T, e *DbEngine, id, texto string, columna float64) {
	t.Helper()
	if err := e.SaveObservationTyped(id, "t/sobre", "un texto sano y suficientemente largo para pasar", columna, "semantic", ScopeLocal, nil); err != nil {
		t.Fatalf("sembrar %s: %v", id, err)
	}
	res, err := e.db.Exec(`UPDATE observations SET content = ? WHERE id = ?`, texto, id)
	if err != nil {
		t.Fatalf("envenenar %s: %v", id, err)
	}
	// Un UPDATE que no matchea nada es un éxito de SQL: sin esto el test seguiría midiendo la fila
	// sana y pasaría por razones que no tienen que ver con lo que dice probar.
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		t.Fatalf("el envenenamiento de %s no tocó su fila (filas=%d, err=%v)", id, n, err)
	}
}

// conSobre arma el texto tal como quedó en producción: el cuerpo, el cierre, y el sobre detrás.
func conSobre(cuerpo string, declarada string) string {
	return cuerpo + "\n</content>\n<parameter name=\"importance\">" + declarada + "\n</invoke>\n"
}

func importanciaDe(t *testing.T, e *DbEngine, id string) float64 {
	t.Helper()
	var v float64
	if err := e.db.QueryRow(`SELECT importance FROM observations WHERE id = ?`, id).Scan(&v); err != nil {
		t.Fatalf("leer importance de %s: %v", id, err)
	}
	return v
}

func nuevoEngineDePrueba(t *testing.T) *DbEngine {
	t.Helper()
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { e.Close() })
	return e
}

// R1 — DEVUELVE LO QUE EL SOBRE DECLARA, Y SÓLO A QUIEN LO DECLARA.
//
// Tres filas que se parecen y tienen que terminar distinto: la que perdió su número lo recupera, la
// que ya lo tiene bien no se toca, y la que nunca lo declaró se queda como está. Sin las tres, un
// apply que escribiera 1.9 en todas pasaría el test.
func TestR1LaReparacionDevuelveLaImportanceDeclarada(t *testing.T) {
	e := nuevoEngineDePrueba(t)

	sembrarConSobre(t, e, "perdida", conSobre("una nota que alguien marcó como importante", "1.9"), 1.0)
	sembrarConSobre(t, e, "yaEstaBien", conSobre("una nota cuyo número sí llegó a su columna", "1.6"), 1.6)
	sembrarConSobre(t, e, "sinNumero", "una nota sin importance en el sobre\n</content>\n</invoke>\n", 1.0)

	n, err := applySwallowedImportance(e)
	if err != nil {
		t.Fatalf("applySwallowedImportance: %v", err)
	}
	if n != 1 {
		t.Errorf("reparó %d filas, esperaba 1: sólo una de las tres tenía algo que devolver", n)
	}
	if got := importanciaDe(t, e, "perdida"); got != 1.9 {
		t.Errorf("perdida quedó en %v, esperaba 1.9: no se le devolvió lo que su sobre declara", got)
	}
	if got := importanciaDe(t, e, "yaEstaBien"); got != 1.6 {
		t.Errorf("yaEstaBien quedó en %v, esperaba 1.6: se tocó una fila que no había que tocar", got)
	}
	if got := importanciaDe(t, e, "sinNumero"); got != 1.0 {
		t.Errorf("sinNumero quedó en %v, esperaba 1.0: se le inventó un número que nadie declaró", got)
	}
}

// R2 — EL TEXTO Y SU HASH QUEDAN INTACTOS.
//
// ES LA RAZÓN POR LA QUE ESTA REPARACIÓN PUEDE EXISTIR. La nota de diseño rechazaba el `apply`
// porque reescribir el content cambia el content_hash, que es la clave del dedup y viaja en el
// sync. Se compara el hash ANTES y DESPUÉS, no el texto: el hash es lo que el dedup y el outbox
// realmente miran, y comparar el texto dejaría pasar una normalización que cambiara el hash.
func TestR2LaReparacionNoTocaElTextoNiSuHash(t *testing.T) {
	e := nuevoEngineDePrueba(t)
	texto := conSobre("una nota que alguien marcó como importante", "2.0")
	sembrarConSobre(t, e, "perdida", texto, 1.0)

	// El hash de la fila se recalcula desde el texto envenenado, que es el estado real de estas
	// filas en producción.
	hashEsperado := ContentHash(texto)
	if _, err := e.db.Exec(`UPDATE observations SET content_hash = ? WHERE id = ?`, hashEsperado, "perdida"); err != nil {
		t.Fatalf("fijar el hash de partida: %v", err)
	}

	if _, err := applySwallowedImportance(e); err != nil {
		t.Fatalf("applySwallowedImportance: %v", err)
	}

	var contenido, hash string
	if err := e.db.QueryRow(`SELECT content, content_hash FROM observations WHERE id = ?`, "perdida").Scan(&contenido, &hash); err != nil {
		t.Fatalf("releer la fila: %v", err)
	}
	if contenido != texto {
		t.Error("la reparación modificó el content: el sobre es la única evidencia del número y no se toca")
	}
	if hash != hashEsperado {
		// primerosN y no un slice: bajo un sabotaje el hash puede no ser un sha256, y un slice
		// fijo paniquea — el pánico se lleva el binario de test y los demás casos no corren.
		t.Errorf("el content_hash cambió (%s -> %s): el dedup deja de reconocer la fila y el nodo que la sincronizó la ve como contenido nuevo", primerosN(hashEsperado, 12), primerosN(hash, 12))
	}
}

// R3 — DESPUÉS DE REPARAR NO HAY FALSO VERDE.
//
// La objeción más fuerte de la nota de diseño era que un `apply` se vería como «arreglado» y
// dejaría con sus columnas en el default a las que no dejaron rastro. Acá se fija lo contrario: el
// check SIGUE avisando por el sobre, y lo único que cae a cero es el conteo de recuperables.
func TestR3ElCheckSigueAvisandoDespuesDeReparar(t *testing.T) {
	e := nuevoEngineDePrueba(t)
	sembrarConSobre(t, e, "perdida", conSobre("una nota importante", "1.9"), 1.0)
	sembrarConSobre(t, e, "sinRastro", "una nota que perdió su mem_type sin dejar rastro\n</content>\n</invoke>\n", 1.0)

	if _, err := applySwallowedImportance(e); err != nil {
		t.Fatalf("applySwallowedImportance: %v", err)
	}

	res := checkSwallowedEnvelope(e)
	if res.Status != "warning" {
		t.Errorf("el check quedó en %q tras reparar: las filas siguen teniendo el sobre y el check tiene que seguir diciéndolo", res.Status)
	}
	if !strings.Contains(res.Message, "2 observación(es)") {
		t.Errorf("el check dejó de contar las 2 filas con sobre: %s", res.Message)
	}
	if strings.Contains(res.Message, "todavía dicen en su sobre") {
		t.Errorf("el check sigue ofreciendo reparar algo que ya reparó: %s", res.Message)
	}
	// Y la misma afirmación en su forma estructurada, que es la que lee el cuerpo y el CLI: sin
	// nada que devolver, ofrecer la reparación sería ofrecer un no-op.
	if res.Repairable {
		t.Error("el check se sigue declarando reparable sin nada que devolver: la próxima corrida es un no-op con cara de arreglo")
	}
}

// R4 — EL NÚMERO SALE DEL SOBRE, NO DE LA PROSA.
//
// Es la trampa propia de este defecto: la observación que DOCUMENTA el bug cita las etiquetas. Si
// el número se leyera de todo el texto, reparar escribiría en la columna un valor que nadie pidió —
// y sobre una fila que ni siquiera está rota.
func TestR4ElNumeroSaleDelSobreYNoDeLaProsa(t *testing.T) {
	e := nuevoEngineDePrueba(t)

	// Documenta el defecto citando las etiquetas, pero su texto NO termina en sobre: hay prosa
	// después del cierre, así que no es el bug.
	documenta := "El sobre se ve así: `<parameter name=\"importance\">1.9`, y el corte pasa en\n" +
		"</content>\nsiempre después de ese cierre, nunca antes."
	sembrarConSobre(t, e, "documenta", documenta, 1.0)

	// Tiene el bug de verdad, y su cuerpo cita OTRO número que el de su sobre.
	mezclada := "Vi una nota con `<importance>1.2` adentro del cuerpo." + conSobre(" Y ésta es la mía.", "2.0")
	sembrarConSobre(t, e, "mezclada", mezclada, 1.0)

	n, err := applySwallowedImportance(e)
	if err != nil {
		t.Fatalf("applySwallowedImportance: %v", err)
	}
	if n != 1 {
		t.Errorf("reparó %d filas, esperaba 1: sólo la que tiene sobre de verdad", n)
	}
	if got := importanciaDe(t, e, "documenta"); got != 1.0 {
		t.Errorf("documenta quedó en %v, esperaba 1.0: se reparó una fila que sólo CITA la etiqueta", got)
	}
	if got := importanciaDe(t, e, "mezclada"); got != 2.0 {
		t.Errorf("mezclada quedó en %v, esperaba 2.0 (la de su sobre, no la 1.2 de su cuerpo)", got)
	}
}

// R5 — LO QUE EL PLAN PROMETE ES LO QUE EL APPLY TOCA.
//
// `Repair` en modo plan le muestra al usuario el número que devuelve `count`, y después `apply`
// hace lo suyo. Si los dos no salieran del MISMO criterio, el plan diría 11 y el apply tocaría
// otro conjunto — las dos cifras plausibles, nadie enterado.
func TestR5ElPlanYElApplyCuentanLoMismo(t *testing.T) {
	e := nuevoEngineDePrueba(t)
	sembrarConSobre(t, e, "a", conSobre("primera nota importante", "1.9"), 1.0)
	sembrarConSobre(t, e, "b", conSobre("segunda nota importante", "1.5"), 1.0)
	sembrarConSobre(t, e, "c", conSobre("tercera, con su número ya en la columna", "1.6"), 1.6)

	planeadas, err := countSwallowedImportance(e)
	if err != nil {
		t.Fatalf("countSwallowedImportance: %v", err)
	}
	aplicadas, err := applySwallowedImportance(e)
	if err != nil {
		t.Fatalf("applySwallowedImportance: %v", err)
	}
	if planeadas != 2 {
		t.Errorf("el plan prometió %d, esperaba 2", planeadas)
	}
	if aplicadas != planeadas {
		t.Errorf("el plan prometió %d y el apply tocó %d", planeadas, aplicadas)
	}

	// Y correrlo de nuevo no toca nada: ya no queda nada que devolver.
	otraVez, err := applySwallowedImportance(e)
	if err != nil {
		t.Fatalf("segundo applySwallowedImportance: %v", err)
	}
	if otraVez != 0 {
		t.Errorf("un segundo apply tocó %d filas: la reparación no es idempotente", otraVez)
	}
}
