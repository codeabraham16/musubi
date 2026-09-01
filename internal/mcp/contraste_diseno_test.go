package mcp

import (
	"sort"
	"strings"
	"testing"
)

// C-CON1 · TODA FORMA TIENE PERFIL, Y NO HAY DOS IGUALES.
//
// Dos formas con el mismo perfil son la misma propuesta con dos nombres, y servirlas juntas es
// exactamente el defecto que esta capa vino a arreglar. Y una forma SIN perfil puntúa 0 en todo, así
// que nunca gana un contraste: quedaría muerta en el pozo sin que nada se ponga rojo.
func TestContrasteTodaFormaTienePerfilYNoSeRepite(t *testing.T) {
	vistos := map[[dimsTotal]int]string{}
	for clave := range formasDeDiseno {
		p, ok := perfilDeForma[clave]
		if !ok {
			t.Errorf("la forma %q no tiene perfil: puntúa 0 en todo y nunca la elegiría el contraste", clave)
			continue
		}
		if otra, repe := vistos[p]; repe {
			t.Errorf("%q y %q tienen el MISMO perfil %v: son la misma propuesta con dos nombres", clave, otra, p)
		}
		vistos[p] = clave
	}
	for clave := range perfilDeForma {
		if _, ok := formasDeDiseno[clave]; !ok {
			t.Errorf("hay perfil para %q y esa forma no existe en el catálogo", clave)
		}
	}
}

// C-CON2 · LO QUE EL BRIEF DICE QUE GANA, LO GANA.
//
// El bloque estampa «[gana en X]» al lado de cada forma. Si esa etiqueta no sale del perfil, el brief
// afirma que una forma gana en algo donde no gana — y el agente elige con esa afirmación. Se verifica
// contra el TEXTO EMITIDO, no contra `destacaDe`: comparar la salida con la función que la produce es
// el control que mide el proxy, y ya me pasó una vez en esta misma tanda (C-ESC4).
func TestContrasteLaEtiquetaSaleDelPerfil(t *testing.T) {
	txt := formasPara("tabla", nil, intencionDeDiseno{})
	if txt == "" {
		t.Fatal("el eje 'tabla' no emitió bloque de forma")
	}
	hubo := 0
	for clave, f := range formasDeDiseno {
		i := strings.Index(txt, f.Nombre+" [gana en ")
		if i < 0 {
			continue
		}
		hubo++
		resto := txt[i+len(f.Nombre)+len(" [gana en "):]
		etiqueta := resto[:strings.Index(resto, "]")]

		// El máximo se recalcula ACÁ, a mano, desde el perfil crudo.
		p := perfilDeForma[clave]
		max := 0
		for _, v := range p {
			if v > max {
				max = v
			}
		}
		for d := 0; d < dimsTotal; d++ {
			dice := strings.Contains(etiqueta, nombreDim[d])
			if dice && p[d] != max {
				t.Errorf("%s: el brief dice que gana en %q y ahí puntúa %d, no su máximo %d",
					f.Nombre, nombreDim[d], p[d], max)
			}
			if !dice && p[d] == max && max > 0 {
				t.Errorf("%s: puntúa el máximo (%d) en %q y el brief no lo dice", f.Nombre, max, nombreDim[d])
			}
		}
	}
	if hubo == 0 {
		t.Fatal("ninguna forma trajo etiqueta «[gana en …]»: el bloque quedó mudo justo en lo que lo distingue")
	}
}

