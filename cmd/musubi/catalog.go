package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"musubi/internal/skillsource"
)

// runCatalog maneja el subcomando `musubi catalog ...`.
// Soporta `validate [ruta]` y `catalog merge <url> [--output <ruta>]`.
func runCatalog(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Uso: musubi catalog <subcomando>")
		fmt.Fprintln(os.Stderr, "  validate [ruta]              Valida un index.json de catálogo (por defecto: index.json)")
		fmt.Fprintln(os.Stderr, "  merge <url> [--output <ruta>]  Obtiene y fusiona un catálogo remoto en index.json")
		os.Exit(1)
	}

	switch args[0] {
	case "validate":
		runValidate(args[1:])
	case "merge":
		if err := runMerge(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "harvest":
		if err := runHarvest(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Subcomando desconocido: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "Uso: musubi catalog validate [ruta]")
		fmt.Fprintln(os.Stderr, "     musubi catalog merge <url> [--output <ruta>]")
		fmt.Fprintln(os.Stderr, "     musubi catalog harvest [--seeds a,b,c] [--top N] [--min-stars N] [--max-per-repo N] [--out ruta]")
		os.Exit(1)
	}
}

// defaultHarvestSeeds son los stacks/keywords sembrados por defecto en la cosecha del
// marketplace cuando no se pasa --seeds. Cubren los ecosistemas más comunes.
var defaultHarvestSeeds = []string{
	"Go", "Python", "Node.js", "TypeScript", "Rust", "Java",
	"Ruby", "PHP", "C#", ".NET", "Kubernetes", "Docker", "SQL",
}

// defaultMarketplaceBaseURL es el host del marketplace por defecto para la cosecha.
const defaultMarketplaceBaseURL = "https://skillsmp.com"

// runHarvest implementa `musubi catalog harvest`: cosecha un catálogo estático de Agent
// Skills del marketplace, curado por seeds (stacks) y estrellas. La API key se lee de una
// env var (por defecto SKILLSMP_API_KEY); vacía ⇒ tier anónimo. Retorna error (no os.Exit)
// para ser testeable. Pega a la red (skillsmp): no es un test unitario, es una herramienta.
func runHarvest(args []string) error {
	// Defaults.
	seeds := defaultHarvestSeeds
	top := 50
	minStars := 0
	maxPerRepo := 3
	out := "marketplace-index.json"
	apiKeyEnv := "SKILLSMP_API_KEY"
	baseURL := defaultMarketplaceBaseURL

	// Parseo manual de flags (mismo estilo que runMerge): --flag valor o --flag=valor.
	for i := 0; i < len(args); i++ {
		arg := args[i]
		next := func() (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s requiere un argumento", arg)
			}
			i++
			return args[i], nil
		}
		switch {
		case arg == "--seeds" || strings.HasPrefix(arg, "--seeds="):
			v := strings.TrimPrefix(arg, "--seeds=")
			if v == arg {
				var err error
				if v, err = next(); err != nil {
					return err
				}
			}
			seeds = splitSeeds(v)
		case arg == "--top" || strings.HasPrefix(arg, "--top="):
			v := strings.TrimPrefix(arg, "--top=")
			if v == arg {
				var err error
				if v, err = next(); err != nil {
					return err
				}
			}
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				return fmt.Errorf("--top debe ser un entero positivo: %q", v)
			}
			top = n
		case arg == "--min-stars" || strings.HasPrefix(arg, "--min-stars="):
			v := strings.TrimPrefix(arg, "--min-stars=")
			if v == arg {
				var err error
				if v, err = next(); err != nil {
					return err
				}
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return fmt.Errorf("--min-stars debe ser un entero ≥ 0: %q", v)
			}
			minStars = n
		case arg == "--max-per-repo" || strings.HasPrefix(arg, "--max-per-repo="):
			v := strings.TrimPrefix(arg, "--max-per-repo=")
			if v == arg {
				var err error
				if v, err = next(); err != nil {
					return err
				}
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return fmt.Errorf("--max-per-repo debe ser un entero ≥ 0 (0 = sin tope): %q", v)
			}
			maxPerRepo = n
		case arg == "--out" || strings.HasPrefix(arg, "--out="):
			v := strings.TrimPrefix(arg, "--out=")
			if v == arg {
				var err error
				if v, err = next(); err != nil {
					return err
				}
			}
			out = v
		case arg == "--api-key-env" || strings.HasPrefix(arg, "--api-key-env="):
			v := strings.TrimPrefix(arg, "--api-key-env=")
			if v == arg {
				var err error
				if v, err = next(); err != nil {
					return err
				}
			}
			apiKeyEnv = v
		case arg == "--url" || strings.HasPrefix(arg, "--url="):
			v := strings.TrimPrefix(arg, "--url=")
			if v == arg {
				var err error
				if v, err = next(); err != nil {
					return err
				}
			}
			baseURL = v
		default:
			return fmt.Errorf("flag desconocido: %q", arg)
		}
	}

	if len(seeds) == 0 {
		return fmt.Errorf("no hay seeds para cosechar (usá --seeds a,b,c)")
	}

	apiKey := ""
	if apiKeyEnv != "" {
		apiKey = os.Getenv(apiKeyEnv)
	}
	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "Aviso: sin API key (env %s vacía); usando el tier anónimo (límite bajo).\n", apiKeyEnv)
	}

	// fetch ligado a baseURL+apiKey; el núcleo HarvestMarketplace es agnóstico de la red.
	fetch := func(ctx context.Context, query string, limit int) ([]skillsource.MarketplaceSkill, error) {
		return skillsource.FetchMarketplaceSkills(ctx, baseURL, apiKey, query, limit)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cat, err := skillsource.HarvestMarketplace(ctx, fetch, seeds, top, minStars, maxPerRepo)
	if err != nil {
		return fmt.Errorf("cosechar el marketplace: %w", err)
	}
	cat.Generated = time.Now().UTC().Format(time.RFC3339)

	if err := writeJSONAtomic(out, cat); err != nil {
		return fmt.Errorf("escribir catálogo en %s: %w", out, err)
	}

	fmt.Printf("Cosecha completa: %d skills curadas de %d seeds escritas en %s.\n",
		len(cat.Skills), len(cat.Seeds), out)
	return nil
}

