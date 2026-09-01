package mcp

// formas_diseno.go — LA CAPA DE FORMA (plan de la capa de forma, fase 1).
//
// POR QUE EXISTE. Hasta acá el brief decía DE QUÉ HABLA el pedido —el eje— y nunca QUÉ FORMA tiene
// la pantalla. Y esa es, según el consenso del campo anti-slop, la causa real de que un diseño
// generado se reconozca: «la igualdad ESTRUCTURAL es la huella de la IA, no la visual»
// (Nutlope/hallmark, MIT). Casi toda interfaz generada es visualmente distinta y estructuralmente
// idéntica: se cambia la paleta y parece otra cosa, se mira el esqueleto y es la misma página.
//
// EL CATÁLOGO ES NUESTRO, Y ESO NO ES ORGULLO: es que el de ellos no sirve acá. Sus 21
// macroestructuras —bento, manifiesto, especimen tipográfico, carta— están pensadas para landings y
// sitios editoriales. De los 67 pedidos de nuestro set dorado, la mayoría son tablas de inventario,
// formularios de alta, tableros de planta y paneles de flota. Aplicarle «manifiesto» a una pantalla
// de ERP la empeora.
//
// Las 12 de abajo salieron de agrupar esos 67 pedidos POR ESQUELETO y no por tema. Medido:
// **cubren el 96 %** (64 de 67). Los tres que no caen —«estado vacío», «actividad reciente», «modo
// oscuro»— no son formas de pantalla, y que queden afuera VALIDA el corte en vez de debilitarlo:
// el vacío es un ESTADO de otra forma, no una forma.
//
// EL MOTOR NO ELIGE: ACOTA. Elegir cuál forma le pega a un pedido es un juicio, y el camino caliente
// es model-free por regla del proyecto. Así que se sirven DOS O TRES candidatas con su descripción y
// el agente elige — el mismo reparto que ya usa todo el motor (el cerebro sirve conocimiento, el
// caller compone).

import "strings"

// formaDiseno es un esqueleto de pantalla: su nombre y qué la define. La descripción es lo que
// viaja en el brief, así que dice cómo se COMPONE, no de qué habla.
type formaDiseno struct {
	Nombre string
	Desc   string
}

var formasDeDiseno = map[string]formaDiseno{
	"tabla-densa": {"tabla densa",
		"muchas filas comparables y una columna por la que se mira todo. El encabezado queda fijo, los números van a la derecha con cifras tabulares, y lo que no apura se apaga en gris"},
	"lista-priorizada": {"lista priorizada",
		"un solo elemento arriba, grande y con su acción adentro, y el resto como una lista corta y callada. La pantalla decide el orden en vez de delegarlo"},
	"tablero-un-numero": {"tablero de un número",
		"una cifra protagonista que ocupa el doble de superficie que todo lo demás junto, y alrededor los secundarios en un ritmo menor. NO cuatro tarjetas iguales"},
	"formulario-guiado": {"formulario guiado",
		"campos agrupados con aire entre grupos, la validación al lado del campo que la causó, y una acción final inconfundible. El ritmo hace que se sienta corto"},
	"detalle-con-lados": {"detalle con lados",
		"un cuerpo principal con una columna angosta al costado para metadatos, historia o acciones. La marginalia acompaña sin competir"},
	"catalogo-elegir": {"catálogo para elegir",
		"un conjunto que se recorre y del que se elige uno: búsqueda arriba, resultados con ritmo variable —no tarjetas idénticas— y lo aplicado siempre visible"},
	"monitor-procesos": {"monitor de procesos",
		"cosas que están corriendo AHORA, cada una con su avance y su estado. Lo detenido rompe la forma del bloque, no sólo su color"},
	"conversacion": {"conversación",
		"el mensaje es el protagonista y todo lo que lo rodea retrocede. El compositor abajo, el estado de envío siempre visible"},
	"lienzo-inspector": {"lienzo con inspector",
		"una superficie grande para explorar y un panel angosto que explica lo que está seleccionado. El lienzo manda; el inspector obedece"},
	"interrupcion": {"interrupción",
		"una sola pantalla o diálogo que corta lo que se estaba haciendo para decir algo o pedir una decisión. Un mensaje, una salida clara, nada más"},
	"rejilla-temporal": {"rejilla temporal",
		"el tiempo es el eje que ordena: una grilla de días, semanas o turnos donde la posición ES el dato. La densidad se elige, no se promedia"},
	"narrativa": {"narrativa",
		"secciones que se leen en orden y construyen un argumento. El ritmo entre secciones importa más que cada sección"},
}

