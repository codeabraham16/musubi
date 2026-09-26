package mcp

import (
	"reflect"
	"slices"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// CADA TOOL QUE TOMA UNA VENTANA DEVUELVE EN `no_visto` TODOS LOS HUECOS DE SU DOMINIO, en su orden
// y adelante de lo propio de la superficie.
//
// Los huecos son límites de diseño que viven en UN lugar —fleet.HuecosDeLaCronologia,
// fleet.HuecosDelContexto— para que una superficie no diga una cosa y otra diga otra. Las guardas de
// la respuesta sólo pedían que `no_visto` no viniera vacío (TestLaCronologiaCruzaLosTresPlanos), o
// buscaban cuatro palabras sobre el texto unido (TestElContextoDeclaraQueEsCorrelacionYNoCausa). La
// auditoría A131 (C3-m5) mandó `HuecosDeLaCronologia()[:1]`: de cinco huecos llegaba uno y fleet y
// mcp quedaron verdes. En el contexto, el recorte a los tres primeros también pasa, porque las cuatro
// palabras están en ellos y en el agregado. Acá se compara contra la FUENTE, hueco por hueco.
//
// Las tools salen del catálogo (toolsDeVentana, la derivación de T8) y no de una lista: una tool nueva
// que tome ventana entra sola, y la prueba le exige que alguien decida qué huecos declara.
//
// EXPOSICIÓN medida por la auditoría: la cronología se llamó 14 veces entre el 2026-08-30 y el
// 2026-09-21 (10 ok), y cada respuesta ok lleva `no_visto`; unos diez lectores reales (agentes) en tres
// semanas. El recorte es hipotético; ninguna alerta se enteraría si llegara.
//
// Sabotaje: que la cronología mande sólo el primer hueco (C3-m5) → cuatro límites dejan de viajar.
// arnes: archivo="internal/mcp/methods_cronologia.go"
// arnes: de="\"no_visto\": fleet.HuecosDeLaCronologia(),"
// arnes: a="\"no_visto\": fleet.HuecosDeLaCronologia()[:1],"
// Sabotaje: que el contexto mande sólo sus tres primeros huecos → dejan de viajar de dónde sale la
// memoria sin proyecto y de dónde salen los términos, y la guarda vieja sigue verde.
// arnes: archivo="internal/mcp/methods_contexto.go"
// arnes: de="\"no_visto\": append(fleet.HuecosDelContexto(),"
// arnes: a="\"no_visto\": append(fleet.HuecosDelContexto()[:3],"
// arnes: colision_ok="TestElContextoDeclaraQueEsCorrelacionYNoCausa"
func TestCadaToolDeVentanaDevuelveTodosLosHuecosDeSuDominio(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarContexto(t, s)
	p := conCaps("infra", map[fleet.Cap][]string{
		fleet.CapMetrics: {"*"}, fleet.CapExec: {"*"}, fleet.CapScreen: {"*"}, fleet.CapShell: {"*"},
	})

	// LA DECISIÓN: qué huecos del dominio declara cada tool. Cerrada contra la derivación de abajo.
	dominio := map[string]func() []string{
		"musubi_fleet_cronologia": fleet.HuecosDeLaCronologia,
		"musubi_fleet_contexto":   fleet.HuecosDelContexto,
	}
	tools := toolsDeVentana(s)
	for nombre := range dominio {
		if !slices.Contains(tools, nombre) {
			t.Errorf("%s ya no toma una ventana (la derivación encontró %v): esta prueba dejó de medir sus huecos", nombre, tools)
		}
	}

	medidas := 0
	for _, tool := range tools {
		huecos, decidida := dominio[tool]
		if !decidida {
			t.Errorf("%s toma una ventana y nadie decidió qué huecos declara: una respuesta sin sus límites "+
				"se lee como «esto es todo lo que pasó»", tool)
			continue
		}
		quiero := huecos()
		// EL PISO: un dominio sin huecos haría pasar la comparación sin mirar nada.
		if len(quiero) == 0 {
			t.Fatalf("el dominio de %s no declara ningún hueco: la comparación no mediría nada", tool)
		}
		res, e := callAsPrincipal(t, s, p, tool, map[string]any{"device": "pc-gio", "horas": 24})
		if e != nil {
			t.Fatalf("%s: %+v", tool, e)
		}
		var viajan []string
		for _, h := range jsonOf(t, res)["no_visto"].([]any) {
			texto, _ := h.(string)
			viajan = append(viajan, texto)
		}
		if len(viajan) < len(quiero) || !reflect.DeepEqual(viajan[:len(quiero)], quiero) {
			var faltan []string
			for _, q := range quiero {
				if !slices.Contains(viajan, q) {
					faltan = append(faltan, q)
				}
			}
			t.Errorf("%s devolvió %d huecos y su dominio declara %d, enteros, en su orden y adelante: faltan %q. "+
				"El que no viaja es el límite que quien lee no va a saber que no se miró",
				tool, len(viajan), len(quiero), faltan)
		}
		medidas++
	}
	if medidas < len(dominio) {
		t.Fatalf("se midieron %d tools de %d: la derivación no encontró las que tienen huecos decididos", medidas, len(dominio))
	}
}
