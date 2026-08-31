package mcp

// formas_diseno_test.go — invariantes de LA CAPA DE FORMA.

import (
	"strings"
	"testing"

	"musubi/internal/memory"
)

// I-FRM1 · EL MOTOR ACOTA, NO ELIGE.
//
// Elegir qué forma le pega a un pedido es un juicio, y el camino caliente es model-free por regla
// del proyecto. Servir UNA forma sería el motor decidiendo; servir las doce sería no acotar nada.
//
// SABOTAJE: devolver una sola candidata ⇒ el motor pasó a elegir y este test se pone rojo.
func TestFormasElMotorAcotaNoElige(t *testing.T) {
	for eje := range formasPorEje {
		b := formasPara(eje, nil, intencionDeDiseno{})
		if b == "" {
			t.Errorf("el eje %q está en la tabla y no propuso ninguna forma", eje)
			continue
		}
		n := strings.Count(b, "\n- ")
		if n < 2 {
			t.Errorf("el eje %q propuso %d forma(s): con una, el motor está eligiendo", eje, n)
		}
		if n > designFormasPropuestas {
			t.Errorf("el eje %q propuso %d formas, sobre el tope %d: deja de acotar", eje, n, designFormasPropuestas)
		}
	}
}

// I-FRM2 · UNA PROPIEDAD NO TIENE FORMA.
//
// `color`, `a11y`, `tipografia`, `terminacion` y `estado-vacio` son propiedades de una pantalla, no
// esqueletos. Proponerle una forma a «cómo se comporta la paleta en modo oscuro» es inventar una
// respuesta — el mismo criterio que ya usa la abstención cuando no hay material.
//
// SABOTAJE: darle candidatas a esos ejes ⇒ el motor propone esqueleto donde no hay pantalla.
func TestFormasUnaPropiedadNoTieneForma(t *testing.T) {
	for _, eje := range []string{"color", "a11y", "tipografia", "terminacion", "estado-vacio"} {
		if b := formasPara(eje, nil, intencionDeDiseno{}); b != "" {
			t.Errorf("el eje %q es una PROPIEDAD y le propusieron forma: %.60s…", eje, b)
		}
	}
	// Guarda de la premisa: si NINGÚN eje propusiera forma, este test pasaría por vacuidad.
	if formasPara("tabla", nil, intencionDeDiseno{}) == "" {
		t.Fatal("ningún eje propone forma: el test pasaría sin que la capa exista")
	}
}

// I-FRM3 · LA ROTACIÓN EXCLUYE, PERO NUNCA DEJA SIN FORMA.
//
// La rotación es una preferencia, no una prohibición: si la historia excluyó todas las candidatas de
// un eje, quedarse sin forma sería peor que repetir una.
//
// SABOTAJE: sacar el repliegue a la lista completa ⇒ un proyecto que ya usó las tres candidatas de
// su eje deja de recibir forma para siempre.
func TestFormasLaRotacionNoDejaSinForma(t *testing.T) {
	cands := formasPorEje["tabla"]
	if len(cands) == 0 {
		t.Fatal("el fixture necesita un eje con candidatas")
	}
	// Una usada: tiene que proponer las otras y NO la usada.
	usadas := map[string]bool{cands[0]: true}
	b := formasPara("tabla", usadas, intencionDeDiseno{})
	if strings.Contains(b, formasDeDiseno[cands[0]].Nombre) {
		t.Errorf("propuso la forma ya usada %q", cands[0])
	}
	if b == "" {
		t.Fatal("con una forma usada se quedó sin proponer nada")
	}
	// TODAS usadas: repliega a la lista completa en vez de quedarse mudo.
	todas := map[string]bool{}
	for _, c := range cands {
		todas[c] = true
	}
	// SE CUENTAN LAS OPCIONES, no se mira si la cadena es vacía. La primera versión comparaba
	// contra "" y el sabotaje pasaba verde: sin candidatas la función devolvía el ENCABEZADO SOLO,
	// o sea un bloque que dice «elegí UNA de estas» sin listar ninguna. Peor que no mandar nada.
	repliegue := formasPara("tabla", todas, intencionDeDiseno{})
	if n := strings.Count(repliegue, "\n- "); n == 0 {
		t.Errorf("con todas las formas usadas no quedó ninguna opción (bloque=%q); la rotación es preferencia, no prohibición", repliegue)
	}
}

