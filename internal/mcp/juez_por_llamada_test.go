package mcp

import (
	"context"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// juez_por_llamada_test.go — «quién paga los 8,5 segundos».
//
// EL PORQUÉ (medido el 2026-08-10 contra la memoria real del cerebro central, 1.303 docs): el juez
// de pertinencia sube nDCG@1 de 0,359 a 0,769 —+114 %, pone lo correcto primero— y cuesta ~8,5 s
// por consulta. No mejora el recall: R@10 se mueve 5,6 %. Es un reordenador, y hace exactamente lo
// que un reordenador debe hacer.
//
// Con esos dos números juntos, la perilla global que existía queda con la forma equivocada:
// encenderla mete 8,5 s en el camino caliente de todo recall; apagarla se lo niega a quien está
// esperando y lo pagaría. La decisión tiene que poder bajar a la llamada — sin cambiarle el default
// a nadie.

// servidorConDial arma un servidor con motor falso y el dial de la config en el estado pedido.
func servidorConDial(t *testing.T, motor *motorEspia, dialEncendido bool) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	estado := dialEncendido
	return NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{},
		WithCognition(motor),
		WithCognitionConfig(config.CognitionConfig{Provider: "fake", ReadTimeRerank: &estado}))
}

func ptrBool(b bool) *bool { return &b }

// ★ R1 — LOS CUATRO ESTADOS DE LA DECISIÓN.
//
// La tabla completa en una sola prueba a propósito: el valor de un tri-estado está en que las
// cuatro combinaciones se lean juntas. Verlas separadas es como se cuela una que quedó al revés.
func TestElJuezSeDecidePorLlamada(t *testing.T) {
	casos := []struct {
		nombre     string
		dial       bool
		pedido     *bool
		esperaJuez bool
		porque     string
	}{
		{"dial apagado, sin pedido", false, nil, false,
			"el default no puede cambiar: sin opinión del llamador manda la config"},
		{"dial encendido, sin pedido", true, nil, true,
			"sin opinión del llamador manda la config, también cuando dice que sí"},
		{"dial apagado, pedido true", false, ptrBool(true), true,
			"quien está esperando la respuesta puede comprar el juez sin tocarle el dial a nadie"},
		{"dial encendido, pedido false", true, ptrBool(false), false,
			"un sondeo tiene que poder correr barato contra un servidor en turbo"},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			motor := &motorEspia{respuesta: `["c","b","a"]`}
			s := servidorConDial(t, motor, c.dial)

			s.rerankSiCorresponde(context.Background(), "consulta "+c.nombre, itemsDePrueba(), c.pedido)

			llamo := motor.llamadas.Load() > 0
			if llamo != c.esperaJuez {
				t.Errorf("juez llamado = %v, se esperaba %v — %s", llamo, c.esperaJuez, c.porque)
			}
		})
	}
}

// ★ R2 — Y NO ES SÓLO QUE LLAME: EL ORDEN CAMBIA.
//
// Contar llamadas al motor demostraría que el cable está enchufado, no que el resultado se usa. Un
// juez que se invoca y cuyo veredicto se descarta pasaría R1 entero.
func TestElPedidoExplicitoReordenaDeVerdad(t *testing.T) {
	motor := &motorEspia{respuesta: `["c","b","a"]`}
	s := servidorConDial(t, motor, false) // dial APAGADO: todo lo que pase es por el pedido

	antes := itemsDePrueba()
	if antes.Items[0].ID != "a" {
		t.Fatalf("precondición rota: el fixture tiene que empezar en 'a', empieza en %q", antes.Items[0].ID)
	}

	got := s.rerankSiCorresponde(context.Background(), "r2 reordena", antes, ptrBool(true))
	if got.Items[0].ID != "c" {
		t.Errorf("con rerank:true el veredicto del juez tiene que aplicarse; el tope quedó en %q", got.Items[0].ID)
	}

	// Y el control: con false, el mismo motor y el mismo fixture no mueven nada.
	motor2 := &motorEspia{respuesta: `["c","b","a"]`}
	s2 := servidorConDial(t, motor2, true) // dial ENCENDIDO
	quieto := s2.rerankSiCorresponde(context.Background(), "r2 quieto", itemsDePrueba(), ptrBool(false))
	if quieto.Items[0].ID != "a" {
		t.Errorf("con rerank:false el orden model-free tiene que quedar intacto; el tope quedó en %q", quieto.Items[0].ID)
	}
}

// ★ R3 — SE ANUNCIA, Y CON SU COSTO.
//
// La lección de #282, aplicada de entrada en vez de descubierta después: un parámetro que el
// handler acepta pero el catálogo no declara es inalcanzable. Y acá hay un agravante — este cuesta
// 8,5 s. Anunciarlo sin decir el precio invita a usarlo en un bucle.
func TestElRerankPorLlamadaSeAnunciaConSuCosto(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())

	var props map[string]Property
	hallada := false
	for i := range s.tools {
		if s.tools[i].Name == "musubi_recall" {
			props, hallada = s.tools[i].InputSchema.Properties, true
			break
		}
	}
	if !hallada {
		t.Fatal("musubi_recall no está registrada")
	}

	prop, ok := props["rerank"]
	if !ok {
		t.Fatal("musubi_recall acepta 'rerank' en el handler pero NO lo declara en su inputSchema: nadie puede pedirlo")
	}
	if prop.Type != "boolean" {
		t.Errorf("'rerank' se anuncia como %q y el handler lo decodifica como *bool", prop.Type)
	}
	if !strings.Contains(prop.Description, "8,5 s") {
		t.Errorf("la descripción de 'rerank' no dice lo que cuesta; un opt-in caro sin precio se usa en bucles: %q", prop.Description)
	}
	if !strings.Contains(strings.ToLower(prop.Description), "ausente") {
		t.Errorf("la descripción no explica qué pasa si no se manda, que es el caso más frecuente: %q", prop.Description)
	}
}

// ★ R4 — UN `rerank:true` COMPRA EL INTENTO, NO UN PRIVILEGIO.
//
// El pedido explícito no puede saltearse el freno de gasto: si pudiera, cualquier llamador tendría
// una puerta para vaciar la cuota del motor con sólo pedirlo, y el freno dejaría de ser un freno.
func TestElPedidoExplicitoNoSalteaElFreno(t *testing.T) {
	motor := &motorEspia{respuesta: `["c","b","a"]`}
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	no := false
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{},
		WithCognition(motor),
		WithCognitionConfig(config.CognitionConfig{Provider: "fake", ReadTimeRerank: &no}),
		WithMotorQuota(1))

	ctx := ctxPrincipal("r4")

	// La primera gasta el único crédito. Query distintas para no comer del caché, que es lo que
	// mediría si las dos fueran iguales.
	s.rerankSiCorresponde(ctx, "r4 primera", itemsDePrueba(), ptrBool(true))
	if got := motor.llamadas.Load(); got != 1 {
		t.Fatalf("la primera con rerank:true tenía que llamar al motor una vez, llamó %d", got)
	}

	s.rerankSiCorresponde(ctx, "r4 segunda", itemsDePrueba(), ptrBool(true))
	if got := motor.llamadas.Load(); got != 1 {
		t.Errorf("rerank:true se saltó el freno de gasto: el motor se llamó %d veces con presupuesto 1", got)
	}
}
