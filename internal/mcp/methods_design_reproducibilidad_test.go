package mcp

// methods_design_reproducibilidad_test.go — invariantes de Musubi Renaissance F5.
//
// Los dos hechos que esta fase ataca, medidos el 2026-08-29 contra el central:
//   · cinco maneras de pedir lo mismo daban un solape Jaccard de 0,09, con tres pedidos en 0,00 —
//     dos formas de pedir lo mismo sin un solo patrón en común;
//   · con 256 bytes de contexto extra se perdían dos tercios del corpus, y con 10 KB el solape con el
//     pedido limpio caía a cero. El motor castigaba la especificidad.
//
// ⚠ Estos tests prueban el MECANISMO, no la magnitud. Cuánto sube la estabilidad de paráfrasis
// depende del embebedor y del acervo reales y sale de la sonda; afirmarlo desde acá sería medir al
// embebedor de prueba.

import (
	"strings"
	"testing"
)

// I-REP1 · el ruido no cambia el material. El embebedor de prueba mira el ANCLA que encuentra en el
// texto; el relleno que va después del tope no llega a viajar, así que la consulta con ruido y la
// limpia tienen que traer exactamente lo mismo.
func TestDesignElRuidoNoCambiaElMaterial(t *testing.T) {
	s := acervoDirigido(t, []entradaDirigida{
		{"design-corpus/uno", "primero", 0.95},
		{"design-corpus/dos", "segundo", 0.90},
		{"design-corpus/tres", "tercero", 0.85},
	})

	pedido := "CONSULTA: una tabla densa de inventario con filtros."
	ruido := strings.Repeat("Contexto adicional del proyecto que no describe el pedido. ", 60)

	limpio := callDesign(t, s, nil, pedido, "web")
	conRuido := callDesign(t, s, nil, pedido+" "+ruido, "web")

	if a, b := corpusIDs(limpio.Corpus), corpusIDs(conRuido.Corpus); !mismosIDs(a, b) {
		t.Errorf("el ruido cambió el material servido: limpio=%v ruido=%v", a, b)
	}

	// LO QUE DE VERDAD IMPORTA: el ruido no llega al buscador. Un tope de CARACTERES no alcanzaba —
	// con un pedido de 50 chars y 600 de tope entran 550 de relleno y el vector sigue arrastrado. Por
	// eso se corta por ORACIONES: el pedido es la cabeza, el contexto viene después.
	usada, rec := normalizarConsulta(pedido + " " + ruido)
	if rec == nil {
		t.Fatal("la consulta se acortó y no se declaró (I-REP3)")
	}
	if !strings.Contains(usada, "tabla densa de inventario") {
		t.Errorf("el pedido tiene que sobrevivir a la normalización; quedó %q", usada)
	}
	if strings.Contains(usada, "Contexto adicional del proyecto que no describe el pedido. Contexto") {
		t.Errorf("el relleno repetido no debería viajar al buscador; quedó %q", usada)
	}
	if rec.CharsUsados >= rec.CharsOriginales {
		t.Errorf("declaración incoherente: usados %d de %d", rec.CharsUsados, rec.CharsOriginales)
	}
	if rec.CharsUsados > designConsultaMax {
		t.Errorf("la consulta usada (%d) supera el tope de caracteres (%d)", rec.CharsUsados, designConsultaMax)
	}

	// SABOTAJE: con el corte por oraciones apagado (n=0) el relleno vuelve a entrar entero. Se compara
	// contra la función real para que el invariante no dependa de recordar tocar una constante.
	if sin := primerasOraciones(pedido+" "+ruido, 0); !strings.Contains(sin, "Contexto adicional del proyecto que no describe el pedido. Contexto") {
		t.Error("el sabotaje no sabotea: sin corte por oraciones el relleno debería entrar")
	}

	// Y el eco tampoco lleva el pedido crudo: quien llamó ya lo tiene, y devolverlo entero gastaba
	// presupuesto que le sacaba lugares al corpus. Fue justo lo que este test encontró.
	if len([]rune(conRuido.Ask)) > designConsultaMax {
		t.Errorf("el eco en `ask` debería venir acotado; mide %d", len([]rune(conRuido.Ask)))
	}
}

