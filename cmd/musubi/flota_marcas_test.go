package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// LO QUE CUSTODIA: que el panel de la flota tenga una marca PROPIA Y DISTINTA para cada estado de
// servicio que el dominio declara — derivando los estados de la fuente del enum, no de una copia.
//
// SALE DE AUDITAR A131 SOBRE `cmd/musubi/flota_test.go`, y esta vez el defecto no hizo falta
// fabricarlo con una mutación: ESTABA VIVO en el árbol, con la guarda en verde.
//
// La guarda anterior verificaba la tabla `marca` contra una lista de CUATRO estados escrita a mano
// —corriendo, detenido, fallado, desconocido— y exigía `len(glifos) != 4`. Cuando nació `ocioso`
// (A70) se le enseñó al exportador de Prometheus y no al panel: `ocioso` no estaba en `marca`, caía
// en el `: 'desconocido'` del renderizador, y se dibujaba con `?`. El tooltip decía textualmente
// «MapsBroker: desconocido» sobre un estado que se SABE — apagado porque nadie lo pidió, que es
// justo la distinción que el comentario del exportador defiende: «No es "no sé" —eso es
// `desconocido`— sino "la pregunta no aplica"».
//
// LA EXPOSICIÓN ERA TOTAL, NO LATENTE. Medido el 2026-09-23 en la base del cerebro: 17 servicios en
// `ocioso`. La tool `musubi_fleet_services` los manda crudos (`string(sv.EstadoActual())`), así que
// los 17 llegaban al panel y los 17 se dibujaban como «no sé».
//
// Y LA PRUEBA DE QUE LA GUARDA VIEJA DEFENDÍA EL DEFECTO: al agregar la marca de `ocioso` se puso
// ROJA — «se declararon 5 marcas distintas, esperaba 4». Premiaba el bug y castigaba el arreglo.
//
// POR QUÉ DERIVADA DE LA FUENTE. Un estado nuevo tiene que poner esta guarda roja HASTA QUE alguien
// decida cómo se dibuja; si no, cae otra vez en el default y el panel vuelve a mentir en silencio.
// El conjunto se puede cerrar —es un `const` de un solo bloque en `internal/fleet/servicio.go`—
// así que la lista se lee de ahí, con el mismo lector que usa la guarda del exportador.
//
// Sabotaje: sacarle a `ocioso` su marca, que es exactamente el estado en que estuvo el panel.
// arnes: archivo="cmd/musubi/assets/flota.html"
// arnes: de="desconocido: '?', ocioso: '○' };"
// arnes: a="desconocido: '?' };"
func TestCadaEstadoDeServicioTieneSuMarcaEnElPanel(t *testing.T) {
	rutaEnum := filepath.Join("..", "..", "internal", "fleet", "servicio.go")
	crudo, err := os.ReadFile(rutaEnum)
	if err != nil {
		t.Fatalf("leer %s: %v", rutaEnum, err)
	}
	re := regexp.MustCompile(`(?m)^\s*(Estado\w+)\s+EstadoServicio\s*=\s*"([a-z]+)"`)
	var estados []string
	for _, m := range re.FindAllStringSubmatch(string(crudo), -1) {
		estados = append(estados, m[2])
	}
	// PISO: sin él, un lector que dejara de reconocer la declaración pasaría en verde sin haber
	// leído un solo estado — con cero estados, «todos tienen marca» es trivialmente cierto.
	if len(estados) < 4 {
		t.Fatalf("sólo leí %d estados de %s: el lector dejó de reconocer la forma del enum y esta "+
			"guarda no estaría midiendo nada (leídos: %v)", len(estados), rutaEnum, estados)
	}

	p := string(assetsFS(t, "assets/flota.html"))
	i := strings.Index(p, "const marca = {")
	if i < 0 {
		t.Fatal("flota.html no declara la tabla de marcas por estado (`const marca = {`)")
	}
	tabla := p[i : strings.Index(p[i:], "}")+i]

	// Cada estado del DOMINIO tiene su entrada. Uno que falte cae en el default del renderizador
	// (`marca[s.estado] ? s.estado : 'desconocido'`) y se dibuja como «no sé».
	marcaDe := map[string]string{}
	for _, campo := range strings.Split(tabla[strings.Index(tabla, "{")+1:], ",") {
		partes := strings.SplitN(campo, ":", 2)
		if len(partes) != 2 {
			continue
		}
		clave := strings.TrimSpace(partes[0])
		g := strings.Trim(strings.TrimSpace(partes[1]), "'\"")
		if clave != "" && g != "" {
			marcaDe[clave] = g
		}
	}
	for _, e := range estados {
		if _, ok := marcaDe[e]; !ok {
			t.Errorf("el estado `%s` (declarado en %s) no tiene marca en el panel.\n"+
				"No es cosmético: cae en el `: 'desconocido'` del renderizador y se dibuja con `?`,\n"+
				"o sea que el panel dice «no sé» sobre un estado que el dominio SABE. Fue lo que le\n"+
				"pasó a `ocioso` con 17 servicios reales. Agregale una marca propia a `marca`.",
				e, filepath.Base(rutaEnum))
		}
	}

	// Y LAS MARCAS SON DISTINTAS ENTRE SÍ: si dos estados comparten glifo, la columna miente en
	// silencio aunque la tabla esté completa.
	porGlifo := map[string]string{}
	for estado, g := range marcaDe {
		if otro, ya := porGlifo[g]; ya {
			t.Errorf("`%s` y `%s` comparten la marca %q: a simple vista el panel no los distingue\n%s",
				otro, estado, g, tabla)
		}
		porGlifo[g] = estado
	}
	t.Logf("%d estados en %s, %d marcas en el panel", len(estados), filepath.Base(rutaEnum), len(marcaDe))
}
