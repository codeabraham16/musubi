package memory

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// EL RUNNER DEPENDE DE DOS PROPIEDADES DEL SLICE QUE NADIE VERIFICABA.
//
// `applyMigrations` es, textualmente:
//
//	for _, m := range migs {
//	    if m.version <= current { continue }
//	    ... aplica ...
//	    current = m.version
//	}
//
// O sea que asume que las versiones vienen ESTRICTAMENTE ASCENDENTES. Si dos migraciones comparten
// número, la segunda entra al `continue` y NO SE APLICA: sin error, sin warning, y con
// `user_version` diciendo que todo se aplicó. Si una viene fuera de orden, se saltea igual.
//
// POR QUÉ ESTO NO ES TEÓRICO, medido el 2026-09-11: los PR #440 y #442 tenían los dos una
// migración 54 —`quien_esta_latiendo_sobre_esta_fila` y `indices_observation_relations`—, escritas
// por dos sesiones que no se veían. Ninguna de las dos está mal. El daño aparece al juntarlas, y
// es peor que un conflicto normal por tres razones:
//
//  1. Git puede NO marcar conflicto: las dos entradas se agregan al final del slice.
//  2. El CI no lo puede ver: mide cada rama contra main, nunca contra la otra rama abierta.
//  3. No deja rojo. Deja una base INCOMPLETA EN VERDE, y el código que espera la tabla que falta
//     revienta después, lejos de la causa y en otra máquina.
//
// Esta guarda no resuelve el choque: lo hace RUIDOSO.

// TestLasVersionesDeMigracionSonUnicasYAscendentes es el veredicto sobre EL REPO, así que corre
// sobre el slice REAL. Una tabla sintética probaría que sé comparar enteros, no que el repo esté
// sano; los casos raros van en el test de abajo, que sí es sintético.
//
// SABOTAJE QUE LA HACE FALLAR: duplicar el número de cualquier migración en `schemaMigrations()`.
// Y EL CONTROL INVERSO, que importa igual: agregar una migración legítima con el número siguiente
// tiene que dejarla VERDE — si no, la guarda estaría prohibiendo agregar migraciones y nadie se
// enteraría hasta que molestara. Los dos están corridos y están abajo, en TestElControlInverso.
func TestLasVersionesDeMigracionSonUnicasYAscendentes(t *testing.T) {
	migs := schemaMigrations()

	// CONTROL DE «MIRÓ ALGO»: el modo de falla más común de una guarda no es una aserción
	// equivocada, es un recorrido que no llega. Cero migraciones revisadas no es «todo limpio».
	if len(migs) < 50 {
		t.Fatalf("sólo se revisaron %d migraciones y el piso es 50: `schemaMigrations()` devolvió otra cosa y esta guarda dejó de mirar", len(migs))
	}

	vistas := make(map[int]string, len(migs))
	nombres := make(map[string]int, len(migs))
	prev := 0
	for i, m := range migs {
		if otra, repetida := vistas[m.version]; repetida {
			t.Errorf("DOS migraciones con la versión %d: %q y %q.\n"+
				"  El runner aplica la primera, pone current=%d, y la segunda entra al `continue`: NO SE APLICA, sin error.\n"+
				"  La base queda incompleta con `user_version` diciendo que todo se aplicó.\n"+
				"  Si esto salió de juntar dos ramas, renumerá una — no borres ninguna.",
				m.version, otra, m.name, m.version)
		}
		vistas[m.version] = m.name

		if m.version <= prev {
			t.Errorf("la migración %d (%q, índice %d) no es mayor que la anterior (%d).\n"+
				"  `applyMigrations` recorre el slice EN ORDEN y saltea todo lo que no supere a `current`:\n"+
				"  una migración fuera de orden no se aplica nunca.",
				m.version, m.name, i, prev)
		}
		prev = m.version

		if m.name == "" {
			t.Errorf("la migración %d no tiene nombre: es lo único que se imprime cuando falla", m.version)
		}
		nombres[m.name]++
	}
	for n, c := range nombres {
		if c > 1 {
			t.Errorf("el nombre de migración %q aparece %d veces: dos ramas copiaron la misma plantilla, y el mensaje de error no va a poder decir cuál falló", n, c)
		}
	}
}

