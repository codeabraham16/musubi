package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
)

// LA ROTACIÓN EN CALIENTE: LOS DOS TOKENS VALEN, Y EL VIEJO MUERE CUANDO EL AGENTE USA EL NUEVO.
//
// Ola 2 del plan empresa. El agente se entera del token nuevo en la RESPUESTA de un latido, o sea
// DESPUÉS de haber usado el viejo. Sin solapamiento quedaría afuera entre que lo recibe y lo
// guarda — y si ese guardado falla, para siempre.
//
// Y la rotación se completa cuando LLEGA un latido con el nuevo, no cuando se emite: llegar es la
// única prueba de que el agente lo guardó. Darlo por cerrado al emitirlo sería creerle al
// remitente en vez de al receptor, que es el mismo error que cerró A78.
//
// Sabotaje que la hace fallar: completar la rotación dentro de AbrirRotacion (el viejo muere
// antes de que el agente sepa nada), o no llamar a CompletarRotacion en el latido (el viejo no
// muere nunca y rotar no rota).
func TestLosDosTokensValenHastaQueElAgenteUsaElNuevo(t *testing.T) {
	s, ts, tokenViejo, _ := servidorConFlota(t)

	// Se abre POR LA TOOL y no por el almacén: el latido saca el token del mapa en memoria, que
	// sólo la tool llena. Ir por abajo dejaría una rotación abierta e inentregable, y la prueba
	// pasaría a medir un camino que nadie usa.
	res, e := call(t, s, "musubi_fleet_rotate", map[string]any{"project": "casa", "device": "pc-gio"})
	if e != nil {
		t.Fatalf("no se pudo abrir la rotación: %+v", e)
	}
	tokenNuevo, _ := jsonOf(t, res)["token"].(string)
	if tokenNuevo == "" {
		t.Fatal("la tool no devolvió el token nuevo")
	}
	if tokenNuevo == tokenViejo {
		t.Fatal("la rotación devolvió el MISMO token: no rotó nada")
	}

	// LOS DOS valen durante la ventana.
	if code, b := postCon(t, ts.URL+fleetHeartbeatPath, tokenViejo, `{"version":"0.1.0"}`); code != 200 {
		t.Fatalf("el token VIEJO dejó de valer al abrir la rotación (%d %s): el agente queda afuera entre que recibe el nuevo y lo guarda", code, b)
	}

	// Y el latido con el viejo TRAE el nuevo, para que el agente pueda guardarlo.
	code, cuerpo := postCon(t, ts.URL+fleetHeartbeatPath, tokenViejo, `{"version":"0.1.0"}`)
	if code != 200 {
		t.Fatalf("%d %s", code, cuerpo)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(cuerpo), &resp); err != nil {
		t.Fatalf("respuesta ilegible: %v", err)
	}
	if resp["token_nuevo"] != tokenNuevo {
		t.Errorf("el latido con el token viejo no trajo el nuevo (%v): el agente no tiene por dónde enterarse", resp["token_nuevo"])
	}

	// Ahora el agente late con el NUEVO: eso completa la rotación.
	if code, b := postCon(t, ts.URL+fleetHeartbeatPath, tokenNuevo, `{"version":"0.1.0"}`); code != 200 {
		t.Fatalf("el token NUEVO no autentica (%d %s)", code, b)
	}

	// Y a partir de ahí el VIEJO deja de valer, que es el punto entero de rotar.
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenViejo, `{"version":"0.1.0"}`); code != 401 {
		t.Errorf("el token viejo sigue valiendo después de completada la rotación (código %d): rotar no rotó nada", code)
	}
	// Y el nuevo ya no viaja: la rotación se cerró.
	_, cuerpo = postCon(t, ts.URL+fleetHeartbeatPath, tokenNuevo, `{"version":"0.1.0"}`)
	if strings.Contains(cuerpo, "token_nuevo") {
		t.Error("el cerebro sigue mandando el token nuevo después de completar la rotación: un secreto que se repite sin razón es superficie")
	}
}

