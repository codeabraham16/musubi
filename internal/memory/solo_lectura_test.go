package memory

import (
	"context"
	"strings"
	"testing"
)

// baseMasNueva deja una base migrada de verdad y después le adelanta el user_version, para que
// este binario no pueda migrarla pero su piso sí la declare legible.
func baseMasNueva(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	eng, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	if err := eng.SaveObservation("", "prueba/escalon",
		"Una nota guardada antes de que la base quedara fuera de alcance de este binario.", nil); err != nil {
		t.Fatalf("sembrar una observación: %v", err)
	}
	eng.Close()

	db := abrirCruda(t, dir)
	if _, err := db.Exec(`PRAGMA user_version = 999`); err != nil {
		t.Fatalf("adelantar user_version: %v", err)
	}
	db.Close()
	return dir
}

// R1 — ABRIR EN SÓLO LECTURA NO MIGRA NADA.
//
// Es la razón de ser del camino: la base está fuera del alcance de este binario. Si migrara,
// estaría haciendo justo lo que no sabe hacer, y sobre datos ajenos.
func TestR1AbrirEnSoloLecturaNoMigra(t *testing.T) {
	dir := baseMasNueva(t)

	eng, err := NewDbEngineSoloLectura(dir)
	if err != nil {
		t.Fatalf("NewDbEngineSoloLectura: %v", err)
	}
	defer eng.Close()

	var uv int
	if err := abrirCruda(t, dir).QueryRow(`PRAGMA user_version`).Scan(&uv); err != nil {
		t.Fatalf("leer user_version: %v", err)
	}
	if uv != 999 {
		t.Errorf("user_version = %d: abrir en sólo lectura tocó el esquema (esperaba 999)", uv)
	}
	if !eng.EsSoloLectura() {
		t.Error("el engine no se declara de sólo lectura")
	}
}

// R2 — LA GARANTÍA LA DA SQLITE, NO NUESTRA DISCIPLINA.
//
// La escritura se intenta por un camino NORMAL del engine, no por una consulta armada en el test:
// lo que se prueba es que un camino cualquiera del producto rebota, no que SQLite sepa rechazar un
// INSERT. Si esto dependiera de una lista de funciones que evitamos llamar, la lista se
// desactualizaría el día que alguien agregue una escritura nueva y nadie se enteraría.
func TestR2SqliteRechazaLaEscrituraEnSoloLectura(t *testing.T) {
	dir := baseMasNueva(t)
	eng, err := NewDbEngineSoloLectura(dir)
	if err != nil {
		t.Fatalf("NewDbEngineSoloLectura: %v", err)
	}
	defer eng.Close()

	err = eng.SetMeta("prueba_de_escritura", "no debería quedar")
	if err == nil {
		t.Fatal("una escritura por el camino normal del engine PASÓ en modo sólo lectura")
	}
	// El texto viene del motor, no nuestro: es la evidencia de que quien frenó fue SQLite.
	if !strings.Contains(strings.ToLower(err.Error()), "readonly") &&
		!strings.Contains(strings.ToLower(err.Error()), "read-only") &&
		!strings.Contains(strings.ToLower(err.Error()), "query_only") {
		t.Errorf("la escritura falló, pero no por sólo-lectura del motor: %v", err)
	}
}

// R3 — LA LECTURA SIGUE FUNCIONANDO, INCLUIDO EL RECALL.
//
// `musubi_recall` escribe —refuerza el acceso de lo que devolvió— y su error era FATAL para la
// consulta. Sin la omisión de ese refuerzo, la tool de lectura principal quedaría inservible justo
// en el modo que existe para poder leer, y el escalón entero sería teatro.
func TestR3ElRecallFuncionaConLaBaseEnSoloLectura(t *testing.T) {
	dir := baseMasNueva(t)
	eng, err := NewDbEngineSoloLectura(dir)
	if err != nil {
		t.Fatalf("NewDbEngineSoloLectura: %v", err)
	}
	defer eng.Close()

	res, err := eng.Recall(context.Background(), "nota guardada antes", RecallOptions{})
	if err != nil {
		t.Fatalf("Recall falló con la base en sólo lectura: %v", err)
	}
	if len(res.Items) == 0 {
		t.Fatal("Recall no devolvió nada: la memoria sembrada no se pudo leer")
	}
	encontrada := false
	for _, it := range res.Items {
		if it.TopicKey == "prueba/escalon" {
			encontrada = true
		}
	}
	if !encontrada {
		t.Errorf("Recall devolvió %d ítems pero ninguno es la nota sembrada", len(res.Items))
	}
}