// TestElRunnerSALTEAEnSilencioUnaVersionRepetida es la EVIDENCIA de por qué la guarda de arriba
// tiene que existir. No prueba el repo: prueba el daño, y por eso sí va con migraciones
// sintéticas — el propio doc de `applyMigrations` dice que separarlo de `schemaMigrations()`
// existe justamente para poder testearlo así.
//
// Si algún día el runner aprende a quejarse de una versión repetida, este test se pone rojo y esa
// es la señal de que la guarda de arriba puede aflojarse. Mientras esté verde, no.
func TestElRunnerSALTEAEnSilencioUnaVersionRepetida(t *testing.T) {
	db := baseVacia(t)

	corrio := map[string]bool{}
	marcar := func(nombre string) func(execQuerier) error {
		return func(x execQuerier) error {
			corrio[nombre] = true
			_, err := x.Exec(fmt.Sprintf(`CREATE TABLE t_%s (id INTEGER)`, nombre))
			return err
		}
	}
	migs := []migration{
		{version: 1, name: "primera", up: marcar("primera")},
		{version: 2, name: "la_de_una_rama", up: marcar("la_de_una_rama")},
		{version: 2, name: "la_de_la_otra_rama", up: marcar("la_de_la_otra_rama")},
	}

	if err := applyMigrations(db, migs); err != nil {
		t.Fatalf("applyMigrations devolvió error: %v\n"+
			"  Si el runner EMPEZÓ a quejarse de una versión repetida, es una buena noticia: actualizá este test y revisá si la guarda de unicidad puede aflojarse.", err)
	}

	if !corrio["la_de_una_rama"] {
		t.Fatal("no corrió ni la primera de las dos: el montaje del test no mide lo que dice")
	}
	if corrio["la_de_la_otra_rama"] {
		t.Fatal("el runner SÍ aplicó la segunda migración con versión repetida: ya no se saltea, y esta prueba quedó vieja")
	}

	// Y el remate, que es lo que lo vuelve peligroso: la base dice que está al día.
	var uv int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&uv); err != nil {
		t.Fatal(err)
	}
	if uv != 2 {
		t.Fatalf("user_version=%d, esperaba 2", uv)
	}
	var existe int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='t_la_de_la_otra_rama'`).Scan(&existe); err != nil {
		t.Fatal(err)
	}
	if existe != 0 {
		t.Fatal("la tabla de la segunda migración existe: el montaje no reprodujo el salteo")
	}
	t.Logf("MEDIDO: user_version=%d (dice «al día») y la tabla de la migración salteada NO existe. Ni error ni warning.", uv)
}

// TestElControlInverso es la mitad que casi nunca se corre: que la guarda no esté prohibiendo lo
// legítimo. Una guarda de unicidad mal escrita —por ejemplo, una que exija contigüidad estricta
// donde no hace falta, o que compare mal el último elemento— dejaría de aceptar la próxima
// migración que alguien agregue, y eso se descubre recién cuando molesta.
func TestElControlInverso(t *testing.T) {
	migs := schemaMigrations()
	ultima := migs[len(migs)-1].version

	// El slice real MÁS una migración nueva y legítima, con el número siguiente.
	conLaNueva := append(append([]migration{}, migs...), migration{
		version: ultima + 1,
		name:    "la_proxima_que_alguien_agregue",
		up:      func(execQuerier) error { return nil },
	})

	if motivo := porQueNoEsUnaSecuenciaAplicable(conLaNueva); motivo != "" {
		t.Fatalf("agregar una migración legítima con el número siguiente (%d) rompe la guarda: %s\n"+
			"  La guarda estaría prohibiendo lo que tiene que permitir.", ultima+1, motivo)
	}
	// Y el mismo slice con el número REPETIDO tiene que ser rechazado.
	conRepetida := append(append([]migration{}, migs...), migration{
		version: ultima,
		name:    "la_de_la_otra_rama",
		up:      func(execQuerier) error { return nil },
	})
	if porQueNoEsUnaSecuenciaAplicable(conRepetida) == "" {
		t.Fatalf("repetir la versión %d no se detecta: la guarda es decorativa", ultima)
	}
}

// porQueNoEsUnaSecuenciaAplicable devuelve "" si el slice cumple lo que `applyMigrations`
// asume, y si no, POR QUÉ no. Existe para que TestElControlInverso pueda preguntar por un
// slice ARMADO sin duplicar el criterio: un criterio escrito dos veces envejece en uno de los dos.
func porQueNoEsUnaSecuenciaAplicable(migs []migration) string {
	prev := 0
	vistas := map[int]bool{}
	for _, m := range migs {
		if vistas[m.version] {
			return fmt.Sprintf("la versión %d (%q) está repetida", m.version, m.name)
		}
		if m.version <= prev {
			return fmt.Sprintf("la versión %d (%q) no supera a la anterior (%d)", m.version, m.name, prev)
		}
		vistas[m.version] = true
		prev = m.version
	}
	return ""
}

// baseVacia abre una SQLite de verdad, sin esquema: el runner se ejercita contra el motor real,
// no contra un doble.
func baseVacia(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatalf("abrir base de prueba: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
