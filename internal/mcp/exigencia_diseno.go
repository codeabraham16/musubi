package mcp

// exigencia_diseno.go — EL BRIEF EXIGE, NO SÓLO PROHÍBE (plan siguiente, jugada 1).
//
// POR QUÉ EXISTE, y sale de una prueba a ciegas con el usuario el 2026-08-30:
//
// Se le mostraron NUEVE diseños —tres pedidos reales de Altura × tres briefs distintos (completo,
// sin corpus, y sin brief)— mezclados. Eligió uno de CADA condición, que es exactamente lo que
// saldría al azar: **el contenido del brief no estaba determinando lo que prefiere**. Y sobre los
// tres dijo lo mismo: «no son tan potentes». Preguntado qué les faltaba, contestó dos cosas:
// **presencia visual** y **terminación de producto real**.
//
// Al mirar el brief con eso en la mano, el diagnóstico salta: TODO lo que contenía era para NO
// EQUIVOCARSE. Reglas de precedencia, tokens que no se inventan, anchos de celda explícitos, y un
// bloque `avoid` —construido ese mismo día— que empuja aún más hacia lo conservador. No había una
// sola línea del otro lado pidiendo una decisión fuerte.
//
// Medido contra el acervo, además, el conocimiento que haría falta casi no existe: de 1.736
// entradas, 14 hablan de un momento focal, 9 de terminación fina y 7 de contraste dramático,
// contra 126 de escala tipográfica y cientos de validación y tablas. **El cerebro está lleno de
// «cómo no equivocarse» y vacío de «cómo hacer que impacte».** Este bloque no arregla esa carencia
// del acervo —eso es ingesta dirigida— pero pone la exigencia donde el agente la lee siempre.
//
// CÓMO SE RECONCILIA CON `avoid`, porque si no sería orden y contraorden (la falla que F1 arregló):
// la exigencia dice DÓNDE gastar la audacia —en un solo lugar— y la prohibición dice dónde no
// gastarla —en todos los demás. No se contradicen: se completan. Esa frase va DENTRO del bloque,
// porque un agente que recibe «hacé una jugada fuerte» y «no uses color como adorno» sin que nadie
// le explique la relación, resuelve la tensión bajando los dos y entrega algo tibio. Que es
// exactamente lo que venía pasando.

import "strings"

// exigenciaDiseno es algo que el diseño TIENE que hacer, con su porqué. Misma forma que tellDiseno
// para que los dos bloques se lean igual.
type exigenciaDiseno struct {
	Eje   string // "" = aplica siempre
	Texto string
}