// UNA ROTACIÓN VENCIDA SE ABANDONA, Y EL TOKEN VIEJO SIGUE VALIENDO.
//
// Es al revés de lo que decía el plan de la Ola 2 («se borra el previo igual, fail-closed»), y el
// motivo es cuál de los dos errores es peor: rotar es HIGIENE, así que abandonar deja el token
// viejo vivo —el estado que ya había, no empeora nada—, mientras que completar a la fuerza deja
// al agente sin ninguna credencial válida, y la máquina más difícil de arreglar es justamente la
// que dejó de latir. Si el token se FILTRÓ, la herramienta es revocar, que es instantáneo.
//
// Sabotaje que la hace fallar: que AbandonarRotacionesVencidas complete la rotación en vez de
// descartarla.
func TestUnaRotacionVencidaSeAbandonaYElTokenViejoSigueValiendo(t *testing.T) {
	s, ts, tokenViejo, _ := servidorConFlota(t)
	d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")

	tokenNuevo, err := s.engine.AbrirRotacion(d.ID, time.Now().Add(-time.Minute)) // ya vencida
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.engine.AbandonarRotacionesVencidas(time.Now())
	if err != nil || n != 1 {
		t.Fatalf("no se abandonó la rotación vencida (n=%d err=%v)", n, err)
	}
	if code, b := postCon(t, ts.URL+fleetHeartbeatPath, tokenViejo, `{"version":"0.1.0"}`); code != 200 {
		t.Fatalf("el token VIEJO dejó de valer tras abandonar la rotación (%d %s): una higiene fallida se convirtió en un apagón", code, b)
	}
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenNuevo, `{"version":"0.1.0"}`); code != 401 {
		t.Errorf("el token de una rotación abandonada sigue autenticando (código %d)", code)
	}
}

// EL TOKEN NUEVO NO SE GUARDA EN LA BASE, NI SIQUIERA DURANTE LA VENTANA.
//
// En reposo hay hashes y no credenciales, en los dos almacenes. Un volcado de la base no puede
// ser un llavero — es exactamente lo que costó A74 con la contraseña de pantalla, y la salida es
// la misma: el secreto vive en memoria del cerebro.
//
// Sabotaje que la hace fallar: guardar el token en claro en `token_sha256_nuevo` para poder
// repetirlo sin el mapa en memoria.
func TestElTokenDeLaRotacionNoQuedaEnClaroEnLaBase(t *testing.T) {
	s, _, _, _ := servidorConFlota(t)
	d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")

	tokenNuevo, err := s.engine.AbrirRotacion(d.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	filas, err := s.engine.ListarDevices("casa", false)
	if err != nil || len(filas) == 0 {
		t.Fatalf("no se pudieron listar los devices: %v", err)
	}
	// La superficie de lectura del dominio no expone la credencial por ningún campo.
	crudo, _ := json.Marshal(filas)
	if strings.Contains(string(crudo), tokenNuevo) {
		t.Error("el token de la rotación aparece en la representación del device: un volcado se vuelve un llavero")
	}
}

// LA TOOL ES ADMIN, y no alcanza con tener capacidades sobre esa máquina.
//
// Rotar cambia la credencial con la que la máquina se autentica: quien puede hacerlo puede dejar
// afuera al agente. Es la misma clase de acto que enrolar o revocar.
//
// Sabotaje: sacar el `if !p.isAdmin()` de toolFleetRotate.
func TestRotarExigeAdmin(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "pc-gio", "tier": "A", "caps": []string{"metrics", "exec"},
		"project": "casa", "os": "linux", "arch": "amd64",
	}); e != nil {
		t.Fatalf("no se pudo enrolar: %+v", e)
	}
	ctx := withPrincipal(context.Background(), &Principal{
		Name: "tecnico", ProjectID: "casa", Role: RoleWriter, Read: ReadAll, Write: WriteOwn,
	})
	_, e := s.toolFleetRotate(ctx, json.RawMessage(`{"project":"casa","device":"pc-gio"}`))
	if e == nil {
		t.Fatal("un principal no-admin rotó el token de una máquina: puede dejar afuera al agente si sale mal")
	}
	if !strings.Contains(e.Message, "admin") {
		t.Errorf("el rechazo no dice que hace falta admin: %s", e.Message)
	}
}
