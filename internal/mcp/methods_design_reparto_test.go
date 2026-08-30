package mcp

// methods_design_reparto_test.go — invariantes del REPARTO del presupuesto (2026-08-30).
//
// El defecto que esta fase cierra, medido contra el central: en un brief real de 2.533 tokens el
// 66 % era prosa CONSTANTE y el corpus —lo único que cambia con el pedido, y lo único con lo que se
// puede componer algo específico— eran cuatro titulares de 86 a 92 chars: el 10 %. El motor le
// entregaba al agente cuatro títulos y un sermón universal, y le pedía un viaje más a
// musubi_memory_expand para ver el material de verdad.
//
// El tope no estaba mal calibrado, estaba mal REPARTIDO. Lo que sigue son los invariantes del
// reparto nuevo, cada uno con el sabotaje que tiene que verlo fallar.

import (
	"strings"
	"testing"
	"unicode/utf8"

	"musubi/internal/memory"
)

// acervoDePatrones siembra el tenant de diseño con patrones de contenido conocido y devuelve el
// servidor. Sin embebedor: el camino léxico alcanza para lo que estos invariantes miden (qué se
// sirve y cuánto), y un embebedor falso mediría al embebedor falso.
func acervoDePatrones(t *testing.T, entradas map[string]string) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	engine.SetProjectID("")
	i := 0
	for topic, texto := range entradas {
		if err := engine.SaveObservationTypedFrom(designCorpusScope, "", "pat"+string(rune('a'+i)),
			topic, texto, 1.0, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
		i++
	}
	return NewMcpServer(engine, t.TempDir(), nil)
}

// I-MAT2 · EL CORPUS VIAJA CON SU TEXTO COMPLETO, NO CON UN TITULAR.
//
// Es el corazón del cambio. Hasta el 2026-08-30 el corpus pasaba por toSearchHits, que devuelve un
// gist de ~90 chars más el id para expandir: quien componía el diseño recibía titulares.
//
// SABOTAJE: volver a servir sólo la cabeza del contenido (o el gist) ⇒ el cuerpo del patrón deja de
// estar en el brief y este test se pone rojo. Verificado en rojo cortando el texto a 90 chars en
// comoPatronItem: "el cuerpo del patrón no llegó al brief".
func TestDesignElCorpusViajaCompletoNoEnTitular(t *testing.T) {
	// La marca del final es lo que distingue "vino entero" de "vino la cabeza": un gist se queda con
	// el principio, así que afirmar sobre el principio no probaría nada.
	cuerpo := "Para una tabla densa, la primera columna ancla la lectura y las numéricas van alineadas a la derecha con tabular-nums. " +
		strings.Repeat("El cuerpo del patrón sigue acá con detalle accionable. ", 6) +
		"CIERRE-DEL-PATRON-QUE-UN-TITULAR-NO-ALCANZA"
	s := acervoDePatrones(t, map[string]string{"design-corpus/tabla-densa": cuerpo})

	b := callDesign(t, s, nil, "tabla densa", "web")
	if len(b.Corpus) == 0 {
		t.Fatal("el acervo tenía material y el corpus salió vacío")
	}
	p := b.Corpus[0]
	if !strings.Contains(p.Texto, "CIERRE-DEL-PATRON-QUE-UN-TITULAR-NO-ALCANZA") {
		t.Errorf("el cuerpo del patrón no llegó al brief; se sirvieron %d chars de %d", len(p.Texto), len(cuerpo))
	}
	if p.Recortado {
		t.Error("un patrón de tamaño normal no debería salir recortado")
	}
	// Y la nota del corpus no puede seguir mandando a expandir lo que ya está servido: era la
	// instrucción correcta cuando se servían gists y es un viaje al pedo ahora.
	if strings.Contains(b.CorpusNote, "gist") {
		t.Errorf("corpus_note todavía habla de gists: %q", b.CorpusNote)
	}
}

