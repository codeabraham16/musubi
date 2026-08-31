package mcp

// esqueleto_diseno.go — LOS TRES BOCETOS, DIBUJADOS POR EL MOTOR.
//
// POR QUÉ EXISTE. El usuario pidió (2026-08-31) que antes de componer se le muestren tres modelos
// para elegir uno: *«si querés te muestro 3 lienzos y de ahí partimos con un modelo»*. Y eligió el
// flujo escalonado — bocetos primero, el elegido después — porque descartar tiene que ser barato.
//
// LOS DIBUJA EL MOTOR Y NO QUIEN COMPONE, y eso no es un detalle de implementación: si cada boceto lo
// improvisa el agente, las tres salen con criterios distintos y dejan de ser comparables. Elegir
// entre tres dibujos que no comparten reglas no es elegir un modelo, es elegir cuál quedó mejor
// dibujado — que es exactamente el ruido que este paso viene a sacar.
//
// Y ADEMÁS HACE VISIBLE `scale`. La cantidad de filas de un boceto NO está escrita a mano: sale del
// alto de fila del registro de esa forma. Así el boceto compacto muestra de verdad más filas que el
// editorial, y la decisión numérica del motor se ve en vez de leerse.
//
// SIGUE SIENDO MODEL-FREE: una tabla de esqueletos y aritmética de cajas. No hay LLM, no hay azar, y
// el mismo pedido da el mismo SVG byte a byte.

import (
	"fmt"
	"strconv"
	"strings"
)

// celda es una caja del boceto. `Repite` y `Cols` la subdividen: una tabla es UNA celda de rol `fila`
// que se repite, no doce celdas escritas a mano — si estuvieran escritas, la cantidad de filas sería
// una constante y el boceto dejaría de reflejar el registro.
type celda struct {
	Rol    string // qué vive ahí; decide cómo se dibuja
	Peso   int    // ancho relativo dentro de su banda
	Repite int    // sub-bloques apilados; 0 = lo calcula el registro (sólo para `fila`)
	Cols   int    // sub-bloques por fila; 0 o 1 = una columna
}

// banda es una franja horizontal de la pantalla. El esqueleto es una pila de bandas, y dentro de cada
// banda las celdas se reparten el ancho. Con eso alcanza para las doce formas: lo que las distingue
// es la PROPORCIÓN y qué manda, no un vocabulario de layout más grande.
type banda struct {
	Peso   int // alto relativo
	Celdas []celda
}

// esqueletosDeForma es el dibujo de cada forma. Los pesos dicen qué manda: en `tablero-un-numero` la
// cifra se lleva 7 de 11 de alto, y esa desproporción ES la forma — no un adorno del boceto.
var esqueletosDeForma = map[string][]banda{
	"tabla-densa": {
		{1, []celda{{"titular", 3, 0, 0}, {"accion", 1, 0, 0}}},
		{1, []celda{{"filtro", 1, 0, 0}}},
		{9, []celda{{"fila", 1, 0, 0}}},
	},
	"lista-priorizada": {
		{4, []celda{{"destacado", 1, 0, 0}}},
		{6, []celda{{"fila", 1, 5, 0}}},
	},
	"tablero-un-numero": {
		{6, []celda{{"cifra", 1, 0, 0}}},
		{2, []celda{{"texto", 1, 0, 0}}},
		{3, []celda{{"tarjeta", 1, 1, 3}}},
	},
	"formulario-guiado": {
		{2, []celda{{"titular", 1, 0, 0}}},
		{4, []celda{{"campo", 1, 2, 0}}},
		{3, []celda{{"campo", 1, 1, 2}}},
		{2, []celda{{"accion", 1, 0, 0}}},
	},
	"detalle-con-lados": {
		{1, []celda{{"titular", 1, 0, 0}}},
		{9, []celda{{"cuerpo", 7, 0, 0}, {"lateral", 3, 0, 0}}},
	},
	"catalogo-elegir": {
		{1, []celda{{"filtro", 1, 0, 0}}},
		{9, []celda{{"tarjeta", 1, 3, 3}}},
	},
	"monitor-procesos": {
		{1, []celda{{"titular", 1, 0, 0}}},
		{9, []celda{{"barra", 1, 6, 0}}},
	},
	"conversacion": {
		{8, []celda{{"mensajes", 1, 0, 0}}},
		{2, []celda{{"campo", 1, 1, 0}}},
	},
	"lienzo-inspector": {
		{10, []celda{{"lienzo", 7, 0, 0}, {"lateral", 3, 0, 0}}},
	},
	"interrupcion": {
		{3, []celda{{"aire", 1, 0, 0}}},
		{3, []celda{{"titular", 1, 0, 0}}},
		{2, []celda{{"texto", 1, 0, 0}}},
		{2, []celda{{"accion", 1, 0, 0}}},
		{2, []celda{{"aire", 1, 0, 0}}},
	},
	"rejilla-temporal": {
		{1, []celda{{"titular", 1, 0, 0}}},
		{9, []celda{{"rejilla", 1, 5, 7}}},
	},
	"narrativa": {
		{4, []celda{{"titular", 1, 0, 0}}},
		{2, []celda{{"texto", 1, 0, 0}}},
		{2, []celda{{"texto", 1, 0, 0}}},
		{3, []celda{{"texto", 1, 0, 0}}},
	},
}

