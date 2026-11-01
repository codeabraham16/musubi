package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// Los custodios de los TRES EJES de la frescura del grafo, cada uno contra SU PROPIO denominador.
//
// ⚠️ ESTE ARCHIVO EXISTE PORQUE readiness_grafo_frescura_test.go NO LOS MEDÍA. Su helper
// `grafoSembrado` indexa todos los archivos que escribe, así que en todos sus montajes
// `archivos_en_grafo == archivos_en_disco` y `ghosts == missing == 0`: cinco magnitudes distintas
// quedan indistinguibles, y un eje ignorado o cruzado de denominador no mueve una sola aserción. Un
// panel adversarial lo midió en `verify` (2026-11-01): tres sabotajes de UNA línea devuelven la
// dimensión de 0.67 a 1.00 y los 48 tests del camino siguen verdes. Uno de ellos —ignorar el eje
// `missing`— REINSTALA el bug que originó todo el cambio: 213 de 798 `.go` sin un solo nodo en el
// grafo, y la dimensión puntuando perfecto.
//
// La cura es el montaje, no más aserciones: acá NINGUNA cuenta coincide con otra, así que un cruce
// de cables se ve. Es la misma lección que `gotcha-guarda-definida-y-desconectada`, en su versión
// difícil: la guarda está conectada y mide bien; lo que no existía era el montaje capaz de
// distinguir que midiera mal.

// grafoAsimetrico arma un mundo donde ninguna cuenta coincide con otra: `indexados` archivos en el
// grafo con DOS nodos cada uno (así `nodos != archivos`), `fantasmas` de ellos borrados del disco
// DESPUÉS de indexar, y `sinIndexar` archivos `.go` en disco que el grafo nunca vio.
func grafoAsimetrico(t *testing.T, indexados, fantasmas, sinIndexar int) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })

	raiz := t.TempDir()
	var files []string
	var nodes []memory.GraphNode
	for i := 0; i < indexados; i++ {
		rel := fmt.Sprintf("i%02d.go", i)
		fp := escribirGo(t, raiz, rel, fmt.Sprintf("package p\n\nfunc F%02d() {}\n", i))
		files = append(files, rel)
		for j := 0; j < 2; j++ {
			nodes = append(nodes, memory.GraphNode{
				Key: fmt.Sprintf("%s#func:F%d", rel, j), Kind: "func", Name: fmt.Sprintf("F%02d_%d", i, j),
				Path: rel, SrcFingerprint: fp,
			})
		}
	}
	edges := []memory.GraphEdge{{
		FromKey: nodes[0].Key, ToKey: nodes[1].Key, Kind: "calls",
		Confidence: 1, Provenance: "test", SrcPath: files[0], SrcFingerprint: nodes[0].SrcFingerprint,
	}}
	if err := engine.UpsertPackageGraph(files, nodes, edges); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < fantasmas; i++ { // el grafo los recuerda, el disco ya no
		if err := os.Remove(filepath.Join(raiz, files[i])); err != nil {
			t.Fatal(err)
		}
	}
	for k := 0; k < sinIndexar; k++ { // en disco, jamás indexados
		escribirGo(t, raiz, fmt.Sprintf("m%02d.go", k), "package p\n\nfunc M() {}\n")
	}
	s := NewMcpServer(engine, raiz, embedding.NoopProvider{})
	if s.arbolFueraDeAlcance() {
		t.Fatal("el árbol salió fuera de alcance: no se mediría frescura")
	}
	return s
}

