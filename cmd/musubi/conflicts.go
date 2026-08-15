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
