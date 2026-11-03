package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// conflicts.go implementa 'musubi conflicts': operaciones sobre el detector de conflictos que no
// pasan por MCP. Hoy sólo el backfill del desglose, que es una reparación de datos de una sola
// vez y por eso vive en la CLI y no en una tool: se corre a mano, sobre una base concreta, y se
// mira el resultado antes de darlo por bueno.

func runConflicts(args []string) {
	if len(args) == 0 {
		conflictsUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "backfill":
		runConflictsBackfill(args[1:])
	case "shadow":
		runConflictsShadow(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Subcomando de conflicts desconocido: %s\n", args[0])
		conflictsUsage()
		os.Exit(1)
	}
}

func conflictsUsage() {
	fmt.Println(cBold("Uso:") + " musubi conflicts <subcomando>")
	fmt.Println("  " + cBold("backfill") + " [--dry-run] [--limit N] [--json]")
	fmt.Println("     Reconstruye el desglose (lex/coseno) de las relaciones que se guardaron sin él,")
	fmt.Println("     para poder recalibrar los umbrales del detector con evidencia y no con 8 pares.")
	fmt.Println("     Sólo rellena huecos: nunca pisa un score que el detector ya había medido.")
	fmt.Println("  " + cBold("shadow") + " [--json]")
	fmt.Println("     Lee el libro del modo sombra: en qué veredictos el motor coincidió con el detector")
	fmt.Println("     y en qué rango léxico cae cada tipo. La lectura del motor nunca se aplicó.")
}

// runConflictsShadow imprime la evidencia acumulada por el modo sombra. Existe porque una tabla
// que nadie puede leer sin abrir SQLite es una tabla que no se va a leer: la sombra se enciende
// para responder una pregunta, y la respuesta tiene que estar a un comando de distancia.
func runConflictsShadow(args []string) {
	fs := flag.NewFlagSet("conflicts shadow", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "emitir el resumen como JSON")
	_ = fs.Parse(args)

	engine, err := memory.NewDbEngine(workspaceDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al abrir la memoria: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	res, err := engine.ShadowAgreementByRelation()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al leer el libro del modo sombra: %v\n", err)
		os.Exit(1)
	}
	// La cobertura se pide SIEMPRE, incluso para el camino del libro vacío, porque es justo el
	// que hoy atribuye la causa equivocada: manda a revisar la config y el motor cuando el techo
	// lo pone el corpus. Un error acá aborta en vez de degradar a la salida vieja — imprimir la
	// tabla sin su denominador es exactamente la lectura que este comando dejó de querer dar.
	cob, err := engine.ShadowCoverage()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al derivar la cobertura del modo sombra: %v\n", err)
		os.Exit(1)
	}
	emitirShadow(os.Stdout, cob, res, *asJSON)
}

// emitirShadow es la COSTURA entre lo que se consultó y lo que se imprime, y vive en una función
// con Writer por el mismo motivo que imprimirCobertura: para que exista un custodio de la SALIDA.
// Sin ella los custodios medían las PIEZAS —el helper del libro vacío, los tags del DTO, los
// motivos en el texto— y ninguno la UNIÓN, así que el comando podía volver a decir «techo CERO»
// teniendo techo 4 —el defecto que R7 vino a arreglar— con el paquete entero en VERDE. Medido: tres
// mutaciones de este cableado (emitir el array en vez del objeto, pasarle un reporte en cero al
// libro vacío, saltear la cobertura) sobrevivían a `go test ./cmd/musubi/`.
func emitirShadow(w io.Writer, cob memory.ShadowCoverageReport, res []memory.ShadowAgreement, asJSON bool) {
	if asJSON {
		b, _ := json.MarshalIndent(shadowJSON{Coverage: cob, ByRelation: res}, "", "  ")
		fmt.Fprintln(w, string(b))
		return
	}
	if len(res) == 0 {
		imprimirLibroVacio(w, cob)
		return
	}
	// La cobertura va ANTES de la tabla y no al pie: el denominador tiene que llegar antes que las
	// tasas, o quien lee se forma una impresión con un 95 % y después la corrige.
	imprimirCobertura(w, cob)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-16s %7s %7s %8s   %s\n", "VEREDICTO", "TOTAL", "ACUERDO", "TASA", "RANGO LÉXICO")
	for _, a := range res {
		rango := "—"
		if a.LexMin != nil && a.LexMax != nil {
			rango = fmt.Sprintf("%.2f – %.2f", *a.LexMin, *a.LexMax)
		}
		fmt.Fprintf(w, "%-16s %7d %7d %7.0f%%   %s\n", a.HeurRelation, a.Total, a.Agreed, a.Rate*100, rango)
	}
	// El recordatorio no es decorativo: la tentación al ver una tasa baja es "ascender" los
	// veredictos del motor, y eso convertiría la medición en una escritura del LLM al libro mayor.
	fmt.Fprintln(w, "\nLa lectura del motor NUNCA se aplicó: esto mide el umbral, no lo corrige.")
}

