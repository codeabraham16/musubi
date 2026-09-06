package main

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

// EL CONTRATO DEL ENVELOPE: qué eventos pueden llevar additionalContext y cuáles no.
//
// Estos tests existen por un fracaso concreto. El hook PreCompact estuvo tres semanas emitiendo
// un envelope que Claude Code descartaba entero —"PreCompact" no está en el enum del validador—
// y su test seguía en VERDE, porque afirmaba que NUESTRO json dijera "PreCompact" en vez de
// afirmar que el otro lado lo aceptara.
//
// La lección no es "revisar mejor". Es que la aserción tiene que ser la señal que SÓLO existe si
// la cosa pasó. Acá esa señal es la lista blanca: lo que no está en ella no se emite.

func TestElEnvelopeNoSaleParaUnEventoQueNoLlevaContexto(t *testing.T) {
	casos := []string{
		"PreCompact",      // el que falló de verdad
		"EventoInventado", // uno cualquiera que nadie definió
		"",                // el nombre vacío, que es lo que sale de una variable sin setear
	}
	for _, ev := range casos {
		if out := assembleHookContext(ev, "un bloque con texto de verdad"); out != "" {
			t.Errorf("el evento %q no admite additionalContext: no se debe emitir nada, obtuve %q", ev, out)
		}
	}
}

func TestElEnvelopeSaleParaLosEventosQueSiLlegan(t *testing.T) {
	for _, ev := range []string{"SessionStart", "UserPromptSubmit", "PreToolUse"} {
		out := assembleHookContext(ev, "un bloque con texto de verdad")
		if out == "" {
			t.Fatalf("%s SÍ admite additionalContext: el envelope tiene que salir", ev)
		}
		var env struct {
			H struct {
				Event string `json:"hookEventName"`
				Ctx   string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			t.Fatalf("%s: el envelope no es json válido: %v", ev, err)
		}
		if env.H.Event != ev {
			t.Errorf("%s: hookEventName quedó en %q", ev, env.H.Event)
		}
		if env.H.Ctx == "" {
			t.Errorf("%s: additionalContext vacío", ev)
		}
	}
}

// PreCompact fuera de la lista, y explícito. Si alguien lo agrega para "arreglar" el hook viejo,
// que se tope con este test y con el porqué antes que con una compactación muda.
func TestPreCompactSigueFueraDeLaLista(t *testing.T) {
	if eventosQueLlevanContexto["PreCompact"] {
		t.Fatal("PreCompact volvió a la lista blanca. No lo agregues: Claude Code descarta el envelope " +
			"entero porque el evento no está en su enum, y el hook queda mudo sin que nada avise. " +
			"El aviso vive ahora en buildDurableNudge (turn.go), por el hook UserPromptSubmit.")
	}
}

// ESTE ES EL TEST QUE HABRÍA CACHADO EL BUG ORIGINAL, y la razón por la que el archivo existe.
//
// No mira un caso: barre el paquete y saca TODOS los eventos para los que se arma un envelope.
// Un hook nuevo apuntado a un evento que no lleva contexto se cae acá, aunque su propio test
// pase. Es la diferencia entre verificar un caso y verificar el invariante.
func TestTodoEventoQueSeEmiteEnElPaqueteEstaEnLaLista(t *testing.T) {
	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("no pude leer el paquete: %v", err)
	}
	// Dos formas de nombrar el evento: la directa y la que pasa por el contabilizador.
	res := []*regexp.Regexp{
		regexp.MustCompile(`assembleHookContext\(\s*"([^"]+)"`),
		regexp.MustCompile(`assembleAccounted\(\s*[A-Za-z_][\w.]*\s*,\s*"([^"]+)"`),
	}

	encontrados := map[string][]string{} // evento -> archivos donde aparece
	for _, e := range entradas {
		n := e.Name()
		// Los _test.go quedan afuera a propósito: este mismo archivo emite "PreCompact"
		// justo para probar que NO sale.
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		src, err := os.ReadFile(n)
		if err != nil {
			t.Fatalf("no pude leer %s: %v", n, err)
		}
		for _, re := range res {
			for _, m := range re.FindAllStringSubmatch(string(src), -1) {
				encontrados[m[1]] = append(encontrados[m[1]], n)
			}
		}
	}

	// SIN ESTO EL TEST ES VACUAMENTE VERDE. Si un refactor renombra las funciones, los regex
	// dejan de matchear, el mapa queda vacío y el bucle de abajo no corre ni una vez: pasaría
	// en verde sin haber verificado nada. Es justo el modo de falla que este archivo existe
	// para evitar.
	//
	// El ancla son eventos CONCRETOS y no un conteo: un piso numérico se sostiene solo con que
	// UNO de los dos regex siga andando, y el otro puede haberse muerto sin que nadie note que
	// dejó de cubrir su mitad. (Medido: con el piso por conteo, romper un regex no ponía el
	// test en rojo.)
	for _, obligatorio := range []string{"SessionStart", "UserPromptSubmit"} {
		if _, ok := encontrados[obligatorio]; !ok {
			t.Fatalf("el barrido no encontró el evento %q, que el paquete SÍ emite. "+
				"Los regex dejaron de matchear: arreglalos o este test no verifica nada. "+
				"Encontrado: %v", obligatorio, encontrados)
		}
	}

	for ev, archivos := range encontrados {
		if !eventosQueLlevanContexto[ev] {
			t.Errorf("se arma un envelope para el evento %q (en %v) pero ese evento NO lleva "+
				"additionalContext: Claude Code lo descartaría entero, en silencio.",
				ev, archivos)
		}
	}
}

// EL LEDGER NO COBRA LO QUE NO ENTRÓ.
//
// assembleAccounted imputaba dentro del bucle, o sea antes de saber si el envelope iba a salir. Con
// la lista blanca eso se volvió agudo: la guarda devuelve "" DESPUÉS de que el ledger ya cobró. El
// medidor reportaba de más, que es la peor dirección posible: hace ajustar presupuestos contra humo.
func TestElLedgerNoCobraUnEnvelopeQueNoSale(t *testing.T) {
	store := newFakeTurnStore()
	out := assembleAccounted(store, "PreCompact", "s1", []accountedBlock{
		{surface: "una_superficie", text: "un bloque con texto de verdad"},
	})
	if out != "" {
		t.Fatalf("PreCompact no lleva contexto: no debe salir envelope, obtuve %q", out)
	}
	if len(store.ledger) != 0 {
		t.Errorf("se imputaron tokens de un bloque que nunca entró al contexto: %v", store.ledger)
	}
}

// La otra mitad del invariante: cuando el envelope SÍ sale, el cobro tiene que ocurrir. Sin esto,
// "no cobrar nunca" pasaría el test de arriba y el ledger quedaría en cero para siempre.
func TestElLedgerSiCobraUnEnvelopeQueSale(t *testing.T) {
	store := newFakeTurnStore()
	out := assembleAccounted(store, "UserPromptSubmit", "s1", []accountedBlock{
		{surface: "una_superficie", text: "un bloque con texto de verdad"},
	})
	if out == "" {
		t.Fatal("UserPromptSubmit sí lleva contexto: el envelope tiene que salir")
	}
	if store.ledger["una_superficie"] <= 0 {
		t.Errorf("el bloque entró al contexto y no se imputó: %v", store.ledger)
	}
}
