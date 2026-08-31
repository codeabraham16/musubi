package mcp

import (
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
