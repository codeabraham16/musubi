package mcp

// intencion_diseno.go — LAS DOS MITADES DE UN REDISEÑO.
//
// POR QUÉ EXISTE. `musubi_design` aceptaba `prompt`, `brand`, `target` y `limit`. **No había dónde
// decir qué se CONSERVA.** Y un pedido de rediseño tiene siempre dos mitades: lo que se mantiene y
// lo que se ataca. El ejemplo con el que lo planteó el usuario el 2026-08-31:
//
//	«quiero un rediseño de las notas · quiero mantener la esencia y el diseño · pero quiero cambiar
//	 modelos, cuadrículas, etc.»
//
// Y el defecto no era sólo que faltara el campo: la consulta se recorta a `designConsultaFrases`
// oraciones y `designConsultaMax` chars antes de buscar en el acervo, así que **la mitad de
// «conservar» —que en un pedido real viene después de las dos primeras oraciones— se caía del
// buscador**. El motor salía a buscar material de rediseño sin saber qué no podía tocar.
//
// QUÉ HACE CADA UNA:
//   - `keep` dice de dónde se PARTE y qué no se toca. Además de viajar como bloque, de ahí sale la
//     forma de origen: la que hay que ALEJARSE al proponer candidatas (contraste_diseno.go).
//   - `change` dice qué se ataca. De ahí salen las DIMENSIONES sobre las que las tres candidatas
//     tienen que separarse.
//
// SIGUE SIENDO MODEL-FREE. Los dos son texto que viaja y coincidencia de vocabulario para elegir
// dimensiones. **Preguntarlos es otra cosa y no pasa por acá**: preguntar es un juicio y el camino
// caliente no puede tener juicios — lo hace la skill del lado del caller, que es también quien tiene
// con quién hablar. El motor sólo garantiza que, cuando la respuesta llega, no se pierda.

import "strings"

// intencionDeDiseno es de dónde viene el diseño y a dónde apunta. Viaja junta porque las dos mitades
// se leen juntas: sin el origen, «cambiar el modelo» no dice de qué modelo alejarse.
type intencionDeDiseno struct {
	Keep   string // lo que se conserva, y de qué forma parte hoy
	Change string // lo que se ataca
}

// bloqueDeConservacion arma el bloque de lo que NO se toca. Devuelve "" cuando no se pidió conservar
// nada, y ese vacío es correcto: un diseño desde cero no tiene de dónde partir, y estampar un
// encabezado que dice «conservá:» sin nada abajo le pide al agente que respete un conjunto vacío.
//
// El aviso va fuerte a propósito. En un rediseño, pisar lo que había que conservar no se lee como un
// error: se lee como que el motor no entendió el pedido, y es el modo de falla que más caro sale
// porque el trabajo entero se descarta.
func bloqueDeConservacion(keep string) string {
	keep = strings.TrimSpace(keep)
	if keep == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("SE CONSERVA — esto NO se toca, y es lo primero que se chequea antes de entregar: ")
	b.WriteString(keep)
	if origen := formaMencionada(keep); origen != "" {
		b.WriteString("\nDE DÓNDE PARTE: hoy la pantalla es una «")
		b.WriteString(formasDeDiseno[origen].Nombre)
		b.WriteString("». Las formas propuestas más abajo están elegidas para ALEJARSE de esa, no para repetirla con otro nombre.")
	}
	b.WriteString("\nOJO CON LA CONFUSIÓN CARA: conservar la esencia NO es conservar el esqueleto. Se puede cambiar la forma entera y mantener intacta la identidad —la paleta, la voz, los gestos— porque son cosas distintas. Si lo que se pide conservar es el esqueleto, tiene que decirlo así de explícito.")
	return b.String()
}

// bloqueDeCambio arma el bloque de lo que SÍ se ataca, y declara qué dimensiones leyó. Declarar la
// lectura importa: el motor la usa para elegir las candidatas, así que si leyó mal, quien compone lo
// ve en el brief en vez de recibir tres propuestas raras sin explicación.
func bloqueDeCambio(change string) string {
	change = strings.TrimSpace(change)
	if change == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("SE ATACA — acá sí hay permiso para romper: ")
	b.WriteString(change)

	dims, _, explicito := dimensionesAMover(change)
	if explicito {
		var ns []string
		for _, d := range dims {
			ns = append(ns, nombreDim[d])
		}
		if len(dims) == dimsTotal {
			b.WriteString("\nEL MOTOR LEYÓ: un replanteo del esqueleto entero, así que las candidatas se separan en las siete dimensiones (" +
				strings.Join(ns, ", ") + ").")
		} else {
			b.WriteString("\nEL MOTOR LEYÓ que hay que mover: " + strings.Join(ns, ", ") +
				". Si leyó mal, la elección de formas de abajo está sesgada y conviene reformular el pedido antes de componer.")
		}
	} else {
		b.WriteString("\nEL MOTOR NO RECONOCIÓ ninguna dimensión concreta en esto, así que las candidatas se separan parejo. NO lo leas como que se pidió cambiar todo.")
	}
	return b.String()
}
