package mcp

// EL CENTRAL DICE DE QUIÉN ES LA CULPA (C1–C4).
//
// LO QUE SE PRUEBA, Y POR QUÉ IMPORTA MÁS DE LO QUE PARECE. Un rechazo de guarda que sale del
// borde MCP como -32603 «error interno» le está diciendo al que llama «me rompí yo, insistí». El
// outbox de un nodo remoto le cree y —a propósito, porque un central caído no puede costar
// memoria— insiste sin tope por conteo. Medido el 2026-09-08 en kernelos-pc: 605 intentos en 74 h
// contra una observación que ninguna espera podía volver aceptable.
//
// Los cuatro tests recorren el lazo completo: el borde elige el código (C1), no degrada lo que no
// entiende (C2), ese código es de los que el cliente trata como definitivos (C3), y la fila
// termina MUERTA en vez de inmortal (C4).

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// contenidoConSobre es la forma medida más común de las 73 del central: el texto se cortó en el
// cierre de su propia llamada y arrastró el sobre.
const contenidoConSobre = "El CI del cuerpo nunca corre en Forgejo: el workflow existe pero el runner no.\n</content>\n</invoke>\n"

// C1 — EL BORDE TRADUCE EL RECHAZO DE LA GUARDA A UN CÓDIGO PERMANENTE.
//
// Se prueban los DOS caminos del save (con id explícito y sin él) porque son dos returns distintos
// de la misma función: arreglar uno solo deja el otro mintiendo, y el que usa el sync es el de id.
func TestC1ElRechazoDeLaGuardaSaleComoCulpaDelLlamador(t *testing.T) {
	casos := []struct {
		nombre string
		args   map[string]interface{}
	}{
		{"sin id (dedup)", map[string]interface{}{"topic_key": "t/c1", "content": contenidoConSobre}},
		{"con id (el camino del sync)", map[string]interface{}{"id": "obs-c1", "topic_key": "t/c1", "content": contenidoConSobre}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			s := newTestServer(t, embedding.NoopProvider{})
			_, rpcErr := call(t, s, "musubi_save_observation", c.args)
			if rpcErr == nil {
				t.Fatal("la guarda no rechazó un content con el sobre adentro")
			}
			if rpcErr.Code != codeInvalidParams {
				t.Errorf("código %d: un rechazo determinista se está anunciando como fallo del servidor (esperaba %d)", rpcErr.Code, codeInvalidParams)
			}
		})
	}
}

// C1b — LA MISMA GUARDA, POR LA PUERTA DE LA CUARENTENA.
//
// musubi_propose_observation guarda por el mismo núcleo, así que hereda la guarda; si su mapeo se
// quedara atrás, el mismo rechazo tendría dos códigos distintos según por dónde entró.
func TestC1bLaCuarentenaTambienDiceDeQuienEsLaCulpa(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	_, rpcErr := call(t, s, "musubi_propose_observation", map[string]interface{}{
		"topic_key": "t/c1b", "content": contenidoConSobre,
	})
	if rpcErr == nil {
		t.Fatal("la guarda no rechazó un content con el sobre adentro por la puerta de cuarentena")
	}
	if rpcErr.Code != codeInvalidParams {
		t.Errorf("código %d: esperaba %d", rpcErr.Code, codeInvalidParams)
	}
}

// C2 — LA REGLA DE DECISIÓN, EN SUS DOS DIRECCIONES.
//
// Se prueba errorDeGuardado directo: es una función pura y la decisión ES la función. Que además
// esté CABLEADA en los caminos reales lo prueban C1, C1b y C4, así que acá no hace falta servidor.
//
// La primera versión de este test inducía el fallo cerrando la base, y era VACUA: con la base
// cerrada el guardado se rompe antes de llegar acá, en otro return que también dice -32603, así que
// pasaba igual con el default roto. Se vio al sabotear, no al escribirlo.
//
// El segundo caso es el que importa más: mapear todo a «culpa del llamador» dejaría C1 en verde y
// haría que un cliente TIRE a dead-letter memoria buena porque al central se le llenó el disco.
func TestC2LaReglaDeDecisionEnSusDosDirecciones(t *testing.T) {
	casos := []struct {
		nombre string
		err    error
		codigo int
	}{
		{"lo que una guarda rechazó mirando el pedido", fmt.Errorf("al guardar: %w", memory.ErrPayloadInvalido), codeInvalidParams},
		{"un fallo que ninguna guarda clasificó", errors.New("disk I/O error"), codeInternalError},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := errorDeGuardado(c.err)
			if got == nil {
				t.Fatal("errorDeGuardado devolvió nil para un error real")
			}
			if got.Code != c.codigo {
				t.Errorf("código %d, esperaba %d", got.Code, c.codigo)
			}
		})
	}
}

