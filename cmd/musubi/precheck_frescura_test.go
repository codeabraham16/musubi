package main

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"musubi/internal/memory"
)

// huellaDe devuelve la huella que el indexador habría guardado para este contenido. Se calcula
// como la calcula memory.FileFingerprint, y no se copia de ninguna constante: una huella tipeada a
// mano en un test es una prueba que verifica su propio literal.
func huellaDe(contenido string) string {
	sum := sha256.Sum256([]byte(contenido))
	return hex.EncodeToString(sum[:])
}

// grafoConHuella arma el mismo grafo que graphStore() pero con la huella que se le pase, para poder
// separar «el archivo cambió» de «el archivo es el que se indexó».
func grafoConHuella(huella string) *fakeCodeStore {
	s := graphStore()
	for archivo, nodos := range s.graphNodes {
		for i := range nodos {
			nodos[i].SrcFingerprint = huella
		}
		s.graphNodes[archivo] = nodos
	}
	return s
}

func contextoDeLectura(t *testing.T, store *fakeCodeStore, root string) string {
	t.Helper()
	in := `{"tool_name":"Read","tool_input":{"file_path":"a.go"},"session_id":"s"}`
	_, ctx := hookAdditionalContext(t, precheckOutput(store, root, strings.NewReader(in)))
	return ctx
}

func contextoDeEdicion(t *testing.T, store *fakeCodeStore, root string) string {
	t.Helper()
	in := `{"tool_name":"Edit","tool_input":{"file_path":"a.go"},"session_id":"s"}`
	_, ctx := hookAdditionalContext(t, precheckOutput(store, root, strings.NewReader(in)))
	return ctx
}

// ═════════════════════════════════════════════════════════════════════════════════════════════
// EL GRAFO INYECTADO DICE SI ESTÁ AL DÍA, Y ANTES AFIRMABA QUE SÍ SIN MIRAR
//
// El mensaje era «"X" está indexado: navegá su estructura SIN leerlo», sin condición y sin marca
// de rancio. Mientras el hook estuvo OPT-IN eso no costaba nada; el 2026-09-05 pasó a encendido
// por default y se empezó a pagar en CADA lectura. Medido ese día en este repo: 798 archivos con
// huella, 7 viejos, cuatro de ellos cambiados por el mismo despliegue que encendió el hook — un
// Read de internal/fleet/procparse.go inyectaba diez símbolos SIN `ElegirTemperatura`, la función
// que ese despliegue había agregado.
//
// Lo que se retira cuando está viejo NO es el mensaje: es la invitación a no leer. Los callers y
// callees viven en OTROS archivos, así que la lectura que se está haciendo no los reemplaza y
// siguen valiendo.
func TestElGrafoInyectadoDiceSiEstaAlDia(t *testing.T) {
	const contenido = "package a\nfunc Alpha(){ beta() }\nfunc beta(){}\n"

	t.Run("al día: la huella coincide y se puede navegar sin leer", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "a.go", contenido)
		ctx := contextoDeLectura(t, grafoConHuella(huellaDe(contenido)), root)
		if !strings.Contains(ctx, "COINCIDE") {
			t.Errorf("con la huella igual debe decir que coincide, obtuve %q", ctx)
		}
		if !strings.Contains(ctx, "SIN leerlo") {
			t.Errorf("al día SÍ vale la invitación a no leer, obtuve %q", ctx)
		}
	})

	t.Run("cambiado: lo dice, retira la invitación y manda a reindexar", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "a.go", contenido)
		// La huella es la de OTRO contenido: es el caso real, el archivo se editó después de indexar.
		ctx := contextoDeLectura(t, grafoConHuella(huellaDe(contenido+"func gamma(){}\n")), root)
		if !strings.Contains(ctx, "CAMBIÓ") {
			t.Errorf("con la huella distinta debe decir que el archivo cambió, obtuve %q", ctx)
		}
		if strings.Contains(ctx, "SIN leerlo") {
			t.Errorf("ESTO es lo que importa: sobre un grafo viejo NO puede invitar a no leer, obtuve %q", ctx)
		}
		if !strings.Contains(ctx, "musubi_codegraph_index") {
			t.Errorf("tiene que decir cómo arreglarlo, obtuve %q", ctx)
		}
		// Y no se calla: los callers/callees no salen del archivo que se está leyendo.
		if !strings.Contains(ctx, "Alpha") || !strings.Contains(ctx, "beta") {
			t.Errorf("la estructura sigue sirviendo aunque esté vieja, obtuve %q", ctx)
		}
	})

	t.Run("sin huella guardada: no afirma que coincide", func(t *testing.T) {
		// El caso de un grafo indexado antes de que se guardaran huellas. La primera versión de
		// medirFrescura saltaba estos nodos y devolvía «al día», o sea afirmaba haber comparado
		// algo que no comparó. Lo destapó TestPrecheckSurfacesCodeGraphWhenEnabled, que arma sus
		// nodos así y pasaba en verde leyendo esa afirmación.
		root := t.TempDir()
		writeFile(t, root, "a.go", contenido)
		ctx := contextoDeLectura(t, graphStore(), root)
		if strings.Contains(ctx, "COINCIDE") {
			t.Errorf("sin huella guardada no se puede afirmar que coincide, obtuve %q", ctx)
		}
		if !strings.Contains(ctx, "NO pude verificar") {
			t.Errorf("tiene que decir que no sabe, obtuve %q", ctx)
		}
	})

	t.Run("el archivo ya no está: dice que no sabe, no que cambió", func(t *testing.T) {
		// Un nodo fantasma y un archivo editado se arreglan distinto —reindexar contra revisar el
		// grafo—, así que confundirlos manda a la persona equivocada al lugar equivocado.
		root := t.TempDir() // el a.go del grafo NO se escribe
		ctx := contextoDeLectura(t, grafoConHuella(huellaDe(contenido)), root)
		if strings.Contains(ctx, "CAMBIÓ") {
			t.Errorf("sin archivo en disco no se sabe si cambió, obtuve %q", ctx)
		}
		if !strings.Contains(ctx, "NO pude verificar") {
			t.Errorf("tiene que decir que no sabe, obtuve %q", ctx)
		}
	})
}