// shadowJSON es la forma del `--json` desde que la respuesta trae la cobertura. Pasó de array a
// objeto a propósito: una bandera aparte habría dejado el denominador OPCIONAL, que es justo lo
// que este cambio quiere evitar. Se midió antes de romperlo —cero consumidores: los únicos usos de
// ShadowAgreementByRelation son este comando y tres tests— y se declara en el CHANGELOG.
//
// Tags explícitos y en minúscula por la regla 6 del repo: un DTO sin ellos serializa en mayúscula,
// el receptor que parsea en minúscula NO falla, y guarda los campos vacíos. Es una falla que se
// lee como un dato.
type shadowJSON struct {
	Coverage   memory.ShadowCoverageReport `json:"coverage"`
	ByRelation []memory.ShadowAgreement    `json:"by_relation"`
}

// imprimirCobertura declara sobre qué parte del corpus habla la tabla. Vive en una función y no
// duplicada en los dos caminos porque el del libro VACÍO es el que más la necesita, y una copia
// pegada se desincroniza justo con la que alguien va a leer.
//
// Cada recorte sale con su MOTIVO, no sólo con su número: un «afuera» que se come casi el corpus
// entero se lee como una falla del detector, y no lo es —es la guarda del upsert haciendo su
// trabajo—; y el balde de «no re-detectables» se lee como corpus podrido cuando medido es trabajo
// CERRADO: el 2026-11-02 los 66 pares `supersedes` tenían el target YA supersedido —64 por su
// propio source y 2 por otro— y NINGUNO sin aplicar.
// Escribe en un Writer y no en stdout directo para que su custodio pueda leer el texto: la
// afirmación que hay que custodiar es «el motivo aparece», y eso es una propiedad de la SALIDA.
func imprimirCobertura(w io.Writer, c memory.ShadowCoverageReport) {
	fmt.Fprintf(w, "%s %d de %d pares re-detectables son medibles por la sombra%s.\n",
		cBold("Cobertura:"), c.Medibles, c.Redetectables, porcentajeMedible(c))
	if c.FirmaAjena > 0 {
		fmt.Fprintf(w, "  %d quedan afuera por veredicto AJENO: la guarda del upsert los protegió, y\n", c.FirmaAjena)
		fmt.Fprintln(w, "     medirlos anotaría como acierto del heurístico lo que decidió otro.")
	}
	if c.PuntaNoVisible > 0 {
		fmt.Fprintf(w, "  %d no son re-detectables: una punta está archivada, superseded o en cuarentena,\n", c.PuntaNoVisible)
		fmt.Fprintln(w, "     así que la detección no vuelve a visitarlas. Suele ser trabajo CERRADO.")
	}
	// Esta línea es la que evita que un cero mande a buscar el problema donde no está: la única
	// cosa que el detector firma solo es el auto-`supersedes`, y aplicarlo deja su propio target
	// invisible. O sea que la vía «firmada por el detector» se vacía sola, y eso no es una falla.
	if c.AutoAplicadas > 0 {
		fmt.Fprintf(w, "  %d de ésas son auto-resoluciones del detector que YA surtieron efecto: el\n", c.AutoAplicadas)
		fmt.Fprintln(w, "     `supersedes` dejó invisible a su propio target, así que el par se autoexcluye.")
		fmt.Fprintln(w, "     Su señal léxica sigue siendo calibrable con `musubi conflicts backfill`.")
	}
	fmt.Fprintf(w, "  %d relaciones en total · %d filas medidas en el libro.\n", c.Relaciones, c.Medidas)
}

