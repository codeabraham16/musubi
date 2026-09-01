package mcp

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// UNA MÁQUINA A LA QUE NADIE LE PIDIÓ QUE MIRARA NO EMITE LA SERIE (A67).
//
// Es la regla que decide si esta alerta sirve o es ruido. Si una máquina sin destinos emitiera 0,
// TODA la flota sin configurar dispararía `MaquinaQueNoAlcanzaSuDestino` desde el día uno — y una
// alarma que suena sin que nada esté mal es cómo se le enseña a alguien a ignorar el canal entero.
//
// Ausente no es falso: es la misma regla que gobierna el track desde S4.
//
// Sabotaje: devolver (0, true) cuando `len(m.Alcance) == 0` → falla acá.
func TestUnaMaquinaSinDestinosNoEmiteLaSerieDeAlcance(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "pc-gio")

	m := &fleet.Muestra{Tomada: time.Now().UTC(), NumCPU: 4, MemTotal: 100, MemUsada: 25}
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tok, cuerpoConMuestra(t, m)); code != http.StatusOK {
		t.Fatal("el latido falló")
	}
	if out := exportar(t, s, nil); strings.Contains(out, "musubi_fleet_device_reach_up") {
		t.Errorf("se emitió reach_up sin destinos configurados: toda la flota sin configurar sonaría desde el día uno:\n%s", out)
	}
}

// LA SERIE DICE «LLEGO A TODO», Y EL DETALLE DE CUÁL FALLA VIVE EN LA TOOL.
//
// La métrica no lleva etiqueta `destino` a propósito: sus valores los elige quien configura CADA
// máquina, así que como etiqueta serían cardinalidad sin techo por flota — la misma decisión que
// este repo ya tomó con el desglose de servicios. La pregunta «cuál» se responde en
// `musubi_fleet_list`, donde una entrada más cuesta una columna y no una serie por máquina.
//
// Sabotaje: devolver 1 aunque alguna sonda falle → falla la primera mitad.
func TestElAlcanceFallaEnLaSerieYSeDetallaEnLaTool(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "pc-gio")

	m := &fleet.Muestra{
		Tomada: time.Now().UTC(), NumCPU: 4, MemTotal: 100, MemUsada: 25,
		Alcance: []fleet.SondaDeAlcance{
			{Destino: "relay:21116", Alcanza: false},
			{Destino: "relay:21117", Alcanza: true},
		},
	}
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tok, cuerpoConMuestra(t, m)); code != http.StatusOK {
		t.Fatal("el latido falló")
	}

	out := exportar(t, s, nil)
	if !strings.Contains(out, "musubi_fleet_device_reach_up") {
		t.Fatalf("no se emitió reach_up con sondas presentes:\n%s", out)
	}
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "musubi_fleet_device_reach_up{") && !strings.HasSuffix(l, " 0") {
			t.Errorf("con una sonda caída la serie no dio 0: %q", l)
		}
	}

	// Y LA TOOL DICE CUÁL. Sólo los que NO llegan: en una flota sana la lista completa sería ruido
	// en cada fila, y lo que se busca al abrir esta tool es qué está mal.
	res, e := call(t, s, "musubi_fleet_list", map[string]any{"project": "casa"})
	if e != nil {
		t.Fatalf("musubi_fleet_list: %v", e)
	}
	crudo := textOf(t, res)
	if !strings.Contains(crudo, "no_alcanza") || !strings.Contains(crudo, "relay:21116") {
		t.Errorf("la tool no dice cuál destino falla:\n%s", crudo)
	}
	if strings.Contains(crudo, "relay:21117") {
		t.Errorf("la tool listó un destino que SÍ se alcanza: en una flota sana eso es ruido en cada fila:\n%s", crudo)
	}
}

// UNA MUESTRA VIEJA —anterior al campo— SE SIGUE LEYENDO.
//
// `last_sample` es una columna de texto con el JSON de la muestra, así que agregar un campo no
// costó migración. El precio de eso es que las filas viejas no lo tienen, y tienen que resolver a
// «no medido» y no a un error que haga desaparecer la máquina del inventario.
//
// Sabotaje: hacer `Alcance` obligatorio en el JSON (sacarle `omitempty` no alcanza; habría que
// fallar el parseo si falta) → falla acá.
func TestUnaMuestraSinElCampoDeAlcanceSeSigueLeyendo(t *testing.T) {
	var m fleet.Muestra
	viejo := `{"tomada":"2026-08-01T10:00:00Z","num_cpu":4,"mem_total":100,"mem_usada":25}`
	if err := json.Unmarshal([]byte(viejo), &m); err != nil {
		t.Fatalf("una muestra anterior al campo dejó de parsear: %v", err)
	}
	if len(m.Alcance) != 0 {
		t.Errorf("una muestra sin el campo inventó %d sondas", len(m.Alcance))
	}
}