// ═════════════════════════════════════════════════════════════════════════════════════════════
// EL RADIO DE IMPACTO NO DESCRIBE: TRANQUILIZA, Y ESO SOBRE UN GRAFO VIEJO ES PEOR
//
// «Ningún símbolo de "X" tiene callers en el grafo: tocarlo no arrastra a nadie conocido» es una
// afirmación de SEGURIDAD, y sobre un grafo viejo falla en la dirección que duele: el caller nuevo
// es exactamente el que el grafo todavía no vio. Y esto corre antes de CADA edición.
//
// Nota: impactMessage nunca estuvo detrás de MUSUBI_CODEGRAPH_HOOK —ver
// TestElRadioDeImpactoNoDependeDelOptInDeLectura—, así que este camino viene encendido desde que
// existe y el hueco es más viejo que el cambio de default.
func TestElRadioDeImpactoNoTranquilizaSobreUnGrafoViejo(t *testing.T) {
	const contenido = "package a\nfunc Alpha(){ beta() }\nfunc beta(){}\n"

	t.Run("al día: puede afirmar", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "a.go", contenido)
		ctx := contextoDeEdicion(t, grafoConHuella(huellaDe(contenido)), root)
		if strings.Contains(ctx, "OJO") {
			t.Errorf("con el grafo al día no hace falta ninguna reserva, obtuve %q", ctx)
		}
	})

	t.Run("cambiado: el encabezado lleva la reserva", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "a.go", contenido)
		ctx := contextoDeEdicion(t, grafoConHuella(huellaDe(contenido+"//x\n")), root)
		if !strings.Contains(ctx, "OJO") || !strings.Contains(ctx, "caller nuevo no aparece") {
			t.Errorf("el radio de impacto sobre un grafo viejo tiene que avisar, obtuve %q", ctx)
		}
	})

	t.Run("«no arrastra a nadie» sólo si el grafo está al día", func(t *testing.T) {
		// El archivo aislado: sin callers. Al día es una afirmación útil; viejo es una trampa.
		root := t.TempDir()
		writeFile(t, root, "a.go", contenido)
		aislado := func(huella string) *fakeCodeStore {
			s := grafoConHuella(huella)
			s.inEdges = map[string][]memory.GraphEdge{} // nadie llama a nada
			return s
		}
		alDia := contextoDeEdicion(t, aislado(huellaDe(contenido)), root)
		if !strings.Contains(alDia, "no arrastra a nadie conocido") {
			t.Errorf("al día tiene que poder decirlo, obtuve %q", alDia)
		}
		viejo := contextoDeEdicion(t, aislado(huellaDe(contenido+"//x\n")), root)
		if strings.Contains(viejo, "no arrastra a nadie conocido") {
			t.Errorf("sobre un grafo viejo NO puede decir que no arrastra a nadie, obtuve %q", viejo)
		}
		if !strings.Contains(viejo, "el grafo no sabe") {
			t.Errorf("tiene que decir de quién es la ignorancia, obtuve %q", viejo)
		}
	})
}
