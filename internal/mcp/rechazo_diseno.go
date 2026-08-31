package mcp

// rechazo_diseno.go — EL CHECKLIST DE RECHAZO (plan de cierre, fase 4).
//
// POR QUÉ EXISTE. El pedido que abrió todo el track fue «lo usé mucho en Altura y no me gusta nada,
// todo full malo». El brief le decía al agente qué hacer y nunca qué RECHAZAR, y un «no hagas X» es
// accionable y verificable de una manera que un principio en positivo no es.
//
// DE DÓNDE SALEN LOS TELLS: del propio acervo, no de otro motor. Medido el 2026-08-30 sobre las
// 1.438 tarjetas de `design-corpus/*`, **315 (22 %) contienen una prohibición** — 347 frases. Este es
// el ranking real de lo que este acervo advierte:
//
//	jerga interna en el texto ... 39 frases
//	ícono sin etiqueta ......... 37
//	jerarquía difusa ........... 27
//	color decorativo ........... 22
//	dato inventado ............. 17
//	gris de bajo contraste ...... 9
//	animación sin causa ......... 9
//
// El problema nunca fue que faltaran: es que estaban DILUIDAS. Sólo llegaban al brief si el ranking
// enganchaba justo esa tarjeta, y el ranking no discriminaba. Por eso el checklist es un bloque
// propio y no más tarjetas.
//
// POR QUÉ VA FILTRADO POR EJE Y NO FIJO. Un bloque constante de ~400 tokens habría bajado M5 —la
// fracción variable del brief, que es la compuerta del banco desde el 2026-08-30— de 0,44 a ~0,39,
// justo debajo de su umbral. O sea: el freno que se puso para que no volviera el sermón habría
// frenado esto. Y tenía razón en frenarlo: un checklist que sirve los mismos catorce avisos para una
// tabla y para un login ES un sermón. Con el ruteo por eje ya sabemos de qué habla el pedido, así que
// se sirven los tells de ESE eje más un núcleo corto que aplica siempre.

import "strings"

// tellDiseno es una cosa que el diseño no tiene que hacer, con su porqué en la misma línea. El
// porqué no es adorno: sin él, el agente no puede decidir los casos que el tell no previó.
type tellDiseno struct {
	Eje   string // "" = aplica siempre
	Texto string
}