// R4 — UN ENGINE NORMAL NO SE DECLARA DE SÓLO LECTURA, Y SIGUE ESCRIBIENDO.
//
// Sin esto, «abrir todo en sólo lectura» pasaría R1, R2 y R3 con las mejores notas y dejaría al
// producto sin memoria.
func TestR4ElEngineNormalNoEsDeSoloLectura(t *testing.T) {
	dir := t.TempDir()
	eng, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer eng.Close()

	if eng.EsSoloLectura() {
		t.Error("un engine normal se declaró de sólo lectura")
	}
	if err := eng.SetMeta("prueba_de_escritura", "sí"); err != nil {
		t.Fatalf("un engine normal no pudo escribir: %v", err)
	}
	// Y el refuerzo de acceso, que en sólo lectura se omite, acá tiene que ocurrir: si se omitiera
	// siempre, la omisión dejaría de ser una excepción del escalón y sería una pérdida silenciosa.
	if err := eng.SaveObservation("", "t/r4", "una nota cualquiera para poder recordarla", nil); err != nil {
		t.Fatalf("guardar: %v", err)
	}
	if _, err := eng.Recall(context.Background(), "una nota cualquiera", RecallOptions{}); err != nil {
		t.Fatalf("recall: %v", err)
	}
	var accesos int
	if err := eng.db.QueryRow(`SELECT COALESCE(MAX(access_count), 0) FROM observations`).Scan(&accesos); err != nil {
		t.Fatalf("leer access_count: %v", err)
	}
	if accesos == 0 {
		t.Error("el refuerzo de acceso no ocurrió en un engine normal: la omisión del escalón se está aplicando siempre")
	}
}

// TestSoloLecturaAplicaLosDivisoresDeSuBase fija el hermano que faltaba: el engine de sólo lectura
// tiene que estimar tokens con la calibración guardada en SU base, igual que NewDbEngine.
//
// Sin esto, un engine degradado usaba los divisores de fábrica (4.0/3.4/2.6) aunque la base
// tuviera otros — y el modo degradado es justo donde más duele, porque se llega ahí cuando el
// esquema es más nuevo que el binario y el operador ya está diagnosticando a ciegas.
func TestSoloLecturaAplicaLosDivisoresDeSuBase(t *testing.T) {
	defer ResetDivisors()
	dir := t.TempDir()

	// Base con una calibración propia, bien distinta del default.
	eng, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	const calProse, calCode, calJSON = 8.0, 7.0, 6.0
	if err := eng.SaveDivisors(calProse, calCode, calJSON); err != nil {
		t.Fatalf("SaveDivisors: %v", err)
	}
	ruta := dir // NewDbEngineSoloLectura recibe el projectPath, no la ruta del archivo
	eng.Close()

	// Se ensucian los divisores del proceso, para que abrir en sólo lectura TENGA que corregirlos.
	ResetDivisors()
	p0, _, _ := CurrentDivisors()
	if p0 == calProse {
		t.Fatalf("el default coincide con la calibración (%v): la prueba no podría distinguir", p0)
	}

	ro, err := NewDbEngineSoloLectura(ruta)
	if err != nil {
		t.Fatalf("NewDbEngineSoloLectura: %v", err)
	}
	defer ro.Close()

	prose, code, jsn := CurrentDivisors()
	if prose != calProse || code != calCode || jsn != calJSON {
		t.Errorf("el engine de sólo lectura no aplicó la calibración de su base: %v/%v/%v, esperaba %v/%v/%v",
			prose, code, jsn, calProse, calCode, calJSON)
	}
}
