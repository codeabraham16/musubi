package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"musubi/internal/mcp"
)

// runToken administra el registro de principals del cerebro central (Track 16 F1 16.1c-2):
//
//	musubi token new    --name X [--project Y] [--role reader|writer|admin] [--file path]
//	musubi token list   [--file path]
//	musubi token revoke --name X [--file path]
//
// El registro guarda el SHA-256 de cada token, nunca el token crudo. `new` imprime el token
// UNA sola vez para entregárselo al miembro. Ruta default: <workspace>/.musubi/principals.yaml.
func runToken(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: musubi token <new|list|revoke> [flags]")
		os.Exit(1)
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "new":
		tokenNew(rest)
	case "list":
		tokenList(rest)
	case "revoke":
		tokenRevoke(rest)
	default:
		fmt.Fprintf(os.Stderr, "subcomando desconocido %q (usá new|list|revoke)\n", sub)
		os.Exit(1)
	}
}

// defaultPrincipalsPath es <workspace>/.musubi/principals.yaml.
func defaultPrincipalsPath() string {
	return filepath.Join(workspaceDir(), ".musubi", "principals.yaml")
}

func tokenNew(args []string) {
	fs := flag.NewFlagSet("token new", flag.ExitOnError)
	name := fs.String("name", "", "nombre del principal (obligatorio)")
	project := fs.String("project", "", "project_id que se le atribuye (aísla su recall); OBLIGATORIO para reader/writer")
	role := fs.String("role", "writer", "rol: reader | writer | admin (reader/writer requieren --project; admin es federado)")
	file := fs.String("file", "", "ruta del registro (default: <workspace>/.musubi/principals.yaml)")
	_ = fs.Parse(args)

	path := *file
	if path == "" {
		path = defaultPrincipalsPath()
	}
	token, err := mcp.AddPrincipal(path, *name, *project, *role)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Principal %q agregado (project=%q, role=%s) en %s\n", *name, *project, *role, path)
	fmt.Println("\nToken (guardalo YA — no se vuelve a mostrar; entregáselo al miembro por un canal seguro):")
	fmt.Println("  " + token)
}

func tokenList(args []string) {
	fs := flag.NewFlagSet("token list", flag.ExitOnError)
	file := fs.String("file", "", "ruta del registro (default: <workspace>/.musubi/principals.yaml)")
	_ = fs.Parse(args)

	path := *file
	if path == "" {
		path = defaultPrincipalsPath()
	}
	infos, err := mcp.ListPrincipalsInfo(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(infos) == 0 {
		fmt.Printf("No hay principals en %s (modo legacy: un único bearer).\n", path)
		return
	}
	fmt.Printf("Principals en %s:\n", path)
	for _, p := range infos {
		proj := p.ProjectID
		if proj == "" {
			proj = "(sin proyecto)"
		}
		fmt.Printf("  %-20s  %-10s  %s\n", p.Name, p.Role, proj)
	}
}

func tokenRevoke(args []string) {
	fs := flag.NewFlagSet("token revoke", flag.ExitOnError)
	name := fs.String("name", "", "nombre del principal a revocar (obligatorio)")
	file := fs.String("file", "", "ruta del registro (default: <workspace>/.musubi/principals.yaml)")
	_ = fs.Parse(args)

	path := *file
	if path == "" {
		path = defaultPrincipalsPath()
	}
	found, err := mcp.RemovePrincipal(path, *name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !found {
		fmt.Printf("No existe un principal %q en %s.\n", *name, path)
		return
	}
	fmt.Printf("Principal %q revocado. Reiniciá musubi-brain.service para aplicar.\n", *name)
}