// Medidas del boceto. Chico a propósito: es una miniatura para comparar TRES de un vistazo, no una
// pantalla. Lo que tiene que leerse es la proporción y la densidad, no el contenido.
const (
	bocetoAncho  = 300
	bocetoAlto   = 190
	bocetoMargen = 10
	bocetoGap    = 4
)

// filasQueEntran deriva de `scale` cuántas filas dibuja una banda: alto disponible dividido el alto de
// fila del registro, llevado a la escala de la miniatura.
//
// ES EL NUDO ENTRE LAS DOS CAPAS. Si la cantidad fuera una constante, el boceto compacto y el
// editorial mostrarían las mismas filas y la decisión de densidad quedaría invisible justo en el paso
// que existe para hacerla visible. Con esto, un registro de 32 px de fila dibuja la mitad más que uno
// de 48.
func filasQueEntran(altoBanda float64, reg registroEscala) int {
	if reg.Fila <= 0 {
		return 6
	}
	// La miniatura mide 190 px de alto y representa una pantalla de ~570: factor 3.
	//
	// Era 4 y lo bajé, pero por un motivo más chico del que escribí primero: con 4, el registro
	// compacto daba 16,5 filas y quedaba CLAVADO EN EL TOPE, así que la proporción entre registros
	// salía recortada (16 contra 10,8 = 1,48) en vez de exacta. Compacto y estándar seguían dando
	// números distintos — mi primera versión de este comentario decía que daban lo mismo y eso era
	// falso, no lo había medido. Con 3 ninguno toca el tope y la proporción dibujada es la proporción
	// real de los altos de fila: 12 · 9 · 8 filas, que es 48/32 clavado.
	const factorMiniatura = 3
	n := int(altoBanda * factorMiniatura / float64(reg.Fila))
	if n < 2 {
		return 2
	}
	if n > 16 {
		return 16 // más que esto es una trama gris: deja de comunicar densidad y la simula
	}
	return n
}

// bocetoDe dibuja una forma con los números de su registro. Devuelve "" si la forma no tiene
// esqueleto — y ese vacío se declara arriba en vez de emitir un SVG en blanco, que se leería como
// «esta forma es una pantalla vacía».
func bocetoDe(forma string) string {
	bandas, ok := esqueletosDeForma[forma]
	if !ok || len(bandas) == 0 {
		return ""
	}
	reg := registrosDeEscala[registroPorDefecto]
	if r, ok := registrosDeEscala[registroDeForma[forma]]; ok {
		reg = r
	}

	totalPeso := 0
	for _, b := range bandas {
		totalPeso += b.Peso
	}
	if totalPeso == 0 {
		return ""
	}

	util := float64(bocetoAlto - 2*bocetoMargen)
	altoUtil := util - float64(bocetoGap*(len(bandas)-1))
	anchoUtil := float64(bocetoAncho - 2*bocetoMargen)

	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg viewBox='0 0 %d %d' width='100%%' role='img' aria-label='boceto: %s'>`,
		bocetoAncho, bocetoAlto, formasDeDiseno[forma].Nombre)
	// DOS AHORROS, Y EL SEGUNDO ES EL QUE IMPORTA.
	//
	// (1) Los atributos que se repiten viven en esta hoja de estilo y no en cada caja: un boceto tiene
	// ~40 cajas y `fill="currentColor"` en todas costaba 22 chars por caja.
	//
	// (2) LAS COMILLAS SON SIMPLES. El SVG viaja adentro de un string JSON, donde cada `"` se escapa a
	// `\"` y cuesta el DOBLE; la comilla simple es XML válido y en JSON no se escapa. Sin esto, medido
	// contra el pedido real de las notas del CRM, los tres bocetos pesaban 4.357 tokens —mucho más que
	// los ~1.500 chars que yo había medido sobre el SVG crudo— y hacían que el brief entero se pasara
	// del tope. El tamaño que importa es el del JSON, no el del dibujo.
	sb.WriteString(`<style>rect{fill:currentColor}.m{fill:none;stroke:currentColor}</style>`)
	// El marco de la pantalla. `currentColor` en todo el boceto: así hereda el color del contenedor y
	// se ve igual en claro y en oscuro sin declarar una paleta — el boceto es estructura, no marca.
	fmt.Fprintf(&sb, `<rect class='m' x='1' y='1' width='%d' height='%d' rx='6' opacity='.18'/>`,
		bocetoAncho-2, bocetoAlto-2)

	y := float64(bocetoMargen)
	for _, b := range bandas {
		h := altoUtil * float64(b.Peso) / float64(totalPeso)
		anchoPeso := 0
		for _, c := range b.Celdas {
			anchoPeso += c.Peso
		}
		if anchoPeso == 0 {
			anchoPeso = 1
		}
		x := float64(bocetoMargen)
		gapH := float64(bocetoGap * (len(b.Celdas) - 1))
		for _, c := range b.Celdas {
			w := (anchoUtil - gapH) * float64(c.Peso) / float64(anchoPeso)
			dibujarCelda(&sb, c, x, y, w, h, reg)
			x += w + float64(bocetoGap)
		}
		y += h + float64(bocetoGap)
	}
	sb.WriteString(`</svg>`)
	return sb.String()
}

