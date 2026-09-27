package memory

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

// A131 · T4 — LA PODA DEL ESTADO DE LAS POLÍTICAS CONSERVA EXACTAMENTE LAS VIVAS.
//
// PodarEstadoDePoliticas borra las filas de las políticas que ya no están configuradas y, con la lista
// VACÍA, no borra nada: «no hay políticas» es también un YAML a medio editar, y borrar el historial de
// cooldowns por eso sería irreversible. La guarda que lo fijaba
// (TestPodarElEstadoDePoliticasConListaVaciaNoBorraNada, en internal/mcp) clavaba dos ejes:
//
//   - LA FORMA DE LA LISTA VACÍA: la pasaba como `nil`. Cambiar `len(vivas) == 0` por `vivas == nil`
//     (P3-m4) la dejaba en verde, y con `[]string{}` —vacía y no nil, la forma que arma
//     `make([]string, 0, n)`— el almacén arma `NOT IN ()`, que SQLite acepta, y BORRA LA TABLA
//     ENTERA (medido: 1 fila, err=nil).
//   - CUÁNTAS VIVAS HAY: su control positivo traía una. Ligar sólo la primera (P3-m5: `marcas := "?"`,
//     `range vivas[:1]`) también la dejaba en verde, y con dos o más políticas cada poda horaria se
//     habría llevado el cooldown de todas menos la primera.
//
// Acá se recorren los dos ejes enteros, con los hechos escritos en cada caso —qué políticas conservan
// su estado— y contra la tabla que queda, no contra el número que devuelve la poda. Las formas de la
// lista vacía son TODAS las que Go puede producir sin `unsafe` (nil; vacía sin capacidad; vacía con
// capacidad), y un PISO las exige clasificando el valor, no el nombre del caso. Otro PISO exige un
// caso con dos o más vivas donde una que no es la primera tiene estado.
//
// Exposición medida (auditoría A131): 0. Hay una sola política configurada, así que el único llamador
// de producción (podarEstadoDePoliticasSiToca) pasa una lista de un elemento, y además corta antes con
// cero políticas; fleet_policy_state tiene 0 filas. El defecto vive en la API exportada del almacén.
//
// Sabotaje: preguntar si la lista es nil en vez de si está vacía (P3-m4). El arreglo es la misma
// pregunta por el largo escrita de otra forma.
// arnes: archivo="internal/memory/comandos.go"
// arnes: de="if len(vivas) == 0 {"
// arnes: a="if vivas == nil {"
// arnes: arreglo_de="if len(vivas) == 0 {"
// arnes: arreglo_a="if len(vivas) < 1 {"
// arnes: colision_ok="TestPodarElEstadoDePoliticasConListaVaciaNoBorraNada"
//
// Sabotaje: ligar sólo la primera política viva (P3-m5). El arreglo arma los mismos marcadores de
// otra forma: la guarda mide qué filas quedan, no cómo se escribe el IN.
// arnes: archivo="internal/memory/comandos.go"
// arnes: de="\tmarcas := strings.TrimSuffix(strings.Repeat(\"?,\", len(vivas)), \",\")\n\targs := make([]interface{}, 0, len(vivas))\n\tfor _, v := range vivas {\n"
// arnes: a="\tmarcas := \"?\"\n\targs := make([]interface{}, 0, len(vivas))\n\tfor _, v := range vivas[:1] {\n"
// arnes: arreglo_de="\tmarcas := strings.TrimSuffix(strings.Repeat(\"?,\", len(vivas)), \",\")\n"
// arnes: arreglo_a="\tmarcas := strings.Repeat(\",?\", len(vivas))[1:]\n"
func TestLaPodaDelEstadoDePoliticasConservaExactamenteLasVivas(t *testing.T) {
	sembradas := []struct{ politica, device, alcance string }{
		{"vaciar-journal", "dev-1", ""},
		{"vaciar-journal", "dev-2", ""},
		{"revivir-web", "dev-1", "nginx"},
		{"revivir-db", "dev-1", "postgres"},
		{"borrada-del-archivo", "dev-1", ""},
	}
	todas := []string{"borrada-del-archivo", "revivir-db", "revivir-web", "vaciar-journal"}
	casos := []struct {
		caso   string
		vivas  []string
		quedan []string // HECHO: las políticas que conservan su estado después de podar
	}{
		{"la lista nil", nil, todas},
		{"la lista vacía que no es nil", []string{}, todas},
		{"la lista vacía con capacidad, la forma que arma make", make([]string, 0, 4), todas},
		{"una viva", []string{"vaciar-journal"}, []string{"vaciar-journal"}},
		{"dos vivas: la segunda conserva lo suyo", []string{"vaciar-journal", "revivir-web"}, []string{"revivir-web", "vaciar-journal"}},
		{"tres vivas: se va sólo la huérfana", []string{"vaciar-journal", "revivir-web", "revivir-db"}, []string{"revivir-db", "revivir-web", "vaciar-journal"}},
		{"el orden de las vivas no importa", []string{"revivir-db", "revivir-web", "vaciar-journal"}, []string{"revivir-db", "revivir-web", "vaciar-journal"}},
		{"una viva que todavía no disparó nunca", []string{"nueva"}, nil},
	}

	// PISO: cada forma de la lista vacía, clasificada mirando el VALOR. Son todas las que Go produce sin
	// `unsafe`: sin ellas, preguntar por nil en vez de por el largo pasa por esta tabla sin verse.
	formas := map[string]bool{}
	for _, c := range casos {
		if len(c.vivas) > 0 {
			continue
		}
		switch {
		case c.vivas == nil:
			formas["nil"] = true
		case cap(c.vivas) == 0:
			formas["vacía sin capacidad"] = true
		default:
			formas["vacía con capacidad"] = true
		}
	}
	// PISO: dos o más vivas, y una que NO es la primera con estado sembrado. Sin eso, ligar sólo la
	// primera no se distingue de ligarlas todas.
	conEstado := map[string]bool{}
	for _, s := range sembradas {
		conEstado[s.politica] = true
	}
	variasVivas := false
	for _, c := range casos {
		for _, v := range c.vivas[min(1, len(c.vivas)):] {
			variasVivas = variasVivas || conEstado[v]
		}
	}
	if len(formas) != 3 || !variasVivas {
		t.Fatalf("PISO: formas de la lista vacía = %v (son tres) y un caso con una viva no primera que tenga estado = %v",
			formas, variasVivas)
	}

	for _, c := range casos {
		t.Run(c.caso, func(t *testing.T) {
			e := newTestEngine(t)
			ahora := time.Now().UTC()
			for _, s := range sembradas {
				if err := e.MarcarDisparoDePolitica(s.politica, s.device, s.alcance, ahora); err != nil {
					t.Fatalf("sembrar %+v: %v", s, err)
				}
			}
			n, err := e.PodarEstadoDePoliticas(c.vivas)
			if err != nil {
				t.Fatalf("PodarEstadoDePoliticas(%#v): %v", c.vivas, err)
			}
			cds, err := e.CooldownsDePoliticas()
			if err != nil {
				t.Fatal(err)
			}
			var quedan []string
			filas := 0
			for politica, porDevice := range cds {
				quedan = append(quedan, politica)
				filas += len(porDevice)
			}
			sort.Strings(quedan)
			quieren := append([]string(nil), c.quedan...)
			sort.Strings(quieren)
			if !reflect.DeepEqual(quedan, quieren) {
				t.Fatalf("con vivas = %#v (len %d, cap %d) conservan estado %v y tenían que ser %v. Lo que se va de más es "+
					"el cooldown de una política configurada —la próxima vez que el cerebro arranque, actúa antes de "+
					"tiempo—; con la lista vacía, es el historial entero por un YAML a medio editar",
					c.vivas, len(c.vivas), cap(c.vivas), quedan, quieren)
			}
			if int(n) != len(sembradas)-filas {
				t.Errorf("la poda dice que borró %d fila/s y en la tabla faltan %d", n, len(sembradas)-filas)
			}
		})
	}
}