// Cada eje se mide contra el denominador que le toca: `stale` y `ghosts` contra los archivos DEL
// GRAFO, `missing` contra los indexables DEL DISCO. El tercer caso es el que separa las dos cosas:
// 2 sin indexar sobre 41 en disco da 4,9 % y pasa, mientras que contra los 39 del grafo daría 5,1 %
// y caería. Un montaje simétrico no puede distinguirlos.
//
// Sabotaje que lo pone en rojo: ignorar el eje `missing` al elegir el peor de los tres. Es el que
// reinstala el bug original.
// arnes: archivo="internal/mcp/methods_readiness.go"
// arnes: de="\tif pMissing > peor {"
// arnes: a="\tif false && pMissing > peor {"
func TestFrescuraMideCadaEjeContraSuPropioDenominador(t *testing.T) {
	casos := []struct {
		nombre                           string
		indexados, fantasmas, sinIndexar int
		pasa                             bool
	}{
		{"solo fantasmas (3 de 20)", 20, 3, 0, false},
		{"solo sin indexar (4 .go nunca vistos)", 20, 0, 4, false},
		{"sin indexar se mide contra los indexables del DISCO", 39, 0, 2, true},
	}
	for _, c := range casos {
		c := c
		t.Run(c.nombre, func(t *testing.T) {
			s := grafoAsimetrico(t, c.indexados, c.fantasmas, c.sinIndexar)
			d := dimGrafo(t, readinessComo(t, s, nil))
			alDia := chequeo(d, nombreAlDia)
			if alDia == nil {
				t.Fatalf("no está el chequeo %q", nombreAlDia)
			}
			// DENOMINADOR: las cinco magnitudes son distintas entre sí, así que si el montaje no
			// reproduce el estado el test muere acá y no en el invariante.
			enDisco := c.indexados - c.fantasmas + c.sinIndexar
			quiero := map[string]float64{
				"nodos": float64(2 * c.indexados), "stale": 0,
				"ghosts": float64(c.fantasmas), "missing": float64(c.sinIndexar),
				"archivos_en_grafo": float64(c.indexados), "archivos_en_disco": float64(enDisco),
			}
			for clave, v := range quiero {
				got, ok := d.Evidence[clave].(float64)
				if !ok || got != v {
					t.Errorf("evidence[%q] = %v, esperaba %v", clave, d.Evidence[clave], v)
				}
			}
			// NUMERADOR: el veredicto, y que el Why nombre cada eje con SU denominador.
			if alDia.Pass != c.pasa {
				t.Errorf("Pass = %v, esperaba %v (Why: %s)", alDia.Pass, c.pasa, alDia.Why)
			}
			for _, trozo := range []string{
				fmt.Sprintf("%d/%d fantasmas", c.fantasmas, c.indexados),
				fmt.Sprintf("%d/%d sin indexar", c.sinIndexar, enDisco),
				"tope 5 %",
			} {
				if !strings.Contains(alDia.Why, trozo) {
					t.Errorf("el Why no trae %q: %s", trozo, alDia.Why)
				}
			}
		})
	}
}