// I-REP4 · una consulta normal no se toca. La normalización junta espacios y nada más; el campo de
// recorte ni aparece.
func TestDesignUnaConsultaNormalNoSeToca(t *testing.T) {
	consulta, rec := normalizarConsulta("  tabla   densa\n\nde inventario con filtros  ")
	if rec != nil {
		t.Errorf("una consulta corta no debería declarar recorte; got %+v", rec)
	}
	if consulta != "tabla densa de inventario con filtros" {
		t.Errorf("la normalización debería juntar espacios y nada más; got %q", consulta)
	}

	s := acervoDirigido(t, []entradaDirigida{{"design-corpus/uno", "primero", 0.95}})
	if b := callDesign(t, s, nil, "CONSULTA", "web"); b.QueryNormalized != nil {
		t.Errorf("sin recorte el campo no debería aparecer; got %+v", b.QueryNormalized)
	}
}

// I-REP2 · un empate se resuelve igual siempre. Entre dos candidatos separados por menos que el ruido
// de una paráfrasis el motor NO SABE cuál es mejor; cuando no se sabe, contestar siempre lo mismo es
// estrictamente mejor que contestar cualquier cosa.
func TestDesignUnEmpateSeResuelveIgualSiempre(t *testing.T) {
	// Dos candidatos separados por MENOS que la resolución, llegando en un orden y en el otro.
	casoA := []searchSource{
		{id: "zzz", topicKey: "design-corpus/z", content: "uno", sim: 0.9000},
		{id: "aaa", topicKey: "design-corpus/a", content: "dos", sim: 0.9002},
	}
	casoB := []searchSource{
		{id: "aaa", topicKey: "design-corpus/a", content: "dos", sim: 0.9002},
		{id: "zzz", topicKey: "design-corpus/z", content: "uno", sim: 0.9000},
	}
	estabilizarOrden(casoA)
	estabilizarOrden(casoB)
	if casoA[0].id != casoB[0].id {
		t.Errorf("el orden de llegada cambió el resultado: %q vs %q", casoA[0].id, casoB[0].id)
	}
	if casoA[0].id != "aaa" {
		t.Errorf("el empate debería romperse por id de forma estable; ganó %q", casoA[0].id)
	}

	// SABOTAJE: sin cuantizar, 0,9002 le gana a 0,9000 y el orden de llegada SÍ importa — que es el
	// comportamiento que producía el 0,09 de estabilidad.
	if cuantizarSim(0.9000) != cuantizarSim(0.9002) {
		t.Error("0,9000 y 0,9002 difieren por menos que la resolución: deberían empatar")
	}

	// Y una diferencia REAL sigue mandando: la cuantización no aplana el ranking entero.
	real := []searchSource{
		{id: "aaa", topicKey: "design-corpus/a", content: "peor", sim: 0.60},
		{id: "zzz", topicKey: "design-corpus/z", content: "mejor", sim: 0.90},
	}
	estabilizarOrden(real)
	if real[0].id != "zzz" {
		t.Errorf("una diferencia real de similitud tiene que ganarle al desempate por id; ganó %q", real[0].id)
	}
}

// Y el motor entero sigue siendo determinista de punta a punta: dos llamadas idénticas, un brief.
func TestDesignDosLlamadasIdenticasUnBrief(t *testing.T) {
	s := acervoDirigido(t, []entradaDirigida{
		{"design-corpus/uno", "primero", 0.9000},
		{"design-corpus/dos", "segundo", 0.9002},
		{"design-corpus/tres", "tercero", 0.9001},
	})
	primero := corpusIDs(callDesign(t, s, nil, "CONSULTA", "web").Corpus)
	for i := 0; i < 4; i++ {
		if otro := corpusIDs(callDesign(t, s, nil, "CONSULTA", "web").Corpus); !mismosIDs(primero, otro) {
			t.Fatalf("corrida %d devolvió otro corpus:\n  %v\n  %v", i, primero, otro)
		}
	}
}

func mismosIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
