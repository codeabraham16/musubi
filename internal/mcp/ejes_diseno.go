package mcp

// ejes_diseno.go — LA TAXONOMÍA DEL ACERVO DE DISEÑO (Musubi Renaissance · plan de cierre, fase 2).
//
// POR QUÉ EXISTE, medido el 2026-08-30 contra el acervo real de 1.736 entradas:
//
// El motor elegía qué servir por similitud del embebedor sobre 1.438 tarjetas, y eso no funcionaba:
// dos maneras de pedir lo mismo devolvían material distinto (M1 = 0,10 sobre 16 pedidos reales).
// La causa medida es que **dos tarjetas de diseño al azar se parecen tanto como una consulta a su
// mejor resultado** — coseno p99 entre pares al azar 0,668 contra 0,533–0,643 de una consulta real.
// A esa granularidad el embebedor no separa nada.
//
// Lo que sí separa, y es el hallazgo que hace posible este archivo: **el mismo embebedor discrimina
// entre 19 EJES bien separados.** El eje top-1 coincide en las tres paráfrasis de un pedido el 73 %
// de las veces, contra el 10 % de las tarjetas. El paso consulta→eje es 7× más estable que
// consulta→tarjeta. No era un embebedor malo: era la granularidad equivocada.
//
// M1 simulada de punta a punta, sobre el acervo real y los 16 pedidos del set dorado:
//
//	motor por similitud sobre tarjetas ....... 0,10
//	ruteo léxico por eje ..................... 0,23
//	eje por embebedor top-2 → tarjetas ....... 0,30
//	eje por embebedor top-1 → tarjetas ....... 0,50
//
// TOP-1 Y NO TOP-2, y es contraintuitivo: sumar el segundo eje EMPEORA (0,30 contra 0,50). El
// segundo eje es bastante menos estable que el primero y lo único que aporta es varianza.

import (
	"context"
	"math"
	"regexp"
	"strings"
	"unicode"

	"musubi/internal/embedding"
)

// ejeDiseno es un eje de la taxonomía: su nombre corto (la etiqueta con la que se rutea), la
// DESCRIPCIÓN que se embebe, y el vocabulario con el que se reconoce una tarjeta que habla de él.
type ejeDiseno struct {
	Nombre string
	Desc   string
	Vocab  string
}

// SE EMBEBE LA DESCRIPCIÓN, NO EL NOMBRE, y es la diferencia entre que ande y no ande: el acervo
// casi nunca dice «a11y» — dice «contraste», «lector de pantalla», «foco visible». Embeber la
// etiqueta dejaría al eje sin nada parecido del otro lado.
var ejesDeDiseno = []ejeDiseno{
	{"tabla", "tablas de datos: columnas, filas, celdas, encabezados, ordenar, grilla de datos densa",
		"tabla tablas columna columnas fila filas celda celdas grilla encabezado ordenar zebra tabular"},
	{"formulario", "formularios: campos de entrada, validacion, errores, etiquetas, enviar datos",
		"formulario campo campos input validacion validar error etiqueta label placeholder enviar"},
	{"login", "pantalla de inicio de sesion: credenciales, contrasena, autenticacion, acceso, registro",
		"login sesion ingresar credencial contrasena password autenticacion registro acceso"},
	{"dashboard", "tablero de control: metricas, indicadores, widgets, resumen del estado del sistema",
		"dashboard tablero panel metrica metricas indicador kpi widget resumen"},
	{"dataviz", "visualizacion de datos: graficos, series, barras, lineas, ejes, escalas, leyendas",
		"grafico graficos serie series barra barras linea eje escala leyenda visualizacion datos"},
	{"navegacion", "navegacion: menu, barra lateral, pestanas, migas de pan, moverse entre secciones",
		"navegacion menu barra breadcrumb pestana ruta enlace sidebar"},
	{"jerarquia", "jerarquia visual: que se lee primero, enfasis, prioridad, accion principal",
		"jerarquia jerarquico primario secundario prioridad enfasis titulo peso contraste importancia"},
	{"color", "color: paleta, acento, tono, saturacion, modo oscuro y claro",
		"color colores paleta acento tono saturacion contraste oscuro claro matiz"},
	{"densidad", "densidad de informacion: compacto contra holgado, espaciado, aire, cuanto entra",
		"densidad denso compacto espaciado aire respirar padding margen holgado apretado"},
	{"layout", "layout y composicion: grilla, columnas, ancho, alineacion, estructura de la pagina",
		"layout grilla columnas ancho contenedor alineacion espaciado composicion estructura seccion"},
	{"motion", "animacion e interaccion: transiciones, movimiento, duracion, respuesta al gesto",
		"animacion animar transicion movimiento duracion easing motion desplazamiento"},
	{"a11y", "accesibilidad: contraste, lector de pantalla, teclado, foco visible, semantica",
		"accesibilidad accesible contraste lector teclado foco aria wcag semantico alternativo"},
	{"microcopy", "microcopy y redaccion: el texto de la interfaz, mensajes, tono de voz, claridad",
		"microcopy texto copy mensaje redaccion palabra tono frase etiqueta leer"},
	{"estado", "estados de la interfaz: cargando, error, exito, deshabilitado, pendiente",
		"estado estados carga cargando error exito pendiente deshabilitado activo"},
	{"estado-vacio", "estado vacio: cuando todavia no hay datos, primera vez, cero resultados",
		"vacio vacia ningun nada primera cero placeholder inicial"},
	{"filtros", "filtrado y busqueda: filtros, criterios, refinar resultados, buscar",
		"filtro filtros filtrar buscar busqueda facet criterio refinar"},
	{"movil", "movil: pantalla pequena, tactil, responsive, gestos, telefono",
		"movil telefono pantalla pequena touch tactil responsive breakpoint gesto"},
	{"chat", "chat y mensajeria: conversacion, mensajes, hilo, burbujas, escribir y responder",
		"chat mensaje mensajes conversacion burbuja hilo escribir responder"},
	{"onboarding", "onboarding: bienvenida, primeros pasos, guia inicial, configuracion inicial",
		"onboarding bienvenida primer paso tutorial guia alta configuracion"},
}