// splitSeeds parte una lista de seeds separadas por coma, descartando vacías y espacios.
func splitSeeds(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// writeJSONAtomic serializa v como JSON indentado y lo escribe atómicamente en path
// (temp en el mismo dir + rename, evita fallos cross-device en Windows; limpia si falla).
func writeJSONAtomic(path string, v any) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return fmt.Errorf("crear archivo temporal en %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	success := false
	defer func() {
		if !success {
			os.Remove(tmpName)
		}
	}()

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar JSON: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("escribir datos en temporal: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cerrar archivo temporal: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renombrar %s → %s: %w", tmpName, path, err)
	}
	success = true
	return nil
}

// runValidate implementa `musubi catalog validate [ruta]`: envoltorio fino sobre
// validateCatalogFile que traduce el resultado a stdout/stderr + os.Exit.
func runValidate(args []string) {
	path := "index.json"
	if len(args) >= 1 {
		path = args[0]
	}
	entries, errs, err := validateCatalogFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al validar %s: %v\n", path, err)
		os.Exit(1)
	}
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "Catalogo INVALIDO (%d error(es)):\n", len(errs))
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %v\n", e)
		}
		os.Exit(1)
	}
	fmt.Printf("Catalogo valido: %d entradas.\n", entries)
}

// validateCatalogFile lee y valida un catálogo en path. Devuelve la cantidad de
// entradas y los errores de validación del catálogo (catálogo inválido, no de
// ejecución). err != nil solo ante fallo de lectura o de parseo del archivo.
func validateCatalogFile(path string) (entries int, validationErrs []error, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, nil, fmt.Errorf("leer %s: %w", path, err)
	}
	var cat skillsource.Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return 0, nil, fmt.Errorf("parsear %s: %w", path, err)
	}
	return len(cat.Entries), skillsource.ValidateCatalog(cat), nil
}

