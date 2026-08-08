package mcp

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"musubi/internal/cognition"
	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// Invariantes del spec «El juez se puede medir» (specs/juez-medible/) que viven en este paquete.

// motorEspia guarda el último prompt que recibió y cuenta las llamadas.
type motorEspia struct {
	respuesta string
	err       error
	llamadas  atomic.Int64
	system    atomic.Pointer[string]
	user      atomic.Pointer[string]
}

func (m *motorEspia) Name() string { return "espia" }

func (m *motorEspia) Ask(_ context.Context, system, user string) (string, error) {
	m.llamadas.Add(1)
	m.system.Store(&system)
	m.user.Store(&user)
	if m.err != nil {
		return "", m.err
	}
	return m.respuesta, nil
}

func (m *motorEspia) prompt() (string, string) {
	s, u := m.system.Load(), m.user.Load()
	if s == nil || u == nil {
		return "", ""
	}
	return *s, *u
}

// servidorConJuez arma un servidor con el juez read-time ENCENDIDO. No necesita base sembrada: las
// pruebas llaman a rerankIfEnabled con un RecallResult armado a mano, lo que las vuelve
// deterministas y las libera del ranking (que no es lo que están midiendo).
func servidorConJuez(t *testing.T, motor cognition.Provider) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	si := true
	return NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{},
		WithCognition(motor),
		WithCognitionConfig(config.CognitionConfig{Provider: "fake", ReadTimeRerank: &si}))
}

func itemsDePrueba() memory.RecallResult {
	return memory.RecallResult{Items: []memory.RecallItem{
		{ID: "a", Gist: "el candado no cruza la red"},
		{ID: "b", Gist: "el bump de accesos es atómico"},
		{ID: "c", Gist: "el juez ordena, no descarta"},
	}}
}

func candidatosDePrueba() []cognition.Candidato {
	return []cognition.Candidato{
		{ID: "a", Gist: "el candado no cruza la red"},
		{ID: "b", Gist: "el bump de accesos es atómico"},
		{ID: "c", Gist: "el juez ordena, no descarta"},
	}
}

// J1 — EL PROMPT ES EL MISMO, BYTE A BYTE.
//
// Es LA prueba del spec. El banco de evaluación (internal/recalleval) llama a cognition.Rerank; el
// servidor llama a rerankIfEnabled. Si los dos caminos armaran prompts distintos, el banco mediría
// una IMITACIÓN del juez y devolvería un número con aspecto de autoridad sobre algo que no corre en
// producción — y el día que el prompt de producción cambie, el número seguiría igual de convincente
// y ya sería falso.
func TestJ1ElPromptDeProduccionEsElDelPaqueteCognition(t *testing.T) {
	const consulta = "j1 prompt identico"

	espiaProd := &motorEspia{respuesta: `["c","a","b"]`}
	s := servidorConJuez(t, espiaProd)
	s.rerankIfEnabled(context.Background(), consulta, itemsDePrueba())
	systemProd, userProd := espiaProd.prompt()
	if systemProd == "" {
		t.Fatal("el juez de producción no llamó al motor: la prueba no está midiendo lo que cree")
	}

	systemPaquete, userPaquete := cognition.PromptJuez(consulta, candidatosDePrueba())

	if systemProd != systemPaquete {
		t.Errorf("el SYSTEM difiere entre producción y el paquete:\nprod   : %q\npaquete: %q", systemProd, systemPaquete)
	}
	if userProd != userPaquete {
		t.Errorf("el USER difiere entre producción y el paquete:\nprod   : %q\npaquete: %q", userProd, userPaquete)
	}
}

// J3 — El caché de PRODUCCIÓN sigue vivo.
//
// Control de J2 (que exige que el juez NO cachee): sacar el caché de adentro del juez no puede
// costar la protección del rate-limit compartido que el caché vino a dar. Dos recalls idénticos ⇒
// una sola llamada al motor.
func TestJ3ElCacheDeProduccionSigueVivo(t *testing.T) {
	espia := &motorEspia{respuesta: `["c","a","b"]`}
	s := servidorConJuez(t, espia)
	// Consulta única por prueba: rerankCache es un global de paquete y una clave compartida con otra
	// prueba haría que ésta pase (o falle) por el motivo equivocado.
	const consulta = "j3 cache de produccion vivo"

	for i := 0; i < 3; i++ {
		s.rerankIfEnabled(context.Background(), consulta, itemsDePrueba())
	}
	if n := espia.llamadas.Load(); n != 1 {
		t.Fatalf("esperaba 1 llamada al motor con el caché de producción activo, hubo %d", n)
	}
}

// J9 — Si el motor falla, el orden model-free SOBREVIVE.
//
// Es la degradación best-effort que ya existía antes de la extracción y que ésta no puede romper:
// el recall nunca se rompe ni cambia por un LLM caído.
func TestJ9SiElMotorFallaElOrdenModelFreeSobrevive(t *testing.T) {
	casos := map[string]*motorEspia{
		"motor caído":           {err: errors.New("connection refused")},
		"respuesta imparseable": {respuesta: "no sé, perdón"},
		"respuesta vacía":       {respuesta: ""},
	}
	for nombre, espia := range casos {
		t.Run(nombre, func(t *testing.T) {
			s := servidorConJuez(t, espia)
			original := itemsDePrueba()
			got := s.rerankIfEnabled(context.Background(), "j9 "+nombre, original)

			var ids []string
			for _, it := range got.Items {
				ids = append(ids, it.ID)
			}
			if strings.Join(ids, ",") != "a,b,c" {
				t.Fatalf("el orden model-free no sobrevivió: obtuve %v, esperaba [a b c]", ids)
			}
		})
	}
}

// Control de J9: cuando el motor SÍ contesta bien, el orden cambia. Sin esta prueba, romper el juez
// entero pasaría en verde — J9 sola se satisface con un juez que no hace nada.
func TestJ9ControlConMotorSanoElOrdenCambia(t *testing.T) {
	espia := &motorEspia{respuesta: `["c","b","a"]`}
	s := servidorConJuez(t, espia)
	got := s.rerankIfEnabled(context.Background(), "j9 control motor sano", itemsDePrueba())

	var ids []string
	for _, it := range got.Items {
		ids = append(ids, it.ID)
	}
	if strings.Join(ids, ",") != "c,b,a" {
		t.Fatalf("esperaba que el juez reordenara a [c b a], obtuve %v", ids)
	}
}