// formasPorEje es el POZO de lo plausible para cada eje. **Ya no es la respuesta.**
//
// Hasta el 2026-08-31 tenía exactamente tres formas por eje y esas tres eran las que viajaban, así
// que para `tabla` salían siempre las mismas y la rotación sólo excluía la del pedido anterior. El
// defecto es medible: dos pedidos opuestos —«esta tabla no se puede comparar» y «esta tabla no me
// dice qué hacer»— recibían LAS MISMAS tres candidatas. El motor no tenía por dónde enterarse de que
// son problemas distintos.
//
// Ahora hay cinco o seis por eje y las tres que viajan se eligen de acá por CONTRASTE
// (contraste_diseno.go): lejos del punto de partida y lejos entre sí, sobre las dimensiones que el
// pedido dice querer mover. El pozo sigue existiendo para que las propuestas sean PLAUSIBLES —sin él,
// un login podría recibir «tabla densa»—, pero elegir tres de seis por distancia es lo que hace que
// dejen de ser genéricas y siempre las mismas.
//
// Un eje que no está acá NO propone forma, y es deliberado: `color`, `a11y`, `tipografia`,
// `estado-vacio` y `terminacion` son PROPIEDADES de una pantalla, no esqueletos. Proponerle una
// forma a «cómo se comporta la paleta en modo oscuro» sería inventar una respuesta — el mismo
// criterio que ya usa la abstención.
//
// El ORDEN dentro de cada pozo importa: es el desempate cuando dos candidatas contrastan igual, así
// que va de más a menos plausible para ese eje.
//
// Y EL TAMAÑO TAMBIÉN, POR ARITMÉTICA PURA. Con un pozo de 4, sacar el origen deja 3 y hay que elegir
// 3: existe UN SOLO conjunto posible, así que el contraste sólo puede cambiar el ORDEN y las tres
// propuestas son siempre las mismas — exactamente lo que el usuario objetó. Medido sobre 12 pedidos
// distintos: pozo 4 → 1 conjunto (chat, login, microcopy, onboarding), pozo 5 → 3-4, pozo 6 → 5-6,
// pozo 7 → 7. Por eso `designPozoMinimo`, y por eso esos cuatro ejes se llevaron a 6: 5 alcanza para
// que haya elección pero ya está contra su propio techo (C(4,3) = 4 conjuntos posibles).
var formasPorEje = map[string][]string{
	"tabla":      {"tabla-densa", "detalle-con-lados", "catalogo-elegir", "rejilla-temporal", "lista-priorizada", "lienzo-inspector", "monitor-procesos"},
	"dataviz":    {"tablero-un-numero", "lienzo-inspector", "narrativa", "rejilla-temporal", "monitor-procesos", "tabla-densa"},
	"dashboard":  {"tablero-un-numero", "monitor-procesos", "lista-priorizada", "rejilla-temporal", "tabla-densa", "detalle-con-lados"},
	"formulario": {"formulario-guiado", "detalle-con-lados", "interrupcion", "narrativa", "lista-priorizada"},
	"login":      {"formulario-guiado", "interrupcion", "detalle-con-lados", "narrativa", "catalogo-elegir", "tablero-un-numero"},
	"filtros":    {"catalogo-elegir", "tabla-densa", "lista-priorizada", "rejilla-temporal", "lienzo-inspector"},
	"navegacion": {"catalogo-elegir", "detalle-con-lados", "lienzo-inspector", "lista-priorizada", "narrativa"},
	"jerarquia":  {"lista-priorizada", "tablero-un-numero", "narrativa", "detalle-con-lados", "tabla-densa"},
	"estado":     {"monitor-procesos", "interrupcion", "lista-priorizada", "tablero-un-numero", "rejilla-temporal"},
	"chat":       {"conversacion", "detalle-con-lados", "lienzo-inspector", "monitor-procesos", "lista-priorizada", "narrativa"},
	"onboarding": {"formulario-guiado", "narrativa", "conversacion", "interrupcion", "lista-priorizada", "catalogo-elegir"},
	"densidad":   {"tabla-densa", "rejilla-temporal", "tablero-un-numero", "lista-priorizada", "catalogo-elegir", "monitor-procesos"},
	"layout":     {"detalle-con-lados", "narrativa", "rejilla-temporal", "lienzo-inspector", "tablero-un-numero", "tabla-densa"},
	"movil":      {"lista-priorizada", "conversacion", "formulario-guiado", "interrupcion", "catalogo-elegir"},
	"motion":     {"monitor-procesos", "lienzo-inspector", "interrupcion", "narrativa", "tablero-un-numero"},
	"microcopy":  {"interrupcion", "narrativa", "conversacion", "lista-priorizada", "detalle-con-lados", "formulario-guiado"},
	"presencia":  {"tablero-un-numero", "narrativa", "lienzo-inspector", "interrupcion", "lista-priorizada"},
}