func caja(sb *strings.Builder, x, y, w, h, r, op float64) {
	if w <= 0 || h <= 0 {
		return
	}
	fmt.Fprintf(sb, `<rect x='%d' y='%d' width='%d' height='%d' rx='%d' opacity='%s'/>`,
		red(x), red(y), red(w), red(h), red(r), op2(op))
}

func marco(sb *strings.Builder, x, y, w, h, r float64) {
	if w <= 0 || h <= 0 {
		return
	}
	fmt.Fprintf(sb, `<rect class='m' x='%d' y='%d' width='%d' height='%d' rx='%d' opacity='.22'/>`,
		red(x), red(y), red(w), red(h), red(r))
}

// apilar reparte `n` sub-bloques en una caja, en `cols` columnas, y llama a `f` con cada uno. Está
// separado porque las cinco formas que repiten algo —filas, campos, tarjetas, barras, celdas de
// rejilla— comparten exactamente esta aritmética, y escribirla cinco veces es donde se cuela el
// boceto que no cierra por dos píxeles.
func apilar(x, y, w, h float64, n, cols int, f func(cx, cy, cw, ch float64)) {
	if n < 1 {
		n = 1
	}
	if cols < 1 {
		cols = 1
	}
	filas := (n + cols - 1) / cols
	gap := 2.0
	ch := (h - gap*float64(filas-1)) / float64(filas)
	cw := (w - gap*float64(cols-1)) / float64(cols)
	for i := 0; i < n; i++ {
		f(x+float64(i%cols)*(cw+gap), y+float64(i/cols)*(ch+gap), cw, ch)
	}
}