// C3 — EL CÓDIGO QUE EMITE EL BORDE ES DE LOS QUE EL CLIENTE DA POR DEFINITIVOS.
//
// Es la junta entre las dos mitades del lazo: el borde puede elegir un código impecable y el
// cliente seguir reintentándolo para siempre si ese código no está en su lista de permanentes.
func TestC3ElCodigoElegidoEsPermanenteParaElCliente(t *testing.T) {
	if !permanentRPCCodes[codeInvalidParams] {
		t.Errorf("el borde emite %d para un rechazo determinista, pero el cliente NO lo trata como permanente: la fila se reintenta igual", codeInvalidParams)
	}
}

// C4 — LA COSA: LA FILA ENVENENADA MUERE EN VEZ DE REINTENTARSE PARA SIEMPRE.
//
// EL CENTRAL DE ESTE TEST ES UN SERVIDOR DE VERDAD, no un stub con el código cableado. El rechazo
// lo produce la guarda real y el código lo elige el borde real; un stub que devolviera -32602 a
// mano dejaría este test en verde con el mapeo roto, que es exactamente lo que existe para atrapar.
//
// La fila se envenena por SQL crudo, sin pasar por el save. No es una comodidad del test: es la
// única forma de reproducir lo que pasó. La observación entró a la base por un binario que todavía
// no tenía la guarda, y la guarda de hoy ya no puede impedir que ESA fila exista — sólo puede
// decidir qué le pasa cuando se la ofrece al central.
func TestC4LaFilaEnvenenadaMuereEnVezDeReintentarseParaSiempre(t *testing.T) {
	central := newTestServer(t, embedding.NoopProvider{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JsonRpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "request ilegible", http.StatusBadRequest)
			return
		}
		resp, _ := central.Dispatch(r.Context(), req)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	dir := memtest.DirSembrado(t)
	engine, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	cliente := NewMcpServer(engine, dir, embedding.NoopProvider{})

	t.Setenv("MUSUBI_TEST_TOKEN", "tok")
	cfg := config.SyncConfig{
		BatchSize: 50, LeaseSeconds: 60, BackoffBaseSeconds: 5, BackoffMaxSeconds: 300,
		RequestTimeoutSeconds: 5, CentralURL: ts.URL,
		AuthTokenEnv: "MUSUBI_TEST_TOKEN", AllowInsecureToken: true,
	}
	sc, err := NewSyncClient(cfg)
	if err != nil {
		t.Fatalf("NewSyncClient: %v", err)
	}
	cliente.SetSyncClient(sc, cfg)

	// Guardar sano (que es lo que encola en el outbox) y recién después envenenar el contenido: el
	// save de hoy rechazaría el texto envenenado, y por eso la fila tiene que nacer sana.
	const id = "obs-c4"
	if err := engine.SaveObservationTyped(id, "t/c4", "una observación sana que después se envenena", 1.0, "semantic", memory.ScopeShared, nil); err != nil {
		t.Fatalf("SaveObservationTyped: %v", err)
	}
	envenenar(t, dir, id, contenidoConSobre)

	if p, _, _ := statsOf(t, cliente); p != 1 {
		t.Fatalf("precondición: esperaba 1 pending, hay %d", p)
	}

	cliente.drainOutboxOnce(context.Background())

	p, sent, dead := statsOf(t, cliente)
	if dead != 1 || p != 0 {
		t.Errorf("tras el drain esperaba dead=1 pending=0, obtuve pending=%d sent=%d dead=%d — una fila que el central nunca va a aceptar quedó viva, y se va a reintentar para siempre", p, sent, dead)
	}
}

// envenenar reescribe el content de una observación por fuera del engine. Abre su propia conexión
// a la MISMA base: el engine sigue abierto y WAL admite el segundo escritor.
func envenenar(t *testing.T, dir, id, contenido string) {
	t.Helper()
	ruta := filepath.Join(dir, config.DirName, config.DBFile)
	db, err := sql.Open("sqlite", ruta+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("abrir la base cruda: %v", err)
	}
	defer db.Close()
	res, err := db.Exec(`UPDATE observations SET content = ? WHERE id = ?`, contenido, id)
	if err != nil {
		t.Fatalf("envenenar el contenido: %v", err)
	}
	// Un UPDATE que no matchea nada es un éxito de SQL: sin este chequeo el test seguiría en verde
	// midiendo una fila sana.
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		t.Fatalf("el envenenamiento no tocó la fila esperada (filas=%d, err=%v)", n, err)
	}
}