// imprimirLibroVacio es el camino que este cambio vino a arreglar: la versión vieja era una línea
// que mandaba a revisar la config y el motor de cognición, y con un techo de cero eso es perseguir
// un fantasma. Primero el techo, que lo pone el CORPUS, y después el interruptor.
func imprimirLibroVacio(w io.Writer, c memory.ShadowCoverageReport) {
	fmt.Fprintln(w, "El libro del modo sombra está vacío: ninguna medición todavía.")
	fmt.Fprintln(w)
	imprimirCobertura(w, c)
	fmt.Fprintln(w)
	if c.Medibles == 0 {
		fmt.Fprintln(w, "Y encenderla no escribiría una sola fila: ni un par del corpus pasa el filtro de")
		fmt.Fprintln(w, "procedencia, así que lo que falta no es el motor. El techo de hoy es CERO.")
		return
	}
	fmt.Fprintf(w, "Encendida, el techo son %d pares: ésa es toda la evidencia que puede dar.\n", c.Medibles)
	fmt.Fprintln(w, "Se enciende en .musubi/config.yaml (conflicts.shadow.enabled) y necesita motor de cognición.")
}

// porcentajeMedible devuelve el porcentaje entre paréntesis, o nada si no hay denominador: con el
// corpus recién nacido Redetectables es 0, y un «NaN%» se lee como un error del comando en vez de
// como lo que es, un corpus sin nada que medir todavía.
func porcentajeMedible(c memory.ShadowCoverageReport) string {
	if c.Redetectables == 0 {
		return ""
	}
	return fmt.Sprintf(" (%.1f%%)", float64(c.Medibles)*100/float64(c.Redetectables))
}

// runConflictsBackfill vuelve a scorear los pares sin desglose. El léxico se recalcula siempre
// (es una función pura del contenido); el coseno sólo cuando ambas puntas tienen vector de la
// procedencia ACTUAL, así que sin embedder el comando igual sirve —y de hecho el léxico es la
// señal que se quería recalibrar—. Por eso, a diferencia de 'embed backfill', acá la semántica
// apagada avisa y sigue en vez de abortar.
func runConflictsBackfill(args []string) {
	fs := flag.NewFlagSet("conflicts backfill", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "cuenta lo que haría y no escribe nada")
	limit := fs.Int("limit", 0, "procesar como mucho N pares (0 = todos), los más recientes primero")
	asJSON := fs.Bool("json", false, "emitir el resultado como JSON")
	_ = fs.Parse(args)

	root := workspaceDir()
	if err := ensureWorkspace(root); err != nil {
		fmt.Fprintf(os.Stderr, "Error al preparar workspace: %v\n", err)
		os.Exit(1)
	}
	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al cargar configuración: %v\n", err)
		os.Exit(1)
	}

	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al abrir la memoria: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	// Misma procedencia que un save normal. Si no hay embedder, vectorModelID queda vacío y
	// observationVector devuelve nil sin consultar: el backfill cae al camino léxico solo.
	embedder := resolveEmbedder(cfg, root)
	if embedding.Enabled(embedder) {
		engine.SetVectorModelID(embedder.Name())
	} else if !*asJSON {
		fmt.Fprintln(os.Stderr, "Semántica apagada: se reconstruye sólo el léxico (el coseno queda en NULL).")
	}

	res, err := engine.BackfillRelationScores(memory.BackfillScoresOptions{
		DryRun: *dryRun,
		Limit:  *limit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error en el backfill (progreso: %d par(es) escaneado(s)): %v\n", res.Scanned, err)
		os.Exit(1)
	}

	if *asJSON {
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(b))
		return
	}
	verbo := "Rellenado"
	if *dryRun {
		verbo = "Se rellenaría (ensayo, no se escribió nada)"
	}
	fmt.Printf("%s: %d par(es) escaneado(s), %d lex, %d coseno.\n", verbo, res.Scanned, res.LexFilled, res.CosineFilled)
	// La señal se informa aparte porque es la única cifra que decide si alcanza para calibrar: el
	// resto de los tipos de relación son volumen. Y los pares sin coseno se declaran en vez de
	// omitirse, para que nadie lea el total como si fueran todos comparables.
	fmt.Printf("  De ésos, %d son de señal (supersedes / conflicts_with); %d quedaron sin coseno.\n", res.Signal, res.NoVector)
	if res.ModelID != "" {
		fmt.Printf("  Procedencia de los vectores: %s\n", res.ModelID)
	}
}
