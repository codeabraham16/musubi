package mcp

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// EL TOPE POR CONTEO DEL OUTBOX NO EXISTE, Y ESTO ES LO QUE LO SOSTIENE DESDE EL LADO DEL CÓDIGO.
//
// `sync-hardening` sacó el corte por `max_attempts` con todas las letras (R3: «el parámetro
// max_attempts NO DEBE causar que un fallo transitorio termine en dead»), porque un central caído
// por horas no puede costar memoria 'shared'. Desde entonces una fila del outbox muere SÓLO por un
// fallo permanente, y eso lo decide el código de error, nunca un contador.
//
// QUÉ SALIÓ MAL Y POR QUÉ HACE FALTA ESTO. La regla quedó escrita en CUATRO lugares y sólo uno era
// cierto. El 2026-09-11 seguían en main, todos afirmando un tope que ya no existía:
//
//   - `scheduler.go`, en la doc de `drainOutboxOnce`: «un fallo transitorio va a dead cuando se
//     alcanzó max_attempts» — o sea que la documentación de la función CONTRADECÍA al comentario en
//     línea veinticinco líneas más abajo, adentro de la misma función;
//   - `syncclient.go`: «Reintentar con backoff; el outbox corta a max_attempts»;
//   - `outbox.go`: «lo usa el drain para decidir el backoff y el corte a dead-letter
//     (attempts+1 >= max_attempts)».
//
// Un commit anterior dice «corregida la cita falsa que quedaba en syncclient.go» y corrigió UNA
// —la del comentario grande de `permanentRPCCodes`—, dejando vivas a las tres hermanas. Es la forma
// dominante de este repo: la lección aprendida de un lado y no del hermano.
//
// Y EL COSTO NO ES ESTÉTICO. Está medido: 605 intentos en 74 h contra un rechazo determinista en
// kernelos-pc. Alguien que lee «corta a max_attempts» y ve una fila reintentando para siempre
// diagnostica un bug donde hay una decisión, o peor, «arregla» la decisión volviendo a cablear el
// tope y vuelve a perder memoria en el próximo corte de red.
//
// POR QUÉ ESTA GUARDA MIRA EL CÓDIGO Y NO EL TEXTO. Una guarda que buscara la frase «corta a
// max_attempts» la satisface cualquier redacción nueva, y este repo ya tuvo siete guardas en verde
// satisfechas por un comentario, un mensaje de error o el archivo vecino. Lo que decide de verdad
// es si ALGUIEN LEE el campo: mientras `Sync.MaxAttempts` no tenga un solo lector fuera de su propio
// default, el tope no puede volver por accidente. Por eso se parsea con `go/ast` y en modo 0 —SIN
// comentarios—, así que ningún comentario, incluidos los que este mismo commit reescribió, puede
// satisfacer esta guarda ni dispararla.
//
// QUÉ PASA SI ALGUIEN LO RECABLEA A PROPÓSITO: esto se pone rojo y hay que venir acá a decir por
// qué. Eso es lo buscado — la decisión de R3 tiene bitácora, y revertirla debe costar una
// conversación, no un renglón.
//
// EL CAMPO NO SE BORRA, Y ESO ES DELIBERADO: el YAML no se parsea en modo estricto, así que un
// `max_attempts: 5` ya escrito —el config del cerebro central lo tiene— seguiría cargando en
// silencio, sólo que sin ningún lugar donde leer que no sirve. El comentario de
// `config.SyncConfig.MaxAttempts` ES ese lugar, y el campo es lo que lo sostiene.
func TestElTopePorConteoDelOutboxNoTieneQuienLoLea(t *testing.T) {
	raiz := filepath.Join("..", "..")
	fset := token.NewFileSet()

	type uso struct {
		archivo string
		linea   int
		expr    string
	}
	var delSync, delMultiagente []uso

	for _, rel := range archivosGo(t, raiz) {
		// Modo 0: SIN comentarios. Es la propiedad que hace que esta guarda mida el cable y no la
		// prosa que lo describe.
		f, err := parser.ParseFile(fset, filepath.Join(raiz, rel), nil, 0)
		if err != nil {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil || sel.Sel.Name != "MaxAttempts" {
				return true
			}
			var b bytes.Buffer
			if printer.Fprint(&b, fset, sel.X) != nil {
				return true
			}
			u := uso{archivo: rel, linea: fset.Position(sel.Pos()).Line, expr: b.String() + ".MaxAttempts"}
			// Hay DOS MaxAttempts en la config y sólo uno está muerto: el del sync. El de
			// MultiAgent gobierna la cola de unidades de trabajo y sí decide dead-letter.
			if strings.Contains(strings.ToLower(b.String()), "sync") {
				delSync = append(delSync, u)
			} else {
				delMultiagente = append(delMultiagente, u)
			}
			return true
		})
	}

	// CONTROL 1 — «no pude medir» no puede salir por la puerta de «medí y está bien». Si el árbol se
	// mueve, o el walk deja de encontrar los `.go`, un cero acá sería un verde que no midió nada.
	if len(delSync)+len(delMultiagente) == 0 {
		t.Fatalf("no encontré NI UNA referencia a `MaxAttempts` en todo el repo desde %s. "+
			"Eso no es «el tope no tiene lectores»: es que este barrido no miró nada", raiz)
	}

	// CONTROL 2 — el barrido tiene que llegar hasta el campo que custodia. Sin esto, un walk que se
	// saltee `internal/config` daría verde justamente por no haber mirado donde el campo vive.
	llegoAlCampo := false
	for _, u := range delSync {
		if strings.HasPrefix(u.archivo, "internal/config/") {
			llegoAlCampo = true
		}
	}
	if !llegoAlCampo {
		t.Fatalf("el barrido no vio ninguna referencia a `Sync.MaxAttempts` en internal/config/. "+
			"El campo tiene que seguir existiendo y auto-defaulteándose ahí (ver el comentario de "+
			"config.SyncConfig.MaxAttempts): si no lo encontré, no medí el árbol que creo que medí. "+
			"Vistos del lado sync: %d, del lado multiagente: %d", len(delSync), len(delMultiagente))
	}

	// CONTROL 3 — CONTRA UNA GUARDA QUE ACUSE A TODO. Si el clasificador mandara cualquier
	// `MaxAttempts` al lado «sync», el test pasaría a ser una prohibición general y se apagaría al
	// primer uso legítimo. El de MultiAgent es ese uso legítimo y tiene que sobrevivir.
	if len(delMultiagente) == 0 {
		t.Fatalf("no vi NINGÚN uso de `MaxAttempts` del lado de MultiAgent, y tiene que haberlo " +
			"(la cola de unidades de trabajo sí corta por conteo). O el clasificador está mandando " +
			"todo al lado sync —y entonces esta guarda prohíbe de más—, o desapareció el uso vivo")
	}

	// LO QUE DECIDE.
	var fuera []uso
	for _, u := range delSync {
		if strings.HasPrefix(u.archivo, "internal/config/") {
			continue
		}
		fuera = append(fuera, u)
	}
	if len(fuera) > 0 {
		var b strings.Builder
		for _, u := range fuera {
			b.WriteString("\n    " + u.archivo + ":" + strconv.Itoa(u.linea) + " — " + u.expr)
		}
		t.Fatalf("alguien volvió a CABLEAR el tope por conteo del outbox: hay %d lector(es) de "+
			"`Sync.MaxAttempts` fuera de internal/config/.%s\n\n"+
			"Ese tope se sacó a propósito (`sync-hardening`, R3: «el parámetro max_attempts NO DEBE "+
			"causar que un fallo transitorio termine en dead»), porque un central caído por horas no "+
			"puede costar memoria 'shared'. Una fila muere SÓLO por un fallo permanente, y eso lo "+
			"decide el código de error.\n"+
			"Si de verdad querés reponer el tope, es una decisión con bitácora: escribila en el "+
			"comentario de config.SyncConfig.MaxAttempts y actualizá esta guarda ahí mismo. Lo que no "+
			"puede pasar es que vuelva en un renglón y nadie se entere.", len(fuera), b.String())
	}
}
