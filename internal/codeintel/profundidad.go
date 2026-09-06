package codeintel

import "fmt"

// profundidad.go deriva CUÁNTA revisión merece un cambio, a partir del cambio mismo.
//
// EL PROBLEMA QUE RESUELVE. La skill adversarial-review sugiere hoy `rounds=2 quorum=2 de 3`
// para cualquier cosa: un typo en un comentario y una refactorización con cuarenta llamadores
// reciben exactamente el mismo panel. Un criterio que no distingue no es un criterio — es un
// número fijo con aire de decisión. Y el costo lo paga siempre el mismo lado: revisar de más
// enseña a saltearse la revisión.
//
// Musubi ya calculaba todas las señales que hacen falta —archivos y rangos del diff, símbolos
// tocados, radio de impacto, cobertura del índice— pero nadie las juntaba: no había una sola
// ruta de código donde un FileDiff terminara en una llamada al grafo.
//
// ESCALONES, NO UNA CURVA. Cada señal aporta 0, 1 ó 2 puntos según en qué escalón cae. Una
// curva continua daría un número más "fino" y peor: se movería por una línea de ruido y nadie
// podría recomputarlo de cabeza. Con escalones, quien lee el veredicto puede rehacer la cuenta
// y discutirla, que es la única forma de que un número automático no se acepte por inercia.
//
// MODEL-FREE Y PURA. Acá no hay juicio: sólo aritmética sobre enteros. Quién decide si el
// cambio está bien sigue siendo el panel. Y es pura a propósito —las señales entran como
// números, no se van a buscar— para que la pueda llamar tanto el MCP (que tiene el grafo)
// como el gate del turno (que no tiene ni base ni MCP).

// NivelRevision es cuánta revisión pide un cambio.
type NivelRevision string

const (
	NivelMinimo   NivelRevision = "minima"
	NivelEstandar NivelRevision = "estandar"
	NivelProfundo NivelRevision = "profunda"
)

// Panel es la forma concreta del debate para un nivel: a cuántos escépticos convocar, cuántas
// rondas de crítica cruzada y cuántos votos necesita el ganador.
type Panel struct {
	Jueces int `json:"jueces"`
	Rondas int `json:"rondas"`
	Quorum int `json:"quorum"`
}

// Los tres paneles. El piso y el techo no son gustos:
//
//   - PISO: nunca menos de un juez y una ronda. Un cambio "mínimo" igual se mira; lo que baja
//     es el costo, no la existencia de la revisión.
//   - TECHO: nunca más de dos rondas, porque DOS es lo que el motor de debate soporta —con más,
//     AdvanceDebate se vuelve un no-op mudo y el panel cree que debatió cuando no debatió—.
var (
	panelMinimo   = Panel{Jueces: 1, Rondas: 1, Quorum: 1}
	panelEstandar = Panel{Jueces: 3, Rondas: 2, Quorum: 2}
	panelProfundo = Panel{Jueces: 5, Rondas: 2, Quorum: 3}
)

// Senales son las siete entradas del cálculo. Las seis primeras puntúan 0-2; la séptima, 0-1.
type Senales struct {
	// Del diff.
	Archivos int `json:"archivos"`
	Lineas   int `json:"lineas"`
	Simbolos int `json:"simbolos"`
	Paquetes int `json:"paquetes"`
	// Del grafo de código: el radio de impacto del cambio.
	CallersEnRadio  int `json:"callers_en_radio"`
	PaquetesEnRadio int `json:"paquetes_en_radio"`
	// Séptima: si el cambio SACA código, no si tocó una línea que existía.
	//
	// La distinción no es un matiz — es la diferencia entre una señal y una constante. En un
	// diff unificado, MODIFICAR una línea se representa como un borrado más un agregado, así
	// que «hay al menos una línea borrada» es cierto en casi todo cambio: medido sobre los PRs
	// de este repo, se encendía en el 83%. Una señal que se enciende cuatro de cada cinco veces
	// no ordena nada; le suma un punto a todo el mundo y desplaza la escala entera.
	//
	// Lo que sí es raro y sí importa —un archivo borrado entero, o uno donde el cambio saca más
	// de lo que pone— pasa en el 12%. Eso es lo que mide ahora.
	HayBorrados bool `json:"hay_borrados"`

	// RadioCiego es el PISO DE HONESTIDAD, y no es una señal más: dice que el radio de impacto
	// no se pudo medir para al menos un archivo tocado (el grafo no lo indexa, o no tiene un
	// solo nodo suyo). Ver aplicarPisoDeHonestidad para qué hace.
	RadioCiego bool `json:"radio_ciego"`
	// MotivoCiego explica POR QUÉ el radio quedó ciego, para que el veredicto lo pueda decir.
	MotivoCiego string `json:"motivo_ciego,omitempty"`
}

