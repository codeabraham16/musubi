package codeintel

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// simbolos_declarados.go es la función INVERSA de FormatSymbols: convierte de vuelta a []Symbol la
// línea de símbolos que quedó guardada en el gist de un archivo.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL EXTRACTOR NO ES LA ÚNICA FUENTE DE SÍMBOLOS, Y LA OTRA YA ESTABA ESCRITA
//
// `ExtractSymbols` deriva del AST y sólo cubre Go entero; TS/JS/Python quedan en clases y
// funciones de nivel superior, y el resto de los lenguajes en nada. La consecuencia no era
// «menos precisión»: anclar una observación a un símbolo de un `.sql`, un `.jsx` o un `.rs`
// devolvía ERROR, así que la marca de rancio —el mecanismo que hace que una nota se venza cuando
// el código que describe se mueve— no existía fuera de Go. Medido: en Altura-erp, el único
// producto no-Go, 529 observaciones y CERO anclas.
//
// Y la otra fuente ya estaba ahí, sin que nadie la leyera. `code_memory.symbols` guarda la línea
// que produce FormatSymbols, y cuando el extractor no da nada, el agente la escribe A MANO. En
// Altura la escribió sin que se lo pidieran y en el formato exacto de la casa:
//
//	src/App.jsx  → "landingForRole L49; RoleProtectedRoute L57; rutas Layout con allowedRoles L80-122"
//	…_cron.sql   → "DO block L25; cron.schedule L60 ('0 * * * *')"
//
// Esa columna se escribe, se imprime y se redacta — y no se parsea en ningún lado. Los tres
// consumidores de símbolos llaman a `ExtractSymbols` y nunca la miran.
//
// PRECEDENCIA, Y ES LO QUE HACE QUE ESTO NO ROMPA NADA: el extractor manda siempre; lo declarado
// es el fallback de lo que el extractor no vio. Go no cambia en un solo caso, porque en Go el
// extractor nunca falla. Lo declarado sólo puede AGREGAR anclas donde hoy hay un error.

// declaredSymbolRe reconoce una entrada de la línea de símbolos.
//
// El formato es deliberadamente laxo porque lo escribe un agente en prosa, no un serializador:
// el nombre puede tener espacios («rutas Layout con allowedRoles»), puede venir calificado
// («Caja.Guardar»), la línea puede ser un rango («L80-122») o aproximada («~L1375»), y puede
// quedar texto colgando atrás («L60 ('0 * * * *')»). Lo único obligatorio es el `L<número>`.
var declaredSymbolRe = regexp.MustCompile(`^\s*(\S.*?)\s+(~?)L(\d+)(?:\s*-\s*(\d+))?(?:\b|$)`)

// ParseSymbolLine convierte la línea de símbolos de un gist en símbolos con rango.
//
// Devuelve Kind == KindDeclarado para dejar en claro, en cualquier consumidor, que estos símbolos
// NO se derivaron del archivo: alguien los declaró. Se ordenan por línea de inicio.
//
// LO QUE DESCARTA, Y POR QUÉ CADA DESCARTE:
//
//   - Las entradas sin `L<número>`: sin línea no hay rango, y sin rango no hay nada que hashear.
//   - Las APROXIMADAS (`~L1375`). El `~` significa «no estoy seguro de dónde está», y calcular un
//     hash de un rango exacto a partir de un inicio incierto fabrica exactamente el ruido que la
//     marca de rancio existe para no producir: saltaría por código que la nota no describe. La
//     ausencia es «no sé», nunca «no está» — la misma disciplina que la serie de vida de red.
func ParseSymbolLine(linea string) []Symbol {
	if strings.TrimSpace(linea) == "" {
		return nil
	}
	var out []Symbol
	for _, parte := range strings.Split(linea, ";") {
		m := declaredSymbolRe.FindStringSubmatch(parte)
		if m == nil {
			continue
		}
		if m[2] == "~" { // aproximado: ver el comentario de arriba
			continue
		}
		nombre := normalizarNombreDeclarado(m[1])
		inicio, err := strconv.Atoi(m[3])
		if err != nil || inicio <= 0 {
			continue
		}
		fin := 0
		if m[4] != "" {
			if f, ferr := strconv.Atoi(m[4]); ferr == nil {
				fin = f
			}
		}
		recv, name := "", nombre
		// EL CORTE ES EN EL PRIMER PUNTO, IGUAL QUE EN symbolMatches, Y ESA SIMETRÍA ES EL PUNTO.
		//
		// `Caja.Guardar` (un método de Go) y `cron.schedule` (una función de verdad en SQL) son
		// indistinguibles como texto: no hay forma de decidir cuál es un receptor. Pero la
		// decisión NO es libre, porque del otro lado `symbolMatches` (internal/memory/origins.go)
		// parte el pedido con el mismo `strings.Cut` y exige `Recv == tipo && Name == metodo`.
		//
		// O sea que la única regla correcta es la que iguala al que va a comparar: si acá se
		// guardara `cron.schedule` entero con Recv vacío, un ancla a «cron.schedule» se partiría
		// del lado del pedido, buscaría Recv=="cron" y NO resolvería nunca. Adivinar «bien» la
		// semántica rompería el ancla; copiar al matcher la hace funcionar en los dos casos.
		if t, mtd, ok := strings.Cut(nombre, "."); ok && t != "" && mtd != "" {
			recv, name = t, mtd
		}
		out = append(out, Symbol{Name: name, Recv: recv, Kind: KindDeclarado, StartLine: inicio, EndLine: fin})
	}
	if len(out) == 0 {
		return nil
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartLine < out[j].StartLine })
	completarRangosDeclarados(out)
	return out
}