// Que UN denominador venga en cero no apaga la medición de los otros ejes. La guarda de los dos
// denominadores en cero existe para el proyecto sin grafo y sin código; acá cada montaje tiene uno
// de los dos en cero y el otro con dato, y el chequeo TIENE que estar y TIENE que caer.
//
// Sabotaje que lo pone en rojo: medir los fantasmas contra los archivos del disco en vez de los del
// grafo. Con el grafo entero fantasma eso da 3/0, que `proporcion` devuelve como 0, y el rojo se
// vuelve verde.
// arnes: archivo="internal/mcp/methods_readiness.go"
// arnes: de="\tpGhosts := proporcion(ghosts, enGrafo)"
// arnes: a="\tpGhosts := proporcion(ghosts, enDisco)"
func TestFrescuraNoSeApagaConUnSoloDenominadorEnCero(t *testing.T) {
	t.Run("todo el grafo es fantasma: 3 de 3 borrados, disco sin un .go", func(t *testing.T) {
		s := grafoAsimetrico(t, 3, 3, 0)
		d := dimGrafo(t, readinessComo(t, s, nil))
		if g, _ := d.Evidence["archivos_en_grafo"].(float64); g != 3 {
			t.Fatalf("archivos_en_grafo = %v, esperaba 3: el montaje no reproduce el estado", d.Evidence["archivos_en_grafo"])
		}
		if k, _ := d.Evidence["archivos_en_disco"].(float64); k != 0 {
			t.Fatalf("archivos_en_disco = %v, esperaba 0: el montaje no reproduce el estado", d.Evidence["archivos_en_disco"])
		}
		c := chequeo(d, nombreAlDia)
		if c == nil {
			t.Fatalf("no está el chequeo %q: con el grafo entero fantasma la guarda de cero lo dejó sin medir", nombreAlDia)
		}
		if c.Pass {
			t.Errorf("el chequeo PASÓ con los 3 archivos del grafo borrados del disco: %s", c.Why)
		}
	})
	t.Run("nodos sin archivo y un .go en disco que nadie indexó", func(t *testing.T) {
		engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { engine.Close() })
		nodo := memory.GraphNode{Key: "#func:HuerfanoSinArchivo", Kind: "func", Name: "HuerfanoSinArchivo"}
		if err := engine.UpsertPackageGraph(nil, []memory.GraphNode{nodo}, nil); err != nil {
			t.Fatal(err)
		}
		raiz := t.TempDir()
		escribirGo(t, raiz, "sin_indexar.go", "package p\n\nfunc X() {}\n")
		s := NewMcpServer(engine, raiz, embedding.NoopProvider{})
		if s.arbolFueraDeAlcance() {
			t.Fatal("el árbol salió fuera de alcance: no se mediría frescura")
		}
		d := dimGrafo(t, readinessComo(t, s, nil))
		if !d.Observed {
			t.Fatal("la dimensión no quedó observada: el montaje no reproduce el estado")
		}
		if g, _ := d.Evidence["archivos_en_grafo"].(float64); g != 0 {
			t.Fatalf("archivos_en_grafo = %v, esperaba 0", d.Evidence["archivos_en_grafo"])
		}
		if k, _ := d.Evidence["archivos_en_disco"].(float64); k != 1 {
			t.Fatalf("archivos_en_disco = %v, esperaba 1", d.Evidence["archivos_en_disco"])
		}
		c := chequeo(d, nombreAlDia)
		if c == nil {
			t.Fatalf("no está el chequeo %q: con un .go sin indexar y cero en el grafo la guarda lo dejó sin medir", nombreAlDia)
		}
		if c.Pass {
			t.Errorf("el chequeo PASÓ con 1 de 1 .go del disco sin indexar (100 %%): %s", c.Why)
		}
	})
}

// El veredicto lo da el PEOR de los tres ejes, no su promedio. Con un eje al 10 % y los otros dos
// sanos, promediar da 3,3 % y pasaría el tope del 5 %.
//
// Sabotaje que lo pone en rojo: ignorar el eje `ghosts` al elegir el peor de los tres.
// arnes: archivo="internal/mcp/methods_readiness.go"
// arnes: de="\tif pGhosts > peor {"
// arnes: a="\tif false && pGhosts > peor {"
func TestElPeorEjeMandaYNoElPromedio(t *testing.T) {
	s := grafoAsimetrico(t, 20, 2, 0) // 2 de 20 fantasmas = 10 %; stale y missing en 0
	d := dimGrafo(t, readinessComo(t, s, nil))
	c := chequeo(d, nombreAlDia)
	if c == nil {
		t.Fatalf("no está el chequeo %q", nombreAlDia)
	}
	if g, _ := d.Evidence["ghosts"].(float64); g != 2 {
		t.Fatalf("evidence[ghosts] = %v, esperaba 2: el montaje no reproduce el estado", d.Evidence["ghosts"])
	}
	if c.Pass {
		t.Errorf("el chequeo PASÓ con un eje al 10 %% (tope 5 %%) y los otros dos sanos: promedia los "+
			"tres ejes en vez de tomar el peor. Why: %s", c.Why)
	}
}