// I-FRM4 · TODA FORMA REFERENCIADA EXISTE EN EL CATÁLOGO.
//
// Es el defecto exacto que se le encontró a otra skill del rubro auditándola: sus seis workflows
// mandaban configurar un «3-Dial System» que no estaba definido en ningún archivo, y su validador
// no lo veía porque comprobaba que las RUTAS resolvieran, no que los conceptos existieran. Un
// nombre invocado y no definido produce un brief que le pide al agente algo que no puede leer.
//
// SABOTAJE: nombrar una forma inexistente en formasPorEje ⇒ el brief propondría una línea con el
// nombre vacío y su descripción vacía, y este test la agarra.
func TestFormasTodaReferenciaExiste(t *testing.T) {
	for eje, cands := range formasPorEje {
		for _, c := range cands {
			f, ok := formasDeDiseno[c]
			if !ok {
				t.Errorf("el eje %q referencia la forma %q, que NO existe en el catálogo", eje, c)
				continue
			}
			if strings.TrimSpace(f.Nombre) == "" || strings.TrimSpace(f.Desc) == "" {
				t.Errorf("la forma %q existe pero está vacía: nombre=%q desc=%q", c, f.Nombre, f.Desc)
			}
		}
	}
	// Y al revés: una forma del catálogo que ningún eje alcanza es material muerto.
	alcanzada := map[string]bool{}
	for _, cands := range formasPorEje {
		for _, c := range cands {
			alcanzada[c] = true
		}
	}
	for nombre := range formasDeDiseno {
		if !alcanzada[nombre] {
			t.Errorf("la forma %q no la alcanza ningún eje: es material muerto", nombre)
		}
	}
}

