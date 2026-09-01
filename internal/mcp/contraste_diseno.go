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

// Las siete dimensiones en las que una forma puede destacar. Son las que DISCRIMINAN el catálogo: se
// eligieron porque separan las doce formas, no porque suenen bien. Una dimensión en la que todas
// puntúan parecido no sirve para elegir por contraste — es peso muerto que diluye la distancia.
const (
	dimDensidad    = iota // cuántas cosas comparables muestra a la vez
	dimComparacion        // si dos elementos se pueden poner al lado y ver la diferencia
	dimDecision           // qué tan rápido te dice qué hacer
	dimProfundidad        // si podés entrar al detalle sin cambiar de pantalla
	dimGuia               // si te lleva paso a paso
	dimPresencia          // si tiene un momento visual que se recuerda
	dimVivo               // si muestra lo que está pasando AHORA
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
	dimVivo:        "estado en vivo",
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
	//                     dens comp deci prof guia pres vivo
	"tabla-densa":       {3, 3, 0, 1, 0, 0, 0},
	"rejilla-temporal":  {3, 3, 1, 0, 0, 1, 1},
	"monitor-procesos":  {2, 2, 2, 1, 0, 1, 3},
	"catalogo-elegir":   {2, 2, 1, 1, 1, 1, 0},
	"lista-priorizada":  {1, 1, 3, 0, 1, 1, 1},
	"tablero-un-numero": {0, 0, 3, 0, 0, 3, 1},
	"detalle-con-lados": {1, 0, 1, 3, 0, 1, 0},
	"lienzo-inspector":  {2, 1, 0, 3, 0, 2, 1},
	"formulario-guiado": {0, 0, 1, 1, 3, 0, 0},
	"narrativa":         {0, 0, 0, 2, 3, 3, 0},
	"interrupcion":      {0, 0, 3, 0, 2, 2, 1},
	// La conversación NO gana en presencia: puntuaba 2 ahí sólo por ser el menos bajo de sus seis
	// valores, y el brief terminaba afirmando que un chat destaca por su momento visual. Lo que un
	// chat sí hace es avanzar de a poco —preguntás, contesta, actuás— y acompañar: decisión y guía.
	"conversacion": {1, 0, 2, 1, 1, 1, 2},
}

// destacaPiso es cuánto hay que puntuar para poder AFIRMAR que se gana en algo. Dos de tres: con uno
// la afirmación es falsa y con tres sólo podría destacar la mitad del catálogo.
const destacaPiso = 2

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
	// PISO PARA AFIRMAR QUE GANA. Sin esto, `destacaDe` devuelve el máximo aunque sea 1, y una forma
	// floja en las siete dimensiones queda anunciada como que «gana» en su valor menos bajo. Fue lo
	// que pasó con la conversación: puntuaba 2 en presencia por descarte y el brief decía que un chat
	// destaca por su momento visual. Si nada llega a 2, la forma no gana en nada y se calla — el
	// mismo criterio que la abstención del motor.
	if max < destacaPiso {
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
	dimVivo:        {"en vivo", "ahora", "tiempo real", "esta corriendo", "está corriendo", "en curso", "progreso", "avance", "estado actual", "monitorear", "seguimiento", "que pasa", "qué pasa"},
}

// palabrasEstructurales son las que piden repensar el ESQUELETO entero y no una dimensión. Cuando
// aparecen, se mueven las siete: pedir «cambiar el modelo» y recibir tres variantes que sólo difieren
// en densidad sería contestar otra pregunta.
var palabrasEstructurales = []string{"modelo", "estructura", "esqueleto", "layout", "disposicion", "disposición", "replantear", "de cero", "otra cosa"}

// polaridad es HACIA DÓNDE hay que mover una dimensión. Existe porque nombrar una dimensión no dice
// en qué dirección: «cambiar las cuadrículas» y «quiero más densidad» nombran la misma y piden cosas
// distintas.
type polaridad int

const (
	polMover  polaridad = iota // se nombra como lo que se cambia, sin decir hacia dónde
	polGanar                   // falta: la pantalla tiene que ganar en eso
	polPerder                  // sobra: la pantalla tiene que ceder en eso
)

// reclamo es una dimensión con su dirección.
type reclamo struct {
	Dim int
	Pol polaridad
}

// Marcadores de dirección. Se buscan POR CLÁUSULA, no en todo el texto: quién gobierna a quién
// importa, y un «no» al principio de la frase no convierte en déficit a todo lo que venga después.
var (
	marcasPerder = []string{"demasiado", "demasiada", "demasiadas", "demasiados", "de más", "de mas",
		"sobra", "sobran", "exceso", "abruma", "satura", "ruido", "menos", "sacar", "quitar", "achicar"}
	marcasGanar = []string{"no se puede", "no puedo", "no se ve", "no me dice", "no dice", "no hay",
		"no llego", "no entra", "no cabe", "cuesta", "dificil", "difícil", "imposible", "falta", "faltan",
		"quiero", "necesito", "me gustaria", "me gustaría", "más", "mejor", "que se pueda", "hay que poder"}
	marcasMover = []string{"cambiar", "cambiá", "cambia ", "cambiale", "otro", "otra", "distinto",
		"distinta", "reemplazar", "replantear"}
)