// designFormasPropuestas es cuántas candidatas viajan. Dos o tres: con una el motor estaría
// eligiendo —y no puede, es un juicio—, y con más de tres el bloque deja de acotar y vuelve a ser
// un catálogo que el agente tiene que leer entero.
const designFormasPropuestas = 3

// designPozoMinimo es el piso de cada pozo, y NO es un número elegido a gusto: se deriva. Hay que
// poder sacar el origen (−1) y que todavía sobre al menos una forma más que las que se proponen, o el
// conjunto queda determinado y el contraste no puede hacer nada.
const designPozoMinimo = designFormasPropuestas + 2

// candidatasDeForma elige QUÉ formas viajan, sin escribir todavía una sola palabra del brief. Está
// separada de `formasPara` porque la elección tiene ahora DOS lectores: el bloque de forma, que la
// describe, y el bloque de escala, que saca de ella el registro numérico. Con la elección adentro del
// armador de texto, la escala habría tenido que rehacerla —y dos maneras de elegir lo mismo es como
// el brief termina proponiendo una forma y sirviendo los números de otra.
func candidatasDeForma(eje string, usadas map[string]bool, intencion intencionDeDiseno) []string {
	pozo := formasPorEje[eje]
	if len(pozo) == 0 {
		return nil
	}

	// El punto de partida sale de lo que se pidió CONSERVAR: un rediseño dice «hoy es una tabla», y
	// esa forma es de la que hay que alejarse. Si el origen no está en el pozo igual sirve como
	// referencia de distancia — de hecho ahí es donde más sirve.
	origen := formaMencionada(intencion.Keep)

	// La rotación y el origen se sacan del pozo, no del cálculo: proponer como novedad la forma que
	// el proyecto acaba de usar, o la que el pedido pide cambiar, es contestar otra pregunta.
	var disponibles []string
	for _, c := range pozo {
		if usadas[c] || c == origen {
			continue
		}
		disponibles = append(disponibles, c)
	}
	// Si la historia y el origen vaciaron el pozo, se vuelve a él entero: quedarse sin forma por
	// rotación sería peor que repetir una. La rotación es una preferencia, no una prohibición.
	if len(disponibles) == 0 {
		disponibles = pozo
	}

	rs, _, _ := dimensionesAMover(intencion.Change)
	return elegirPorContraste(disponibles, origen, rs, hayDireccion(rs), designFormasPropuestas)
}

// formasPara arma el bloque de forma para un eje, excluyendo las que ya se usaron (`usadas`, que
// viene de la memoria del proyecto y hoy llega vacía — la rotación es la fase siguiente).
//
// Devuelve "" cuando el eje no tiene formas plausibles. Un bloque vacío es correcto y se declara:
// proponerle una forma a un pedido sobre la paleta sería inventar una respuesta.
func formasPara(eje string, usadas map[string]bool, intencion intencionDeDiseno) string {
	elegidas := candidatasDeForma(eje, usadas, intencion)

	// Sin opciones no se emite el encabezado. Un bloque que dice «elegí UNA de estas» y no lista
	// ninguna es peor que no mandar nada: le pide al agente que elija de un conjunto vacío. Lo
	// destapó el sabotaje del repliegue, que quedaba VERDE porque el test miraba si la cadena era
	// vacía y la función devolvía el encabezado solo.
	if len(elegidas) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("FORMA DE LA PANTALLA — elegí UNA de estas y decila en tu entrega. La igualdad estructural es lo que hace que un diseño se reconozca como generado: dos pantallas del mismo proyecto no deberían compartir esqueleto.")
	for _, c := range elegidas {
		f := formasDeDiseno[c]
		b.WriteString("\n- ")
		b.WriteString(f.Nombre)
		// EN QUÉ DESTACA, al lado del nombre. Sin esto, tres formas elegidas por contraste se leen
		// igual que tres sacadas de una lista y el agente no tiene cómo saber que cada una gana en
		// algo distinto — que es todo el punto de haberlas elegido así.
		if d := destacaDe(c); len(d) > 0 {
			b.WriteString(" [gana en ")
			b.WriteString(strings.Join(d, " y "))
			b.WriteString("]")
		}
		b.WriteString(": ")
		b.WriteString(f.Desc)
	}

	rs, estructural, explicito := dimensionesAMover(intencion.Change)
	b.WriteString("\n")
	b.WriteString(notaDeContraste(formaMencionada(intencion.Keep), rs, estructural, explicito))
	return b.String()
}