var exigenciasDeDiseno = []exigenciaDiseno{
	// ── NÚCLEO (siempre) ────────────────────────────────────────────────────────────────────
	{"", "ELEGÍ UN MOMENTO y gastá ahí toda la escala. Una pantalla tiene un solo protagonista: el número que decide, el estado que apura, la acción que se vino a hacer. Ese elemento va grande de verdad —tres o cuatro veces el cuerpo de texto, no un 20 % más— y todo lo demás se calla para que se lea."},
	{"", "USÁ EL RANGO ENTERO DE LA ESCALA TIPOGRÁFICA. Si tu pantalla va de 12px a 16px, no tiene jerarquía: tiene tamaños. Del dato protagonista al pie de nota tiene que haber una diferencia que se vea desde lejos."},
	{"", "TERMINALO COMO PRODUCTO, NO COMO BOCETO. Cifras tabulares en todo lo que se compara en columna, estados de hover/foco/activo/deshabilitado en lo que se toca, alineación óptica y no matemática en íconos junto a texto, y una transición corta (150–250 ms) en lo que cambia de estado."},
	{"", "COMPROMETETE CON UNA DECISIÓN VISIBLE. Un diseño que podría ser de cualquier producto no está terminado: elegí una cosa —el tratamiento del dato, la forma del contenedor, el ritmo de la grilla— y hacela consistente en toda la pantalla hasta que se reconozca."},

	// ── POR EJE ─────────────────────────────────────────────────────────────────────────────
	{"tabla", "DALE PESO A LA COLUMNA QUE IMPORTA. En una tabla densa hay una columna por la que se mira todo; esa va más ancha, más oscura o con el dato más grande, y el resto se subordina. Una tabla donde todas las columnas pesan igual es una planilla."},
	{"dashboard", "EL NÚMERO QUE DECIDE VA PRIMERO Y VA ENORME. Un tablero de cuatro tarjetas iguales no jerarquiza nada: elegí cuál se mira todos los días y dale el doble de superficie que a las otras juntas."},
	{"jerarquia", "HACÉ QUE EL SALTO SE VEA A UN METRO DE DISTANCIA. Si tenés que acercarte para saber qué es lo principal, no hay jerarquía."},
	{"estado", "EL ESTADO QUE APURA TIENE QUE INTERRUMPIR. Lo urgente no se comunica con un chip chiquito del mismo tamaño que todo: cambia la forma del bloque, no sólo su color."},
	{"color", "GASTÁ EL ACENTO UNA VEZ Y QUE SE NOTE. Un acento repartido en seis lugares es un fondo; en uno solo es una decisión."},
	{"dataviz", "EL GRÁFICO TIENE QUE DECIR SU CONCLUSIÓN SIN QUE LA BUSQUEN. Marcá el punto que importa —el pico, el cruce, el último valor— en vez de entregar una curva pareja y que la lea quien pueda."},
	{"microcopy", "ESCRIBÍ COMO ALGUIEN QUE SABE, NO COMO UN SISTEMA. Una línea con una idea concreta vale más que tres genéricas, y el tono es parte del diseño."},
	{"formulario", "QUE SE VEA DÓNDE EMPIEZA Y DÓNDE TERMINA. Un formulario sin ritmo visible se siente largo aunque sea corto: agrupá, dejá aire entre grupos y hacé que la acción final sea inconfundible."},
	{"login", "UNA PANTALLA DE ACCESO ES LA PRIMERA IMPRESIÓN DEL PRODUCTO. Es la única pantalla donde casi no hay datos que respetar: usá ese espacio para que se note de quién es el software."},
	{"estado-vacio", "EL VACÍO ES UNA OPORTUNIDAD, NO UN HUECO. Es la única pantalla sin datos compitiendo: ahí entra una ilustración, una frase con carácter y una acción clara."},
	{"layout", "ROMPÉ LA GRILLA UNA VEZ. Una composición donde todo cae en la misma columna es correcta y olvidable; un solo elemento que se sale, a propósito, le da ritmo a toda la pantalla."},
	{"motion", "QUE EL MOVIMIENTO TENGA UNA CAUSA Y SE SIENTA. Una transición de 80 ms no se percibe y una de 500 ms molesta: 150–250 ms, y sólo donde algo cambió de estado."},
	{"densidad", "LA DENSIDAD ES UNA DECISIÓN, NO UN PROMEDIO. O es una cabina que muestra mucho y se lee rápido, o es una pantalla que respira y muestra poco. El punto medio no se ve elegido, se ve tibio."},
	{"navegacion", "QUE SE SEPA DÓNDE ESTÁ PARADO SIN LEER. El lugar actual se marca con peso y color, no sólo con un subrayado de 1px."},
	{"movil", "EN EL TELÉFONO LA JERARQUÍA IMPORTA MÁS, NO MENOS. Menos ancho es menos lugar para repartir: el protagonista ocupa más, y lo secundario se va abajo o se esconde."},
	{"a11y", "EL CONTRASTE ALTO NO ES UNA CONCESIÓN, ES MEJOR DISEÑO. Lo que se lee de lejos también se lee mejor de cerca."},
	{"onboarding", "LA PRIMERA PANTALLA TIENE QUE PROMETER ALGO. Quien recién llega decide en segundos si vale la pena: decile qué va a poder hacer, no cómo se configura."},
	{"chat", "EL MENSAJE ES EL PROTAGONISTA. Todo lo que rodea a la conversación —barras, íconos, metadatos— tiene que retroceder para que se lea el texto."},
	{"filtros", "QUE SE VEA QUÉ ESTÁ FILTRANDO SIN ABRIR NADA. Lo aplicado va visible y con peso; si hay que abrir un panel para saber qué se está viendo, el filtro esconde en vez de acotar."},
}

// designExigenciasPorEje acota cuántas exigencias DE EJE entran, igual que el checklist de rechazo.
const designExigenciasPorEje = 2

// exigenciasPara arma el bloque, con el núcleo universal más las del eje ruteado.
func exigenciasPara(eje string) string {
	var b strings.Builder
	b.WriteString("EXIGÍTE ESTO (si tu diseño no lo tiene, todavía no está terminado):")
	for _, x := range exigenciasDeDiseno {
		if x.Eje == "" {
			b.WriteString("\n- ")
			b.WriteString(x.Texto)
		}
	}
	if eje != "" {
		n := 0
		for _, x := range exigenciasDeDiseno {
			if x.Eje != eje || n >= designExigenciasPorEje {
				continue
			}
			b.WriteString("\n- ")
			b.WriteString(x.Texto)
			n++
		}
	}
	// LA RECONCILIACIÓN VA ADENTRO DEL BLOQUE, no en un comentario del código. Un agente que
	// recibe «hacé una jugada fuerte» y «no uses el color como adorno» sin que nadie le explique
	// cómo conviven, resuelve la tensión bajando las dos y entrega algo tibio.
	b.WriteString("\n\nCÓMO CONVIVE ESTO CON 'avoid': la exigencia dice DÓNDE gastar la audacia —en un solo lugar— y la prohibición dice dónde NO gastarla —en todos los demás. No se contradicen. Un diseño potente es un movimiento fuerte rodeado de silencio, no seis movimientos medianos.")
	return b.String()
}
