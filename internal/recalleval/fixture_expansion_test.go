package recalleval

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// EL ETIQUETADO POR EXPANSIÓN (v57). Ver EtiquetadoPorExpansion en fixture_real.go: la etiqueta
// sale de `expand_count`, o sea de un agente que eligió pedir el contenido entero teniendo el gist
// delante. Es la única etiqueta del banco que NO deriva de la similitud, y por eso es la única que
// puede juzgar a MMR sin el sesgo que infla su costo.

// marcarExpandidas sube expand_count de los ids indicados, simulando el uso real.
func marcarExpandidas(t *testing.T, ruta string, ids ...string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+ruta)
	if err != nil {
		t.Fatalf("abrir para marcar: %v", err)
	}
	defer db.Close()
	for _, id := range ids {
		r, err := db.Exec(`UPDATE observations SET expand_count = expand_count + 1 WHERE id = ?`, id)
		if err != nil {
			t.Fatalf("marcar %q: %v", id, err)
		}
		if n, _ := r.RowsAffected(); n != 1 {
			t.Fatalf("marcar %q afectó %d filas: el id no existe y el test estaría midiendo nada", id, n)
		}
	}
}

func consultaPorID(fx *Fixture, id string) *Query {
	for i := range fx.Queries {
		if fx.Queries[i].ID == id {
			return &fx.Queries[i]
		}
	}
	return nil
}

// X1 — RELEVANTE ES LO QUE ALGUIEN ELIGIÓ, NO TODO EL TÓPICO.
//
// Es la guarda central del etiquetado nuevo. El tópico sigue decidiendo cuál es la CONSULTA —para
// que las dos corridas pregunten lo mismo y el delta sea atribuible a la etiqueta— pero los
// relevantes se recortan a los que de verdad se expandieron. Si entraran los cuatro, esto sería el
// etiquetado por tópico con otro nombre: un informe impecable midiendo lo de siempre.
func TestX1ElEtiquetadoPorExpansionSoloMarcaLoElegido(t *testing.T) {
	ruta := baseDePrueba(t, map[string]int{"tema/alfa": 4})
	marcarExpandidas(t, ruta, "tema/alfa#0", "tema/alfa#2")

	fx, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{Etiquetado: EtiquetadoPorExpansion})
	if err != nil {
		t.Fatalf("FixtureDesdeDB: %v", err)
	}
	q := consultaPorID(fx, "expand:tema/alfa")
	if q == nil {
		t.Fatalf("falta la consulta del tópico; hay %d consultas", len(fx.Queries))
	}
	if len(q.Relevant) != 2 {
		t.Fatalf("relevantes = %v (%d), esperaba las 2 expandidas: el resto del tópico no lo eligió nadie", q.Relevant, len(q.Relevant))
	}
	for _, id := range q.Relevant {
		if id != "tema/alfa#0" && id != "tema/alfa#2" {
			t.Errorf("%q está etiquetado como relevante y nunca se expandió", id)
		}
	}
	// EL CORPUS NO SE RECORTA, sólo la etiqueta. Los cuatro documentos siguen siendo candidatos: si
	// el ranker devuelve los no elegidos, eso tiene que CONTAR como ruido, no desaparecer del
	// universo. Recortar el corpus a lo expandido le regalaría al ranker la mitad del problema.
	if len(fx.Docs) != 4 {
		t.Errorf("el corpus quedó en %d documentos, esperaba los 4: la etiqueta se recorta, el universo no", len(fx.Docs))
	}
	if !strings.Contains(q.Note, "expand_count") {
		t.Errorf("la nota de la consulta no dice de dónde salió la etiqueta: %q", q.Note)
	}
}

// X2 — EL ETIQUETADO POR DEFECTO NO CAMBIA.
//
// La mitad que se olvida: agregar una fuente de etiquetas no puede mover la que ya se usó para
// medir. Sin esta guarda, todo número histórico del banco pasaría a ser incomparable en silencio.
func TestX2SinPedirloElEtiquetadoSigueSiendoElDelTopico(t *testing.T) {
	ruta := baseDePrueba(t, map[string]int{"tema/alfa": 4})
	marcarExpandidas(t, ruta, "tema/alfa#0")

	fx, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{})
	if err != nil {
		t.Fatalf("FixtureDesdeDB: %v", err)
	}
	q := consultaPorTopico(fx, "tema/alfa")
	if q == nil {
		t.Fatal("falta la consulta por tópico")
	}
	if len(q.Relevant) != 4 {
		t.Errorf("relevantes por tópico = %d, esperaba 4: la expansión de una fila no puede alterar el etiquetado histórico", len(q.Relevant))
	}
}

