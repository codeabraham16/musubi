package memory

import (
	"context"
	"testing"
)

// LA EXPANSIÓN ES LA ÚNICA SEÑAL EXÓGENA DEL RECALL, Y HASTA LA v57 SE GUARDABA EN LA COLUMNA DE LA
// ENDÓGENA.
//
// `bumpAccess` lo llamaban dos caminos que no significan lo mismo: recall.go bumpea lo que ACABA DE
// SERVIR —el ranker escribiendo sobre su propia salida, el lazo que la invariante N4 amortigua— y
// expand.go bumpea lo que el agente ELIGIÓ de entre lo servido, con el titular delante. Sumadas en
// `access_count`, la segunda era irrecuperable.
//
// Lo que estas guardas fijan es la SEPARACIÓN, no el uso: el ranking no cambia y `access_count`
// sigue recibiendo exactamente los mismos incrementos que antes.

// contadoresDe lee las dos columnas de una observación. Va a la base y no a una struct de Go a
// propósito: lo que se está afirmando es qué quedó ESCRITO, no qué devolvió una función.
func contadoresDe(t *testing.T, e *DbEngine, id string) (accesos, expansiones int) {
	t.Helper()
	if err := e.db.QueryRow(
		`SELECT COALESCE(access_count,0), COALESCE(expand_count,0) FROM observations WHERE id = ?`, id,
	).Scan(&accesos, &expansiones); err != nil {
		t.Fatalf("leer contadores de %q: %v", id, err)
	}
	return accesos, expansiones
}

// E1 — SERVIR NO ES ELEGIR.
//
// Es la guarda central: después de un recall, la observación fue ACCEDIDA pero NO expandida. Si
// `expand_count` subiera acá, la señal nacería contaminada con la salida del propio ranker y no
// valdría como etiqueta para nada — que es exactamente la situación anterior a la v57, sólo que
// con una columna más para disimularlo.
func TestE1ElRecallNoCuentaComoExpansion(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	if err := e.SaveObservation("obs-e1", "senal/expansion", "El recall sirve titulares; expandir es otra cosa.", nil); err != nil {
		t.Fatal(err)
	}

	res, err := e.Recall(ctx, "el recall sirve titulares", RecallOptions{})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(res.Items) == 0 {
		t.Fatal("precondición: el recall no devolvió nada, así que no hay bump que juzgar")
	}

	accesos, expansiones := contadoresDe(t, e, "obs-e1")
	if accesos != 1 {
		t.Errorf("access_count tras un recall = %d, esperaba 1: el refuerzo de acceso tiene que seguir intacto", accesos)
	}
	if expansiones != 0 {
		t.Errorf("expand_count tras un recall = %d, esperaba 0: SERVIR un gist no es que el agente lo haya ELEGIDO", expansiones)
	}
}

// E2 — ELEGIR SE CUENTA APARTE, Y ADEMÁS.
//
// Las dos mitades importan. Que `expand_count` suba es la señal nueva; que `access_count` suba
// TAMBIÉN es la promesa de que esto no cambia el ranking: el contador que el ranker lee queda
// bit-idéntico a como venía. Una implementación que moviera el bump de acceso al contador nuevo
// pasaría media guarda y le cambiaría la frecuencia a toda la memoria en silencio.
func TestE2ExpandirCuentaEnSuColumnaSinTocarLaDelAcceso(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	if err := e.SaveObservation("obs-e2", "senal/expansion", "De todos los gists que me diste, éste lo quiero entero.", nil); err != nil {
		t.Fatal(err)
	}

	if _, _, err := e.GetObservationsBudgetCtx(ctx, []string{"obs-e2"}, 0); err != nil {
		t.Fatalf("expandir: %v", err)
	}

	accesos, expansiones := contadoresDe(t, e, "obs-e2")
	if expansiones != 1 {
		t.Errorf("expand_count tras UNA expansión = %d, esperaba 1: la elección del agente no se registró", expansiones)
	}
	if accesos != 1 {
		t.Errorf("access_count tras una expansión = %d, esperaba 1: separar la señal NO puede cambiar lo que el ranker lee", accesos)
	}
}