// I-MAT3 · UNA TARJETA GORDA NO SE LLEVA EL BRIEF PUESTO.
//
// Servir el contenido completo abre la puerta que el gist tenía cerrada: el acervo real tiene 268
// artículos `ingested/*` de ~12.000 chars. Sin tope por tarjeta, uno solo desborda el brief entero.
//
// SABOTAJE: subir designPatronItemMax por encima del artículo (o sacarlo). Verificado en rojo con el
// tope en 999999, y el modo de falla es peor de lo que esperaba: el corpus sale VACÍO. Sin tope por
// tarjeta el artículo no desborda el brief —la escalera del presupuesto lo impide— sino que se lleva
// puesto al material entero para hacerle lugar. El tope duro se respeta y el brief queda sin nada.
func TestDesignUnPatronGordoNoSeLlevaElBrief(t *testing.T) {
	// UN SOLO patrón sembrado, y es el gordo. La primera versión sembraba también una tarjeta normal
	// y buscaba el artículo entre lo servido: cuando el artículo no entraba por esa consulta, el test
	// caía en un t.Skip y pasaba VERDE bajo el sabotaje. Un test con salida de emergencia mide que la
	// emergencia no ocurrió, no el invariante. Con un solo patrón, o se sirve ése o no hay corpus.
	gordo := strings.Repeat("un artículo entero sin destilar sobre tablas densas que ocupa muchísimo espacio. ", 300) // ~24.000 chars
	s := acervoDePatrones(t, map[string]string{"ingested/articulo-largo": gordo})

	b := callDesign(t, s, nil, "tablas densas", "web")
	if n := tokensDeBrief(b); n > designBriefBudget {
		t.Errorf("un solo artículo desbordó el brief: %d tokens sobre el tope %d", n, designBriefBudget)
	}
	if len(b.Corpus) != 1 || !strings.HasPrefix(b.Corpus[0].Topic, prefijoCrudo) {
		t.Fatalf("el artículo tenía que ser el único patrón servido; got=%d %+v", len(b.Corpus), b.Corpus)
	}
	p := b.Corpus[0]
	if len(p.Texto) > designPatronItemMax+64 { // +64: el aviso de recorte pega al final
		t.Errorf("el artículo entró con %d chars, sobre el tope por tarjeta %d", len(p.Texto), designPatronItemMax)
	}
	if !p.Recortado {
		t.Error("se recortó el artículo y no se declaró en 'recortado'")
	}
	// El costo entero es lo que le dice al agente si vale el viaje a musubi_memory_expand. Sin ese
	// número, "recortado" avisa que falta algo pero no cuánto.
	if p.FullTokens <= 0 {
		t.Error("un patrón recortado tiene que declarar cuánto mide entero")
	}
}

// I-MAT4 · EL RECORTE NO PARTE UN CARÁCTER EN DOS.
//
// El recorte viejo hacía `txt[:max]` a secas. En castellano eso corta una vocal acentuada entre sus
// dos bytes y el JSON sale con un U+FFFD donde había una letra. Con tarjetas de 245 chars casi nunca
// se veía; con artículos de 12.000 iba a pasar seguido.
//
// SABOTAJE: volver a cortar por byte crudo ⇒ el texto servido deja de ser UTF-8 válido. Verificado
// en rojo reemplazando recortarTexto por txt[:max].
func TestDesignElRecorteNoParteUnCaracter(t *testing.T) {
	// EL FIXTURE VERIFICA SU PROPIA PREMISA. La primera versión elegía el largo del prefijo a mano
	// para que el corte cayera adentro de un carácter de dos bytes, y le erré dos veces: el corte caía
	// justo en el borde y el sabotaje del corte por byte crudo pasaba VERDE. Un test que depende de
	// que yo cuente bytes bien es un test que pasa por suerte. Acá el relleno se ajusta hasta que el
	// byte del corte NO sea principio de carácter, y si no se logra el test lo dice en vez de pasar.
	relleno := "tablas densas "
	for (designPatronItemMax-len(relleno))%2 == 0 {
		relleno += "x"
	}
	contenido := relleno + strings.Repeat("ó", designPatronItemMax) + " FIN"
	if utf8.RuneStart(contenido[designPatronItemMax]) {
		t.Fatalf("el fixture no ejercita el invariante: el corte en %d cae en un borde de carácter", designPatronItemMax)
	}
	s := acervoDePatrones(t, map[string]string{"design-corpus/acentos": contenido})

	b := callDesign(t, s, nil, "tablas", "web")
	if len(b.Corpus) == 0 {
		t.Fatal("corpus vacío")
	}
	for _, p := range b.Corpus {
		if !utf8.ValidString(p.Texto) {
			t.Errorf("el recorte partió un carácter: el texto de %s no es UTF-8 válido", p.Topic)
		}
		if strings.ContainsRune(p.Texto, '�') {
			t.Errorf("el recorte dejó un carácter de reemplazo en %s", p.Topic)
		}
	}
}