// C-CON3 · DOS RECLAMOS DISTINTOS NO PUEDEN RECIBIR LO MISMO.
//
// Es EL invariante del pedido del usuario: *«no pueden ser genéricos ni siempre los mismos, porque
// esos 3 dependen de cómo está el diseño actual y de a dónde quiere apuntar el nuevo»*. Con el mapa
// fijo anterior, «esta tabla no se puede comparar» y «esta tabla no me dice qué hacer» recibían las
// MISMAS tres candidatas — el motor no tenía por dónde enterarse de que son problemas distintos.
func TestContrasteReclamosDistintosDanPropuestasDistintas(t *testing.T) {
	base := "hoy es una tabla densa de inventario"
	casos := map[string]string{
		"comparar":    "no se puede comparar nada entre lotes",
		"decidir":     "no me dice qué hacer, necesito ver la urgencia de un vistazo",
		"profundidad": "no puedo entrar al detalle sin salir de la pantalla",
		"guia":        "hay que acompañar paso a paso al que carga",
	}
	vistos := map[string]string{}
	for nombre, cambio := range casos {
		got := strings.Join(candidatasDeForma("tabla", nil, intencionDeDiseno{Keep: base, Change: cambio}), "|")
		if got == "" {
			t.Fatalf("%s: no se propuso ninguna forma", nombre)
		}
		if otro, repe := vistos[got]; repe {
			t.Errorf("«%s» y «%s» reciben LAS MISMAS tres candidatas (%s): el motor no distingue los dos reclamos",
				nombre, otro, got)
		}
		vistos[got] = nombre
	}
}

// C-CON4 · LO QUE SE PIDE GANAR, SE GANA.
//
// Este invariante nació de un error medido: la primera versión sólo maximizaba distancia y al pedido
// «esta tabla no se puede comparar» le contestaba con «detalle con lados», que tiene comparación 0.
// La distancia premia igual subir que bajar, y una tabla ya está arriba, así que lo más lejano era lo
// PEOR en lo que se estaba pidiendo. La primera candidata tiene que estar entre las mejores del pozo
// en la dimensión reclamada.
func TestContrasteLaPrimeraGanaEnLoPedido(t *testing.T) {
	for _, c := range []struct {
		eje, cambio string
		dim         int
	}{
		{"tabla", "no se puede comparar nada entre lotes", dimComparacion},
		{"tabla", "no me dice qué hacer, es urgente", dimDecision},
		{"tabla", "quiero entrar al detalle sin salir", dimProfundidad},
		{"dashboard", "está todo muy vacío, no cabe nada, quiero más densidad", dimDensidad},
	} {
		cands := candidatasDeForma(c.eje, nil, intencionDeDiseno{Change: c.cambio})
		if len(cands) == 0 {
			t.Fatalf("%s / %q: sin candidatas", c.eje, c.cambio)
		}
		// El techo se calcula sobre el POZO, no sobre lo elegido: comparar lo elegido contra sí mismo
		// pasaría verde siempre.
		techo := 0
		for _, f := range formasPorEje[c.eje] {
			if v := perfilDeForma[f][c.dim]; v > techo {
				techo = v
			}
		}
		if got := perfilDeForma[cands[0]][c.dim]; got < techo {
			t.Errorf("%s / %q: la primera candidata es %q con %s=%d, y en el pozo hay %d — se propuso lo lejano en vez de lo bueno",
				c.eje, c.cambio, cands[0], nombreDim[c.dim], got, techo)
		}
	}
}

// C-CON5 · NO SE PROPONE COMO NOVEDAD LA FORMA DE LA QUE SE VIENE.
//
// Si el pedido dice «hoy es una tabla densa y quiero cambiarla», ofrecer «tabla densa» entre las tres
// es contestar otra pregunta. El origen sale de `keep` con la misma lectura tolerante que la rotación.
func TestContrasteElOrigenNoVuelveComoPropuesta(t *testing.T) {
	for _, keep := range []string{
		"hoy es una tabla densa y funciona pero cansa",
		"la pantalla actual es un catálogo para elegir",
		"hoy es un lienzo con inspector, la identidad se mantiene",
	} {
		origen := formaMencionada(keep)
		if origen == "" {
			t.Fatalf("no se reconoció ninguna forma en %q — la premisa del caso no se cumple", keep)
		}
		for _, eje := range []string{"tabla", "filtros", "navegacion", "densidad"} {
			for _, c := range candidatasDeForma(eje, nil, intencionDeDiseno{Keep: keep, Change: "cambiar el modelo"}) {
				if c == origen {
					t.Errorf("eje %s: se propone %q, que es justo de donde se viene (%q)", eje, c, keep)
				}
			}
		}
	}
}