// E3 — LO QUE NO SE DEVOLVIÓ NO SE CUENTA.
//
// `hydrateByIDs` corta por presupuesto de tokens: pedir tres ids con lugar para uno devuelve uno. El
// bump va sobre `found` —lo que efectivamente se empaquetó— y no sobre lo que se pidió. Contar los
// tres volvería a `expand_count` una medida de INTENCIÓN y no de material entregado, y la etiqueta
// del banco marcaría como elegido algo que el agente nunca llegó a leer.
func TestE3SoloSeCuentaLoQueDeVerdadEntroEnElPresupuesto(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	largo := ""
	for i := 0; i < 300; i++ {
		largo += "contenido de relleno para empujar el costo en tokens bien por encima del techo. "
	}
	for _, id := range []string{"obs-e3a", "obs-e3b"} {
		if err := e.SaveObservation(id, "senal/expansion", largo, nil); err != nil {
			t.Fatal(err)
		}
	}

	out, _, err := e.GetObservationsBudgetCtx(ctx, []string{"obs-e3a", "obs-e3b"}, 50)
	if err != nil {
		t.Fatalf("expandir con presupuesto: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("precondición: esperaba que el presupuesto dejara entrar 1 sola, entraron %d", len(out))
	}

	if _, exp := contadoresDe(t, e, "obs-e3a"); exp != 1 {
		t.Errorf("la que SÍ entró tiene expand_count = %d, esperaba 1", exp)
	}
	if _, exp := contadoresDe(t, e, "obs-e3b"); exp != 0 {
		t.Errorf("la que NO entró tiene expand_count = %d, esperaba 0: se contó material que el agente nunca recibió", exp)
	}
}

// E4 — FUNDAMENTAR UNA PREGUNTA NO ES ELEGIR.
//
// `HydrateForGroundingCtx` es el camino de musubi_ask: trae contenido por id para fundamentar una
// respuesta, y ya existe justamente porque ahí NO hay una elección humana ni de agente — los ids
// los eligió el Recall. Si este camino contara expansiones, cada pregunta al motor inyectaría
// etiquetas de relevancia fabricadas por el propio ranker, y el banco estaría midiéndose contra su
// salida con una columna nueva.
func TestE4ElGroundingDeAskNoCuentaComoExpansion(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	if err := e.SaveObservation("obs-e4", "senal/expansion", "Material que el motor usa para fundamentar una respuesta.", nil); err != nil {
		t.Fatal(err)
	}

	if _, _, err := e.HydrateForGroundingCtx(ctx, []string{"obs-e4"}, 0); err != nil {
		t.Fatalf("grounding: %v", err)
	}

	accesos, expansiones := contadoresDe(t, e, "obs-e4")
	if expansiones != 0 {
		t.Errorf("expand_count tras el grounding = %d, esperaba 0: los ids los eligió el Recall, no un agente", expansiones)
	}
	if accesos != 0 {
		t.Errorf("access_count tras el grounding = %d, esperaba 0: ese camino existe para NO contar el acceso dos veces", accesos)
	}
}

// E5 — EN SÓLO LECTURA SE SIGUE PUDIENDO EXPANDIR.
//
// El modo sólo-lectura existe para poder LEER una base que este binario no sabe migrar. El contador
// nuevo es un UPDATE, o sea exactamente la clase de escritura que `PRAGMA query_only` rechaza: sin
// la misma guarda que ya tiene bumpAccess, agregar esta señal habría roto `musubi_memory_expand`
// justo en el modo que existe para que la memoria siga siendo legible. Lo que se pierde es el
// refuerzo de esta sesión, no el contenido.
func TestE5EnSoloLecturaLaExpansionNoRompeLaLectura(t *testing.T) {
	dir := baseMasNueva(t)
	eng, err := NewDbEngineSoloLectura(dir)
	if err != nil {
		t.Fatalf("NewDbEngineSoloLectura: %v", err)
	}
	defer eng.Close()

	res, err := eng.Recall(context.Background(), "nota guardada antes", RecallOptions{})
	if err != nil || len(res.Items) == 0 {
		t.Fatalf("precondición: el recall en sólo lectura tiene que traer la nota sembrada (err=%v, items=%d)", err, len(res.Items))
	}

	out, _, err := eng.GetObservationsBudgetCtx(context.Background(), []string{res.Items[0].ID}, 0)
	if err != nil {
		t.Fatalf("expandir en sólo lectura falló: %v — el contador nuevo dejó inservible a memory_expand", err)
	}
	if len(out) != 1 {
		t.Fatalf("expandir en sólo lectura devolvió %d observaciones, esperaba 1", len(out))
	}
}
