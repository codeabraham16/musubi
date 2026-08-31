package mcp

// contraste_diseno.go — LAS TRES CANDIDATAS SE ELIGEN, NO SE BUSCAN EN UNA TABLA.
//
// POR QUÉ EXISTE. La capa de forma servía las candidatas con `formasPorEje`, un mapa fijo de eje →
// exactamente tres formas. Para `tabla` salían siempre las mismas tres, y la rotación sólo excluía la
// del pedido anterior. El usuario lo cortó de raíz el 2026-08-31:
//
//	«esos 3 modelos o bocetos que quiero que vea son los primordiales, cada uno tiene que destacar en
//	 algo, y tampoco pueden ser genéricos ni siempre los mismos — porque esos 3 dependen de cómo está
//	 el diseño actual y de a dónde quiere apuntar el nuevo.»
//
// Tiene razón, y el defecto es medible: con el mapa fijo, dos pedidos opuestos —«esta tabla no se
// puede comparar» y «esta tabla no me dice qué hacer»— recibían LAS MISMAS tres candidatas. El motor
// no tenía por dónde enterarse de que son problemas distintos.
//
// EL CAMBIO DE MECANISMO. `formasPorEje` deja de ser la respuesta y pasa a ser el POZO de lo
// plausible (ahora de cinco a seis formas por eje, no tres). Las tres que viajan se eligen de ahí por
// DISTANCIA: lejos del punto de partida y lejos entre sí, sobre las dimensiones que el pedido dice
// querer mover. Así cada una destaca en algo distinto, y cambian cuando cambia el pedido.
//
// SIGUE SIENDO MODEL-FREE. Es una tabla de perfiles y una distancia Manhattan con selección greedy
// max-min — la misma familia que el MMR que ya usa el corpus. Ningún juicio abstracto: qué dimensión
// mover sale del vocabulario del pedido, y elegir cuál de las tres se usa lo sigue haciendo el agente.

import (
	"sort"
	"strings"
)

// Las seis dimensiones en las que una forma puede destacar. Son las que DISCRIMINAN el catálogo: se
// eligieron porque separan las doce formas, no porque suenen bien. Una dimensión en la que todas
// puntúan parecido no sirve para elegir por contraste — es peso muerto que diluye la distancia.
const (
	dimDensidad    = iota // cuántas cosas comparables muestra a la vez
	dimComparacion        // si dos elementos se pueden poner al lado y ver la diferencia
	dimDecision           // qué tan rápido te dice qué hacer
	dimProfundidad        // si podés entrar al detalle sin cambiar de pantalla
	dimGuia               // si te lleva paso a paso
	dimPresencia          // si tiene un momento visual que se recuerda
	dimsTotal
)

// nombreDim es cómo se llama cada dimensión en el brief. Va en castellano porque lo lee quien compone.
var nombreDim = [dimsTotal]string{
	dimDensidad:    "densidad",
	dimComparacion: "comparación",
	dimDecision:    "decisión",
	dimProfundidad: "profundidad",
	dimGuia:        "guía",
	dimPresencia:   "presencia",
}

// perfilDeForma puntúa cada forma de 0 a 3 en cada dimensión. Es el corazón del contraste: dos formas
// con el mismo perfil son la misma propuesta con otro nombre, y servirlas juntas es justo el defecto
// que esto viene a arreglar.
//
// Los valores salen de la definición de cada forma en `formasDeDiseno`, no de una opinión suelta: una
// tabla densa muestra muchas filas comparables (densidad 3, comparación 3) y no te dice qué hacer
// (decisión 0); un tablero de un número te lo dice de un vistazo (decisión 3, presencia 3) y no
// compara nada (0). El invariante C-CON1 verifica que no haya dos perfiles idénticos.
var perfilDeForma = map[string][dimsTotal]int{
	//                     dens comp deci prof guia pres
	"tabla-densa":       {3, 3, 0, 1, 0, 0},
	"rejilla-temporal":  {3, 3, 1, 0, 0, 1},
	"monitor-procesos":  {2, 2, 2, 1, 0, 1},
	"catalogo-elegir":   {2, 2, 1, 1, 1, 1},
	"lista-priorizada":  {1, 1, 3, 0, 1, 1},
	"tablero-un-numero": {0, 0, 3, 0, 0, 3},
	"detalle-con-lados": {1, 0, 1, 3, 0, 1},
	"lienzo-inspector":  {2, 1, 0, 3, 0, 2},
	"formulario-guiado": {0, 0, 1, 1, 3, 0},
	"narrativa":         {0, 0, 0, 2, 3, 3},
	"interrupcion":      {0, 0, 3, 0, 2, 2},
	"conversacion":      {1, 0, 1, 1, 1, 2},
}