// polaridadDe lee la dirección de UNA cláusula. Devuelve `ok=false` si la cláusula no declara ninguna,
// para que pueda heredar la de la cláusula anterior.
//
// El exceso se chequea PRIMERO porque es más específico: «no quiero tanta densidad» tiene un «no» y
// un «tanta», y lo que manda es el segundo.
func polaridadDe(clausula string) (polaridad, bool) {
	for _, w := range marcasPerder {
		if strings.Contains(clausula, w) {
			return polPerder, true
		}
	}
	for _, w := range marcasGanar {
		if strings.Contains(clausula, w) {
			return polGanar, true
		}
	}
	for _, w := range marcasMover {
		if strings.Contains(clausula, w) {
			return polMover, true
		}
	}
	return polMover, false
}

// enClausulas parte el pedido para poder leer cada reclamo con su propia dirección. El verbo de una
// cláusula gobierna lo que le sigue hasta el próximo corte, así que en «cambiar modelos, cuadrículas»
// el «cambiar» alcanza también a las cuadrículas — por eso una cláusula sin marcador HEREDA la de la
// anterior en vez de caer al default.
func enClausulas(txt string) []string {
	for _, sep := range []string{";", ".", "·", ":", "\n", " pero ", " aunque "} {
		txt = strings.ReplaceAll(txt, sep, ",")
	}
	var out []string
	for _, c := range strings.Split(txt, ",") {
		if c = strings.TrimSpace(c); c != "" {
			out = append(out, c)
		}
	}
	return out
}

// dimensionesAMover lee del pedido QUÉ dimensiones hay que atacar y HACIA DÓNDE.
//
// LA DIRECCIÓN NO ESTABA, Y SE NOTÓ EN VIVO. La versión anterior trataba toda dimensión nombrada como
// «hay que ganar en esto», así que al pedido real del usuario —«cambiar modelos, cuadrículas»—
// entendía «quiero más densidad» y le proponía una TABLA para una pantalla de notas. Y no es un caso
// raro: «cambiá X» es la manera normal de pedir un rediseño, y es justo la que no dice hacia dónde.
//
// Ahora cada dimensión viene con su polaridad y el mérito la respeta: ganar suma el perfil, perder
// suma su complemento, y mover NO SUMA NADA — se cae a puro contraste, que es la lectura correcta de
// «cambialo, no sé a qué». Medido: con esto, «cambiar modelos, cuadrículas» sobre un catálogo propone
// primero «lienzo con inspector», que es lo más lejos del punto de partida.
//
// Devuelve también si el pedido dijo algo: «no dijo nada» y «dijo todo» dan el mismo conjunto y no son
// lo mismo, y el brief los declara distinto.
func dimensionesAMover(cambio string) (rs []reclamo, estructural, explicito bool) {
	bajo := strings.ToLower(cambio)
	if strings.TrimSpace(bajo) == "" {
		return todosLosReclamos(polMover), true, false
	}
	for _, w := range palabrasEstructurales {
		if strings.Contains(bajo, w) {
			estructural = true
			break
		}
	}

	// Una dimensión puede aparecer varias veces con direcciones distintas («está vacío… quiero más
	// densidad»). Gana la ÚLTIMA explícita: la gente se corrige mientras habla, y lo que dice al
	// final es lo que quiso decir. Una mención sin dirección no pisa a una que sí la tenía.
	pol := map[int]polaridad{}
	explicita := map[int]bool{}
	var orden []int
	heredada, hayHeredada := polMover, false

	for _, c := range enClausulas(bajo) {
		p, ok := polaridadDe(c)
		if ok {
			heredada, hayHeredada = p, true
		} else if hayHeredada {
			p = heredada
		}
		for d := 0; d < dimsTotal; d++ {
			for _, w := range vocabularioDeCambio[d] {
				if !strings.Contains(c, w) {
					continue
				}
				if _, visto := pol[d]; !visto {
					orden = append(orden, d)
				}
				if ok || !explicita[d] {
					pol[d] = p
					explicita[d] = explicita[d] || ok
				}
				break
			}
		}
	}

	if len(orden) == 0 {
		return todosLosReclamos(polMover), true, estructural
	}
	for _, d := range orden {
		rs = append(rs, reclamo{Dim: d, Pol: pol[d]})
	}
	return rs, estructural, true
}

