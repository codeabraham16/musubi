package main

import (
	"encoding/json"
	"flag"
	"fmt"
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
	if *asJSON {
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(b))
		return
	}
	if len(res) == 0 {
		fmt.Println("El libro del modo sombra está vacío.")
		fmt.Println("Se enciende en .musubi/config.yaml (conflicts.shadow.enabled) y necesita motor de cognición.")
		return
	}
	fmt.Printf("%-16s %7s %7s %8s   %s\n", "VEREDICTO", "TOTAL", "ACUERDO", "TASA", "RANGO LÉXICO")
	for _, a := range res {
		rango := "—"
		if a.LexMin != nil && a.LexMax != nil {
			rango = fmt.Sprintf("%.2f – %.2f", *a.LexMin, *a.LexMax)
		}
		fmt.Printf("%-16s %7d %7d %7.0f%%   %s\n", a.HeurRelation, a.Total, a.Agreed, a.Rate*100, rango)
	}
	// El recordatorio no es decorativo: la tentación al ver una tasa baja es "ascender" los
	// veredictos del motor, y eso convertiría la medición en una escritura del LLM al libro mayor.
	fmt.Println("\nLa lectura del motor NUNCA se aplicó: esto mide el umbral, no lo corrige.")
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
