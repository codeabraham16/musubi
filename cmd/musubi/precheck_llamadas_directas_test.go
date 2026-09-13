package main

// «0 llamadas DIRECTAS vistas por el grafo», nunca «nadie lo llama».
//
// El grafo de código sólo registra llamadas directas: no ve una llamada por interfaz ni un método
// pasado como valor. Medido el 2026-09-13 en este repo, DbEngine.ListGraphNodesForFileCtx figuraba
// con callers [] teniendo 5 llamadas reales (4 por interfaz), y McpServer.toolCodeGraph —registrado
// como valor— también. Con eso, «tocarlo no arrastra a nadie conocido» era falso aun con el grafo
// al día. Estas pruebas fijan la frase que lo reemplaza en las TRES formas en que el hook puede
// hablar de un símbolo sin callers. Las frases van literales a propósito: comparar contra la misma
// constante que usa el código sería una prueba que verifica su propio literal.

import (
	"strings"
	"testing"

	"musubi/internal/memory"
)

const (
	fraseDirectas = "0 llamadas DIRECTAS vistas por el grafo"
	fraseCeguera  = "el grafo no ve llamadas por interfaz ni métodos pasados como valor"
)

func TestElGrafoDiceLlamadasDirectas(t *testing.T) {
	t.Setenv("MUSUBI_CODEGRAPH_HOOK", "")
	const contenido = "package a\nfunc Alpha(){ beta() }\nfunc beta(){}\n"
	aislado := func(huella string) *fakeCodeStore {
		s := grafoConHuella(huella)
		s.inEdges = map[string][]memory.GraphEdge{} // el grafo no conoce a ningún caller
		return s
	}
	exigir := func(t *testing.T, ctx string, frases ...string) {
		t.Helper()
		for _, f := range frases {
			if !strings.Contains(ctx, f) {
				t.Errorf("falta %q en %q", f, ctx)
			}
		}
	}
	prohibir := func(t *testing.T, ctx string, frases ...string) {
		t.Helper()
		for _, f := range frases {
			if strings.Contains(ctx, f) {
				t.Errorf("sobra %q en %q", f, ctx)
			}
		}
	}

	t.Run("editar un archivo aislado con el grafo al día", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "a.go", contenido)
		ctx := contextoDeEdicion(t, aislado(huellaDe(contenido)), root)
		exigir(t, ctx, fraseDirectas, fraseCeguera)
		prohibir(t, ctx, "no arrastra a nadie conocido", "tiene callers en el grafo")
	})

	t.Run("editar un archivo aislado con el grafo viejo", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "a.go", contenido)
		ctx := contextoDeEdicion(t, aislado(huellaDe(contenido+"//x\n")), root)
		exigir(t, ctx, fraseDirectas, fraseCeguera, "el grafo no sabe")
		prohibir(t, ctx, "tiene callers EN EL GRAFO")
	})

	t.Run("leer: el símbolo sin callers no dice «—»", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "a.go", contenido)
		// graphStore: Alpha llama a beta y a Alpha no lo llama nadie que el grafo vea.
		ctx := contextoDeLectura(t, grafoConHuella(huellaDe(contenido)), root)
		exigir(t, ctx, "- Alpha → llama a: beta | ← lo llaman: "+fraseDirectas, fraseCeguera)
		prohibir(t, ctx, "lo llaman: —")
	})

	t.Run("leer: si todos tienen callers la nota no se paga", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "a.go", contenido)
		s := grafoConHuella(huellaDe(contenido))
		s.inEdges["a.go#func:Alpha"] = []memory.GraphEdge{{FromKey: "main.go#func:main", ToKey: "a.go#func:Alpha", Kind: "CALLS"}}
		ctx := contextoDeLectura(t, s, root)
		prohibir(t, ctx, fraseDirectas)
	})

	t.Run("editar con callers: el conteo también es de directas", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "a.go", contenido)
		ctx := contextoDeEdicion(t, grafoConHuella(huellaDe(contenido)), root)
		exigir(t, ctx, "beta ← 1 directo(s)", fraseCeguera)
	})
}
