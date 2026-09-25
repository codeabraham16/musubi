package mcp

// fleet_ventana_tope_test.go custodia la ventana de las tools de fase 5 en su TOPE, donde vivía
// un defecto que ninguna guarda miraba (auditoría A131, tema T8, C2-vivo1).
//
// Las guardas de la ventana medían VentanaHasta SUELTA, con Duracion(). En la tool la ventana no
// viaja suelta: ventanaDeArgs la arma, la tool la normaliza y el motor la valida. Normalizar
// redondea hacia afuera, así que la ventana del máximo exacto salía con 720h0m1s y el motor la
// rechazaba: `horas: 720` y `horas: 1000` contestaban -32603 en musubi_fleet_cronologia y en
// musubi_fleet_contexto. Y del otro lado de la conversión, `horas` enormes desbordaban el int64 y
// devolvían la ventana DEFAULT de 24 h en vez del máximo, sin error. Pedir TODO era, según cuánto
// se pidiera, un error o un día.
//
// EXPOSICIÓN medida por la auditoría: 18 llamadas a estas dos tools desde 2026-08-30 (cronologia:
// 10 ok, 4 con error; contexto: 4 ok). Los 4 errores NO se pueden atribuir: tool_invocations no
// guarda argumentos ni el texto del error, y el journal legible empieza el 2026-09-23, después del
// último. Leer ese «0 rechazos visibles» como «nadie lo pisó» sería el cero equivocado.

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// toolsDeVentana DERIVA del catálogo qué tools aceptan una ventana: las que declaran `desde`,
// `hasta` y `horas`. No es una lista escrita a mano: una tool nueva de fase 5 que tome ventana
// entra sola en la prueba de abajo.
func toolsDeVentana(s *McpServer) []string {
	var out []string
	for _, te := range s.tools {
		p := te.InputSchema.Properties
		_, desde := p["desde"]
		_, hasta := p["hasta"]
		_, horas := p["horas"]
		if desde && hasta && horas {
			out = append(out, te.Name)
		}
	}
	sort.Strings(out)
	return out
}

// ventanaDeRespuesta lee la ventana que la tool DICE haber aplicado.
func ventanaDeRespuesta(t *testing.T, res interface{}) fleet.Ventana {
	t.Helper()
	v, ok := jsonOf(t, res)["ventana"].(map[string]any)
	if !ok {
		t.Fatalf("la respuesta no trae `ventana`: %s", textOf(t, res))
	}
	desde, err1 := time.Parse(time.RFC3339, fmt.Sprint(v["desde"]))
	hasta, err2 := time.Parse(time.RFC3339, fmt.Sprint(v["hasta"]))
	if err1 != nil || err2 != nil {
		t.Fatalf("la ventana devuelta no es RFC3339: %v (%v, %v)", v, err1, err2)
	}
	return fleet.Ventana{Desde: desde, Hasta: hasta}
}

// PEDIR EL MÁXIMO, O MÁS, DA EL MÁXIMO EN TODA TOOL QUE TOMA UNA VENTANA — por `horas` y por
// `desde`/`hasta` con fracción—, y un pedido que se pasa de verdad se rechaza como ARGUMENTO
// inválido (-32602), no como falla interna (-32603).
//
// Es la prueba del camino real que las guardas de fleet no podían ver: argumentos JSON → ventanaDeArgs
// → Normalizada → CronologiaDeDevice/ObservacionesEnVentana, que validan otra vez.
//
// Sabotaje: que el recorte al tope de Normalizada deje la ventana un segundo por encima → el motor
// la rechaza y la tool contesta -32603 a quien pidió el máximo.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\t\tdesde = hasta.Add(-VentanaMax)\n"
// arnes: a="\t\tdesde = hasta.Add(-VentanaMax - time.Second)\n"
func TestLasToolsDeVentanaDanElMaximoCuandoSePideDeMas(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	p := conCaps("infra", map[fleet.Cap][]string{fleet.CapExec: {"*"}})

	tools := toolsDeVentana(s)
	// EL PISO: si la derivación no encontrara nada, esta prueba pasaría sin llamar a ninguna tool.
	if !strings.Contains(strings.Join(tools, " "), "musubi_fleet_cronologia") {
		t.Fatalf("la derivación de tools con ventana no encontró musubi_fleet_cronologia (encontró %v): "+
			"la prueba no estaría midiendo nada", tools)
	}

	maxHs := fleet.VentanaMax.Hours()
	// Un `desde` CON FRACCIÓN a exactamente el máximo del `hasta`: pasa Valida suelta y, sin el
	// recorte, sale de Normalizada con un segundo de más.
	desdeFrac := time.Date(2026, 8, 1, 0, 0, 0, 500_000_000, time.UTC)
	explicitaAlTope := map[string]any{
		"desde": desdeFrac.Format(time.RFC3339Nano),
		"hasta": desdeFrac.Add(fleet.VentanaMax).Format(time.RFC3339Nano),
	}

	for _, tool := range tools {
		pedidos := []struct {
			nombre string
			args   map[string]any
		}{
			{"horas = el máximo", map[string]any{"horas": maxHs}},
			{"horas = el máximo + 1", map[string]any{"horas": maxHs + 1}},
			{"horas = 1000", map[string]any{"horas": 1000}},
			{"horas = 1e7 (desborda el int64 al convertir)", map[string]any{"horas": 1e7}},
			{"horas = 1e300", map[string]any{"horas": 1e300}},
			{"desde/hasta a exactamente el máximo, con fracción", explicitaAlTope},
		}
		for _, pedido := range pedidos {
			args := map[string]any{"device": "pc-gio"}
			for k, v := range pedido.args {
				args[k] = v
			}
			res, e := callAsPrincipal(t, s, p, tool, args)
			if e != nil {
				t.Errorf("%s con %s devolvió un error en vez de la ventana máxima: %+v", tool, pedido.nombre, e)
				continue
			}
			if d := ventanaDeRespuesta(t, res).Duracion(); d != fleet.VentanaMax {
				t.Errorf("%s con %s aplicó una ventana de %s: pedir el máximo o más tiene que dar el máximo, %s",
					tool, pedido.nombre, d, fleet.VentanaMax)
			}
		}

		// CONTROL NEGATIVO: un segundo más que el máximo, explícito, es un argumento inválido. Sin
		// esto, una tool que recortara cualquier cosa al máximo pasaría todo lo de arriba.
		deMas := map[string]any{
			"device": "pc-gio",
			"desde":  "2026-08-01T00:00:00Z",
			"hasta":  time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC).Add(fleet.VentanaMax + time.Second).Format(time.RFC3339),
		}
		if _, e := callAsPrincipal(t, s, p, tool, deMas); e == nil || e.Code != codeInvalidParams {
			t.Errorf("%s con una ventana explícita de un segundo más que el máximo devolvió %+v: tiene que "+
				"rechazarse como argumento inválido (%d), no aceptarse ni fallar adentro", tool, e, codeInvalidParams)
		}

		// CONTROL POSITIVO: lo chico se respeta — el recorte no se come ventanas que no están en el tope.
		res, e := callAsPrincipal(t, s, p, tool, map[string]any{"device": "pc-gio", "horas": 24})
		if e != nil {
			t.Fatalf("control: %s con `horas: 24` falla: %+v", tool, e)
		}
		if d := ventanaDeRespuesta(t, res).Duracion(); d < 24*time.Hour || d > 24*time.Hour+time.Second {
			t.Errorf("control: %s con `horas: 24` aplicó %s; lo normal es 24 h más, a lo sumo, un segundo de redondeo", tool, d)
		}
	}
}