// tellsDeDiseno — ordenados por la frecuencia medida en el acervo. Los de eje vacío son el núcleo
// que viaja siempre; el resto entra sólo cuando el pedido cayó en ese eje.
var tellsDeDiseno = []tellDiseno{
	// ── NÚCLEO (siempre). Corto a propósito: es lo constante, y lo constante ocupa canal.
	{"", "NO uses jerga interna ni nombres del sistema en la interfaz. Se nombra lo que la persona reconoce, no cómo está construido: «notificaciones», no «webhook config»."},
	{"", "NO pongas más de una acción primaria por vista. Tres CTA con el mismo peso no son tres oportunidades: son ninguna jerarquía."},
	{"", "NO uses el color como adorno. Un tono se gana teniendo un trabajo — estado, categoría, o el acento de la marca. Repartirlo por todos lados lo vacía."},
	{"", "NO inventes datos para llenar la pantalla. Un vacío se explica diciendo qué lo va a llenar; un error NUNCA se disfraza de dato."},
	{"", "NO tapes un layout flojo con sombras, glows o gradientes. Si la composición no se sostiene en gris, no la arregla el brillo."},
	// ── Este tell entró el 2026-08-30 por el camino más caro y más confiable: el usuario lo vio.
	// En una prueba a ciegas eligió los tres diseños del brief nuevo y sobre uno dijo «esas rayitas
	// no me gustan, se ven muy raras», señalando la franja de color al costado de un bloque de
	// alerta. Vale anotar cómo se perdió: la lista de tells de TidyFactor/Styler lo trae como su
	// #7 («arbitrary 4px colored left-borders on content cards») y yo lo descarté entero por
	// construir la lista desde NUESTRO acervo. La decisión de no copiar sigue siendo correcta; lo
	// que estuvo mal fue no mirar lo ajeno como hipótesis a verificar. Y es un tic propio: lo uso
	// por reflejo cuando quiero marcar importancia sin ganármela con jerarquía.
	{"", "NO uses una franja de color al costado de un bloque para marcar que importa. Es adorno con cara de semántica: no dice QUÉ pasa ni CUÁNTO, y se lee como decoración pegada. Si algo importa, ganátelo con escala, peso o superficie — no con una rayita."},

	// ── LA CARA DE «HECHO POR IA», 2026 ─────────────────────────────────────────────────────
	// Estos NO salieron del acervo: salieron de mirar qué produce hoy un modelo cuando no se le
	// exige nada, y de la lista de TidyFactor/Styler. Se traen como HIPÓTESIS VERIFICABLES y no
	// como copia — la distinción dejó de ser teórica el 2026-08-30, cuando el usuario señaló la
	// franja de color al costado (el #7 de esa lista) sin conocerla. Discardar lo ajeno entero
	// costó ese tell; verificarlo es lo que corresponde.
	//
	// Ojo con la fecha: el look de IA SE MUEVE. El violeta-a-azul de 2024 ya no engaña a nadie y
	// el crema con serifas de 2026 sí. Esta lista se revisa, no se acumula.
	{"", "NO caigas en las dos paletas por defecto de 2026: crema #F4F1EA con serifa de display y acento terracota, o casi-negro con un solo acento verde ácido o bermellón. Las dos se reconocen a un metro. Elegir una a propósito para una marca que la pide está bien; llegar a ella sin decidir es la firma de que nadie eligió."},
	{"layout", "NO abras con un degradado violeta-a-azul de ancho completo. Es el hero de plantilla; ya no dice «moderno», dice «no había dirección de arte»."},
	{"", "NO pongas el mismo radio grande en todo. `rounded-2xl` en tarjetas, botones, inputs y avatares por igual es el tell más rápido de interfaz sin terminar. El radio tiene jerarquía: píldoras full, controles 8-12, superficies 12-16, y el contenedor siempre mayor que su contenido."},
	{"layout", "NO pongas la misma sombra en todo. `shadow-lg` en cada bloque aplana la jerarquía en vez de crearla: si todo flota a la misma altura, nada está adelante."},
	{"layout", "NO centres todo. Una página donde cada bloque está centrado no tiene composición, tiene una columna. La alineación a la izquierda es la que deja leer; el centrado se reserva para lo que de verdad es un momento."},
	{"jerarquia", "NO pongas un eyebrow en mayúsculas arriba de cada sección. Un rótulo tracked-out sobre cada título deja de jerarquizar apenas se repite: es ornamento con forma de estructura."},
	{"jerarquia", "NO numeres cosas que no son una secuencia. Los marcadores 01/02/03 sólo informan si el orden importa de verdad; sobre tres beneficios sueltos, son decoración que finge método."},
	{"microcopy", "NO uses emoji como marcador de sección ni como ícono de sistema. En una interfaz de trabajo se lee como plantilla, y encima cambia de forma según el sistema operativo de quien mira."},
	{"color", "NO apliques degradado al texto. `background-clip: text` sobre un titular es el recurso que se usa cuando el titular no se sostiene solo, y en cuerpos de texto además arruina la legibilidad."},
	{"a11y", "NO uses gris tenue para lo que hay que leer. #94a3b8 sobre blanco no llega a 4.5:1: se ve elegante en el mock y desaparece en una pantalla real con luz."},
	{"color", "NO llenes el fondo con grillas de plano, mallas de puntos o glows ambientales. Decoran el vacío en vez de resolverlo; si la sección necesita textura para no verse pobre, lo que falta es contenido o jerarquía."},
	{"motion", "NO animes todo al mismo tiempo al entrar. Diez elementos apareciendo en cascada no es una entrada orquestada: es que se aplicó el mismo efecto a todo. Si algo se anima, que sea porque merece la atención."},
	{"", "NO entregues un componente que se vería igual en un ERP, un e-commerce y un blog. Si el mismo bloque sirve para cualquier rubro sin tocarlo, no está diseñado para éste: está tomado de un catálogo."},

	// ── POR EJE.
	{"jerarquia", "NO destaques todo. Lo que se destaca se define por lo que NO se destaca; si todo pesa igual, el ojo no tiene por dónde entrar."},
	{"microcopy", "NO expliques con texto lo que el diseño debería mostrar solo. Un cartel que aclara la interfaz es una interfaz que falló."},
	{"microcopy", "NO escribas errores que sólo dicen que algo salió mal. Un error dice qué pasó y qué hacer, sin disculpas ni vaguedad."},
	{"navegacion", "NO uses íconos sin etiqueta en navegación. El ícono solo se reconoce si ya se conoce; el que llega por primera vez adivina."},
	{"color", "NO uses la escala de semáforo cuando no hay estado que comunicar. Verde/amarillo/rojo es vocabulario reservado, no una paleta."},
	{"a11y", "NO uses gris tenue para texto que hay que leer. Debajo de 4.5:1 hay gente que directamente no lo ve."},
	{"a11y", "NO entregues sólo hover. Sin foco visible, quien navega con teclado se queda sin saber dónde está parado."},
	{"motion", "NO animes lo que no cambió de estado. La animación sin causa es ruido con presupuesto de cómputo."},
	{"tabla", "NO escondas ni trunques columnas en silencio. Recortar está bien; recortar sin decir cuánto quedó afuera entrega una tabla mutilada con cara de completa."},
	{"tabla", "NO alinees números a la izquierda. Las columnas numéricas van a la derecha y con cifras tabulares, o no se pueden comparar de un vistazo."},
	{"formulario", "NO uses el placeholder como etiqueta. Desaparece al escribir y deja a la persona sin saber qué estaba llenando."},
	{"formulario", "NO juntes todos los errores arriba. La validación va al lado del campo que la causó."},
	{"estado", "NO dejes una acción sin estado de carga. Sin señal, la persona vuelve a apretar."},
	{"estado-vacio", "NO entregues una caja vacía. Un estado vacío dice qué va a aparecer ahí y cómo hacer que aparezca."},
	{"login", "NO digas «usuario o contraseña incorrectos» y nada más si podés ser útil sin filtrar quién existe. La ambigüedad protege la cuenta, no justifica dejar a la persona sin salida: ofrecé recuperar."},
	{"densidad", "NO llenes el espacio porque está. El aire es parte de la composición, no sobra."},
	{"dataviz", "NO cortes el eje sin declararlo. Un eje truncado exagera la diferencia y eso es mentir con una forma."},
	{"dashboard", "NO ordenes los widgets por cuándo se crearon. Arriba va la métrica que se mira todos los días."},
	{"movil", "NO uses blancos de menos de 44px para lo que se toca. El dedo no tiene la puntería del cursor, y un objetivo chico se falla más veces de las que se acierta."},
	{"chat", "NO pierdas el estado de envío. Quien manda un mensaje tiene que saber si salió."},
	{"filtros", "NO apliques un filtro sin mostrar cuál está activo y cómo se saca."},
	{"onboarding", "NO obligues a leer antes de dejar hacer. La guía acompaña; no es un peaje."},
	{"layout", "NO inventes anchos que no caen en ninguna columna de la grilla."},
}

// designTellsPorEje acota cuántos tells DE EJE entran. El núcleo va siempre y es corto; esto evita
// que un eje con muchos tells desbalancee el bloque.
const designTellsPorEje = 4

// tellsPara arma el bloque de rechazo: el núcleo universal más los tells del eje ruteado. Sin eje
// (el motor no ruteó) va sólo el núcleo — inventar tells de un tema que no se identificó sería
// exactamente el sermón que este diseño evita.
func tellsPara(eje string) string {
	var b strings.Builder
	b.WriteString("NO ENTREGUES ESTO (rechazá y rehacé si tu diseño lo tiene):")
	n := 0
	for _, t := range tellsDeDiseno {
		if t.Eje != "" {
			continue
		}
		b.WriteString("\n- ")
		b.WriteString(t.Texto)
	}
	if eje == "" {
		return b.String()
	}
	for _, t := range tellsDeDiseno {
		if t.Eje != eje || n >= designTellsPorEje {
			continue
		}
		b.WriteString("\n- ")
		b.WriteString(t.Texto)
		n++
	}
	return b.String()
}
