package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// NINGÚN JOIN DE FLOTA PUEDE EMPAREJAR SÓLO POR `device`.
//
// Ola 1 del plan empresa. Con un solo tenant, `on(device)` y `on(project, device)` se comportan
// igual — por eso este error es invisible hoy y por eso hace falta una prueba y no la vista.
//
// El día que dos clientes tengan cada uno un `srv-01`, `on(device)` empareja muchos-con-muchos y
// Prometheus **no evalúa la regla**: la descarta entera con un error. O sea que `MaquinaCaida`
// dejaría de vigilar A TODA LA FLOTA —no sólo a esas dos máquinas— y el síntoma sería silencio,
// que es exactamente lo que no se nota.
//
// `project` está en las cuatro etiquetas de toda serie de flota (device, project, tier, os), así
// que ponerlo no cuesta nada y no hay excepción legítima. Por eso esta prueba no tiene lista de
// exenciones: la primera que haga falta es una conversación, no un append.
//
// Sabotaje que la hace fallar: volver cualquier `on(project, device)` a `on(device)` en
// deploy/musubi-alerts-flota.yml.
func TestNingunJoinDeFlotaEmparejaSoloPorDevice(t *testing.T) {
	ruta := filepath.Join("..", "..", "deploy", "musubi-alerts-flota.yml")
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", ruta, err)
	}
	// `device` solo, sin `project` al lado, en cualquier forma de emparejamiento de PromQL.
	malo := regexp.MustCompile(`\b(on|ignoring|by|without)\s*\(\s*device\s*[,)]`)
	for i, l := range strings.Split(string(b), "\n") {
		limpia := strings.TrimSpace(l)
		if strings.HasPrefix(limpia, "#") {
			continue // los comentarios se revisan leyendo, no acá
		}
		if m := malo.FindString(l); m != "" {
			t.Errorf("%s:%d empareja por `device` sin `project` (%q).\n"+
				"Con dos clientes que tengan una máquina del mismo nombre, Prometheus descarta la "+
				"regla ENTERA por emparejamiento muchos-con-muchos, y esa alerta deja de vigilar a "+
				"toda la flota en silencio.\n  %s", ruta, i+1, m, limpia)
		}
	}
}

// Y EL AGRUPADO DE ALERTMANAGER TAMBIÉN LLEVA `project`.
//
// Es la otra mitad, y falla distinto: sin `project` en el `group_by`, el `srv-01` de un cliente y
// el de otro caen en el MISMO hilo de notificación, y un `resolved` de uno cierra el aviso del
// otro. El que queda sin arreglar es justo el que deja de verse.
//
// Lo mismo para las inhibiciones que acotan por máquina: `equal: ['device']` haría que la caída
// del `srv-01` de un cliente callara los avisos del `srv-01` de otro.
//
// Sabotaje: sacarle `project` al `group_by`, o a cualquier `equal` que nombre `device`.
func TestElAgrupadoYLasInhibicionesLlevanProject(t *testing.T) {
	ruta := filepath.Join("..", "..", "deploy", "prometheus", "alertmanager.yml")
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", ruta, err)
	}
	var vistos int
	for i, l := range strings.Split(string(b), "\n") {
		limpia := strings.TrimSpace(l)
		if strings.HasPrefix(limpia, "#") {
			continue
		}
		esGroupBy := strings.HasPrefix(limpia, "group_by:")
		esEqual := strings.HasPrefix(limpia, "equal:")
		if !esGroupBy && !esEqual {
			continue
		}
		// Sólo importan los que ya nombran `device`: un `equal` sin device no agrupa por máquina.
		if !strings.Contains(limpia, "device") {
			continue
		}
		vistos++
		if !strings.Contains(limpia, "project") {
			t.Errorf("%s:%d agrupa o inhibe por `device` sin `project`: dos clientes con una "+
				"máquina del mismo nombre comparten hilo, y un `resolved` de uno cierra el aviso "+
				"del otro.\n  %s", ruta, i+1, limpia)
		}
	}
	if vistos == 0 {
		t.Error("no se encontró ningún group_by/equal con `device`: la prueba no está mirando lo que cree, y un verde acá no significa nada")
	}
}