func dibujarCelda(sb *strings.Builder, c celda, x, y, w, h float64, reg registroEscala) {
	switch c.Rol {
	case "aire":
		// No se dibuja, y es el punto: en una interrupción el vacío alrededor ES la forma.

	case "titular":
		caja(sb, x, y, w*0.62, h*0.44, 2, .70)
		caja(sb, x, y+h*0.62, w*0.40, h*0.20, 2, .28)

	case "texto":
		apilar(x, y, w, h, 3, 1, func(cx, cy, cw, ch float64) {
			caja(sb, cx, cy, cw*0.92, ch*0.42, 1.5, .22)
		})

	case "cifra":
		// El protagonista. Ocupa casi toda su banda, que es lo que la forma promete.
		caja(sb, x, y, w*0.46, h*0.86, 4, .78)
		caja(sb, x+w*0.52, y+h*0.30, w*0.34, h*0.14, 2, .26)

	case "fila":
		n := c.Repite
		if n == 0 {
			n = filasQueEntran(h, reg)
		}
		apilar(x, y, w, h, n, 1, func(cx, cy, cw, ch float64) {
			caja(sb, cx, cy, cw*0.34, ch*0.62, 1.5, .30)         // la columna por la que se mira
			caja(sb, cx+cw*0.62, cy, cw*0.16, ch*0.62, 1.5, .20) // números a la derecha
			caja(sb, cx+cw*0.84, cy, cw*0.16, ch*0.62, 1.5, .20) // (la tercera columna)
		})

	case "destacado":
		marco(sb, x, y, w, h, 4)
		caja(sb, x+6, y+6, w*0.52, h*0.26, 2, .72)
		caja(sb, x+6, y+h*0.46, w*0.68, h*0.14, 1.5, .24)
		caja(sb, x+w-52, y+h-20, 46, 13, 6, .55) // la acción, adentro

	case "tarjeta":
		apilar(x, y, w, h, maxInt(c.Repite*maxInt(c.Cols, 1), 1), maxInt(c.Cols, 1), func(cx, cy, cw, ch float64) {
			marco(sb, cx, cy, cw, ch, 3)
			caja(sb, cx+3, cy+3, cw*0.55, minF(ch*0.30, 6), 1.5, .34)
		})

	case "campo":
		apilar(x, y, w, h, maxInt(c.Repite*maxInt(c.Cols, 1), 1), maxInt(c.Cols, 1), func(cx, cy, cw, ch float64) {
			caja(sb, cx, cy, cw*0.34, minF(ch*0.26, 5), 1.5, .26) // la etiqueta
			marco(sb, cx, cy+ch*0.36, cw, ch*0.60, 3)
		})

	case "barra":
		// LOS AVANCES SON DISTINTOS ENTRE SÍ, y no es adorno: con todas las barras al mismo largo el
		// boceto se lee igual que una tabla densa —dos pilas de líneas— y el paso entero deja de
		// separar dos formas que son muy distintas de operar. Y una está DETENIDA: la forma dice que
		// lo detenido rompe la FORMA del bloque, no sólo su color, así que se dibuja hueca.
		avances := []float64{0.86, 0.42, 1.00, 0.18, 0.63}
		i := 0
		apilar(x, y, w, h, maxInt(c.Repite, 1), 1, func(cx, cy, cw, ch float64) {
			hb := minF(ch*0.32, 6)
			caja(sb, cx, cy, cw*0.30, minF(ch*0.30, 5), 1.5, .30)
			if i == 3 { // el detenido
				marco(sb, cx, cy+ch*0.52, cw, hb, hb/2)
			} else {
				caja(sb, cx, cy+ch*0.52, cw, hb, hb/2, .10)
				caja(sb, cx, cy+ch*0.52, cw*avances[i%len(avances)], hb, hb/2, .58)
			}
			i++
		})

	case "filtro":
		for i, p := range []float64{0, 0.19, 0.36, 0.50} {
			op := .18
			if i == 0 {
				op = .48 // el aplicado, siempre visible
			}
			caja(sb, x+w*p, y+h*0.22, w*0.15, h*0.56, 5, op)
		}

	case "accion":
		caja(sb, x+w-58, y+h*0.20, 52, minF(h*0.60, 15), 6, .62)

	case "lateral":
		marco(sb, x, y, w, h, 4)
		apilar(x+5, y+6, w-10, h-12, 6, 1, func(cx, cy, cw, ch float64) {
			caja(sb, cx, cy, cw*0.80, minF(ch*0.34, 5), 1.5, .20)
		})

	case "cuerpo":
		caja(sb, x, y, w*0.56, 8, 2, .55)
		apilar(x, y+16, w, h-16, 7, 1, func(cx, cy, cw, ch float64) {
			caja(sb, cx, cy, cw*0.94, minF(ch*0.36, 5), 1.5, .20)
		})

	case "rejilla":
		// UNA REJILLA TEMPORAL SON LÍNEAS, NO TREINTA Y CINCO TARJETAS. Dibujada con `tarjeta` salían
		// 70 cajas —marco + etiqueta por celda— y 5.303 chars, más que los otros dos bocetos juntos, y
		// era lo que empujaba el brief contra el tope duro. Con líneas son ~20 cajas, y además se lee
		// mejor: un calendario se reconoce por la retícula, no por el borde de cada casilla.
		//
		// (Antes probé un guard de «no dibujes el detalle si la celda es chica» creyendo que las
		// celdas medían 5 px. Medí: miden 25. El guard no disparaba nunca y su comentario afirmaba un
		// arreglo que no existía.)
		cols, filas := maxInt(c.Cols, 1), maxInt(c.Repite, 1)
		for i := 0; i <= cols; i++ {
			caja(sb, x+w*float64(i)/float64(cols), y, 1, h, 0, .10)
		}
		for j := 0; j <= filas; j++ {
			caja(sb, x, y+h*float64(j)/float64(filas), w, 1, 0, .10)
		}
		// Lo ocupado: la POSICIÓN es el dato, así que se marcan unas pocas casillas y no todas.
		cw, ch := w/float64(cols), h/float64(filas)
		for _, p := range [][2]int{{1, 0}, {2, 0}, {4, 1}, {0, 2}, {3, 2}, {5, 2}, {2, 3}, {6, 4}} {
			if p[0] < cols && p[1] < filas {
				caja(sb, x+cw*float64(p[0])+2, y+ch*float64(p[1])+2, cw-4, ch-4, 2, .30)
			}
		}

	case "lienzo":
		marco(sb, x, y, w, h, 4)
		caja(sb, x+w*0.22, y+h*0.24, w*0.34, h*0.30, 3, .30)
		caja(sb, x+w*0.46, y+h*0.48, w*0.28, h*0.24, 3, .18)

	case "mensajes":
		apilar(x, y, w, h, 5, 1, func(cx, cy, cw, ch float64) {
			caja(sb, cx, cy, cw*0.62, ch*0.62, 4, .22)
		})
		caja(sb, x+w*0.42, y+h*0.22, w*0.58, h*0.14, 4, .40) // uno propio, del otro lado

	default:
		marco(sb, x, y, w, h, 3)
	}
}

