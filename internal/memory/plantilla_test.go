package memory

// Pruebas de A45: la plantilla de esquema que hace que la suite entre bajo `-race`.

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
)

// LA PLANTILLA TIENE QUE ESTAR AL DÍA, o no ahorra nada y nadie se entera.
//
// Éste es el modo de fallo que hace peligroso todo el arreglo: si la copia llega con un
// `user_version` viejo —o vacío, que es lo que pasa si nadie hace el checkpoint del WAL—, cada
// prueba vuelve a migrar de cero. La suite sigue en VERDE y sólo un poco más lenta, así que A45
// queda abierto y nadie vuelve a mirarlo.
//
// Sabotaje que la hace fallar: registrar la plantilla SIN cerrar el engine antes. Con la base
// abierta el archivo principal son 4 KB y el esquema entero vive en el `-wal`; la copia llega con
// `user_version` en 0. Ejecutado: la prueba dice exactamente eso.
//
// NO hay un sabotaje sobre un `wal_checkpoint` explícito, porque no hay tal llamada: se probó
// quitarla y la suite quedó verde —SQLite hace checkpoint solo al cerrar la última conexión— así
// que la llamada se sacó en vez de dejarla con un comentario que decía que era imprescindible.
// Lo que se custodia acá es la PROPIEDAD (el archivo copiado está al día), no el mecanismo.
func TestLaPlantillaLlegaConElEsquemaAlDia(t *testing.T) {
	dir := t.TempDir()
	if err := SembrarPlantillaDePruebas(dir); err != nil {
		t.Fatalf("sembrar: %v", err)
	}
	ruta := filepath.Join(dir, config.DirName, config.DBFile)

	info, err := os.Stat(ruta)
	if err != nil {
		t.Fatalf("la plantilla no dejó archivo: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("la plantilla copió un archivo VACÍO: sin checkpoint el esquema se quedó en el -wal")
	}

	// Se abre A MANO, sin NewDbEngine, porque NewDbEngine migraría y taparía justo lo que se mide.
	db, err := sql.Open("sqlite", ruta)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatalf("no se pudo leer user_version de la copia: %v", err)
	}
	if quiere := latestSchemaVersion(); v != quiere {
		t.Fatalf("la plantilla llegó en el esquema %d y la última migración es la %d: cada prueba "+
			"volvería a migrar de cero, la suite quedaría igual de lenta y en VERDE", v, quiere)
	}
	// Y trae el esquema de verdad, no sólo el número: un user_version escrito sin las tablas
	// haría que todo falle raro en vez de fallar acá.
	var tablas int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table'`).Scan(&tablas); err != nil {
		t.Fatal(err)
	}
	if tablas < 10 {
		t.Errorf("la copia trae %d tablas: el user_version está al día y el esquema no", tablas)
	}
}

// ABRIR SOBRE LA PLANTILLA NO APLICA UNA SOLA MIGRACIÓN. Es la propiedad de la que sale todo el
// ahorro, y se comprueba mirando el contador de migraciones aplicadas y no el reloj: medir tiempo
// en una prueba la vuelve intermitente en una máquina cargada.
//
// Sabotaje que la hace fallar: sembrar con una base sin migrar (o borrar el archivo sembrado
// antes de abrir).
func TestAbrirSobreLaPlantillaNoMigraNada(t *testing.T) {
	dir := t.TempDir()
	if err := SembrarPlantillaDePruebas(dir); err != nil {
		t.Fatalf("sembrar: %v", err)
	}
	ruta := filepath.Join(dir, config.DirName, config.DBFile)

	antes := versionDeArchivo(t, ruta)
	eng, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine sobre la plantilla: %v", err)
	}
	defer eng.Close()

	// El engine abre y funciona: la copia no es sólo un archivo con el número correcto.
	if v, err := eng.schemaVersion(); err != nil {
		t.Fatalf("el engine no pudo leer su esquema: %v", err)
	} else if v != latestSchemaVersion() {
		t.Errorf("el engine ve el esquema %d y la última es %d", v, latestSchemaVersion())
	}
	if antes != latestSchemaVersion() {
		t.Errorf("el archivo sembrado estaba en %d antes de abrir: la plantilla no venía al día "+
			"y NewDbEngine migró, que es justo lo que este arreglo evita", antes)
	}
}

