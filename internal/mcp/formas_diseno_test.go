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
		b := formasPara(eje, nil)
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
		if b := formasPara(eje, nil); b != "" {
			t.Errorf("el eje %q es una PROPIEDAD y le propusieron forma: %.60s…", eje, b)
		}
	}
	// Guarda de la premisa: si NINGÚN eje propusiera forma, este test pasaría por vacuidad.
	if formasPara("tabla", nil) == "" {
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
	b := formasPara("tabla", usadas)
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
	repliegue := formasPara("tabla", todas)
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
