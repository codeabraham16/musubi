package main

import (
	"encoding/json"
	"fmt"
	"os"

	"musubi/internal/skillsource"
)

// runCatalog maneja el subcomando `musubi catalog ...`.
// Por ahora solo soporta `validate [ruta]` (por defecto index.json).
func runCatalog(args []string) {
	if len(args) < 1 || args[0] != "validate" {
		fmt.Println("Uso: musubi catalog validate [ruta]   (por defecto: index.json)")
		os.Exit(1)
	}

	path := "index.json"
	if len(args) >= 2 {
		path = args[1]
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