// runMerge implementa `musubi catalog merge <url> [--output <ruta>]`.
// Retorna error en lugar de llamar os.Exit directamente para ser testeable.
func runMerge(args []string) error {
	// Parseo manual de argumentos: --output <ruta> o --output=<ruta>
	var url, target string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--output" {
			if i+1 >= len(args) {
				return fmt.Errorf("--output requiere un argumento")
			}
			i++
			target = args[i]
		} else if strings.HasPrefix(arg, "--output=") {
			target = strings.TrimPrefix(arg, "--output=")
		} else if !strings.HasPrefix(arg, "--") {
			if url != "" {
				return fmt.Errorf("argumento posicional inesperado: %q", arg)
			}
			url = arg
		} else {
			return fmt.Errorf("flag desconocido: %q", arg)
		}
	}

	if url == "" {
		return fmt.Errorf("se requiere un argumento <url>\nUso: musubi catalog merge <url> [--output <ruta>]")
	}
	if target == "" {
		target = "index.json"
	}

	// Cargar catálogo base (archivo faltante → catálogo vacío, no error).
	base, err := loadBaseCatalog(target)
	if err != nil {
		return fmt.Errorf("leer catálogo base de %s: %w", target, err)
	}

	// Obtener catálogo remoto (con timeout para no colgarse ante un server lento).
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	incoming, err := skillsource.FetchCatalog(ctx, url)
	if err != nil {
		return fmt.Errorf("obtener catálogo de %s: %w", url, err)
	}

	// Fusionar.
	merged, collisions := skillsource.MergeCatalogs(base, incoming)

	// Advertir si las versiones difieren (solo cuando base tiene entradas).
	if base.CatalogVersion != 0 && base.CatalogVersion != incoming.CatalogVersion {
		fmt.Fprintf(os.Stderr, "Advertencia: versión del catálogo base (%d) difiere de la del remoto (%d). Se conserva la versión base.\n",
			base.CatalogVersion, incoming.CatalogVersion)
	}

	// Validar antes de escribir.
	if errs := skillsource.ValidateCatalog(merged); len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "Catálogo fusionado INVÁLIDO (%d error(es)):\n", len(errs))
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %v\n", e)
		}
		return fmt.Errorf("validación del catálogo fusionado falló, no se escribió nada")
	}

	// Escribir atómicamente.
	if err := writeCatalogAtomic(target, merged); err != nil {
		return fmt.Errorf("escribir catálogo en %s: %w", target, err)
	}

	// Imprimir reporte de colisiones a stdout.
	for _, id := range collisions {
		fmt.Printf("[overwrite] %s: local replaced by incoming\n", id)
	}

	// Resumen de éxito a stdout.
	fmt.Printf("Merge complete: %d entries written (%d overwritten).\n", len(merged.Entries), len(collisions))
	return nil
}

// loadBaseCatalog lee el catálogo JSON desde path.
// Si el archivo no existe (os.IsNotExist), devuelve un Catalog vacío sin error.
// Cualquier otro error de lectura o parseo se propaga.
func loadBaseCatalog(path string) (skillsource.Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return skillsource.Catalog{}, nil
		}
		return skillsource.Catalog{}, fmt.Errorf("leer %s: %w", path, err)
	}
	var cat skillsource.Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return skillsource.Catalog{}, fmt.Errorf("parsear %s: %w", path, err)
	}
	return cat, nil
}

// writeCatalogAtomic escribe cat como JSON indentado en path usando
// un archivo temporal en el mismo directorio (para evitar fallos cross-device en Windows).
// Si ocurre un error tras crear el temporal, lo elimina antes de retornar.
func writeCatalogAtomic(path string, cat skillsource.Catalog) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return fmt.Errorf("crear archivo temporal en %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	// Limpiar temporal en caso de error.
	success := false
	defer func() {
		if !success {
			os.Remove(tmpName)
		}
	}()

	data, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar catálogo: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("escribir datos en temporal: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cerrar archivo temporal: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renombrar %s → %s: %w", tmpName, path, err)
	}

	success = true
	return nil
}
