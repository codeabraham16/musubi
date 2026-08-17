package codeintel

import "testing"

const modPathTD = "ejemplo"

// paqueteMemoria es el paquete DESTINO: un tipo con métodos y una func top-level.
var paqueteMemoria = map[string]string{
	"internal/memory/engine.go": `package memory

type DbEngine struct{}

func (e *DbEngine) AutoEmbedBackfill(f func(string) error) {}
func (e *DbEngine) Uno()                                   {}
func (e *DbEngine) Dos()                                   {}
func Nuevo() *DbEngine                                     { return nil }
`,
}

func pendientePorNombre(g PackageGraph, name string) *PendingCall {
	for i := range g.PendingCalls {
		if g.PendingCalls[i].Name == name {
			return &g.PendingCalls[i]
		}
	}
	return nil
}

// EL CASO REAL que destapó el hueco (2026-08-17): `autoBackfill(engine *memory.DbEngine)` llama a
// `engine.AutoEmbedBackfill(...)` y el grafo devolvía `callers: []`. El tipo de `engine` está
// ESCRITO en la firma; no hace falta inferir nada para saber a qué método apunta.
func TestMetodoSobreParametroDeOtroPaqueteGeneraArista(t *testing.T) {
	gCmd := DerivePackage("cmd/app", map[string]string{
		"cmd/app/main.go": `package main

import "ejemplo/internal/memory"

func autoBackfill(engine *memory.DbEngine) {
	engine.AutoEmbedBackfill(nil)
}
`,
	}, modPathTD)

	pc := pendientePorNombre(gCmd, "DbEngine.AutoEmbedBackfill")
	if pc == nil {
		t.Fatalf("no se emitió el pendiente hacia el método; pendientes = %+v", gCmd.PendingCalls)
	}
	if pc.ImportPath != "ejemplo/internal/memory" {
		t.Errorf("import path equivocado: %q", pc.ImportPath)
	}

	idx := NewModuleIndex(modPathTD)
	idx.Add(DerivePackage("internal/memory", paqueteMemoria, modPathTD).Nodes)
	edges := ResolveCrossPackageCalls(gCmd.PendingCalls, idx)

	if len(edges) != 1 {
		t.Fatalf("esperaba 1 arista, obtuve %d: %+v", len(edges), edges)
	}
	if got, want := edges[0].ToKey, SymbolKey("internal/memory/engine.go", KindMethod, "DbEngine.AutoEmbedBackfill"); got != want {
		t.Errorf("la arista no apunta al método:\n  got  %s\n  want %s", got, want)
	}
	if got, want := edges[0].FromKey, SymbolKey("cmd/app/main.go", KindFunc, "autoBackfill"); got != want {
		t.Errorf("la arista no sale del caller:\n  got  %s\n  want %s", got, want)
	}
}

// `var e memory.DbEngine` también declara el tipo a la vista. El mismo test cubre que un `:=` NO se
// resuelva: ahí el tipo sale del retorno de otra función, y eso sí es inferencia.
func TestVarConTipoResuelveYAsignacionCortaNo(t *testing.T) {
	g := DerivePackage("cmd/app", map[string]string{
		"cmd/app/main.go": `package main

import "ejemplo/internal/memory"

func correr() {
	var declarado memory.DbEngine
	declarado.Uno()

	inferido := memory.Nuevo()
	inferido.Dos()
}
`,
	}, modPathTD)

	if pendientePorNombre(g, "DbEngine.Uno") == nil {
		t.Errorf("el `var` con tipo explícito debería emitir pendiente; %+v", g.PendingCalls)
	}
	if pc := pendientePorNombre(g, "DbEngine.Dos"); pc != nil {
		t.Errorf("un `:=` NO debe resolverse (el tipo sale del retorno = inferencia): %+v", pc)
	}
	// Y la func de paquete sigue emitiéndose como siempre: el cambio no rompe lo que ya andaba.
	if pendientePorNombre(g, "Nuevo") == nil {
		t.Errorf("se perdió el pendiente hacia la func top-level; %+v", g.PendingCalls)
	}
}

// ⚠️ EL TEST QUE DEFIENDE LA POLÍTICA. Un método sobre un tipo de TERCEROS no puede generar arista
// hacia un método propio que se llame igual. Es el falso positivo que hizo descartar la alternativa
// barata (indexar por nombre pelado), y sería del peor tipo: manda a revisar código que no participa.
func TestMetodoSobreTipoDeTercerosNoEmitePendiente(t *testing.T) {
	g := DerivePackage("cmd/app", map[string]string{
		"cmd/app/main.go": `package main

import "net/http"

func correr(c *http.Client) {
	c.Do(nil)
}
`,
	}, modPathTD)

	if len(g.PendingCalls) != 0 {
		t.Fatalf("un tipo de terceros no debe emitir pendientes: %+v", g.PendingCalls)
	}
}

// Una variable SOMBREA a un import homónimo, igual que el receptor. Si ganara el import, la llamada
// se le adjudicaría al paquete equivocado — el mismo razonamiento que ya estaba escrito para el
// receptor, aplicado ahora a los parámetros.
func TestLaVariableSombreaAlImportHomonimo(t *testing.T) {
	g := DerivePackage("cmd/app", map[string]string{
		"cmd/app/main.go": `package main

import "ejemplo/internal/memory"

func correr(memory *memory.DbEngine) {
	memory.Uno()
}
`,
	}, modPathTD)

	if pendientePorNombre(g, "DbEngine.Uno") == nil {
		t.Errorf("la variable no sombreó al import homónimo: %+v", g.PendingCalls)
	}
	if pc := pendientePorNombre(g, "Uno"); pc != nil {
		t.Errorf("resolvió hacia una func del paquete en vez del método de la variable: %+v", pc)
	}
}

// ⚠️ UN `var` DENTRO DE UN BLOQUE ANIDADO NO CUENTA, y la restricción es de correctitud, no de
// simplicidad: dos bloques hermanos pueden declarar el mismo nombre con tipos DISTINTOS, y quien
// leyera todo el cuerpo le atribuiría al primero el tipo del último. Sería una arista equivocada.
func TestVarEnBloqueAnidadoNoSeUsa(t *testing.T) {
	g := DerivePackage("cmd/app", map[string]string{
		"cmd/app/main.go": `package main

import "ejemplo/internal/memory"

func correr(cond bool) {
	if cond {
		var e memory.DbEngine
		e.Uno()
	}
}
`,
	}, modPathTD)

	if pc := pendientePorNombre(g, "DbEngine.Uno"); pc != nil {
		t.Errorf("un var de bloque anidado no debe emitir pendiente (arriesga atribuir mal): %+v", pc)
	}
}

// El resolver sigue OMITIENDO lo ambiguo: si dos tipos del mismo paquete tienen el mismo método y
// el índice ve dos candidatos con la misma clave, no inventa. Acá se comprueba el otro lado: un
// método que NO existe en el destino no produce arista, no un error ni una arista rota.
func TestMetodoInexistenteSeOmite(t *testing.T) {
	g := DerivePackage("cmd/app", map[string]string{
		"cmd/app/main.go": `package main

import "ejemplo/internal/memory"

func correr(e *memory.DbEngine) {
	e.NoExiste()
}
`,
	}, modPathTD)

	idx := NewModuleIndex(modPathTD)
	idx.Add(DerivePackage("internal/memory", paqueteMemoria, modPathTD).Nodes)
	if edges := ResolveCrossPackageCalls(g.PendingCalls, idx); len(edges) != 0 {
		t.Errorf("un método inexistente no debe generar arista: %+v", edges)
	}
}