// destacaDe dice EN QUÉ destaca una forma, y se DERIVA del perfil en vez de escribirse aparte. Es
// deliberado: una etiqueta escrita a mano se desincroniza del número la primera vez que alguien
// ajusta el perfil, y entonces el brief afirma que una forma gana en algo donde no gana. El
// invariante C-CON2 lo ata.
func destacaDe(forma string) []string {
	p, ok := perfilDeForma[forma]
	if !ok {
		return nil
	}
	max := 0
	for _, v := range p {
		if v > max {
			max = v
		}
	}
	if max == 0 {
		return nil
	}
	var out []string
	for d := 0; d < dimsTotal; d++ {
		if p[d] == max {
			out = append(out, nombreDim[d])
		}
	}
	return out
}

// vocabularioDeCambio es cómo el motor lee, del pedido, QUÉ dimensión hay que mover. Es coincidencia
// de vocabulario —el mismo mecanismo que `ejesDeTarjeta`— y no un juicio: si el pedido no nombra
// ninguna, se mueven todas y el contraste sale parejo.
var vocabularioDeCambio = [dimsTotal][]string{
	dimDensidad:    {"densidad", "denso", "compacto", "apretado", "cuadricula", "cuadrícula", "grilla", "rejilla", "filas", "cabe", "espacio desaprovechado", "aire", "vacío", "vacio"},
	dimComparacion: {"comparar", "comparacion", "comparación", "lado a lado", "ranking", "ordenar", "orden", "versus", "contra", "cuál", "cual es mejor"},
	dimDecision:    {"decidir", "decision", "decisión", "qué hacer", "que hacer", "accion", "acción", "urgente", "urgencia", "prioridad", "priorizar", "de un vistazo", "rapido", "rápido", "alerta"},
	dimProfundidad: {"detalle", "profundidad", "inspector", "expandir", "desglose", "sin salir", "entrar", "drill", "contexto al lado"},
	dimGuia:        {"guiar", "guia", "guía", "paso a paso", "onboarding", "acompañar", "acompanar", "flujo", "asistente", "wizard"},
	dimPresencia:   {"presencia", "impacto", "potente", "potencia", "fuerte", "memorable", "protagonista", "caracter", "carácter", "identidad", "aburrido", "generico", "genérico", "soso"},
}

// palabrasEstructurales son las que piden repensar el ESQUELETO entero y no una dimensión. Cuando
// aparecen, se mueven las seis: pedir «cambiar el modelo» y recibir tres variantes que sólo difieren
// en densidad sería contestar otra pregunta.
var palabrasEstructurales = []string{"modelo", "estructura", "esqueleto", "layout", "disposicion", "disposición", "replantear", "de cero", "otra cosa"}