// designEjeMinHits es cuántos términos del vocabulario de un eje tiene que compartir una tarjeta
// para quedar etiquetada con él. Dos y no uno: con uno, cualquier tarjeta que diga «contraste» una
// sola vez cae a la vez en `color`, `jerarquia` y `a11y`, y la etiqueta deja de significar algo.
// Medido con este umbral, el 41 % del acervo recibe al menos un eje.
const designEjeMinHits = 2

// designEjeMinSim es el piso de similitud para aceptar el eje que eligió el embebedor. Por debajo,
// el motor NO rutea y cae al camino por similitud: un pedido que no se parece a ningún eje es
// justamente el caso donde forzar una taxonomía inventa una respuesta.
const designEjeMinSim = 0.45

var noAlfaNum = regexp.MustCompile(`[^a-z0-9]+`)

// palabrasNormalizadas devuelve las palabras de 4+ letras de s, en minúscula y SIN acentos. Sin
// quitar los acentos, «validacion» del vocabulario no matchea «validación» del acervo, y el
// etiquetado se pierde justo las tarjetas escritas con más cuidado.
func palabrasNormalizadas(s string) map[string]bool {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		b.WriteRune(sinAcento(r))
	}
	out := map[string]bool{}
	for _, w := range noAlfaNum.Split(b.String(), -1) {
		if len(w) >= 4 {
			out[w] = true
		}
	}
	return out
}

// sinAcento mapea las vocales acentuadas y la eñe del castellano a su letra base. Es deliberadamente
// una tabla y no unicode.Mn + normalización NFD: sólo hay que cubrir el castellano, y una tabla de
// diez runas se lee de un vistazo.
func sinAcento(r rune) rune {
	switch r {
	case 'á', 'à', 'ä', 'â':
		return 'a'
	case 'é', 'è', 'ë', 'ê':
		return 'e'
	case 'í', 'ì', 'ï', 'î':
		return 'i'
	case 'ó', 'ò', 'ö', 'ô':
		return 'o'
	case 'ú', 'ù', 'ü', 'û':
		return 'u'
	case 'ñ':
		return 'n'
	}
	if r > unicode.MaxASCII {
		return ' '
	}
	return r
}

// vocabDeEje son los vocabularios ya normalizados, UNA vez. Etiquetar una tarjeta recorre 19 ejes;
// recalcular la bolsa de cada vocabulario adentro de ese bucle multiplicaba el trabajo por 19 en el
// camino caliente, y son constantes del binario.
var vocabDeEje = func() map[string]map[string]bool {
	m := make(map[string]map[string]bool, len(ejesDeDiseno))
	for _, e := range ejesDeDiseno {
		m[e.Nombre] = palabrasNormalizadas(e.Vocab)
	}
	return m
}()