// completarRangosDeclarados le pone fin a los símbolos que sólo declararon su inicio.
//
// LA REGLA INGENUA —«hasta la línea del siguiente menos uno»— ESTÁ MAL SOBRE LOS DATOS REALES, y
// por eso acá hay tres condiciones y no una. Los gists que el agente YA escribió no vienen
// ordenados ni son disjuntos: «rutas Layout con allowedRoles L80-122» declara un rango explícito
// que se SOLAPA con los símbolos de adentro. Un rango mal calculado alimenta el hash y produce
// una marca que salta siempre o nunca, que es peor que no tener marca.
//
// Entonces: sólo se deriva para el que no declaró fin, sólo si el siguiente empieza DESPUÉS, y
// nunca se pisa un rango explícito. Al último, y a cualquiera que no cumpla, le queda fin =
// inicio: hashear la línea de la declaración es poco, pero es CIERTO — detecta que la firma
// cambió y no afirma nada sobre un cuerpo cuya extensión nadie declaró.
func completarRangosDeclarados(syms []Symbol) {
	for i := range syms {
		if syms[i].EndLine >= syms[i].StartLine {
			continue // rango explícito: no se toca
		}
		fin := syms[i].StartLine
		if i+1 < len(syms) && syms[i+1].StartLine > syms[i].StartLine {
			fin = syms[i+1].StartLine - 1
		}
		syms[i].EndLine = fin
	}
}

// normalizarNombreDeclarado saca la firma que a veces viene pegada al nombre.
//
// SALE DE MIRAR LOS GISTS DE VERDAD, no de imaginar formatos. Sobre los 13 de Altura-erp
// aparecen estas dos formas escritas por el agente:
//
//	DetalleBadges() L57
//	RoleSelector({value,onChange,disabled,title}) L19
//
// Sin normalizar, el símbolo se llama «DetalleBadges()» y quien vaya a anclar va a escribir
// «#DetalleBadges» —nadie ancla con la firma—, así que no resolvería nunca. Y el modo de falla es
// el peor: no es un error visible, es un ancla que no matchea y una nota que no se vence.
//
// Sólo se saca un grupo entre paréntesis PEGADO AL FINAL: ningún símbolo de verdad se llama así.
// Lo que viene después del `L<n>` no llega hasta acá —el regex ya lo dejó afuera—, así que el
// `('0 * * * *')` de `cron.schedule L60 ('0 * * * *')` no corre riesgo de confundirse con una firma.
func normalizarNombreDeclarado(crudo string) string {
	n := strings.TrimSpace(crudo)
	if !strings.HasSuffix(n, ")") {
		return n
	}
	if i := strings.LastIndex(n, "("); i > 0 {
		if sin := strings.TrimSpace(n[:i]); sin != "" {
			return sin
		}
	}
	return n
}