// grafoEnDirectorioSaltado: el único archivo del grafo vive en `testdata/`, que `walkSourceTree`
// saltea. El grafo lo recuerda (1 archivo); en disco está legible y fresco (`borrar=false`) o ya no
// está (`borrar=true`); y los indexables que el walker enumera son CERO en los dos casos.
func grafoEnDirectorioSaltado(t *testing.T, borrar bool) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	raiz := t.TempDir()
	if err := os.MkdirAll(filepath.Join(raiz, "testdata"), 0o755); err != nil {
		t.Fatal(err)
	}
	rel := "testdata/a.go"
	fp := escribirGo(t, raiz, rel, "package p\n\nfunc F() {}\n")
	nodo := memory.GraphNode{Key: rel + "#func:F", Kind: "func", Name: "F", Path: rel, SrcFingerprint: fp}
	if err := engine.UpsertPackageGraph([]string{rel}, []memory.GraphNode{nodo}, nil); err != nil {
		t.Fatal(err)
	}
	if borrar {
		if err := os.Remove(filepath.Join(raiz, rel)); err != nil {
			t.Fatal(err)
		}
	}
	s := NewMcpServer(engine, raiz, embedding.NoopProvider{})
	if s.arbolFueraDeAlcance() {
		t.Fatal("el árbol salió fuera de alcance: no se mediría frescura")
	}
	return s
}

// Un eje SIN denominador no es podredumbre: `proporcion` lo documenta —si no hay indexables en
// disco no puede faltar ninguno, y la proporción honesta es 0, no un NaN ni un 1—. Éste es el único
// estado donde esa regla se observa sin que la guarda de los dos denominadores en cero la tape: el
// único archivo del grafo está legible y fresco, pero vive en un directorio que `walkSourceTree`
// saltea, así que el walker enumera CERO indexables y `missing`/`enDisco` queda en 0/0.
//
// QUÉ NO DECIDE ESTE TEST: si en ese cuadrante el chequeo tiene que publicarse o no. Ése es un
// hueco de contrato abierto que resuelve el dueño (ver la pregunta abierta de la spec), y por eso
// el aserto tolera que el chequeo no esté. Fija lo único que no está en discusión: SI el chequeo
// está, un eje sin denominador no puede ser la causa de un rojo.
//
// Sabotaje que lo pone en rojo: que `proporcion` devuelva 1 cuando el denominador es cero.
// arnes: archivo="internal/mcp/methods_readiness.go"
// arnes: de="\tif total <= 0 {\n\t\treturn 0\n\t}"
// arnes: a="\tif total <= 0 {\n\t\treturn 1\n\t}"
func TestUnEjeSinDenominadorNoPoneRojo(t *testing.T) {
	s := grafoEnDirectorioSaltado(t, false)
	d := dimGrafo(t, readinessComo(t, s, nil))

	// DENOMINADOR: 1 archivo en el grafo, 0 indexables en disco. Si el walker algún día entra a
	// `testdata/`, el montaje deja de medir lo que dice y el test tiene que morir ruidoso.
	if g, _ := d.Evidence["archivos_en_grafo"].(float64); g != 1 {
		t.Fatalf("archivos_en_grafo = %v, esperaba 1: el montaje no reproduce el estado", d.Evidence["archivos_en_grafo"])
	}
	if k, _ := d.Evidence["archivos_en_disco"].(float64); k != 0 {
		t.Fatalf("archivos_en_disco = %v, esperaba 0 (el walker saltea testdata/): el montaje no reproduce el estado", d.Evidence["archivos_en_disco"])
	}
	for _, clave := range []string{"stale", "ghosts", "missing"} {
		if v, ok := d.Evidence[clave].(float64); !ok || v != 0 {
			t.Fatalf("evidence[%q] = %v, esperaba 0: el montaje no reproduce el estado", clave, d.Evidence[clave])
		}
	}

	// NUMERADOR: ningún eje está en rojo, así que el chequeo, si está, pasa.
	if c := chequeo(d, nombreAlDia); c != nil && !c.Pass {
		t.Errorf("el chequeo CAYÓ con el único archivo del grafo fresco y 0 indexables en disco: un eje "+
			"sin denominador no es podredumbre. Why: %s", c.Why)
	}
}
