package mcp

import (
	"encoding/json"
	"net/http"
	"testing"

	"musubi/internal/buildid"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// latirConInventario manda un latido declarando capver y, opcionalmente, el motivo por el que la
// máquina no pudo enumerar. Se arma con `fleet.CuerpoLatido` y no con JSON a mano a propósito: un
// literal se quedaría viejo el día que el campo se renombre, y esta prueba habla justamente de qué
// pasa cuando el contrato se mueve.
func latirConInventario(t *testing.T, ts, token string, capver int, motivo string) {
	t.Helper()
	cuerpo, err := json.Marshal(fleet.CuerpoLatido{
		Version:        "0.139.0-prueba",
		Capver:         capver,
		ServiciosError: motivo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code, body := postCon(t, ts+fleetHeartbeatPath, token, string(cuerpo)); code != http.StatusOK {
		t.Fatalf("el latido con capver=%d falló: %d %s", capver, code, body)
	}
}

// UN 0 QUE SIGNIFICA «NO SÉ» ES PEOR QUE NINGUNA SERIE, Y ACÁ DEJABA UNA ALERTA MUDA.
//
// `musubi_fleet_device_services_unknown` devolvía 0 en cuanto `ServiciosError` venía vacío. Un
// agente anterior al capver 2 NO MANDA ese campo, así que llega `""` — idéntico al `""` de uno que
// enumeró bien. O sea que el cerebro afirmaba «esta máquina enumeró» sobre una que nunca contestó
// la pregunta, y `MaquinaNoPuedeEnumerar` quedaba VERDE POR IGNORANCIA.
//
// Y el modo de falla es el peor de los dos posibles: la alerta dispara con `== 1`, así que un
// falso 0 es una alerta PERDIDA, no una falsa. Nadie ve nada.
//
// MEDIDO EL 2026-09-09 EN LA FLOTA REAL: las cuatro máquinas con la serie en 0. Dos de ellas
// —`davantis-1` en 0.139.1 y `gio` en 0.131.0— con agentes que no conocen el campo, y `altura-db`
// sin agente ninguno. Cuatro ceros, y dos no significaban nada.
//
// LA REGLA QUE SE APLICA ES LA QUE YA GOBIERNA ESTE EXPORTADOR: un dato ausente no es un cero. Del
// lado de Prometheus «no sé» se pregunta con `absent()`, que es su forma natural; un `-1` habría
// que enseñárselo a cada regla y la que se olvide lo lee como un número.
//
// SABOTAJES CORRIDOS:
//   - ROJO: sacar la compuerta de capver de `services_unknown` → la máquina vieja vuelve a
//     publicar un 0 y esta prueba lo caza.
//   - ROJO: gatear con `>` en vez de `>=` → la máquina que SÍ sabe contestar (capver == 2) pierde
//     su serie, que es el defecto opuesto y también deja la alerta sin poder disparar.
//   - VERDE, y tiene que serlo: subir `buildid.Capver` a 3. `CapverConInventarioExplicado` se
//     queda en 2 porque un agente en 2 sigue sabiendo contestar, y esta prueba no se entera —
//     que es la diferencia entre fijar UNA CAPACIDAD y fijar el último contrato.
func TestLasSeriesDelInventarioSeOmitenSiElAgenteNoSabeContestar(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)

	vieja := enrolarDePrueba(t, s, "casa", "vieja")
	sana := enrolarDePrueba(t, s, "casa", "sana")
	rota := enrolarDePrueba(t, s, "casa", "rota")

	latirConInventario(t, ts.URL, vieja, buildid.CapverConInventarioExplicado-1, "")
	latirConInventario(t, ts.URL, sana, buildid.CapverConInventarioExplicado, "")
	latirConInventario(t, ts.URL, rota, buildid.CapverConInventarioExplicado, "powershell: exit status 1")

	out := exportar(t, s, nil)

	// 1 · LA VIEJA NO APORTA NINGUNA DE LAS DOS SERIES.
	for _, m := range []string{"musubi_fleet_device_services_unknown", "musubi_fleet_device_services_omitted"} {
		if v, hay := serieDe(out, m, "vieja"); hay {
			t.Errorf("%s da %q para una máquina en capver %d, que NO manda ese campo.\n"+
				"Ese valor no es una medición: es el default de un campo que la máquina nunca "+
				"llenó, y publicado como número hace que `MaquinaNoPuedeEnumerar` no pueda "+
				"disparar sobre ella. La serie tiene que estar AUSENTE — `absent()` es como se "+
				"pregunta «no sé» en Prometheus.\n%s",
				m, v, buildid.CapverConInventarioExplicado-1, out)
		}
	}

	// 2 · LA SANA SÍ, Y EN 0. Sin esto la compuerta se podría satisfacer omitiendo SIEMPRE, que
	// dejaría la alerta igual de muda por el otro lado.
	if v, hay := serieDe(out, "musubi_fleet_device_services_unknown", "sana"); !hay || v != "0" {
		t.Errorf("services_unknown para la máquina que SÍ sabe contestar dio %q (hay=%v); "+
			"esperaba 0 presente. Omitir siempre deja la alerta tan muda como el 0 falso.\n%s", v, hay, out)
	}
	if _, hay := serieDe(out, "musubi_fleet_device_services_omitted", "sana"); !hay {
		t.Errorf("services_omitted no se emitió para una máquina en capver %d\n%s",
			buildid.CapverConInventarioExplicado, out)
	}

	// 3 · Y LA ROTA EN 1, que es el único valor que hace disparar la alerta.
	if v, hay := serieDe(out, "musubi_fleet_device_services_unknown", "rota"); !hay || v != "1" {
		t.Errorf("services_unknown para la máquina con enumerador roto dio %q (hay=%v); "+
			"esperaba 1, que es con lo que `MaquinaNoPuedeEnumerar` dispara\n%s", v, hay, out)
	}
}