func todosLosReclamos(p polaridad) []reclamo {
	rs := make([]reclamo, dimsTotal)
	for i := range rs {
		rs[i] = reclamo{Dim: i, Pol: p}
	}
	return rs
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
// sobre las siete). Sólo mérito daría tres formas que ganan en lo mismo —una propuesta con tres
// nombres—; sólo contraste da lo que ya falló en la medición: la más lejana es la peor en lo pedido.
const escalaMerito = 6

// meritoEn puntúa cuánto RESPONDE una forma al reclamo, con su dirección:
//   - ganar  → suma el perfil (más es mejor)
//   - perder → suma el complemento (menos es mejor)
//   - mover  → NO SUMA NADA, porque el pedido no dijo hacia dónde. Ahí sólo manda el contraste, que
//     es la lectura correcta de «cambialo, no sé a qué».
//
// El promedio va sobre las que SÍ tienen dirección: si sumaran las de `mover`, un pedido con una
// dirección y cinco menciones sueltas diluiría la única que dijo algo.
func meritoEn(forma string, rs []reclamo) int {
	p, ok := perfilDeForma[forma]
	if !ok {
		return 0
	}
	total, n := 0, 0
	for _, r := range rs {
		switch r.Pol {
		case polGanar:
			total += p[r.Dim]
			n++
		case polPerder:
			total += 3 - p[r.Dim]
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return total * escalaMerito / n
}

// hayDireccion dice si algún reclamo declaró hacia dónde. Sin eso el mérito no puede pesar y la
// elección se cae a puro contraste — el mismo camino que un pedido sin intención ninguna.
func hayDireccion(rs []reclamo) bool {
	for _, r := range rs {
		if r.Pol != polMover {
			return true
		}
	}
	return false
}

// `conMerito` es false cuando el pedido NO nombró ninguna dimensión, y entonces el mérito NO PESA.
//
// Lo destapó un test que ya existía: al eje `tabla` dejó de proponerle «tabla densa». Con el pedido
// mudo, `dims` son las siete, así que el mérito pasa a ser «suma de todas las capacidades» y rankea
// primero a las formas más parejas — un criterio que nadie pidió y que además castiga a las formas
// especialistas, que son justo las buenas cuando sí sabés qué querés.
//
// Sin mérito, la primera es la primera del pozo (la más plausible para ese eje) y las otras dos
// contrastan con ella. O sea: **un pedido normal se comporta como antes** y sólo un rediseño con un
// reclamo dicho re-ordena. Cero regresión para quien ya usaba el motor.
func elegirPorContraste(pozo []string, origen string, rs []reclamo, conMerito bool, n int) []string {
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
			var puntaje int
			if conMerito {
				puntaje = meritoEn(cand, rs)
			}
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
func notaDeContraste(origen string, rs []reclamo, estructural, explicito bool) string {
	var b strings.Builder
	b.WriteString("POR QUÉ ESTAS TRES: no salen de una lista fija — se eligieron por CONTRASTE, lo más lejos posible entre sí")
	if origen != "" {
		b.WriteString(" y del punto de partida (")
		b.WriteString(formasDeDiseno[origen].Nombre)
		b.WriteString(")")
	}
	b.WriteString(". ")

	var ganar, perder, mover []string
	for _, r := range rs {
		switch r.Pol {
		case polGanar:
			ganar = append(ganar, nombreDim[r.Dim])
		case polPerder:
			perder = append(perder, nombreDim[r.Dim])
		default:
			mover = append(mover, nombreDim[r.Dim])
		}
	}

	switch {
	case !explicito:
		b.WriteString("El pedido no nombró ninguna dimensión, así que se buscaron las que más se separan entre sí — NO lo leas como que se pidió cambiar todo")
	case !hayDireccion(rs):
		// «Cambiá X» nombra la dimensión y NO dice hacia dónde. Tratarlo como «quiero más X» es el
		// error que el motor cometió en vivo con «cambiar modelos, cuadrículas».
		b.WriteString("El pedido nombra qué cambiar (" + strings.Join(mover, ", ") +
			") pero NO dice hacia dónde, así que no se asumió una dirección: las candidatas se eligieron por qué tan lejos quedan del punto de partida y entre sí")
	default:
		var partes []string
		if len(ganar) > 0 {
			partes = append(partes, "GANAR en "+strings.Join(ganar, ", "))
		}
		if len(perder) > 0 {
			partes = append(partes, "CEDER en "+strings.Join(perder, ", "))
		}
		b.WriteString("Lo que el pedido dice querer: " + strings.Join(partes, " y "))
		if len(mover) > 0 {
			b.WriteString(" (y nombra " + strings.Join(mover, ", ") + " sin decir hacia dónde, así que ahí no se asumió nada)")
		}
		if estructural {
			b.WriteString(". Además habla de cambiar el modelo, así que ninguna forma quedó descartada de entrada")
		}
	}
	b.WriteString(". Cada una gana en algo distinto y eso está dicho al lado de su nombre; elegí por lo que el trabajo necesite, no por cuál suena mejor.")
	return b.String()
}