// ─────────────────────────────────────────────────────────────────────────────────────────────────
// LA ROTACIÓN POR MEMORIA
// ─────────────────────────────────────────────────────────────────────────────────────────────────

// formaUsadaTopic es donde el CALLER anota qué forma usó al entregar. Vive en el proyecto de quien
// diseña, no en el acervo compartido: la historia de formas es de cada proyecto.
const formaUsadaTopic = "diseno/forma-usada"

// EL MOTOR NO ESCRIBE, Y NO ES UN RODEO: ES LA ARQUITECTURA.
//
// `musubi_design` está declarada readOnly, y en este código readOnly decide la AUTORIZACIÓN — un
// principal con write=none sólo puede llamar tools de lectura. Está así a propósito para que la
// cabina del cuerpo y una sesión stdio puedan diseñar. Si el motor escribiera para recordar la
// forma anterior habría que marcarla readOnly=false, y eso le sacaría el motor de diseño a todos
// los lectores, además de ponerla bajo candado exclusivo en cada llamada.
//
// Así que escribe el CALLER: el brief le pide que, al entregar, guarde con qué forma lo hizo. Es la
// misma división que ya sostiene todo el motor —el cerebro sirve conocimiento, el agente compone— y
// la misma que hace posible que el camino caliente sea model-free.
//
// De paso: es exactamente el mecanismo que las skills del rubro tienen que falsificar. Ellas
// estampan un comentario en el CSS del artefacto esperando reencontrarlo la próxima vez, porque no
// tienen estado. Nosotros tenemos un cerebro que recuerda por proyecto, con fecha y procedencia, y
// sobrevive a que se borre el archivo y a que cambie la máquina.

// formasUsadasPor lee del proyecto la última forma anotada. Devuelve el conjunto a excluir y si
// había historia — las dos cosas, porque «no hay historia» y «hay historia y da vacío» son
// distintas y el brief las declara distinto.
//
// La clave es el proyecto DEL PRINCIPAL, no la marca pedida. Son cosas distintas y se confunden
// fácil: `musubi_design` acepta `brand` para diseñar a nombre de otro proyecto, y si la historia se
// llaveara por marca, la sala de mando le escribiría la rotación a Altura.
func (s *McpServer) formasUsadasPor(proyecto string) (usadas map[string]bool, hubo bool) {
	if strings.TrimSpace(proyecto) == "" {
		return nil, false
	}
	txt, found, err := s.engine.LatestObservationByTopicInProject(formaUsadaTopic, proyecto)
	if err != nil || !found {
		return nil, false
	}
	// El contenido lo escribe un agente, así que se lee con tolerancia: alcanza con que el nombre
	// de una forma del catálogo aparezca ahí adentro. Exigir un formato exacto haría que la
	// rotación se apague ante la primera nota escrita a mano, y se apagaría en silencio.
	bajo := strings.ToLower(txt)
	usadas = map[string]bool{}
	for clave, f := range formasDeDiseno {
		if strings.Contains(bajo, clave) || strings.Contains(bajo, strings.ToLower(f.Nombre)) {
			usadas[clave] = true
		}
	}
	if len(usadas) == 0 {
		return nil, false
	}
	return usadas, true
}

// notaDeRotacion es lo que el brief le pide al caller para que la rotación exista la próxima vez.
// Va SIEMPRE que haya bloque de forma: si sólo apareciera cuando ya hay historia, nunca habría una
// primera anotación y el mecanismo no arrancaría jamás.
func notaDeRotacion(hubo bool) string {
	base := "Al entregar, guardá con qué forma compusiste: musubi_save_observation con topic_key='" +
		formaUsadaTopic + "' y el nombre de la forma en el contenido. Es lo que hace que el próximo " +
		"diseño de este proyecto no repita el mismo esqueleto."
	if hubo {
		return "HISTORIA: este proyecto ya usó otra forma hace poco y quedó excluida de las candidatas de arriba. " + base
	}
	return "HISTORIA: no hay registro de formas anteriores en este proyecto, así que las candidatas de arriba son todas. " + base
}

// proyectoDelPrincipal devuelve el tenant de quien llama, o "" si no hay principal (stdio local).
// Existe como función y no inline para que quede UN solo lugar donde se decide de quién es la
// historia de formas — el bug fácil acá es tomarla del argumento `brand`.
func proyectoDelPrincipal(p *Principal) string {
	if p == nil {
		return ""
	}
	return p.ProjectID
}