// versionDeArchivo lee el user_version sin pasar por NewDbEngine.
func versionDeArchivo(t *testing.T, ruta string) int {
	t.Helper()
	db, err := sql.Open("sqlite", ruta)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// UNA BASE SEMBRADA SE COMPORTA IGUAL QUE UNA MIGRADA DE CERO. Si no, el arreglo cambia lo que
// las pruebas prueban — y una suite que corre rápido sobre un esquema distinto del de producción
// es peor que una suite lenta.
//
// Se comparan los NOMBRES de tabla y el DDL de cada una: un CREATE TABLE que difiera en una
// columna o en una PK haría que las pruebas validen un esquema que el servidor no tiene.
//
// Sabotaje que la hace fallar: hacer que la plantilla se construya con una migración de menos.
func TestLaBaseSembradaEsIdenticaALaMigradaDeCero(t *testing.T) {
	sembrada := t.TempDir()
	if err := SembrarPlantillaDePruebas(sembrada); err != nil {
		t.Fatalf("sembrar: %v", err)
	}
	engSembrada, err := NewDbEngine(sembrada)
	if err != nil {
		t.Fatal(err)
	}
	defer engSembrada.Close()

	// La de control migra de cero, que es lo que hacían las 362 pruebas antes de A45.
	engDeCero, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engDeCero.Close()

	a := esquemaDe(t, engSembrada)
	b := esquemaDe(t, engDeCero)
	if len(a) == 0 {
		t.Fatal("la base sembrada no tiene objetos: la comparación no probaría nada")
	}
	if len(a) != len(b) {
		t.Fatalf("la sembrada tiene %d objetos y la migrada de cero %d", len(a), len(b))
	}
	for nombre, ddl := range b {
		otro, hay := a[nombre]
		if !hay {
			t.Errorf("la base sembrada NO tiene %q, que sí crea la migración de cero", nombre)
			continue
		}
		if otro != ddl {
			t.Errorf("%q difiere entre la sembrada y la migrada de cero:\n  sembrada: %s\n  de cero:  %s",
				nombre, otro, ddl)
		}
	}
}

// esquemaDe devuelve el DDL de cada objeto, por nombre.
func esquemaDe(t *testing.T, e *DbEngine) map[string]string {
	t.Helper()
	filas, err := e.db.Query(
		`SELECT name, COALESCE(sql, '') FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer filas.Close()
	out := map[string]string{}
	for filas.Next() {
		var n, ddl string
		if err := filas.Scan(&n, &ddl); err != nil {
			t.Fatal(err)
		}
		out[n] = ddl
	}
	return out
}

// EL ARGUMENTO ENTERO DE A45 DESCANSA EN QUE PRODUCCIÓN TOME EL MISMO CAMINO.
//
// La plantilla es segura porque `NewDbEngine` no sabe que existe: la prueba siembra el archivo y
// la función corre su `runMigrations` de siempre, que no encuentra nada que hacer. El día que
// alguien llame a `SembrarPlantillaDePruebas` desde un camino de producción —para «acelerar el
// arranque», que suena razonable— el servidor empezaría a abrir bases que nunca migró, y esta
// optimización de PRUEBAS se volvería una decisión de PRODUCCIÓN que nadie tomó.
//
// Se comprueba barriendo el árbol y no confiando en que se acuerden.
//
// Sabotaje que la hace fallar: llamar a SembrarPlantillaDePruebas desde cualquier archivo que no
// sea de prueba.
func TestLaPlantillaDePruebasNoTieneLlamadorDeProduccion(t *testing.T) {
	raiz := filepath.Join("..", "..")
	vistos := 0
	err := filepath.WalkDir(raiz, func(ruta string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if n := d.Name(); n == ".git" || n == "node_modules" || n == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(ruta, ".go") || strings.HasSuffix(ruta, "_test.go") {
			return nil
		}
		crudo, err := os.ReadFile(ruta)
		if err != nil {
			return err
		}
		vistos++
		texto := string(crudo)
		if !strings.Contains(texto, "SembrarPlantillaDePruebas") {
			return nil
		}
		// El propio archivo que la define y el envoltorio `memtest` son los dos únicos lugares
		// legítimos fuera de las pruebas. Se comparan por nombre de archivo para que mover la
		// función a otro lado rompa la guarda en vez de aflojarla en silencio.
		base := filepath.Base(ruta)
		if base == "plantilla.go" || base == "memtest.go" {
			return nil
		}
		// Una MENCIÓN en un comentario no es una llamada; lo que se busca es la invocación.
		if strings.Contains(texto, "SembrarPlantillaDePruebas(") {
			t.Errorf("%s llama a SembrarPlantillaDePruebas y no es un archivo de prueba.\n"+
				"  La plantilla es segura PORQUE producción no la conoce: NewDbEngine corre su\n"+
				"  runMigrations de siempre y no encuentra nada que hacer. Un llamador de\n"+
				"  producción convertiría una optimización de PRUEBAS en una decisión de\n"+
				"  PRODUCCIÓN que nadie tomó: el servidor abriría bases que nunca migró.", ruta)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Si el barrido deja de encontrar archivos, pasaría vacío y en verde.
	if vistos < 50 {
		t.Fatalf("el barrido sólo miró %d archivos .go: no está mirando donde cree", vistos)
	}
}

// LAS MIGRACIONES TIENEN QUE ESTAR EN ORDEN ASCENDENTE, Y NO HABÍA NADA QUE LO EXIGIERA.
//
// `latestSchemaVersion()` devuelve la versión del ÚLTIMO elemento del slice, no la mayor. Así que
// insertar una migración nueva en el medio —cosa fácil de hacer buscando «la de al lado» para
// copiar su forma— deja al binario creyendo que la última versión es una anterior:
//
//   - `applyMigrations` corre de todas formas todo lo pendiente, así que el esquema queda BIEN;
//   - pero la guarda de «esta base viene del futuro» compara contra un número más chico, y una
//     base migrada por un binario nuevo se vería como corrupta desde uno viejo… y al revés.
//
// Pasó mientras se escribía la migración 40: se insertó antes de la 39 y `latestSchemaVersion`
// pasó a decir 39 con la 40 ya escrita. Lo atrapó de rebote la guarda de la plantilla de A45, que
// existe para otra cosa. Esta prueba lo atrapa de frente.
//
// Sabotaje que la hace fallar: mover cualquier migración de lugar en el slice.
func TestLasMigracionesEstanEnOrdenAscendenteYSinHuecos(t *testing.T) {
	ms := schemaMigrations()
	if len(ms) < 30 {
		t.Fatalf("sólo hay %d migraciones: la prueba no está mirando lo que cree", len(ms))
	}
	for i := 1; i < len(ms); i++ {
		if ms[i].version <= ms[i-1].version {
			t.Errorf("la migración %d (%q) viene después de la %d (%q): el slice no está en orden "+
				"ascendente, y latestSchemaVersion() devuelve la versión del ÚLTIMO elemento — no "+
				"la mayor. Con esto, la guarda de «base del futuro» compara contra el número "+
				"equivocado.", ms[i].version, ms[i].name, ms[i-1].version, ms[i-1].name)
		}
	}
	// Y el último es el mayor, que es lo que latestSchemaVersion() afirma sin comprobarlo.
	mayor := 0
	for _, m := range ms {
		if m.version > mayor {
			mayor = m.version
		}
	}
	if got := latestSchemaVersion(); got != mayor {
		t.Errorf("latestSchemaVersion() = %d y la migración más alta es la %d", got, mayor)
	}
	// Sin nombres repetidos: el nombre es lo que se lee en un diagnóstico, y dos iguales mandan a
	// mirar la migración equivocada.
	vistos := map[string]int{}
	for _, m := range ms {
		if otra, ya := vistos[m.name]; ya {
			t.Errorf("las migraciones %d y %d comparten el nombre %q", otra, m.version, m.name)
		}
		vistos[m.name] = m.version
	}
}
