package mcp

import (
	"sort"
	"strings"
	"testing"
	"time"

	"musubi/internal/buildid"
	"musubi/internal/embedding"
)

// LO QUE CUSTODIA: que el TEXTO al que la métrica manda esté realmente en la fila.
//
// `musubi_fleet_device_services_unknown` contesta 1/0 y su HELP delega el motivo con todas las
// letras: «POR QUÉ no pudo se mira en `musubi_fleet_list`: el motivo es texto libre de la máquina
// y como etiqueta sería cardinalidad sin techo». Ese texto NO estaba en la fila. El agente lo
// mandaba, `FijarServiciosError` lo guardaba, el exportador lo usaba para decidir el 1 — y el
// único camino documentado para leerlo no lo mostraba.
//
// SE DESCUBRIÓ CON LA ALERTA SONANDO. El 2026-09-10 `MaquinaNoPuedeEnumerar` estaba disparando
// sobre `davantis-1`: el aviso mandaba a `musubi_fleet_list` y la fila no traía el motivo. Es una
// afirmación que nadie cruzó, y del lado caro — quien la lee está en medio de un incidente.
//
// POR ESO LA PRUEBA CRUZA LAS DOS MITADES en vez de mirar sólo el campo. Una prueba que afirmara
// «la fila trae servicios_error» dejaría que alguien cambie el HELP para que mande a otro lado y
// las dos quedarían verdes apuntando a lugares distintos, que es exactamente cómo nació esto.

func TestElMotivoDeEnumeracionEstaDondeLaMetricaDiceQueEsta(t *testing.T) {
	// MITAD 1 — LA PROMESA. Se lee del catálogo real del exportador, no de una copia.
	var help string
	for _, m := range seriesDeFlota(time.Now(), time.Minute, "0.0.0-prueba", nil) {
		if m.Nombre == "musubi_fleet_device_services_unknown" {
			help = m.Ayuda
		}
	}
	if help == "" {
		t.Fatal("no se encontró musubi_fleet_device_services_unknown en el catálogo del exportador: " +
			"si se renombró, esta guarda dejó de mirar lo que dice mirar")
	}
	if !strings.Contains(help, "musubi_fleet_list") {
		t.Fatalf("el HELP de services_unknown ya no manda a `musubi_fleet_list` a buscar el motivo. "+
			"Si se movió a otro lado, hay que mover esta guarda con él: una promesa y su cumplimiento "+
			"en archivos distintos se separan solos.\nHELP: %s", help)
	}

	// MITAD 2 — EL CUMPLIMIENTO. La fila real, con una máquina que no pudo enumerar.
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "pc-gio")
	d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")

	const motivo = "Get-Service: acceso denegado al SCM"
	if err := s.engine.ActualizarCapver(d.ID, buildid.CapverConInventarioExplicado); err != nil {
		t.Fatalf("ActualizarCapver: %v", err)
	}
	if err := s.engine.FijarServiciosError(d.ID, motivo); err != nil {
		t.Fatalf("FijarServiciosError: %v", err)
	}

	fila := filaDeLaFlota(t, s, "pc-gio")
	got, hay := fila["servicios_error"].(string)
	if !hay {
		t.Fatalf("la máquina no pudo enumerar y la fila NO trae `servicios_error`, que es donde el "+
			"HELP de la métrica manda a buscarlo. Con la alerta sonando, el operador se queda sin el "+
			"único dato que dice qué arreglar.\nCampos que sí trae: %v", clavesDeLaFila(fila))
	}
	if got != motivo {
		t.Errorf("el motivo llegó alterado: querÍa %q y vino %q", motivo, got)
	}
}

// EL CAPVER ES LA MITAD QUE SEPARA «NO SÉ» DE «ENUMERÉ BIEN», y va en su propio caso porque es la
// que se pierde al agregar un campo: un agente anterior a `CapverConInventarioExplicado` NO manda
// `servicios_error`, así que su `""` llega igual que el de uno que enumeró perfecto. Emitir el
// campo vacío ahí volvería a juntar los dos significados que el capver existe para separar.
func TestUnAgenteViejoNoAparentaHaberEnumeradoBien(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "pc-gio")
	d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")

	// Un agente de contrato viejo: declara capver 1 y, justamente por viejo, no manda el campo.
	if err := s.engine.ActualizarCapver(d.ID, buildid.CapverConInventarioExplicado-1); err != nil {
		t.Fatalf("ActualizarCapver: %v", err)
	}
	if err := s.engine.FijarServiciosError(d.ID, ""); err != nil {
		t.Fatalf("FijarServiciosError: %v", err)
	}

	fila := filaDeLaFlota(t, s, "pc-gio")
	if v, hay := fila["servicios_error"]; hay {
		t.Errorf("un agente que no sabe contestar la pregunta aparece contestándola: la fila trae "+
			"`servicios_error` = %q. Ausente y vacío tienen que verse distinto, que es para lo que "+
			"existe CapverConInventarioExplicado", v)
	}
}

// filaDeLaFlota devuelve la fila de una máquina tal como la emite `musubi_fleet_list`.
func filaDeLaFlota(t *testing.T, s *McpServer, nombre string) map[string]any {
	t.Helper()
	res, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_list", map[string]any{})
	if e != nil {
		t.Fatalf("musubi_fleet_list: %+v", e)
	}
	out := jsonOf(t, res)
	filas, _ := out["devices"].([]any)
	for _, f := range filas {
		m, _ := f.(map[string]any)
		if n, _ := m["name"].(string); n == nombre {
			return m
		}
	}
	t.Fatalf("la flota no trajo la máquina %q: %v", nombre, out)
	return nil
}

// clavesDeLaFila lista los campos que la fila SÍ trae. El mensaje de un fallo tiene que decir qué
// hay, no sólo qué falta: es la diferencia entre «lo renombraron» y «no lo emite nadie».
func clavesDeLaFila(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