// C-CON6 · EL BRIEF ES UNA FUNCIÓN DE SUS ENTRADAS.
//
// Las mismas entradas tienen que dar el mismo brief, siempre. El riesgo concreto es iterar un mapa de
// Go —cuyo orden es aleatorio por diseño— en vez del pozo ordenado: el bug saldría una corrida de cada
// tantas y sería casi imposible de reproducir a mano.
func TestContrasteEsDeterminista(t *testing.T) {
	// EL TEXTO TIENE QUE SER AMBIGUO O EL TEST NO PRUEBA NADA. La primera versión usaba «hoy es una
	// tabla densa», donde una sola forma del catálogo aparece: con un único candidato el orden del
	// mapa no puede cambiar el resultado, así que el sabotaje de iterar el mapa pasó VERDE. El caso
	// tiene que nombrar DOS formas para que el orden decida cuál gana.
	ambiguo := "hoy es una tabla densa con un lienzo con inspector al costado"
	var matchean []string
	for clave, f := range formasDeDiseno {
		if strings.Contains(ambiguo, clave) || strings.Contains(ambiguo, strings.ToLower(f.Nombre)) {
			matchean = append(matchean, clave)
		}
	}
	if len(matchean) < 2 {
		t.Fatalf("la premisa del caso no se cumple: %q nombra %v, y con menos de dos formas el orden no decide nada",
			ambiguo, matchean)
	}

	in := intencionDeDiseno{Keep: ambiguo, Change: "cambiar el modelo y las cuadriculas"}
	primero := strings.Join(candidatasDeForma("tabla", nil, in), "|")
	origen := formaMencionada(ambiguo)
	for i := 0; i < 300; i++ {
		if got := strings.Join(candidatasDeForma("tabla", nil, in), "|"); got != primero {
			t.Fatalf("corrida %d dio %q y la primera dio %q", i, got, primero)
		}
		if got := formaMencionada(ambiguo); got != origen {
			t.Fatalf("corrida %d: formaMencionada dio %q y la primera dio %q — el desempate no es estable", i, got, origen)
		}
	}
}

// C-CON7 · LO QUE SE PIDE CONSERVAR NO SE PIERDE.
//
// Éste es el defecto que originó el campo: la consulta se recorta a `designConsultaFrases` oraciones
// y `designConsultaMax` chars antes de buscar en el acervo, así que en un pedido de rediseño la mitad
// de «conservar» —que viene después de las dos primeras oraciones— se caía. `keep` viaja aparte y
// entero, y su bloque tiene que decirlo textual.
func TestContrasteLoQueSeConservaLlegaEntero(t *testing.T) {
	keep := "mantener la esencia de las notas, la voz del producto y el ámbar de la marca. " +
		strings.Repeat("Además el encabezado y el pie no se tocan. ", 12) +
		"Y sobre todo NO se pierde el buscador por lote."
	b := bloqueDeConservacion(keep)
	if b == "" {
		t.Fatal("se pidió conservar algo y el bloque salió vacío")
	}
	if !strings.Contains(b, keep) {
		t.Error("el bloque no lleva el texto de `keep` entero")
	}
	// La cola es lo que se caía del recorte de la consulta: si sobrevive eso, sobrevive todo.
	if !strings.Contains(b, "NO se pierde el buscador por lote") {
		t.Error("se perdió el final de `keep`, que es justo la parte que el recorte de la consulta se comía")
	}
	// Y sin pedido de conservación no se estampa un encabezado vacío.
	if bloqueDeConservacion("   ") != "" {
		t.Error("sin nada que conservar el bloque tiene que ser vacío, no un encabezado suelto")
	}
}

