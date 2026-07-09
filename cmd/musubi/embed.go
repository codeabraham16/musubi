package main

import (
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
	default:
		fmt.Fprintf(os.Stderr, "Subcomando de embed desconocido: %s\n", args[0])
		embedUsage()
		os.Exit(1)
	}
}

func embedUsage() {
	fmt.Println(cBold("Uso:") + " musubi embed pull [modelo] [--out DIR] [--mirror URL]")
	fmt.Println("  Descarga una tabla estática de embeddings (checksum pinneado) para la búsqueda semántica.")
	fmt.Println("  Modelos conocidos:")
	names := make([]string, 0, len(embedding.KnownModels))
	for n := range embedding.KnownModels {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Printf("    - %s\n", n)
	}
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
	fmt.Fprintln(os.Stderr, "\nListo. Para activar la memoria semántica, en .musubi/config.yaml:")
	fmt.Fprintln(os.Stderr, "  embedding:")
	fmt.Fprintln(os.Stderr, "    provider: static")
	fmt.Fprintf(os.Stderr, "    static_path: %q\n", dest)
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
