package motor

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// Prueba de punta a punta contra el binario `claude` REAL. Está detrás de una variable de
// entorno porque consume la cuota del usuario y necesita una sesión autenticada: en CI y en
// una corrida normal de `go test ./...` se saltea.
//
//	MOTOR_E2E=1 go test ./internal/motor -run E2E -v
//
// Vale tenerla igual: todo el resto del paquete se prueba contra un proceso falso, así que
// sin esto nada verifica que las banderas del argv sigan existiendo en el binario, ni que
// el formato del stream no haya cambiado. Cuando el CLI cambie, este test es el que avisa.
func TestE2EUnTurnoRealSeSirveYCorta(t *testing.T) {
	if os.Getenv("MOTOR_E2E") == "" {
		t.Skip("necesita MOTOR_E2E=1 y el binario claude autenticado (consume cuota)")
	}

	l := &Lanzador{Espera: 2 * time.Minute}
	p := Pedido{
		SystemPrompt: "Respondé con una sola palabra, sin explicaciones y sin usar tools.",
		Mensajes: []Mensaje{
			TextoDeUsuario("¿Cuál es la capital de Francia?"),
		},
	}

	var evs []Evento
	var texto strings.Builder
	arranque := time.Now()
	err := l.Turno(context.Background(), p, func(e Evento) error {
		evs = append(evs, e)
		if e.Type == EvTextDelta {
			texto.WriteString(e.Delta)
		}
		return nil
	})
	tardo := time.Since(arranque)
	if err != nil {
		t.Fatalf("Turno: %v", err)
	}

	t.Logf("tardó %s · %d eventos · forma: %v", tardo.Round(time.Millisecond), len(evs), forma(evs))
	t.Logf("texto: %q", texto.String())

	// 1) El turno abrió y cerró, una sola vez.
	if len(evs) == 0 || evs[0].Type != EvStart {
		t.Fatalf("el primer evento debe ser start: %v", tipos(evs))
	}
	fin := evs[len(evs)-1]
	if fin.Type != EvDone {
		t.Fatalf("el último evento debe ser done, got %s (%s)", fin.Type, fin.ErrorMessage)
	}
	var starts int
	for _, e := range evs {
		if e.Type == EvStart {
			starts++
		}
	}
	if starts != 1 {
		t.Errorf("start: got %d, want 1 — el corte no funcionó contra el binario real", starts)
	}

	// 2) Llegó texto en DELTAS, que es lo único que el cliente sabe consumir.
	if texto.Len() == 0 {
		t.Error("no llegó un solo text_delta: ¿se perdió --include-partial-messages?")
	}
	if !strings.Contains(strings.ToLower(texto.String()), "par") {
		t.Errorf("la respuesta no se parece a la esperada: %q", texto.String())
	}

	// 3) El usage es real, no ceros. Es lo que el cliente muestra como consumo.
	if fin.Usage == nil || fin.Usage.Input == 0 || fin.Usage.Output == 0 {
		t.Errorf("usage vacío: %+v", fin.Usage)
	}
	if fin.Usage != nil && fin.Usage.TotalTokens == 0 {
		t.Error("totalTokens en cero: el stream no lo trae y hay que calcularlo")
	}
}