// X3 — SIN SEÑAL SUFICIENTE NO HAY FIXTURE FLACO: HAY ERROR, Y DICE POR QUÉ.
//
// Un tópico con UNA sola expansión no da para medir orden (Recall@10 sólo puede dar 0 o 1). La
// salida cómoda sería emitir la consulta igual y dejar que las métricas salgan raras; la peligrosa
// sería caer al etiquetado por tópico. Las dos producen un informe que se lee sano.
//
// Y el MENSAJE importa tanto como el error: nombrar los umbrales del tópico (min=3, max=50) manda a
// tocar la perilla equivocada cuando lo que falta es USO — la columna arrancó vacía en la v57.
func TestX3SinExpansionesSuficientesFallaNombrandoLaCausaReal(t *testing.T) {
	ruta := baseDePrueba(t, map[string]int{"tema/alfa": 4})
	marcarExpandidas(t, ruta, "tema/alfa#0") // una sola: por debajo del mínimo

	fx, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{Etiquetado: EtiquetadoPorExpansion})
	if err == nil {
		t.Fatalf("esperaba error y obtuve un fixture con %d consultas: una etiqueta de un solo relevante no mide orden", len(fx.Queries))
	}
	if !strings.Contains(err.Error(), "expandidos") {
		t.Errorf("el error no nombra la causa real (faltan expansiones): %v", err)
	}
	if strings.Contains(err.Error(), "pasó los filtros") {
		t.Errorf("el error culpa a los umbrales del tópico, que no son lo que mordió: %v", err)
	}
}

// X4 — UNA BASE SIN LA COLUMNA NO SE ETIQUETA POR TÓPICO EN SILENCIO.
//
// El fixture abre en mode=ro: no puede migrar. Contra una base anterior a la v57 la columna no
// existe, y caer al etiquetado por tópico daría el informe MÁS peligroso que existe — uno correcto
// y completo, midiendo otra cosa que la pedida. Es la misma trampa que el alias del proxy en la
// calibración de tokens: el número converge igual y no dice contra qué se midió.
func TestX4UnaBaseSinLaColumnaFallaEnVezDeMedirOtraCosa(t *testing.T) {
	ruta := baseDePrueba(t, map[string]int{"tema/alfa": 4})

	// Retroceder el esquema: SQLite no sabe DROP COLUMN en todas las versiones, así que se
	// reconstruye la tabla sin la columna. Es la forma más fiel de «una base vieja».
	db, err := sql.Open("sqlite", "file:"+ruta)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE observations DROP COLUMN expand_count`); err != nil {
		db.Close()
		t.Skipf("este SQLite no soporta DROP COLUMN: %v", err)
	}
	db.Close()

	_, err = FixtureDesdeDB(ruta, OpcionesFixtureReal{Etiquetado: EtiquetadoPorExpansion})
	if err == nil {
		t.Fatal("una base sin expand_count devolvió un fixture: el banco habría medido por tópico creyendo medir por expansión")
	}
	// LA FRASE QUE SE EXIGE ES LA MÍA, NO UNA QUE PUEDA PONER OTRO. Pedir que el error diga
	// "expand_count" parecía suficiente y no lo es: SQLite dice `no such column: expand_count` solo,
	// así que una implementación que NO chequee la columna y deje explotar la consulta satisface esa
	// aserción sin haber decidido nada. Es la falla 3 del corredor de sabotajes —la guarda mirando a
	// un vecino— y acá el vecino habla el mismo idioma. "esquema anterior a la v57" sólo la puede
	// escribir el chequeo explícito.
	if !strings.Contains(err.Error(), "esquema anterior a la v57") {
		t.Errorf("el error no explica QUÉ le falta a la base ni desde cuándo existe la columna: %v", err)
	}
}
