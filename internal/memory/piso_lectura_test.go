package memory

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"musubi/internal/config"

	_ "modernc.org/sqlite"
)

// abrirCruda abre el archivo de una base ya migrada, sin pasar por NewDbEngine: hace falta para
// ejercitar el runner contra una base que este binario NO habría podido migrar.
func abrirCruda(t *testing.T, dir string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, config.DirName, config.DBFile))
	if err != nil {
		t.Fatalf("abrir la base cruda: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// P1 — EL PISO ES UNA PROPIEDAD DE LA CLASIFICACIÓN, NO UN NÚMERO.
//
// No se compara pisoDeLectura contra una segunda copia de su propia expresión —eso sería medir el
// proxy—, sino contra la propiedad que el piso PROMETE: de él para arriba no queda ninguna
// migración que le cambie el resultado a un lector viejo, y él mismo es una que sí.
func TestP1PorEncimaDelPisoNoQuedaNingunaQueRompaLectura(t *testing.T) {
	migs := schemaMigrations()
	piso := pisoDeLectura(migs)

	for _, m := range migs {
		if m.version > piso && !m.readCompatible {
			t.Errorf("la migración v%d (%s) NO es readCompatible y está POR ENCIMA del piso v%d: el piso miente",
				m.version, m.name, piso)
		}
	}
	if piso == 0 {
		t.Fatal("el piso dio 0: o no hay ninguna migración clasificada como no-compatible, o la clasificación se perdió")
	}
	// Y el piso tiene que ser exactamente una migración que rompe lectura; si fuera una
	// compatible, estaría más alto de lo necesario y negaría binarios que sí podían leer.
	enElPiso := false
	for _, m := range migs {
		if m.version == piso {
			enElPiso = !m.readCompatible
		}
	}
	if !enElPiso {
		t.Errorf("el piso v%d no corresponde a una migración que rompa lectura", piso)
	}
	t.Logf("piso de lectura derivado: v%d (esquema del binario: v%d)", piso, latestSchemaVersion())
}

// P2 — EL CERO DE GO ES «NO COMPATIBLE», ASÍ QUE UNA MIGRACIÓN SIN CLASIFICAR SUBE EL PISO.
//
// Es el fail-safe de la pieza y no se puede leer del código de arriba: readCompatible=false y «me
// olvidé de ponerlo» son el MISMO valor, y tienen que serlo. Si el default fuera compatible, una
// migración nueva que nadie miró dejaría entrar a leer a binarios que van a devolver otra cosa.
func TestP2UnaMigracionSinClasificarSubeElPiso(t *testing.T) {
	nada := func(execQuerier) error { return nil }
	migs := []migration{
		{version: 1, name: "base", readCompatible: true, up: nada},
		{version: 2, name: "aditiva", readCompatible: true, up: nada},
		// Sin el campo: el cero de Go. Tiene que contar como «rompe lectura».
		{version: 3, name: "nadie_la_clasifico", up: nada},
		{version: 4, name: "otra_aditiva", readCompatible: true, up: nada},
	}
	if got := pisoDeLectura(migs); got != 3 {
		t.Errorf("pisoDeLectura = %d, esperaba 3: una migración sin clasificar tiene que contar como no-compatible", got)
	}
}

// P3 — MIGRAR DEJA EL PISO GRABADO EN LA BASE.
//
// Es el único dato de esta pieza que tiene que viajar por la BASE y no por el código: el binario
// viejo no puede derivarlo porque no conoce las migraciones futuras. Si no queda grabado, la
// pieza entera no hace nada y no se nota.
func TestP3MigrarDejaElPisoGrabado(t *testing.T) {
	dir := t.TempDir()
	eng, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	eng.Close()

	db := abrirCruda(t, dir)
	var piso, porEsquema int
	if err := db.QueryRow(`SELECT read_floor, set_by_schema FROM schema_floor WHERE id = 1`).Scan(&piso, &porEsquema); err != nil {
		t.Fatalf("leer schema_floor: %v", err)
	}
	if piso != PisoDeLectura() {
		t.Errorf("la base grabó read_floor=%d y el binario declara %d", piso, PisoDeLectura())
	}
	if porEsquema != latestSchemaVersion() {
		t.Errorf("set_by_schema=%d, esperaba el esquema que migró (%d)", porEsquema, latestSchemaVersion())
	}
}

// P4 — EL PISO SE RE-GRABA AL ABRIR, NO SÓLO CUANDO HAY MIGRACIONES QUE APLICAR.
//
// Si sólo se escribiera al aplicar una migración, TODA base que hoy existe —ya migrada por un
// binario anterior a esta pieza— se quedaría sin piso para siempre: nadie volvería a tocarla. El
// caso se simula borrando la fila y reabriendo.
func TestP4ElPisoSeRegrabaEnUnaBaseQueYaEstabaMigrada(t *testing.T) {
	dir := t.TempDir()
	eng, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	eng.Close()

	db := abrirCruda(t, dir)
	if _, err := db.Exec(`DELETE FROM schema_floor`); err != nil {
		t.Fatalf("borrar el piso: %v", err)
	}
	var quedan int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_floor`).Scan(&quedan); err != nil || quedan != 0 {
		t.Fatalf("el escenario no se armó: quedan %d filas (err=%v)", quedan, err)
	}
	db.Close()

	eng2, err := NewDbEngine(dir) // no hay migraciones pendientes: ya está en la última
	if err != nil {
		t.Fatalf("reabrir: %v", err)
	}
	eng2.Close()

	// Este test afirma que la fila VUELVE, y no cuánto vale: que el valor sea el correcto es lo
	// que mide P3. Separarlos deja que un sabotaje al valor y uno a la re-escritura se distingan;
	// con las dos afirmaciones acá, cualquiera de los dos pondría rojos a los dos tests y ninguno
	// diría cuál se rompió.
	db2 := abrirCruda(t, dir)
	var piso int
	if err := db2.QueryRow(`SELECT read_floor FROM schema_floor WHERE id = 1`).Scan(&piso); err != nil {
		t.Fatalf("el piso no volvió a grabarse en una base sin migraciones pendientes: %v", err)
	}
	t.Logf("el piso volvió a grabarse: %d", piso)
}

// P5 — UNA BASE MÁS NUEVA SE CONTESTA DISTINTO SEGÚN SI ESTE BINARIO LLEGA A SU PISO.
//
// Es la razón de ser de la pieza. Antes las dos situaciones daban el MISMO error y el caller no
// tenía con qué distinguirlas, así que la única respuesta posible era negarse. Se verifica además
// que el error legible SIGUE siendo ErrSchemaTooNew: si dejara de serlo, todo caller que ya
// preguntaba por ese error pasaría a ver un fallo desconocido, que es peor que antes.
func TestP5LaBaseMasNuevaDistingueLegibleDeIlegible(t *testing.T) {
	dir := t.TempDir()
	eng, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	eng.Close()

	reales := schemaMigrations()

	// EL PISO SE FIJA ACÁ A MANO, y no se toma del que grabó el arranque. Lo que este test mide
	// es la REGLA DE DECISIÓN de la guarda —dado un piso en la base, ¿legible o no?—, y tomarlo
	// de quien lo escribe lo ataría al acierto de esa escritura: un piso mal grabado pondría rojo
	// a este test además del que sí vigila la escritura, y ninguno de los dos diría cuál falló.
	const piso = 43
	if _, err := abrirCruda(t, dir).Exec(`UPDATE schema_floor SET read_floor = ? WHERE id = 1`, piso); err != nil {
		t.Fatalf("fijar el piso del escenario: %v", err)
	}

	// recortar devuelve las migraciones hasta `hasta`, que es lo que conocería un binario viejo.
	recortar := func(hasta int) []migration {
		var out []migration
		for _, m := range reales {
			if m.version <= hasta {
				out = append(out, m)
			}
		}
		return out
	}

	t.Run("llega al piso: legible", func(t *testing.T) {
		db := abrirCruda(t, dir)
		err := applyMigrations(db, recortar(piso))
		if err == nil {
			t.Fatal("aceptó migrar una base más nueva")
		}
		if !errors.Is(err, ErrSchemaTooNew) {
			t.Errorf("dejó de ser ErrSchemaTooNew: los callers que ya preguntaban por él quedan ciegos\n  %v", err)
		}
		if !errors.Is(err, ErrEsquemaLegible) {
			t.Errorf("un binario que llega al piso v%d no fue declarado legible:\n  %v", piso, err)
		}
	})

	t.Run("no llega al piso: ilegible", func(t *testing.T) {
		db := abrirCruda(t, dir)
		err := applyMigrations(db, recortar(piso-1))
		if err == nil {
			t.Fatal("aceptó migrar una base más nueva")
		}
		if !errors.Is(err, ErrSchemaTooNew) {
			t.Errorf("esperaba ErrSchemaTooNew: %v", err)
		}
		if errors.Is(err, ErrEsquemaLegible) {
			t.Errorf("un binario por DEBAJO del piso v%d fue declarado legible:\n  %v", piso, err)
		}
	})

	t.Run("sin piso grabado: ilegible aunque alcance", func(t *testing.T) {
		db := abrirCruda(t, dir)
		if _, err := db.Exec(`DELETE FROM schema_floor`); err != nil {
			t.Fatalf("borrar el piso: %v", err)
		}
		err := applyMigrations(db, recortar(piso))
		if err == nil {
			t.Fatal("aceptó migrar una base más nueva")
		}
		// Sin evidencia no hay permiso: una base migrada por un binario anterior a esta pieza no
		// dice nada sobre qué puede leer quien la abre.
		if errors.Is(err, ErrEsquemaLegible) {
			t.Errorf("declaró legible una base SIN piso grabado:\n  %v", err)
		}
	})
}
