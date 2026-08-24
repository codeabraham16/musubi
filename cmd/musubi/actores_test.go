package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// pedirCenso hace un GET al handler del panel y devuelve la respuesta decodificada.
func pedirCenso(t *testing.T, h http.HandlerFunc, url string) censoRespuesta {
	t.Helper()
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, url, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("el panel respondió %d; el estado del cerebro no puede viajar como error HTTP", rec.Code)
	}
	var c censoRespuesta
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatalf("la respuesta no parsea: %v (%s)", err, rec.Body.String())
	}
	return c
}

// A10 — sin central, el panel DICE que no hay censo en vez de servir una lista vacía.
//
// Es la misma falla que el riel tuvo que resolver: «no pasó nada» y «no estoy conectado» se
// dibujan idénticos si lo único que viaja es la lista. Acá es peor todavía, porque el censo es
// justamente el dato que en esta máquina no existe (230.682 llamadas, ninguna con credencial).
func TestA10SinCentralElCensoSeDeclaraApagado(t *testing.T) {
	c := pedirCenso(t, handlerActores(nuevoRelay("", ""), &cacheCenso{}), "/api/actores")
	if c.Estado != "apagado" {
		t.Fatalf("estado = %q, esperaba «apagado»", c.Estado)
	}
	if len(c.Censo) != 0 {
		t.Errorf("sin central no puede viajar un censo: %s", c.Censo)
	}
	if c.Detalle == "" {
		t.Error("un estado que no es «vivo» tiene que traer la frase que explica por qué")
	}
}

// A11 — un 404 del central es «cerebro viejo», NUNCA «no hay actores».
//
// Un central anterior a este endpoint devuelve 404 al mismo pedido que un central al día
// contesta lleno. Si el panel tradujera eso a una lista vacía, dibujaría un sistema desierto
// sobre un cerebro trabajando — y no habría nada en la pantalla que permitiera notarlo.
func TestA11UnCentralViejoNoSeLeeComoCeroActores(t *testing.T) {
	central := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer central.Close()

	c := pedirCenso(t, handlerActores(nuevoRelay(central.URL, "tok"), &cacheCenso{}), "/api/actores")
	if c.Estado != "viejo" {
		t.Fatalf("estado = %q, esperaba «viejo»", c.Estado)
	}
	if len(c.Censo) != 0 {
		t.Errorf("un 404 no puede producir un censo: %s", c.Censo)
	}
}

// A12 — el token va al CENTRAL y no al navegador.
//
// El proxy existe precisamente para que la credencial no baje a la página. Un handler que la
// reenvíe por descuido —en el cuerpo, en un detalle de error— anula la única razón por la que
// este código existe, y el test tiene que mirar los bytes que salen, no la intención.
func TestA12ElTokenViajaAlCerebroYNoAlNavegador(t *testing.T) {
	const secreto = "tok-super-secreto-42"
	var visto atomic.Value
	central := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visto.Store(r.Header.Get("Authorization"))
		// Se responde un error para ejercitar el camino donde el cuerpo ajeno llega a la
		// pantalla: es por donde un secreto se filtraría sin que nadie lo mirara.
		http.Error(w, "boom "+secreto, http.StatusInternalServerError)
	}))
	defer central.Close()

	rec := httptest.NewRecorder()
	handlerActores(nuevoRelay(central.URL, secreto), &cacheCenso{})(rec, httptest.NewRequest(http.MethodGet, "/api/actores", nil))

	if auth, _ := visto.Load().(string); auth != "Bearer "+secreto {
		t.Fatalf("el cerebro recibió Authorization=%q; el token tiene que ir en la petición al central", auth)
	}
	if strings.Contains(rec.Body.String(), secreto) {
		t.Fatalf("FUGA: el token salió hacia el navegador en la respuesta: %s", rec.Body.String())
	}
}

// A13 — el censo se cachea: abrir cinco pestañas no son cinco COUNT(DISTINCT) en el central.
//
// Es la lección de /api/pulse, que corría un Diagnose() entero por sondeo y terminó tardando 50 s
// contra una base de 54 MB. El volumen histórico de cada actor cambia en horas; volver a pedirlo
// por cada carga es gasto puro contra la máquina que además atiende a todo el equipo.
func TestA13ElCensoNoSeLePideAlCentralEnCadaCarga(t *testing.T) {
	var veces atomic.Int64
	central := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		veces.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"days":7,"actores":[{"principal":"gio","calls":3}],"sin_principal":0}`))
	}))
	defer central.Close()

	h := handlerActores(nuevoRelay(central.URL, "tok"), &cacheCenso{})
	for i := 0; i < 5; i++ {
		c := pedirCenso(t, h, "/api/actores")
		if c.Estado != "vivo" {
			t.Fatalf("carga %d: estado = %q, esperaba «vivo» (%s)", i, c.Estado, c.Detalle)
		}
		if !strings.Contains(string(c.Censo), `"gio"`) {
			t.Fatalf("carga %d: el censo no llegó entero: %s", i, c.Censo)
		}
	}
	if n := veces.Load(); n != 1 {
		t.Fatalf("el panel le pegó %d veces al central para 5 cargas; el censo tiene que cachearse", n)
	}

	// Y una ventana distinta NO se sirve del cache: el cache es por (días), no global. Servir
	// 7 días cuando alguien pidió 30 sería devolver otro dato con cara del pedido.
	if c := pedirCenso(t, h, "/api/actores?days=30"); c.Estado != "vivo" {
		t.Fatalf("otra ventana: estado = %q", c.Estado)
	}
	if n := veces.Load(); n != 2 {
		t.Fatalf("pedir otra ventana tenía que ir al central; llamadas totales = %d", n)
	}
}
