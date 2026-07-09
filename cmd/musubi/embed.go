package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"musubi/internal/embedding"
)

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
	fs := flag.NewFlagSet("embed pull", flag.ExitOnError)
	out := fs.String("out", "", "directorio destino (default: <workspace>/.musubi/embeddings/<modelo>)")
	mirror := fs.String("mirror", "", "base URL alternativa para bajar los archivos (re-hostear en tu infra); default: la fuente pinneada")
	_ = fs.Parse(args)

	model := "potion-multilingual-128M" // multilingüe ES+EN por default
	if fs.NArg() > 0 {
		model = fs.Arg(0)
	}
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