// dimensionesAMover lee del pedido qué dimensiones hay que atacar. Devuelve también si el pedido dijo
// algo: «no dijo nada» y «dijo todo» dan el mismo conjunto pero no son lo mismo, y el brief las
// declara distinto — el motor no puede afirmar que el usuario pidió mover las seis cuando en realidad
// no pidió ninguna.
// LO QUE SE NOMBRA ES LO QUE HAY QUE GANAR, no de lo que hay que alejarse.
//
// La primera versión sólo maximizaba distancia, y salió mal en la medición: al pedido «esta tabla no
// se puede comparar» le contestaba con «detalle con lados», que tiene comparación 0. La distancia
// premia igual subir que bajar, y una tabla ya está en comparación 3, así que lo más lejano es lo
// PEOR en lo que se estaba pidiendo. Por eso las dimensiones nombradas se usan como MÉRITO —cuánto
// gana la forma en eso— y la distancia queda para que las tres no sean la misma propuesta.
//
// Y una palabra estructural («cambiar el modelo») ya no tapa a una específica que venga al lado:
// «cambiar modelos, cuadrículas» nombra la densidad, y la versión anterior cortocircuitaba en
// `modelo` y devolvía las seis, con lo que el pedido de Altura recibía exactamente lo mismo que un
// pedido sin intención ninguna. Ahora lo estructural abre el juego y lo específico sigue pesando.
func dimensionesAMover(cambio string) (dims []int, estructural, explicito bool) {
	bajo := strings.ToLower(cambio)
	if strings.TrimSpace(bajo) == "" {
		return todasLasDims(), true, false
	}
	for _, w := range palabrasEstructurales {
		if strings.Contains(bajo, w) {
			estructural = true
			break
		}
	}
	for d := 0; d < dimsTotal; d++ {
		for _, w := range vocabularioDeCambio[d] {
			if strings.Contains(bajo, w) {
				dims = append(dims, d)
				break
			}
		}
	}
	if len(dims) == 0 {
		return todasLasDims(), true, estructural
	}
	return dims, estructural, true
}

func todasLasDims() []int {
	d := make([]int, dimsTotal)
	for i := range d {
		d[i] = i
	}
	return d
}