// Senal es una señal ya puntuada, para que el veredicto muestre la cuenta y no sólo el total.
// Un número que no se puede auditar se obedece o se ignora, pero no se discute.
type Senal struct {
	Nombre string `json:"nombre"`
	Valor  string `json:"valor"`
	Punto  int    `json:"punto"`
}

// Veredicto es el resultado: el nivel, el panel que le corresponde y la cuenta que lo produjo.
type Veredicto struct {
	Nivel   NivelRevision `json:"nivel"`
	Puntos  int           `json:"puntos"`
	Panel   Panel         `json:"panel"`
	Senales []Senal       `json:"senales"`
	Motivos []string      `json:"motivos"`
}

// escalon puntúa una señal entera: 0 si no pasa de `bajo`, 1 si no pasa de `alto`, 2 si más.
func escalon(valor, bajo, alto int) int {
	switch {
	case valor <= bajo:
		return 0
	case valor <= alto:
		return 1
	default:
		return 2
	}
}

// Los escalones y los cortes, CALIBRADOS CONTRA LOS PRs DE ESTE REPO (2026-09-06).
//
// La primera versión eligió estos números a ojo, y la medición mostró lo caro que sale: sobre
// los 108 PRs reales —descontados los back-merges, que no son un PR sino «todo lo que main
// ganó», y los duplicados— el reparto era 1% mínima / 30% estándar / 67% profunda. Dos de cada
// tres cambios pedían el panel más caro, así que el número había dejado de ser una decisión.
//
// La causa era que cada `alto` estaba POR DEBAJO de la mediana observada: `lineas` topeaba en
// 200 con una mediana de 373, `simbolos` en 9 con una mediana de 15. Seis de cada diez PRs
// sacaban el máximo en esas dos, y una señal que casi siempre topea no ordena nada.
//
// Ahora cada borde sale de un cuantil medido —`bajo` cerca del P33, `alto` cerca del P75— y se
// redondea a un número que una persona pueda rehacer de cabeza, que es la razón de ser de los
// escalones. Reparto resultante: 22% / 49% / 28%.
//
// AL RECALIBRAR: la medición se rehace con el grafo indexado (sin él las dos señales del radio
// dan 0 y todo puntaje es un piso), sobre el diff de cada merge contra su primer padre, y
// llamando a estas mismas funciones. Una reimplementación mediría otra cosa.
const (
	archivosBajo, archivosAlto = 4, 12
	lineasBajo, lineasAlto     = 150, 800
	simbolosBajo, simbolosAlto = 7, 40
	paquetesBajo, paquetesAlto = 2, 5
	callersBajo, callersAlto   = 4, 35
	radioPkgBajo, radioPkgAlto = 1, 2

	// Los cortes de nivel sobre el total de 0..13.
	corteMinima   = 2
	corteEstandar = 7
)

// Profundidad convierte las señales en un nivel de revisión.
//
// El total va de 0 a 13 (seis señales de hasta 2 puntos, más la de borrados). Los cortes:
//
//	0-2   mínima     1 juez  · 1 ronda  · quórum 1
//	3-7   estándar   3 jueces· 2 rondas · quórum 2
//	>=8   profunda   5 jueces· 2 rondas · quórum 3
func Profundidad(s Senales) Veredicto {
	puntoBorrados := 0
	if s.HayBorrados {
		puntoBorrados = 1
	}
	senales := []Senal{
		{"archivos", fmt.Sprint(s.Archivos), escalon(s.Archivos, archivosBajo, archivosAlto)},
		{"lineas", fmt.Sprint(s.Lineas), escalon(s.Lineas, lineasBajo, lineasAlto)},
		{"simbolos", fmt.Sprint(s.Simbolos), escalon(s.Simbolos, simbolosBajo, simbolosAlto)},
		{"paquetes", fmt.Sprint(s.Paquetes), escalon(s.Paquetes, paquetesBajo, paquetesAlto)},
		{"callers_en_radio", fmt.Sprint(s.CallersEnRadio), escalon(s.CallersEnRadio, callersBajo, callersAlto)},
		{"paquetes_en_radio", fmt.Sprint(s.PaquetesEnRadio), escalon(s.PaquetesEnRadio, radioPkgBajo, radioPkgAlto)},
		{"hay_borrados", fmt.Sprint(s.HayBorrados), puntoBorrados},
	}

	puntos := 0
	for _, x := range senales {
		puntos += x.Punto
	}

	v := Veredicto{Puntos: puntos, Senales: senales}
	switch {
	case puntos <= corteMinima:
		v.Nivel = NivelMinimo
	case puntos <= corteEstandar:
		v.Nivel = NivelEstandar
	default:
		v.Nivel = NivelProfundo
	}
	v.Motivos = append(v.Motivos, fmt.Sprintf("%d punto(s) sobre 13 ⇒ %s", puntos, v.Nivel))
	aplicarPisoDeHonestidad(&v, s)
	v.Panel = panelDe(v.Nivel)
	return v
}

