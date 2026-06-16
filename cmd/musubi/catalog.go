package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	default:
		fmt.Fprintf(os.Stderr, "Subcomando desconocido: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "Uso: musubi catalog validate [ruta]")
		fmt.Fprintln(os.Stderr, "     musubi catalog merge <url> [--output <ruta>]")
		os.Exit(1)
	}
}

// runValidate implementa `musubi catalog validate [ruta]`.
func runValidate(args []string) {
	path := "index.json"
	if len(args) >= 1 {
		path = args[0]
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al leer %s: %v\n", path, err)
		os.Exit(1)
	}

	var cat skillsource.Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		fmt.Fprintf(os.Stderr, "Error al parsear %s: %v\n", path, err)
		os.Exit(1)
	}

	if errs := skillsource.ValidateCatalog(cat); len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "Catalogo INVALIDO (%d error(es)):\n", len(errs))
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %v\n", e)
		}
		os.Exit(1)
	}

	fmt.Printf("Catalogo valido: %d entradas.\n", len(cat.Entries))
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