// I-FRM5 · LA FORMA LLEGA AL BRIEF CUANDO SE RUTEÓ.
//
// SABOTAJE: no poblar Shape ⇒ el catálogo existe y nunca sale.
func TestFormasLaFormaLlegaAlBrief(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	engine.SetProjectID("")
	for i := 0; i < 8; i++ {
		if err := engine.SaveObservationTypedFrom(designCorpusScope, "", "f"+string(rune('a'+i)),
			designCorpusPrefix+"tabla-"+string(rune('a'+i)),
			"las columnas de la tabla se ordenan y el encabezado queda fijo al desplazar las filas",
			1.0, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
	}
	s := NewMcpServer(engine, t.TempDir(), &embebedorDeEje{eje: "tabla"})

	b := callDesign(t, s, nil, "una tabla densa", "web")
	if b.Axis != "tabla" {
		t.Fatalf("el fixture no ruteó a tabla (axis=%q): el test no prueba la forma", b.Axis)
	}
	if b.Shape == "" {
		t.Error("se ruteó a un eje CON formas y el brief no trajo ninguna")
	}
	if !strings.Contains(b.Shape, "tabla densa") {
		t.Errorf("el bloque de forma no propone la forma del eje: %.90s…", b.Shape)
	}
}

// ─── LA ROTACIÓN ─────────────────────────────────────────────────────────────────────────────

func servidorConFormas(t *testing.T) (*McpServer, *memory.DbEngine) {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	engine.SetProjectID("")
	for i := 0; i < 8; i++ {
		if err := engine.SaveObservationTypedFrom(designCorpusScope, "", "r"+string(rune('a'+i)),
			designCorpusPrefix+"tabla-"+string(rune('a'+i)),
			"las columnas de la tabla se ordenan y el encabezado queda fijo al desplazar las filas",
			1.0, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
	}
	return NewMcpServer(engine, t.TempDir(), &embebedorDeEje{eje: "tabla"}), engine
}

// I-ROT1 · LA ROTACIÓN EXCLUYE LA FORMA ANTERIOR DE ESTE PROYECTO.
//
// Es el mecanismo que las skills del rubro tienen que falsificar estampando un comentario en el CSS
// del artefacto, porque no tienen estado. Nosotros lo leemos de la memoria del proyecto.
//
// SABOTAJE: ignorar la historia y pasar siempre nil ⇒ la forma anterior vuelve a proponerse.
func TestRotacionExcluyeLaFormaAnterior(t *testing.T) {
	s, engine := servidorConFormas(t)
	admin := &Principal{Name: "sala", ProjectID: "proy-a", Read: "all", Write: "all"}

	antes := callDesign(t, s, admin, "una tabla densa", "web")
	if antes.Shape == "" {
		t.Fatal("sin historia ya no propone forma: el fixture no prueba la rotación")
	}
	if !strings.Contains(antes.Shape, "tabla densa") {
		t.Fatalf("el fixture esperaba «tabla densa» entre las candidatas: %.80s…", antes.Shape)
	}

	// El caller anota qué usó, en SU proyecto.
	if err := engine.SaveObservationTypedFrom("proy-a", "", "usada-1", formaUsadaTopic,
		"compuse con la forma tabla-densa", 1.0, "episodic", "local", nil); err != nil {
		t.Fatal(err)
	}

	despues := callDesign(t, s, admin, "una tabla densa", "web")
	if strings.Contains(despues.Shape, "tabla densa") {
		t.Errorf("la forma ya usada volvió a proponerse: %.100s…", despues.Shape)
	}
	if despues.Shape == "" {
		t.Error("la rotación dejó al brief sin forma; es preferencia, no prohibición")
	}
}

// I-ROT2 · LA HISTORIA ES DEL PROYECTO DEL PRINCIPAL, NO DE LA MARCA PEDIDA.
//
// `musubi_design` acepta `brand` para diseñar a nombre de otro proyecto. Si la historia se llaveara
// por marca, la sala de mando le escribiría la rotación a Altura cada vez que diseña para ellos.
//
// SABOTAJE: llavear por brandScope ⇒ la historia de un proyecto contamina la del otro.
func TestRotacionSeLlaveaPorElPrincipalNoPorLaMarca(t *testing.T) {
	s, engine := servidorConFormas(t)
	// La historia vive en el proyecto de la SALA.
	if err := engine.SaveObservationTypedFrom("sala-proy", "", "usada-2", formaUsadaTopic,
		"usé tabla-densa", 1.0, "episodic", "local", nil); err != nil {
		t.Fatal(err)
	}
	sala := &Principal{Name: "sala", ProjectID: "sala-proy", Read: "all", Write: "all"}

	// Diseña a nombre de OTRO proyecto: la historia que manda sigue siendo la de la sala.
	b := callDesignBrand(t, s, sala, "una tabla densa", "web", "altura")
	if strings.Contains(b.Shape, "tabla densa") {
		t.Error("no aplicó la historia del principal al diseñar con otra marca")
	}
	// Y la historia NO se leyó del proyecto de la marca: si se hubiera leído de «altura», que no
	// tiene ninguna, «tabla densa» seguiría propuesta. La aserción de arriba lo cubre.
}

// I-ROT3 · SIN HISTORIA, EL BRIEF LO DICE — Y PIDE LA PRIMERA ANOTACIÓN.
//
// Es el bug que este plan predijo: la rotación depende de que el caller registre, y se va a olvidar.
// Un silencio que se lee igual que «este proyecto es nuevo» es el antipatrón de esta casa. Y la
// nota va SIEMPRE, porque si sólo apareciera con historia no existiría nunca la primera anotación.
//
// SABOTAJE: emitir la nota sólo cuando hay historia ⇒ la rotación no arranca jamás.
func TestRotacionSinHistoriaSeDeclaraYSePide(t *testing.T) {
	s, _ := servidorConFormas(t)
	admin := &Principal{Name: "sala", ProjectID: "proy-nuevo", Read: "all", Write: "all"}

	b := callDesign(t, s, admin, "una tabla densa", "web")
	if b.ShapeHistory == "" {
		t.Fatal("hay bloque de forma y no se declaró el estado de la historia")
	}
	if !strings.Contains(b.ShapeHistory, "no hay registro") {
		t.Errorf("un proyecto sin historia tiene que decirlo: %q", b.ShapeHistory)
	}
	if !strings.Contains(b.ShapeHistory, formaUsadaTopic) {
		t.Error("la nota no dice DÓNDE anotar la forma usada: la rotación no puede arrancar")
	}
}

// I-ROT4 · UNA NOTA ESCRITA A MANO NO APAGA LA ROTACIÓN.
//
// El contenido lo escribe un agente, no un formato. Exigir una estructura exacta haría que la
// rotación se apague ante la primera nota redactada distinto — y se apagaría en silencio.
//
// SABOTAJE: exigir un formato exacto (JSON, o el nombre solo) ⇒ una nota en prosa deja de contar.
func TestRotacionToleraLaNotaEnProsa(t *testing.T) {
	for _, nota := range []string{
		"tabla-densa",
		"Compuse la pantalla con la forma «tabla densa», que era la que mejor pegaba.",
		"forma: TABLA DENSA · 2026-08-31",
	} {
		s, engine := servidorConFormas(t)
		if err := engine.SaveObservationTypedFrom("p", "", "u", formaUsadaTopic, nota, 1.0, "episodic", "local", nil); err != nil {
			t.Fatal(err)
		}
		usadas, hubo := s.formasUsadasPor("p")
		if !hubo || !usadas["tabla-densa"] {
			t.Errorf("no reconoció la forma en la nota %q (usadas=%v)", nota, usadas)
		}
	}
}