// C-CON8 · «NO DIJO NADA» NO ES «DIJO TODO».
//
// Los dos casos devuelven las seis dimensiones, pero significan cosas opuestas y el brief no puede
// confundirlos: afirmar que el usuario pidió replantear el esqueleto cuando no pidió nada le pone en
// la boca una decisión que no tomó.
func TestContrasteNoDecirNadaNoEsDecirTodo(t *testing.T) {
	dimsVacio, _, explVacio := dimensionesAMover("")
	dimsTodo, _, explTodo := dimensionesAMover("replantear el modelo de cero")
	if len(dimsVacio) != dimsTotal || len(dimsTodo) != dimsTotal {
		t.Fatalf("los dos casos deberían dar las seis dimensiones: %d y %d", len(dimsVacio), len(dimsTodo))
	}
	if explVacio {
		t.Error("un pedido vacío se declaró como explícito")
	}
	if !explTodo {
		t.Error("«replantear el modelo de cero» no se declaró como explícito")
	}
	sinNada := formasPara("tabla", nil, intencionDeDiseno{})
	conTodo := formasPara("tabla", nil, intencionDeDiseno{Change: "replantear el modelo de cero"})
	if !strings.Contains(sinNada, "no nombró ninguna dimensión") {
		t.Error("sin intención, la nota no declara que el motor no leyó nada")
	}
	if strings.Contains(conTodo, "no nombró ninguna dimensión") {
		t.Error("con un pedido estructural, la nota dice que no se nombró nada")
	}
}

// C-CON9 · LA MISMA DIMENSIÓN EN TRES DIRECCIONES DA TRES RESPUESTAS.
//
// Es el invariante de la polaridad. Nombrar una dimensión no dice hacia dónde, y la versión anterior
// trataba las tres igual: al pedido real del usuario —«cambiar modelos, cuadrículas»— entendía «quiero
// más densidad» y proponía una TABLA para una pantalla de notas.
func TestContrasteLaDireccionCambiaLaRespuesta(t *testing.T) {
	keep := "hoy es un catálogo para elegir"
	primera := func(cambio string) string {
		c := candidatasDeForma("filtros", nil, intencionDeDiseno{Keep: keep, Change: cambio})
		if len(c) == 0 {
			t.Fatalf("%q: sin candidatas", cambio)
		}
		return c[0]
	}
	ganar := primera("quiero más densidad, no cabe nada")
	ceder := primera("está demasiado denso, abruma, sobra información")
	mover := primera("cambiar modelos, cuadriculas, etc")

	if ganar == ceder {
		t.Errorf("pedir MÁS densidad y pedir MENOS densidad dan la misma primera candidata (%q)", ganar)
	}
	// Y cada una tiene que ir para su lado en el perfil, no sólo diferir.
	if pg, pc := perfilDeForma[ganar][dimDensidad], perfilDeForma[ceder][dimDensidad]; pg <= pc {
		t.Errorf("pedir MÁS densidad propuso %q (densidad %d) y pedir MENOS propuso %q (densidad %d)",
			ganar, pg, ceder, pc)
	}
	// `mover` no puede coincidir con las dos: si coincidiera, la dirección no estaría haciendo nada.
	if mover == ganar && mover == ceder {
		t.Error("«cambiá X» da lo mismo que «más X» y que «menos X»: la polaridad no se está usando")
	}
}

// C-CON10 · «CAMBIÁ X» NO ASUME DIRECCIÓN.
//
// El caso exacto que falló en vivo, con el texto del usuario. Sin dirección declarada el mérito no
// puede pesar: la elección se cae a puro contraste, que es la lectura correcta de «cambialo, no sé a
// qué». Se verifica contra el PERFIL y no contra un nombre de forma, para que el test no se rompa el
// día que se agregue una forma mejor al pozo.
func TestContrasteCambiarNoEsQuererMas(t *testing.T) {
	rs, estructural, explicito := dimensionesAMover("cambiar modelos, cuadriculas, etc")
	if !explicito {
		t.Fatal("no se reconoció ninguna dimensión: la premisa del caso no se cumple")
	}
	if !estructural {
		t.Error("«cambiar modelos» no se leyó como estructural")
	}
	if hayDireccion(rs) {
		t.Errorf("se asumió una dirección donde el pedido no la dijo: %+v", rs)
	}
	// Y el efecto: la primera candidata es la más LEJANA del origen, no la que más densidad tiene.
	origen := "catalogo-elegir"
	cands := candidatasDeForma("filtros", nil, intencionDeDiseno{
		Keep: "hoy es un catálogo para elegir", Change: "cambiar modelos, cuadriculas, etc"})
	lejos, mejorDist := "", -1
	for _, f := range formasPorEje["filtros"] {
		if f == origen {
			continue
		}
		if d := distanciaEn(f, origen, todasLasDims()); d > mejorDist {
			lejos, mejorDist = f, d
		}
	}
	if cands[0] != lejos {
		t.Errorf("sin dirección declarada la primera debería ser la más lejana del origen (%q, distancia %d) y fue %q",
			lejos, mejorDist, cands[0])
	}
}

