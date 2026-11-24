package mcp

import (
	"strings"
	"testing"

	"musubi/internal/embedding"
)

// LA CABINA VE EL TABLERO Y NO LO TOCA. musubi_work no es readOnly (plan, claim, complete mutan la
// pizarra), así que la autorización por tool entera dejaba a un reader read=all —la cabina, p. ej.
// la columna de musubi-shell— sin poder siquiera MIRAR el tablero. accionesDeLectura abre sólo
// status/bids/savings, y sólo a read=all: la pizarra no está acotada por proyecto.
//
// Sabotaje que la hace fallar: sacar las accionesDeLectura de musubi_work en el registro.
// arnes: archivo="internal/mcp/registry.go"
// arnes: de="\t\t\taccionesDeLectura: []string{\"status\", \"bids\", \"savings\"},\n"
// arnes: a=""
func TestLaCabinaLeeElTableroYNoLoToca(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	duenio := &Principal{Name: "mando", ProjectID: "musubi", Role: RoleAdmin, Read: ReadAll, Write: WriteAny}
	if _, e := callAsPrincipal(t, s, duenio, "musubi_work", map[string]any{
		"action": "plan", "batch": "lote-a", "units": []map[string]any{{"title": "unidad uno"}},
	}); e != nil {
		t.Fatalf("el dueño no pudo crear el batch: %+v", e)
	}

	cabina := &Principal{Name: "musubi-shell-lectura", Role: RoleReader, Read: ReadAll, Write: WriteNone}
	for _, accion := range []string{"status", " status "} { // el handler hace TrimSpace: la authz también
		res, e := callAsPrincipal(t, s, cabina, "musubi_work", map[string]any{"action": accion, "batch": "lote-a"})
		if e != nil {
			t.Fatalf("la cabina no pudo leer el tablero con action=%q: %+v", accion, e)
		}
		if !strings.Contains(textOf(t, res), "lote-a") {
			t.Errorf("la cabina leyó el tablero sin el batch:\n%s", textOf(t, res))
		}
	}
	if _, e := callAsPrincipal(t, s, cabina, "musubi_work", map[string]any{"action": "savings", "batch": "lote-a"}); e != nil {
		t.Errorf("la cabina no pudo leer savings: %+v", e)
	}

	// LEER NO ES TRABAJAR: nada que mute la pizarra, ni una action vacía o desconocida.
	for _, args := range []map[string]any{
		{"action": "claim", "batch": "lote-a", "agent": "x"},
		{"action": "complete", "id": "u1", "agent": "x", "result": "r"},
		{"action": "plan", "batch": "lote-b", "units": []map[string]any{{"title": "u9"}}},
		{"action": "clear", "batch": "lote-a"},
		{"action": ""},
		{"action": "STATUS"},
		{},
	} {
		if _, e := callAsPrincipal(t, s, cabina, "musubi_work", args); e == nil || e.Code != codeUnauthorized {
			t.Errorf("la cabina write=none pudo llamar musubi_work con %v (err=%+v): debe ser rechazo por rol", args, e)
		}
	}
	res, _ := callAsPrincipal(t, s, duenio, "musubi_work", map[string]any{"action": "status", "batch": "lote-a"})
	if crudo := textOf(t, res); !strings.Contains(crudo, `"open":1,"claimed":0,"done":0`) {
		t.Errorf("la pizarra cambió por la cabina:\n%s", crudo)
	}
	res, _ = callAsPrincipal(t, s, duenio, "musubi_work", map[string]any{"action": "status", "batch": "lote-b"})
	if crudo := textOf(t, res); strings.Contains(crudo, "u9") || !strings.Contains(crudo, `"total":0`) {
		t.Errorf("la cabina creó un batch:\n%s", crudo)
	}
}

// UN READER read=own SIGUE SIN VER EL TABLERO: la pizarra no está acotada por proyecto, así que
// abrírsela le mostraría los batches de otros tenants. Queda exactamente como antes del cambio.
//
// Sabotaje que la hace fallar: quitar la guarda de read=all en autorizaPorAccion.
// arnes: archivo="internal/mcp/principals.go"
// arnes: de="\tif read, _ := p.caps(); read != ReadAll {\n\t\treturn false\n\t}\n"
// arnes: a=""
func TestUnReaderAcotadoSigueSinVerElTablero(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	acotado := &Principal{Name: "crm-lector", ProjectID: "crm", Role: RoleReader, Read: ReadOwn, Write: WriteNone}
	if _, e := callAsPrincipal(t, s, acotado, "musubi_work", map[string]any{"action": "status"}); e == nil || e.Code != codeUnauthorized {
		t.Errorf("un reader read=own leyó el tablero global (err=%+v): vería los batches de otros proyectos", e)
	}
	// Y la apertura es por tool: otra tool no-readOnly con action=status no se cuela.
	cabina := &Principal{Name: "cabina", Role: RoleReader, Read: ReadAll, Write: WriteNone}
	if _, e := callAsPrincipal(t, s, cabina, "musubi_phase", map[string]any{"action": "status"}); e == nil || e.Code != codeUnauthorized {
		t.Errorf("musubi_phase action=status no declaró acciones de lectura y la cabina igual pasó (err=%+v)", e)
	}
}