// red redondea al pixel entero. La miniatura mide 300x190: la decima de pixel no se ve y costaba dos
// caracteres por numero en ~200 numeros por boceto.
func red(v float64) int { return int(v + 0.5) }

// op2 escribe la opacidad sin el cero de adelante ni ceros de mas: ".3" en vez de "0.30".
func op2(op float64) string {
	s := strconv.FormatFloat(op, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimSuffix(s, ".")
	return strings.TrimPrefix(s, "0")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// bocetosDe arma los bocetos de las candidatas, listos para mirar uno al lado del otro. Cada uno
// lleva EN QUÉ GANA porque el boceto solo no lo dice: dos wireframes se parecen mucho más entre sí
// que las pantallas que van a salir de ellos, y sin la etiqueta la elección se vuelve estética.
type bocetoCandidata struct {
	Forma    string `json:"forma"`
	Nombre   string `json:"nombre"`
	GanaEn   string `json:"gana_en"`
	Registro string `json:"registro"`
	SVG      string `json:"svg"`
}

func bocetosDe(formas []string) []bocetoCandidata {
	var out []bocetoCandidata
	for _, f := range formas {
		svg := bocetoDe(f)
		if svg == "" {
			continue
		}
		reg := registroDeForma[f]
		if reg == "" {
			reg = registroPorDefecto
		}
		out = append(out, bocetoCandidata{
			Forma:    f,
			Nombre:   formasDeDiseno[f].Nombre,
			GanaEn:   strings.Join(destacaDe(f), " y "),
			Registro: registrosDeEscala[reg].Nombre,
			SVG:      svg,
		})
	}
	return out
}

// notaDeBocetos es lo que el brief le pide al caller hacer con ellos. Sin esto, tres SVG en un campo
// del JSON se leen como decoración y el agente los ignora — que es como el paso escalonado se
// desarma y volvemos a componer tres pantallas completas para tirar dos.
const notaDeBocetos = `LOS BOCETOS SON PARA MOSTRAR, NO PARA GUARDAR. Son SVG autocontenidos que heredan el color del contenedor (currentColor): pegalos tal cual, uno al lado del otro, con su nombre y su «gana en» debajo. Mostralos ANTES de componer y que la persona elija UNO — descartar tiene que ser barato, y el trabajo caro se gasta una sola vez sobre la estructura ya aprobada. NO compongas las tres pantallas completas: es tres veces el trabajo y dos tercios van a la basura. La cantidad de filas de cada boceto NO es decorativa: sale del alto de fila del registro de esa forma, así que lo que se ve es la densidad real que vas a componer.`

// bocetosSiSePiden y notaSiHayBocetos existen para que la condición viva en UN solo lugar. Con el
// `if` repetido en los dos campos, el modo de falla es servir la nota sin los bocetos —o al revés— y
// que el brief le diga al agente que muestre algo que no llegó.
func bocetosSiSePiden(pedidos bool, formas []string) []bocetoCandidata {
	if !pedidos {
		return nil
	}
	return bocetosDe(formas)
}

func notaSiHayBocetos(pedidos bool, formas []string) string {
	if len(bocetosSiSePiden(pedidos, formas)) == 0 {
		return ""
	}
	return notaDeBocetos
}