// C-CON11 · EL VERBO GOBIERNA LO QUE LE SIGUE.
//
// «cambiar modelos, cuadrículas» son dos cláusulas y sólo la primera trae el verbo. Si cada una se
// leyera aislada, «cuadrículas» quedaría sin dirección propia y caería a un default — y el default
// que importaba era justo el equivocado. Por eso una cláusula sin marcador HEREDA la de la anterior.
func TestContrasteLaDireccionSeHereda(t *testing.T) {
	// La premisa: la segunda cláusula, SOLA, no declara dirección ninguna.
	if _, ok := polaridadDe("cuadriculas"); ok {
		t.Fatal("la premisa no se cumple: «cuadriculas» sola ya declara una dirección")
	}
	rs, _, _ := dimensionesAMover("no se puede comparar nada, y encontrar cuesta")
	for _, r := range rs {
		if r.Pol != polGanar {
			t.Errorf("%s se leyó como %v y toda la frase es un déficit", nombreDim[r.Dim], r.Pol)
		}
	}
	// Y al revés: el exceso también se hereda.
	rs2, _, _ := dimensionesAMover("sobra de todo: densidad, filas")
	if !hayDireccion(rs2) {
		t.Fatal("«sobra de todo: densidad» no declaró dirección")
	}
	for _, r := range rs2 {
		if r.Pol != polPerder {
			t.Errorf("%s se leyó como %v y la frase dice que sobra", nombreDim[r.Dim], r.Pol)
		}
	}
}

// C-CON12 · UNA MENCIÓN SIN DIRECCIÓN NO PISA A UNA QUE SÍ LA TENÍA.
//
// La gente se corrige mientras habla: «está todo vacío… quiero más densidad». La dirección explícita
// tiene que sobrevivir a una mención posterior que no diga nada.
func TestContrasteLaDireccionExplicitaNoSePisa(t *testing.T) {
	// EL CASO TIENE QUE EJERCER LA GUARDA. La primera versión usaba «quiero más densidad, y la
	// cuadricula»: la segunda cláusula no trae marcador, así que HEREDA el mismo «ganar» y sobrescribir
	// no cambia nada — el sabotaje pasó verde. Hace falta que en el medio aparezca OTRA dirección, para
	// que lo heredado por la mención muda sea distinto de lo que la dimensión ya tenía declarado.
	const pedido = "quiero más densidad. Cambiá el modelo, la cuadricula"
	if _, ok := polaridadDe("la cuadricula"); ok {
		t.Fatal("la premisa no se cumple: «la cuadricula» sola ya declara una dirección")
	}
	if _, ok := polaridadDe("cambiá el modelo"); !ok {
		t.Fatal("la premisa no se cumple: «cambiá el modelo» tiene que declarar una dirección para pisar")
	}

	rs, _, _ := dimensionesAMover(pedido)
	if !hayDireccion(rs) {
		t.Fatal("se perdió la dirección: «quiero más densidad» la declaraba")
	}
	for _, r := range rs {
		if r.Dim == dimDensidad && r.Pol != polGanar {
			t.Errorf("la densidad quedó en %v: una mención posterior SIN dirección propia le pisó el «quiero más» que sí la tenía", r.Pol)
		}
	}
}