// 🔴 EL E2E QUE NO ES OPCIONAL. El formato del archivo de sesión que escribe
// archivoDeSesion es INTERNO del CLI, no una interfaz publicada: ningún proceso falso lo
// valida, y una actualización del binario puede cambiarlo sin aviso. El síntoma NO sería un
// error sino un turno que PIERDE la historia — exactamente el defecto que la partición vino
// a arreglar, y que pasó siete semanas sin que nadie lo notara porque el test con el proceso
// falso daba verde.
//
// La prueba es un dato que el modelo sólo puede saber si vio el prefijo. Si la partición
// falla, no contesta mal: contesta que no sabe.
//
//	MOTOR_E2E=1 go test ./internal/motor -run E2ELaHistoria -v
func TestE2ELaHistoriaLlegaCompletaAlModelo(t *testing.T) {
	if os.Getenv("MOTOR_E2E") == "" {
		t.Skip("necesita MOTOR_E2E=1 y el binario claude autenticado (consume cuota)")
	}

	l := &Lanzador{Espera: 2 * time.Minute}
	p := Pedido{
		Modelo: "sonnet",
		// El id RESUELTO, que es lo que va escrito en cada turno del asistente del
		// archivo de sesión. Con el alias ("sonnet") el CLI descarta los bloques de
		// thinking firmados en silencio; acá no hay thinking, pero el campo se llena bien
		// igual porque es el camino que usa producción.
		ModeloResuelto: "claude-sonnet-5",
		SystemPrompt:   "Respondé con una sola palabra o número, sin explicaciones y sin usar tools.",
		Mensajes: []Mensaje{
			// Esto va por el ARCHIVO DE SESIÓN (prefijo)...
			TextoDeUsuario("Guardá este dato: mi número de inventario es 7391. No lo olvides."),
			TextoDeAsistente("Anotado: tu número de inventario es 7391."),
			TextoDeUsuario("Perfecto, gracias."),
			// ...y esto por STDIN (cola).
			TextoDeAsistente("De nada. ¿Algo más?"),
			TextoDeUsuario("¿Cuál era mi número de inventario?"),
		},
	}

	var texto strings.Builder
	var starts int
	err := l.Turno(context.Background(), p, func(e Evento) error {
		switch e.Type {
		case EvTextDelta:
			texto.WriteString(e.Delta)
		case EvStart:
			starts++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Turno: %v", err)
	}
	t.Logf("respuesta: %q", texto.String())

	if starts != 1 {
		t.Errorf("start: got %d, want 1", starts)
	}
	// EL INVARIANTE. El 7391 está sólo en el prefijo, que viaja por el archivo de sesión:
	// si el CLI no lo reanudó, el modelo no tiene de dónde sacarlo.
	if !strings.Contains(texto.String(), "7391") {
		t.Errorf("el modelo no vio el prefijo de la historia: %q", texto.String())
	}
}

// 🔴 EL TOOL_RESULT, que es el caso que estaba ROTO. Medido el 2026-10-28, mandando todo el
// transcript por stdin el CLI reemplazaba el resultado de la tool por
// "[Tool result missing due to internal error]": o sea que en toda continuación de un agent
// loop —TODO lo que un cliente hace después del primer turno— el modelo nunca veía lo que su
// propia tool había devuelto.
//
//	MOTOR_E2E=1 go test ./internal/motor -run E2EElToolResult -v
func TestE2EElToolResultLlegaIntacto(t *testing.T) {
	if os.Getenv("MOTOR_E2E") == "" {
		t.Skip("necesita MOTOR_E2E=1 y el binario claude autenticado (consume cuota)")
	}

	l := &Lanzador{Espera: 2 * time.Minute}
	p := Pedido{
		Modelo:         "sonnet",
		ModeloResuelto: "claude-sonnet-5",
		SystemPrompt:   "Respondé con una sola palabra, sin explicaciones y sin volver a usar tools.",
		Mensajes: []Mensaje{
			TextoDeUsuario("Leé el archivo inventario.txt y decime qué ciudad dice."),
			{Rol: "assistant", Contenido: []json.RawMessage{
				json.RawMessage(`{"type":"text","text":"Voy a leerlo."}`),
				json.RawMessage(`{"type":"tool_use","id":"toolu_01ZARAGOZA","name":"Read","input":{"file_path":"inventario.txt"}}`),
			}},
			{Rol: "user", Contenido: []json.RawMessage{
				json.RawMessage(`{"type":"tool_result","tool_use_id":"toolu_01ZARAGOZA","content":"ciudad: ZARAGOZA"}`),
			}},
		},
	}

	var texto strings.Builder
	err := l.Turno(context.Background(), p, func(e Evento) error {
		if e.Type == EvTextDelta {
			texto.WriteString(e.Delta)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Turno: %v", err)
	}
	t.Logf("respuesta: %q", texto.String())

	// El dato está SÓLO en el tool_result. Si el CLI lo sustituyó por su placeholder de
	// error, el modelo va a decir que no pudo leer el archivo.
	if !strings.Contains(strings.ToUpper(texto.String()), "ZARAGOZA") {
		t.Errorf("el tool_result no llegó al modelo: %q", texto.String())
	}
}
