package mcp

// marca_diseno_test.go — invariantes de la RESOLUCIÓN DE MARCA (plan de cierre, fase 5 / F8).

import (
	"strings"
	"testing"

	"musubi/internal/memory"
)

func servidorConMarca(t *testing.T, tenant, texto string) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	engine.SetProjectID("")
	if err := engine.SaveObservationTypedFrom(tenant, "", "marca-"+tenant,
		brandTopicKey, texto, 1.0, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	return NewMcpServer(engine, t.TempDir(), nil)
}

// I-MRC1 · LA MARCA NO SE PIERDE POR UNA MAYÚSCULA.
//
// Medido en producción el 2026-08-30: con `brand: "Altura"` el motor devolvía «SIN MARCA DEFINIDA
// para este proyecto» y le decía al agente que no usara la identidad de otro proyecto — cuando la
// correcta era la de Altura, que existe con el tenant en minúscula.
//
// SABOTAJE: sacar el ToLower de brandScopeFor ⇒ la variante con mayúscula vuelve a perder la marca.
func TestMarcaNoSePierdePorUnaMayuscula(t *testing.T) {
	const texto = "MARCA DE PRUEBA — el acento es el ámbar y la elevación es plana."
	s := servidorConMarca(t, "altura", texto)
	admin := &Principal{Name: "sala", ProjectID: "musubi", Read: "all", Write: "all"}

	for _, forma := range []string{"altura", "Altura", "ALTURA", "  Altura  "} {
		b := callDesignBrand(t, s, admin, "una tabla densa", "web", forma)
		if b.BrandSource != "project" {
			t.Errorf("brand=%q: brand_source quedó en %q y la marca EXISTE", forma, b.BrandSource)
		}
		if !strings.Contains(b.Brand, "MARCA DE PRUEBA") {
			t.Errorf("brand=%q: no llegó la marca del proyecto; llegó %.50s…", forma, b.Brand)
		}
	}
}

// I-MRC2 · «NO HAY MARCA» Y «PEDISTE UNA QUE NO EXISTE» NO SE DICEN IGUAL.
//
// Es el antipatrón de la casa: el valor de fallo idéntico al tranquilizador. Quien pide `brand:
// "altur"` por un dedazo recibía un brief que se lee legítimo y compone con el método universal, sin
// un solo indicio de que la marca que pidió nunca se encontró.
//
// SABOTAJE: devolver "" siempre en avisoDeMarca ⇒ el dedazo vuelve a ser indistinguible.
func TestMarcaPedidaQueNoExisteSeDeclara(t *testing.T) {
	s := servidorConMarca(t, "altura", "MARCA DE PRUEBA")
	admin := &Principal{Name: "sala", ProjectID: "musubi", Read: "all", Write: "all"}

	dedazo := callDesignBrand(t, s, admin, "una tabla densa", "web", "altur")
	if dedazo.BrandNote == "" {
		t.Error("se pidió una marca inexistente y el brief no lo declaró")
	}
	if !strings.Contains(dedazo.BrandNote, "altur") {
		t.Errorf("el aviso no dice QUÉ marca se pidió: %q", dedazo.BrandNote)
	}

	// Y el caso legítimo NO lleva aviso: una nota que aparece siempre es ruido.
	propio := callDesign(t, s, nil, "una tabla densa", "web")
	if propio.BrandNote != "" {
		t.Errorf("un pedido sin argumento de marca no debería llevar aviso: %q", propio.BrandNote)
	}
}
