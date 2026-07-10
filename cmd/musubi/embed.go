package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/logx"
	"musubi/internal/memory"
)

// defaultEmbedModel es la tabla multilingüe (ES+EN) por default: la que baja `embed pull`
// y la que la auto-detección busca en la ubicación estándar para encender la semántica.
const defaultEmbedModel = "potion-multilingual-128M"

// runEmbed maneja `musubi embed <subcomando>`. Hoy: `pull` para descargar una tabla estática
// de embeddings (model2vec/POTION) con checksum pinneado, dejando la memoria semántica lista
// para encender SIN correr ninguna red ni modelo en runtime (model-free at inference).
func runEmbed(args []string) {
	if len(args) == 0 {
		embedUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "pull":
		runEmbedPull(args[1:])
	case "backfill":
		runEmbedBackfill(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Subcomando de embed desconocido: %s\n", args[0])
		embedUsage()
		os.Exit(1)
	}
}

func embedUsage() {
	fmt.Println(cBold("Uso:") + " musubi embed <subcomando>")
	fmt.Println("  " + cBold("pull") + " [modelo] [--out DIR] [--mirror URL]")
	fmt.Println("     Descarga una tabla estática de embeddings (checksum pinneado) para la búsqueda semántica.")
	fmt.Println("  " + cBold("backfill"))
	fmt.Println("     Re-embebe las observaciones del histórico que no tienen vector del modelo actual,")
	fmt.Println("     para volverlas recuperables por la búsqueda semántica (tras encender o cambiar el embedder).")
	fmt.Println("  Modelos conocidos (para pull):")
	names := make([]string, 0, len(embedding.KnownModels))
	for n := range embedding.KnownModels {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Printf("    - %s\n", n)
	}
}

// runEmbedBackfill re-embebe las observaciones históricas sin vector del modelo ACTUAL, para
// cerrar el hueco que WarnOnEmbedModelSwitch avisaba (memoria previa a encender la semántica, o de
// otro embedder, queda invisible para el recall semántico). Resuelve el mismo embedder que
// serve/daemon (auto-detección + degradación elegante); si la semántica no está encendida, no hay
// nada que backfillear y sale con error claro.
func runEmbedBackfill(args []string) {
	fs := flag.NewFlagSet("embed backfill", flag.ExitOnError)
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

	embedder := resolveEmbedder(cfg, root)
	if !embedding.Enabled(embedder) {
		fmt.Fprintln(os.Stderr, "La memoria semántica no está encendida (recall léxico): no hay nada que backfillear.")
		fmt.Fprintln(os.Stderr, "Encendé la semántica primero: 'musubi embed pull' (tabla estática) o configurá un provider en .musubi/config.yaml.")
		os.Exit(1)
	}

	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al arrancar base de datos: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()
	// Misma procedencia que un save normal (serve/daemon): el model_id del embedder actual.
	engine.SetVectorModelID(embedder.Name())

	fmt.Printf("Re-embebiendo el histórico con procedencia %q ...\n", embedder.Name())
	res, err := engine.EmbedBackfill(func(text string) ([]float32, error) {
		return embedder.Embed(context.Background(), text)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error en backfill (progreso: %d re-embebidas): %v\n", res.Embedded, err)
		os.Exit(1)
	}
	fmt.Printf("Backfill completo: %d pendiente(s), %d re-embebida(s), %d omitida(s). Procedencia: %s\n",
		res.Scanned, res.Embedded, res.Skipped, res.ModelID)
}

func runEmbedPull(args []string) {
	// El modelo es un posicional; se extrae ANTES de parsear flags porque el paquete flag de
	// Go deja de parsear en el primer no-flag (así `embed pull <modelo> --out X` funciona).
	model := defaultEmbedModel
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		model = args[0]
		args = args[1:]
	}
	fs := flag.NewFlagSet("embed pull", flag.ExitOnError)
	out := fs.String("out", "", "directorio destino (default: <workspace>/.musubi/embeddings/<modelo>)")
	mirror := fs.String("mirror", "", "base URL alternativa para bajar los archivos (re-hostear en tu infra); default: la fuente pinneada")
	_ = fs.Parse(args)

	spec, ok := embedding.KnownModels[model]
	if !ok {
		fmt.Fprintf(os.Stderr, "Modelo desconocido: %s\n", model)
		embedUsage()
		os.Exit(1)
	}
	if *mirror != "" {
		spec = applyMirror(spec, *mirror)
	}

	dest := *out
	if dest == "" {
		dest = filepath.Join(workspaceDir(), ".musubi", "embeddings", model)
	}

	fmt.Fprintf(os.Stderr, "Descargando %s a %s\n", model, dest)
	if err := embedding.PullModel(dest, spec, nil, embedPullProgress()); err != nil {
		fmt.Fprintf(os.Stderr, "\nError al descargar la tabla: %v\n", err)
		os.Exit(1)
	}
	// La memoria semántica es AUTO-ON (resolveEmbedder): si la tabla quedó en la ubicación
	// estándar del modelo default, se detecta sola al reiniciar y NO hay que tocar config.
	// Solo cuando quedó fuera de esa ruta (--out o un modelo no-default) hace falta declararla.
	autoPath := filepath.Join(workspaceDir(), ".musubi", "embeddings", defaultEmbedModel)
	if dest == autoPath {
		fmt.Fprintln(os.Stderr, "\nListo. La memoria semántica se enciende SOLA: reiniciá el daemon (o la")
		fmt.Fprintln(os.Stderr, "sesión) y Musubi la auto-detecta. No hace falta tocar config.yaml.")
	} else {
		fmt.Fprintln(os.Stderr, "\nListo. La tabla quedó fuera de la ubicación estándar; activala en .musubi/config.yaml:")
		fmt.Fprintln(os.Stderr, "  embedding:")
		fmt.Fprintln(os.Stderr, "    provider: static")
		fmt.Fprintf(os.Stderr, "    static_path: %q\n", dest)
	}
	fmt.Println(dest) // stdout: la ruta, para capturarla desde un script
}

// applyMirror reescribe las URLs de la tabla para bajarlas de un mirror propio (p. ej. tu
// Forgejo/servidor en el tailnet): <mirror>/<nombre-de-archivo>. El checksum sigue pinneado,
// así que un mirror comprometido no puede colar una tabla distinta.
func applyMirror(spec embedding.ModelSpec, mirror string) embedding.ModelSpec {
	base := strings.TrimRight(mirror, "/")
	out := spec
	out.Files = make([]embedding.ModelFile, len(spec.Files))
	for i, f := range spec.Files {
		f.URL = base + "/" + f.Name
		out.Files[i] = f
	}
	return out
}

// resolveEmbedder decide el proveedor de embeddings con AUTO-DETECCIÓN y DEGRADACIÓN
// ELEGANTE (16.2f): "prendido cuando se puede, nunca falla". Si no hay provider explícito
// (none/vacío) y existe una tabla en la ubicación estándar (<root>/.musubi/embeddings/
// <modelo-default>, la que baja `embed pull`), enciende la semántica; si no, se queda en
// recall léxico. Un provider explícito (static/ollama/openai) se respeta. Ante CUALQUIER
// error al construir el proveedor, cae a léxico (NoopProvider) en vez de abortar el arranque.
func resolveEmbedder(cfg config.Config, root string) embedding.Provider {
	ec := cfg.Embedding
	if ec.Provider == "" || ec.Provider == "none" {
		def := filepath.Join(root, ".musubi", "embeddings", defaultEmbedModel)
		if hasStaticTable(def) {
			ec.Provider = "static"
			ec.StaticPath = def
			logx.Info("memoria semántica auto-detectada (tabla presente)", "tabla", defaultEmbedModel)
		}
	}
	prov, err := embedding.NewProvider(ec)
	if err != nil {
		logx.Warn("no se pudo inicializar el proveedor de embeddings; se usa recall léxico",
			"provider", ec.Provider, "error", err)
		return embedding.NoopProvider{}
	}
	return prov
}

// hasStaticTable indica si dir tiene los dos archivos de una tabla estática cargable.
func hasStaticTable(dir string) bool {
	for _, f := range []string{"model.safetensors", "tokenizer.json"} {
		if st, err := os.Stat(filepath.Join(dir, f)); err != nil || st.IsDir() {
			return false
		}
	}
	return true
}

// embedPullProgress devuelve un callback de avance que imprime porcentaje por archivo en
// stderr, throttled (~cada 500ms) para no inundar la terminal en descargas de cientos de MB.
func embedPullProgress() func(string, int64, int64) {
	var last time.Time
	var lastFile string
	return func(file string, done, total int64) {
		now := time.Now()
		if file != lastFile {
			lastFile = file
			last = time.Time{}
		}
		if total <= 0 {
			return
		}
		if done < total && now.Sub(last) < 500*time.Millisecond {
			return
		}
		last = now
		pct := float64(done) / float64(total) * 100
		fmt.Fprintf(os.Stderr, "\r  %-20s %5.1f%% (%d/%d MB)   ", file, pct, done>>20, total>>20)
		if done >= total {
			fmt.Fprintln(os.Stderr)
		}
	}
}