// `horas` POSITIVO DA UNA DURACIÓN POSITIVA, CRECIENTE Y ACOTADA AL MÁXIMO, en todo su dominio.
//
// La conversión `time.Duration(horas * float64(time.Hour))` rompía los dos extremos sin error: más
// de ~2,56 millones de horas desbordan el int64 —en amd64 da MinInt64— y menos de un nanosegundo
// trunca a 0; en los dos casos VentanaHasta lo leía como «sin duración» y devolvía 24 h. Medido en
// esta máquina: 1e7 → -2562047h47m16.85s, 1e-13 → 0s.
//
// La tabla está ORDENADA y la prueba exige que la duración no decrezca: así cualquier borde que
// vuelva a caer al default aparece como un escalón para abajo, sin tener que nombrarlo.
//
// Sabotaje: sacar el tope previo a la conversión → `horas: 1e7` desborda y da 24 h.
// arnes: archivo="internal/mcp/methods_cronologia.go"
// arnes: de="\tif horas >= fleet.VentanaMax.Hours() {\n\t\treturn fleet.VentanaMax\n\t}"
// arnes: a="\tif false {\n\t\treturn fleet.VentanaMax\n\t}"
// Sabotaje: devolver la conversión cruda → `horas: 1e-13` trunca a 0 y da 24 h.
// arnes: archivo="internal/mcp/methods_cronologia.go"
// arnes: de="\tif d := time.Duration(horas * float64(time.Hour)); d > 0 {\n\t\treturn d\n\t}\n\treturn time.Nanosecond"
// arnes: a="\treturn time.Duration(horas * float64(time.Hour))"
func TestVentanaDeArgsConvierteHorasSinDesbordar(t *testing.T) {
	ahora := time.Date(2026, 9, 24, 14, 42, 18, 700_000_000, time.UTC)
	maxHs := fleet.VentanaMax.Hours()
	horas := []float64{
		1e-13, 1e-9, 1e-3, 1, 6, 24, maxHs - 1, maxHs - 1e-6, maxHs, maxHs + 1e-6, maxHs + 1,
		1000, 2.5e6, 2.6e6, 1e7, 1e300,
	}
	if !sort.Float64sAreSorted(horas) {
		t.Fatal("la tabla tiene que estar ordenada: la prueba mide que la duración no decrezca")
	}
	var anterior time.Duration
	for i, h := range horas {
		v, e := ventanaDeArgs("", "", h, ahora)
		if e != nil {
			t.Errorf("horas=%g devolvió un error: %+v", h, e)
			continue
		}
		d := v.Duracion()
		if d <= 0 || d > fleet.VentanaMax {
			t.Errorf("horas=%g dio una ventana de %s: tiene que ser positiva y a lo sumo %s", h, d, fleet.VentanaMax)
		}
		if h >= maxHs && d != fleet.VentanaMax {
			t.Errorf("horas=%g dio %s: pedir el máximo o más tiene que dar el máximo, no el default ni otra cosa", h, d)
		}
		if i > 0 && d < anterior {
			t.Errorf("horas=%g dio %s, MENOS que horas=%g (%s): pedir más devolvió menos", h, d, horas[i-1], anterior)
		}
		if err := v.Normalizada().Valida(); err != nil {
			t.Errorf("horas=%g armó una ventana que normalizada no pasa Valida (%v): la tool contestaría -32603", h, err)
		}
		anterior = d
	}
	// Dos hechos del mundo, para que «creciente y acotada» no se satisfaga con una constante.
	for _, c := range []struct {
		horas float64
		d     time.Duration
	}{{6, 6 * time.Hour}, {24, 24 * time.Hour}} {
		if v, _ := ventanaDeArgs("", "", c.horas, ahora); v.Duracion() != c.d {
			t.Errorf("horas=%g dio %s, esperaba %s", c.horas, v.Duracion(), c.d)
		}
	}
}
