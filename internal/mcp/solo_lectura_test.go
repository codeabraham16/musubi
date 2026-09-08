package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"

	_ "modernc.org/sqlite"
)

// servidorSoloLectura arma un servidor sobre una base REAL abierta en el escalón: se migra, se le
// adelanta el user_version y se reabre sin migrar. No hay engine falso de por medio a propósito —
// lo que se prueba incluye que el servidor le pregunte al engine de verdad quién es.
func servidorSoloLectura(t *testing.T) *McpServer {
	t.Helper()
	dir := t.TempDir()
	eng, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	eng.Close()

	db, err := sql.Open("sqlite", filepath.Join(dir, config.DirName, config.DBFile))
	if err != nil {
		t.Fatalf("abrir la base cruda: %v", err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 999`); err != nil {
		t.Fatalf("adelantar user_version: %v", err)
	}
	db.Close()

	ro, err := memory.NewDbEngineSoloLectura(dir)
	if err != nil {
		t.Fatalf("NewDbEngineSoloLectura: %v", err)
	}
	t.Cleanup(func() { ro.Close() })
	return NewMcpServer(ro, dir, embedding.NoopProvider{},
		WithVersion("0.0.0-solo-lectura-de-prueba"),
		WithSoloLectura("la base está en el esquema v999 y este binario llega a v48"))
}

// R5 — EL HANDSHAKE DECLARA EL ESCALÓN Y QUÉ SE PUEDE LLAMAR.
//
// Que lo diga en el handshake es lo que evita que el cliente descubra el modo a fuerza de errores.
// La lista sale del MISMO predicado que decide el despacho, así que no hay una segunda lista que
// se pueda desincronizar.
func TestR5ElHandshakeDeclaraElEscalonYSusTools(t *testing.T) {
	s := servidorSoloLectura(t)

	crudo, err := json.Marshal(s.handleInitialize())
	if err != nil {
		t.Fatalf("serializar el handshake: %v", err)
	}
	var sobre struct {
		Meta struct {
			ReadOnly *struct {
				Reason      string   `json:"reason"`
				ToolsUsable []string `json:"tools_usable"`
			} `json:"musubi/readonly"`
		} `json:"_meta"`
	}
	if err := json.Unmarshal(crudo, &sobre); err != nil {
		t.Fatalf("parsear: %v", err)
	}
	if sobre.Meta.ReadOnly == nil {
		t.Fatal("el handshake no declaró el escalón de sólo lectura")
	}
	if !strings.Contains(sobre.Meta.ReadOnly.Reason, "v999") {
		t.Errorf("el motivo no dice qué pasó: %q", sobre.Meta.ReadOnly.Reason)
	}
	if len(sobre.Meta.ReadOnly.ToolsUsable) == 0 {
		t.Fatal("declaró el escalón sin ninguna tool servible: eso es el modo degradado con otro nombre")
	}
	// Y la lista tiene que incluir la tool de lectura principal, que NO es readOnly: si no
	// estuviera, el escalón sería teatro.
	tiene := func(n string) bool {
		for _, x := range sobre.Meta.ReadOnly.ToolsUsable {
			if x == n {
				return true
			}
		}
		return false
	}
	if !tiene("musubi_recall") {
		t.Error("musubi_recall no figura entre las tools servibles del escalón")
	}
	if tiene("musubi_save_observation") {
		t.Error("musubi_save_observation figura como servible y escribe")
	}
}

// R6 — LA TOOL QUE ESCRIBE SE RECHAZA CON SU PROPIO CÓDIGO; LA QUE LEE PASA.
//
// Las dos mitades importan y son distintas. Rechazar todo sería el modo degradado; dejar pasar
// todo dejaría que la escritura muera dentro de SQLite, a mitad de un handler y con un mensaje que
// el agente no puede accionar.
func TestR6ElEscalonRechazaLoQueEscribeYDejaPasarLoQueLee(t *testing.T) {
	s := servidorSoloLectura(t)

	llamar := func(nombre string, args map[string]interface{}) JsonRpcResponse {
		t.Helper()
		params, _ := json.Marshal(map[string]interface{}{"name": nombre, "arguments": args})
		resp, ok := s.Dispatch(context.Background(), JsonRpcRequest{JsonRpc: "2.0", ID: 1, Method: "tools/call", Params: params})
		if !ok {
			t.Fatalf("%s: no hubo respuesta", nombre)
		}
		return resp
	}

	esc := llamar("musubi_save_observation", map[string]interface{}{"topic_key": "t/x", "content": "no debería entrar"})
	if esc.Error == nil {
		t.Fatal("una tool que escribe pasó el escalón")
	}
	if esc.Error.Code != codeReadOnly {
		t.Errorf("código %d, esperaba codeReadOnly (%d): %q", esc.Error.Code, codeReadOnly, esc.Error.Message)
	}
	if !strings.Contains(esc.Error.Message, "v999") {
		t.Errorf("el error no explica por qué el servidor está así: %q", esc.Error.Message)
	}

	// Y la lectura pasa. No se afirma sobre el CONTENIDO —la base está vacía— sino sobre que no
	// la frenó el escalón, que es lo que este invariante vigila.
	lec := llamar("musubi_recall", map[string]interface{}{"query": "lo que sea"})
	if lec.Error != nil && lec.Error.Code == codeReadOnly {
		t.Errorf("el escalón frenó una tool de lectura: %q", lec.Error.Message)
	}
}

// R7 — UN SERVIDOR NORMAL NO DECLARA EL ESCALÓN NI RECHAZA ESCRITURAS POR ÉL.
//
// Sin esto, «declarar sólo lectura siempre» pasaría R5 y R6 y dejaría al producto sin memoria.
func TestR7ElServidorNormalNoDeclaraElEscalon(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	crudo, err := json.Marshal(s.handleInitialize())
	if err != nil {
		t.Fatalf("serializar: %v", err)
	}
	if strings.Contains(string(crudo), "musubi/readonly") {
		t.Errorf("un servidor normal declaró el escalón de sólo lectura:\n  %s", crudo)
	}

	params, _ := json.Marshal(map[string]interface{}{
		"name":      "musubi_save_observation",
		"arguments": map[string]interface{}{"topic_key": "t/r7", "content": "una nota cualquiera que sí tiene que entrar"},
	})
	resp, ok := s.Dispatch(context.Background(), JsonRpcRequest{JsonRpc: "2.0", ID: 1, Method: "tools/call", Params: params})
	if !ok {
		t.Fatal("no hubo respuesta")
	}
	if resp.Error != nil && resp.Error.Code == codeReadOnly {
		t.Errorf("un servidor normal rechazó una escritura por el escalón: %q", resp.Error.Message)
	}
}