// pedidosDeBanco son doce reclamos que barren las siete dimensiones en sus tres direcciones. Vive acá
// y no en producción: es el instrumento con el que se mide si un pozo puede contrastar.
var pedidosDeBanco = []string{
	"", "cambiar el modelo",
	"no se puede comparar nada", "está demasiado denso, sobra información",
	"no me dice qué hacer, es urgente", "quiero entrar al detalle sin salir",
	"hay que acompañar paso a paso", "está aburrido, no tiene carácter",
	"quiero ver qué está corriendo ahora", "quiero más densidad, no cabe nada",
	"sobra guía, abruma", "no se ve el progreso en vivo",
}

// C-CON13 · UN POZO TIENE QUE PODER DAR MÁS DE UN CONJUNTO.
//
// Es aritmética, no gusto: con un pozo de 4, sacar el origen deja 3 y hay que elegir 3 — existe UN
// SOLO conjunto posible. El contraste sólo puede cambiar el orden y las tres propuestas son siempre
// las mismas, que es exactamente lo que el usuario objetó. Medido antes del arreglo: chat, login,
// microcopy y onboarding daban 1 conjunto para los doce pedidos del banco.
func TestContrasteNingunPozoEstaForzado(t *testing.T) {
	// EL PISO SE DERIVA, no se elige. Con un pozo de n, sacar el origen deja n−1 y hay que elegir
	// `designFormasPropuestas`: el conjunto queda determinado en cuanto n−1 ≤ propuestas. Así que el
	// mínimo que todavía deja elegir es propuestas+2, y eso se verifica acá — bajar la constante sola
	// no rompe nada hoy (ningún pozo está en el límite), así que sin esta comprobación el número
	// quedaría suelto y el próximo pozo chico entraría sin que nada avise.
	if designPozoMinimo-1 <= designFormasPropuestas {
		t.Errorf("designPozoMinimo=%d deja %d formas tras sacar el origen y se proponen %d: el conjunto queda forzado",
			designPozoMinimo, designPozoMinimo-1, designFormasPropuestas)
	}
	if designPozoMinimo-2 > designFormasPropuestas {
		t.Errorf("designPozoMinimo=%d es más alto de lo que hace falta: con %d ya se podía elegir",
			designPozoMinimo, designPozoMinimo-1)
	}

	// UNA FORMA QUE NUNCA GANA INFLA EL POZO SIN DAR VARIEDAD. Se midió y con este catálogo NO PASA:
	// ninguna de las doce formas está dominada por otra en las siete dimensiones, así que todas ganan
	// al menos uno de los doce pedidos del banco. Por eso no hay un invariante aparte para eso: un
	// test que no puede fallar es ruido, y el sabotaje —meterle cuatro formas de relleno al eje
	// `estado`— pasó VERDE justo porque las cuatro sí ganaban algo.
	for eje, pozo := range formasPorEje {
		if len(pozo) < designPozoMinimo {
			t.Errorf("el pozo de %q tiene %d formas y el piso es %d: sacando el origen queda un único conjunto posible",
				eje, len(pozo), designPozoMinimo)
		}
		// Y se verifica el EFECTO, no sólo el tamaño: el peor caso es rediseñar la forma más típica
		// del eje, que es la primera del pozo.
		origen := "hoy es un " + formasDeDiseno[pozo[0]].Nombre
		sets := map[string]bool{}
		for _, p := range pedidosDeBanco {
			c := candidatasDeForma(eje, nil, intencionDeDiseno{Keep: origen, Change: p})
			if len(c) == 0 {
				t.Fatalf("%s: el eje no propuso ninguna forma", eje)
			}
			orden := append([]string(nil), c...)
			sort.Strings(orden)
			sets[strings.Join(orden, "|")] = true
		}
		if len(sets) < 2 {
			t.Errorf("%s: los doce pedidos del banco reciben SIEMPRE el mismo conjunto de tres", eje)
		}
	}
}