// I-PRE5 · EL MATERIAL TIENE PISO DURO: LA MARCA CEDE ANTES DE ROMPERLO.
//
// La escalera anterior vaciaba método y corpus HASTA CERO antes de tocar la marca, y el banco lo
// mostró en vivo: con una marca gigante el brief salía con corpus 0, método 0 y `degraded` en FALSO.
// Un brief sin una sola pieza de conocimiento de diseño, con cara de completo, entregado a alguien
// que pidió que le diseñen algo.
//
// Que la marca gane por PRECEDENCIA no es ganar por ESPACIO: la precedencia decide quién manda
// cuando dos partes se contradicen, no la autoriza a quedarse con todo el canal.
//
// SABOTAJE: reponer la escalera vieja (los casos `len(b.Method) > 0` / `len(b.Corpus) > 0` antes de
// tocar la marca) ⇒ el corpus vuelve a cero. Verificado en rojo: "el corpus cayó a 0".
func TestDesignLaMarcaCedeAntesDeVaciarElMaterial(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	engine.SetProjectID("")

	// Acervo suficiente para llenar los dos pisos con material de verdad.
	for i := 0; i < 12; i++ {
		if err := engine.SaveObservationTypedFrom(designCorpusScope, "", "pat"+string(rune('a'+i)),
			"design-corpus/patron"+string(rune('a'+i)),
			"tabla densa: patrón número "+string(rune('a'+i))+" con su contenido accionable", 1.0, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 8; i++ {
		if err := engine.SaveObservationTypedFrom(designCorpusScope, "", "met"+string(rune('a'+i)),
			designMethodPrefix+"criterio"+string(rune('a'+i)),
			"el criterio universal número "+string(rune('a'+i)), 1.0, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
	}
	// Y una marca que sola no entra en el presupuesto.
	marcaGorda := strings.Repeat("La identidad de este cliente se describe con muchísimo detalle. ", 900)
	if err := engine.SaveObservationTypedFrom("cliente-gordo", "", "marca-gorda",
		brandTopicKey, marcaGorda, 1.0, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	s := NewMcpServer(engine, t.TempDir(), nil)

	admin := &Principal{Name: "sala", ProjectID: "musubi", Read: "all", Write: "all"}
	b := callDesignBrand(t, s, admin, "tabla densa", "web", "cliente-gordo")

	if n := tokensDeBrief(b); n > designBriefBudget {
		t.Errorf("el tope duro se rompió: %d tokens sobre %d", n, designBriefBudget)
	}
	if len(b.Corpus) < designPisoCorpus {
		t.Errorf("el corpus cayó a %d, bajo su piso duro de %d — la marca se comió el material", len(b.Corpus), designPisoCorpus)
	}
	if len(b.Method) < designPisoBloque {
		t.Errorf("el método cayó a %d, bajo su piso de %d", len(b.Method), designPisoBloque)
	}
	// Y la marca que cedió tiene que decirlo: sus prohibiciones viven al final.
	if !strings.Contains(b.Brand, "LA MARCA SE RECORTÓ") {
		t.Error("la marca cedió espacio y no avisó al lector")
	}
	if b.Truncated == nil || b.Truncated.Brand == nil {
		t.Error("el recorte de la marca no quedó declarado en 'truncated'")
	}
}

// I-PRE6 · EL PISO DEL CORPUS ES MÁS ALTO QUE EL DEL MÉTODO.
//
// Cuando falta lugar, lo que tiene que sobrevivir es lo ESPECÍFICO. El método es universal —el mismo
// criterio para cualquier pedido— y el corpus es lo único que cambia con lo que se pidió. Con los dos
// pisos iguales, el corpus cedía a la par de algo que no aportaba diferencia.
//
// SABOTAJE: igualar los dos pisos ⇒ este test se pone rojo y deja de haber nada que defienda al
// material específico por encima del universal.
func TestDesignElCorpusSeDefiendeMasQueElMetodo(t *testing.T) {
	if designPisoCorpus <= designPisoBloque {
		t.Errorf("el piso del corpus (%d) tiene que ser MÁS ALTO que el del método (%d): cuando falta lugar sobrevive lo específico, no lo universal",
			designPisoCorpus, designPisoBloque)
	}
}