// aplicarPisoDeHonestidad sube el nivel cuando el radio de impacto NO se pudo medir.
//
// ESTO ES LO QUE HACE HONESTO AL NÚMERO, y conviene entender por qué. El radio de impacto es
// estructuralmente un PISO, nunca un techo: el grafo omite a propósito las llamadas que no
// resuelve, y sólo indexa .go salvo que el binario traiga los seis tags de treesitter.
// Entonces «radio 0» significa DOS cosas incompatibles —«no arrastra a nadie» y «no puedo
// saberlo»— y las dos puntúan igual: cero.
//
// Un cero que puede querer decir «no pude medir» es el valor de fallo disfrazado de valor
// tranquilizador. Así que cuando el radio queda ciego, el nivel NO puede bajar a mínima, y el
// motivo va escrito en el veredicto en vez de quedar en un cero mudo.
//
// Efecto práctico que conviene aceptar de entrada: en este repo hay archivos .go sin un solo
// nodo en el grafo, así que «mínima» va a ser raro hasta que alguien indexe. Es correcto y es
// honesto, pero va a parecer un bug si nadie lo escribió antes. Queda escrito.
func aplicarPisoDeHonestidad(v *Veredicto, s Senales) {
	if !s.RadioCiego || v.Nivel != NivelMinimo {
		return
	}
	v.Nivel = NivelEstandar
	motivo := s.MotivoCiego
	if motivo == "" {
		motivo = "el grafo no cubre alguno de los archivos tocados"
	}
	v.Motivos = append(v.Motivos,
		"sube a estandar: el radio de impacto no se pudo medir ("+motivo+"), y un radio 0 sin cobertura "+
			"no dice «no arrastra a nadie» sino «no puedo saberlo»")
}

// panelDe traduce nivel a panel. Un nivel desconocido cae en estándar: ante la duda se revisa
// de más, que es el error barato.
func panelDe(n NivelRevision) Panel {
	switch n {
	case NivelMinimo:
		return panelMinimo
	case NivelProfundo:
		return panelProfundo
	default:
		return panelEstandar
	}
}

// SenalesDelDiff deriva del diff las cuatro señales que no necesitan el grafo, más la de
// borrados. Las dos del radio (callers y paquetes) las completa quien tenga el grafo a mano:
// esta función no va a buscar nada, para poder correr donde no hay base ni MCP.
//
// `Lineas` suma agregadas Y borradas: sin eso, un hunk de borrado puro —que no deja rango
// nuevo— mediría igual que no tocar nada. Ese punto ciego lo cubre ESTA cuenta, no la séptima
// señal; por eso la séptima puede permitirse ser exigente.
//
// Los binarios se saltean, igual que en detect_changes: no tienen líneas ni símbolos que
// revisar y contarlos como archivo inflaría la señal más barata de inflar.
func SenalesDelDiff(files []FileDiff, simbolos int) Senales {
	s := Senales{Simbolos: simbolos}
	paquetes := map[string]bool{}
	for _, fd := range files {
		if fd.Binary {
			continue
		}
		s.Archivos++
		s.Lineas += fd.Agregadas + fd.Borradas
		paquetes[directorioDe(fd.Path)] = true
		// El predicado se evalúa POR ARCHIVO, no sobre el total del cambio: el acto destructivo
		// es que UN archivo pierda su contenido, y un PR que borra un módulo mientras agrega
		// otro más grande lo escondería si se sumara todo antes de comparar.
		//
		// `Borradas > Agregadas` es lo que distingue sacar de editar: una modificación aporta
		// un borrado y un agregado por línea, así que empata; sólo sale positivo cuando el
		// cambio se lleva más de lo que trae.
		if fd.ChangeType == ChangeDeleted || fd.Borradas > fd.Agregadas {
			s.HayBorrados = true
		}
	}
	s.Paquetes = len(paquetes)
	return s
}

// directorioDe devuelve el directorio de un path del diff, que en Go equivale al paquete.
// Se usa path textual (siempre "/" en la salida de git), no filepath, para que la señal no
// dependa del separador del sistema donde corre.
func directorioDe(p string) string {
	if i := lastSlash(p); i >= 0 {
		return p[:i]
	}
	return "."
}

func lastSlash(p string) int {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return i
		}
	}
	return -1
}