// ejesDeTarjeta etiqueta una tarjeta: devuelve los ejes cuyo vocabulario comparte al menos
// designEjeMinHits términos con su topic + contenido.
func ejesDeTarjeta(topic, contenido string) map[string]bool {
	bolsa := palabrasNormalizadas(topic + " " + contenido)
	out := map[string]bool{}
	for nombre, vocab := range vocabDeEje {
		hits := 0
		for w := range vocab {
			if bolsa[w] {
				hits++
				if hits >= designEjeMinHits {
					out[nombre] = true
					break
				}
			}
		}
	}
	return out
}

// ─────────────────────────────────────────────────────────────────────────────────────────────────
// EL RUTEO
// ─────────────────────────────────────────────────────────────────────────────────────────────────

// vectoresDeEje embebe las 19 descripciones UNA sola vez por proceso y las cachea. Son constantes
// del binario, así que recalcularlas por pedido sería 19 llamadas al embebedor para obtener siempre
// lo mismo — y el embebedor se comparte con recall y save.
func (s *McpServer) vectoresDeEje(ctx context.Context) map[string][]float32 {
	s.ejesMu.Lock()
	defer s.ejesMu.Unlock()
	if s.ejesVec != nil {
		return s.ejesVec
	}
	if !embedding.Enabled(s.embedder) {
		return nil
	}
	vecs := make(map[string][]float32, len(ejesDeDiseno))
	for _, e := range ejesDeDiseno {
		embCtx, cancel := context.WithTimeout(ctx, designEmbedTimeout)
		v, err := s.embedder.Embed(embCtx, e.Desc)
		cancel()
		if err != nil {
			// Sin la tabla COMPLETA no se rutea. Con la tabla a medias, un eje que no se pudo
			// embeber nunca gana y el ruteo manda en silencio a los pedidos de ese tema a otro
			// lado — una falla que se lee como una decisión.
			return nil
		}
		vecs[e.Nombre] = v
	}
	s.ejesVec = vecs
	return vecs
}

// ejeDeConsulta elige el eje top-1 para el vector de una consulta. Devuelve el nombre, su similitud
// y si superó el piso.
//
// TOP-1 Y NO TOP-2: medido el 2026-08-30, sumar el segundo eje BAJA M1 de 0,50 a 0,30. El segundo
// eje es mucho menos estable entre paráfrasis y lo único que agrega es varianza.
func (s *McpServer) ejeDeConsulta(ctx context.Context, vec []float32) (string, float32, bool) {
	vecs := s.vectoresDeEje(ctx)
	if len(vecs) == 0 || len(vec) == 0 {
		return "", 0, false
	}
	mejor, mejorSim := "", float32(-1)
	for _, e := range ejesDeDiseno { // recorrido en orden fijo: el desempate no puede salir de un map
		sim := cosenoDe(vec, vecs[e.Nombre])
		if sim > mejorSim {
			mejor, mejorSim = e.Nombre, sim
		}
	}
	return mejor, mejorSim, mejorSim >= designEjeMinSim
}

// cosenoDe asume vectores del mismo largo; si no lo son devuelve 0 en vez de entrar en pánico.
func cosenoDe(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}

// tarjetasDelEje trae las tarjetas del acervo etiquetadas con `eje`, en orden de IMPORTANCIA.
//
// Importancia y no similitud, y no es un detalle: el orden por similitud es justamente el que no es
// reproducible entre paráfrasis (M1 = 0,10). Una vez que el eje ya acotó el tema, el desempate tiene
// que ser una propiedad de la TARJETA y no de cómo se escribió el pedido. La simulación que decidió
// esta fase midió 0,50 con este orden exacto.
func (s *McpServer) tarjetasDelEje(eje string, n int) []searchSource {
	obs, err := s.engine.ObservationsByTopicPrefixInProject(designCorpusScope, designCorpusPrefix, designBarridoEjes)
	if err != nil {
		return nil
	}
	out := make([]searchSource, 0, n)
	for _, o := range obs {
		if !ejesDeTarjeta(o.TopicKey, o.Content)[eje] {
			continue
		}
		out = append(out, searchSource{id: o.ID, topicKey: o.TopicKey, content: o.Content})
		if len(out) >= n {
			break
		}
	}
	return out
}