// formaMencionada busca el nombre de una forma del catálogo dentro de un texto libre. Se usa para
// leer de dónde PARTE el rediseño (`keep` suele decir «hoy es una tabla»). Tolerante a propósito: lo
// escribe una persona o un agente, y exigir un formato exacto apagaría el mecanismo en silencio ante
// la primera frase escrita a mano — el mismo criterio que ya usa `formasUsadasPor`.
func formaMencionada(txt string) string {
	bajo := strings.ToLower(txt)
	if strings.TrimSpace(bajo) == "" {
		return ""
	}
	// Orden estable: el mapa de formas se itera sin orden, y dos claves presentes en el texto darían
	// resultados distintos entre corridas. El brief tiene que ser una función de sus entradas.
	var claves []string
	for k := range formasDeDiseno {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	for _, k := range claves {
		if strings.Contains(bajo, k) || strings.Contains(bajo, strings.ToLower(formasDeDiseno[k].Nombre)) {
			return k
		}
	}
	return ""
}

// distanciaEn mide cuánto se separan dos formas, mirando SÓLO las dimensiones que se quieren mover.
// Manhattan y no euclídea: acá interesa el total de cambio repartido, no un salto grande en una sola
// dimensión disfrazado de distancia.
func distanciaEn(a, b string, dims []int) int {
	pa, oka := perfilDeForma[a]
	pb, okb := perfilDeForma[b]
	if !oka || !okb {
		return 0
	}
	total := 0
	for _, d := range dims {
		if pa[d] > pb[d] {
			total += pa[d] - pb[d]
		} else {
			total += pb[d] - pa[d]
		}
	}
	return total
}

// elegirPorContraste saca del pozo las `n` formas que más se separan del origen y entre sí, sobre las
// dimensiones pedidas. Greedy max-min: la primera es la más lejana del origen; cada siguiente es la
// que maximiza su distancia MÍNIMA a las ya elegidas.
//
// El max-min importa y no es un detalle: con «suma de distancias» las tres podían terminar apiladas
// en el mismo extremo —las tres muy lejos del origen y pegadas entre sí— y eso vuelve a ser una sola
// propuesta con tres nombres, que es el defecto original.
//
// Desempate por posición en el pozo, que ya viene ordenado por plausibilidad para el eje. Sin
// desempate estable el brief dejaría de ser una función de sus entradas.
// Las dos mitades del puntaje se pesan IGUAL y a propósito: mérito y contraste llegan los dos a la
// escala 0–18 (mérito se normaliza por cuántas dimensiones se pidieron; el contraste es Manhattan
// sobre las seis). Sólo mérito daría tres formas que ganan en lo mismo —una propuesta con tres
// nombres—; sólo contraste da lo que ya falló en la medición: la más lejana es la peor en lo pedido.
const escalaMerito = 6

func meritoEn(forma string, dims []int) int {
	p, ok := perfilDeForma[forma]
	if !ok || len(dims) == 0 {
		return 0
	}
	total := 0
	for _, d := range dims {
		total += p[d]
	}
	return total * escalaMerito / len(dims)
}

func elegirPorContraste(pozo []string, origen string, dims []int, n int) []string {
	idx := map[string]int{}
	for i, f := range pozo {
		idx[f] = i
	}
	// El contraste se mide sobre LAS SEIS dimensiones aunque el mérito mire sólo las pedidas: «cada
	// una destaca en algo distinto» es una afirmación sobre el perfil entero, no sobre el eje del
	// reclamo. Midiéndolo sólo en las pedidas, dos formas idénticas salvo en profundidad contarían
	// como la misma propuesta.
	todas := todasLasDims()
	restantes := append([]string(nil), pozo...)
	var elegidas []string

	for len(elegidas) < n && len(restantes) > 0 {
		mejor, mejorPuntaje := -1, -1
		for i, cand := range restantes {
			puntaje := meritoEn(cand, dims)
			if len(elegidas) == 0 {
				// La primera: la que más gana en lo pedido, y a igualdad la que más se aleja del
				// punto de partida. Si no hay origen, el desempate lo hace el orden del pozo.
				if origen != "" {
					puntaje += distanciaEn(cand, origen, todas)
				}
			} else {
				min := distanciaEn(cand, elegidas[0], todas)
				for _, ya := range elegidas[1:] {
					if d := distanciaEn(cand, ya, todas); d < min {
						min = d
					}
				}
				puntaje += min
			}
			if puntaje > mejorPuntaje || (puntaje == mejorPuntaje && mejor >= 0 && idx[cand] < idx[restantes[mejor]]) {
				mejor, mejorPuntaje = i, puntaje
			}
		}
		if mejor < 0 {
			break
		}
		elegidas = append(elegidas, restantes[mejor])
		restantes = append(restantes[:mejor], restantes[mejor+1:]...)
	}
	return elegidas
}

// notaDeContraste explica, en el brief, POR QUÉ vinieron esas tres y no otras. Va siempre: sin esto,
// tres formas elegidas por distancia se leen igual que tres sacadas de una lista, y el agente no
// tiene cómo saber que cada una gana en algo distinto.
func notaDeContraste(origen string, dims []int, estructural, explicito bool) string {
	var b strings.Builder
	b.WriteString("POR QUÉ ESTAS TRES: no salen de una lista fija — se eligieron por CONTRASTE, lo más lejos posible entre sí")
	if origen != "" {
		b.WriteString(" y del punto de partida (")
		b.WriteString(formasDeDiseno[origen].Nombre)
		b.WriteString(")")
	}
	b.WriteString(". ")
	var ns []string
	for _, d := range dims {
		ns = append(ns, nombreDim[d])
	}
	switch {
	case !explicito:
		b.WriteString("El pedido no nombró ninguna dimensión, así que se buscaron las que más se separan entre sí — NO lo leas como que se pidió cambiar todo")
	case len(dims) == dimsTotal:
		b.WriteString("El pedido habla de replantear el esqueleto entero, así que juegan las seis dimensiones")
	default:
		b.WriteString("Lo que el pedido dice querer GANAR: " + strings.Join(ns, ", ") + " — las candidatas se eligieron por cuánto ganan en eso, no sólo por qué tan distintas son")
		if estructural {
			b.WriteString(" (y además habla de cambiar el modelo, así que ninguna forma quedó descartada de entrada)")
		}
	}
	b.WriteString(". Cada una gana en algo distinto y eso está dicho al lado de su nombre; elegí por lo que el trabajo necesite, no por cuál suena mejor.")
	return b.String()
}
