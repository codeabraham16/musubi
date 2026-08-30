package mcp

// Pruebas de `musubi_fleet_rename` (A64).
//
// Lo que se custodia es UNA idea: renombrar una máquina NO es cosmético, es un cambio de
// autorización disfrazado — y la tool tiene que decirlo antes, no después.

import (
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

func regConNombres(concede, allow string) *PrincipalRegistry {
	return &PrincipalRegistry{principals: []Principal{
		{Name: "op-concede", Role: RoleWriter, ProjectID: "infra",
			Fleet: map[fleet.Cap][]string{fleet.CapExec: {concede}}},
		{Name: "op-allow", Role: RoleWriter, ProjectID: "infra",
			Fleet:     map[fleet.Cap][]string{fleet.CapExec: {"*"}},
			ExecAllow: map[string][]string{allow: {"systemctl"}}},
		// El del comodín NO tiene que aparecer nunca: sobrevive a cualquier rename, y listarlo
		// sería ruido que tapa lo que sí se rompe.
		{Name: "op-comodin", Role: RoleWriter, ProjectID: "infra",
			Fleet: map[fleet.Cap][]string{fleet.CapExec: {"*"}}},
	}}
}

// SIN `confirmar` NO RENOMBRA, y devuelve el informe. El default de «no lo pensé» es que no pase
// nada — la misma forma que tiene todo lo caro de este track.
//
// Sabotaje: renombrar igual sin confirmar → falla acá.
func TestRenombrarNoHaceNadaSinConfirmar(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	s.buscarPrincipal = regConNombres("pc-gio", "pc-gio")

	out := jsonOf(t, renombrarOk(t, s, nil,
		map[string]any{"device": "pc-gio", "nuevo": "davantis-1", "project": "infra"}))
	if out["renombrado"] != false {
		t.Fatalf("renombró sin confirmar: %v", out)
	}
	if _, existe, _ := s.engine.DevicePorNombre("infra", "pc-gio"); !existe {
		t.Error("la máquina cambió de nombre igual")
	}
	// El informe tiene que NOMBRAR a quién le va a romper la autorización.
	crudo := textOf(t, renombrarOk(t, s, nil,
		map[string]any{"device": "pc-gio", "nuevo": "davantis-1", "project": "infra"}))
	for _, quiero := range []string{"op-concede", "op-allow", "principals.yaml"} {
		if !strings.Contains(crudo, quiero) {
			t.Errorf("el informe no menciona %q: %s", quiero, crudo)
		}
	}
	// Y el del comodín NO, porque no se rompe.
	if strings.Contains(crudo, "op-comodin") {
		t.Errorf("listó la concesión con comodín, que sobrevive al rename: %s", crudo)
	}
}

// EL SENTIDO PELIGROSO: si algo YA nombraba el nombre nuevo, la máquina renombrada HEREDA esa
// autorización sin que nadie se la haya dado. Es el que a nadie se le ocurre mirar.
//
// Sabotaje: informar sólo el impacto del nombre viejo → falla acá.
func TestRenombrarAvisaLoQueLaMaquinaVaAHEREDAR(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	// Nadie nombra `pc-gio`; alguien SÍ nombra el destino.
	s.buscarPrincipal = regConNombres("davantis-1", "davantis-1")

	crudo := textOf(t, renombrarOk(t, s, nil,
		map[string]any{"device": "pc-gio", "nuevo": "davantis-1", "project": "infra"}))
	if !strings.Contains(crudo, "HEREDA") {
		t.Errorf("no avisó que la máquina hereda la autorización del nombre nuevo: %s", crudo)
	}
	out := jsonOf(t, renombrarOk(t, s, nil,
		map[string]any{"device": "pc-gio", "nuevo": "davantis-1", "project": "infra"}))
	hereda := out["hereda_del_nombre_nuevo"].(map[string]any)
	if hereda["concesiones"] == nil {
		t.Error("el bloque de herencia vino vacío teniendo una concesión que nombra el destino")
	}
}

// CON `confirmar` RENOMBRA Y CONSERVA EL ID — que es toda la razón de que esta tool exista. Dar de
// baja y volver a enrolar daba un id nuevo y perdía la bitácora.
//
// Sabotaje: implementar el rename como baja+alta → falla acá, porque el id cambia y los comandos
// dejan de encontrarse.
func TestRenombrarConservaElIdYConElLaBitacora(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	s.buscarPrincipal = regConNombres("otra-cosa", "otra-cosa")

	out := jsonOf(t, renombrarOk(t, s, nil,
		map[string]any{"device": "pc-gio", "nuevo": "davantis-1", "project": "infra", "confirmar": true}))
	if out["renombrado"] != true {
		t.Fatalf("no renombró: %v", out)
	}
	nuevo, existe, err := s.engine.DevicePorNombre("infra", "davantis-1")
	if err != nil || !existe {
		t.Fatalf("no quedó con el nombre nuevo: %v %v", existe, err)
	}
	if nuevo.ID != d.ID {
		t.Fatalf("EL ID CAMBIÓ (%s -> %s): con eso se pierde toda la bitácora", d.ID, nuevo.ID)
	}
	if _, existe, _ := s.engine.DevicePorNombre("infra", "pc-gio"); existe {
		t.Error("el nombre viejo sigue existiendo")
	}
	// Y la bitácora sigue ahí, que es lo que el id conserva.
	hechos, _, err := s.engine.CronologiaDeDevice("infra", nuevo.ID,
		fleet.VentanaHasta(time.Now().UTC(), fleet.VentanaDefault), 50, time.Now().UTC())
	if err != nil || len(hechos) == 0 {
		t.Errorf("se perdió el historial al renombrar: %d hechos, err=%v", len(hechos), err)
	}
}

// El nombre nuevo no puede pisar a otra máquina, y el error tiene que ser legible: un
// "UNIQUE constraint failed" no le dice nada a nadie.
func TestRenombrarNoPuedePisarAOtraMaquina(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]interface{}{
		"name": "ocupada", "tier": "A", "project": "infra", "caps": []string{"metrics"}}); e != nil {
		t.Fatal(e)
	}
	_, e := call(t, s, "musubi_fleet_rename", map[string]interface{}{
		"device": "pc-gio", "nuevo": "ocupada", "project": "infra", "confirmar": true})
	if e == nil {
		t.Fatal("dejó pisar el nombre de otra máquina")
	}
	if !strings.Contains(e.Message, "ocupada") || strings.Contains(strings.ToUpper(e.Message), "UNIQUE") {
		t.Errorf("el error no es legible: %s", e.Message)
	}
}

// Un nombre con caracteres que rompen otras superficies se rechaza. Va a etiquetas de Prometheus,
// a los selectores de principals.yaml y a cada línea de log.
func TestUnNombreQueRompeOtrasSuperficiesSeRechaza(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	for _, malo := range []string{"con\nsalto", "con,coma", "con\"comilla", "", strings.Repeat("x", fleet.NombreDeviceMax+1)} {
		if _, e := call(t, s, "musubi_fleet_rename", map[string]interface{}{
			"device": "pc-gio", "nuevo": malo, "project": "infra", "confirmar": true}); e == nil {
			t.Errorf("aceptó el nombre %q", malo)
		}
	}
}

// Renombrar es ADMIN: cambia a qué apuntan las concesiones, así que no puede hacerlo cualquiera
// que tenga `exec` sobre la máquina.
func TestRenombrarEsAdmin(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	escritor := conCaps("infra", map[fleet.Cap][]string{fleet.CapExec: {"*"}})
	_, e := callAsPrincipal(t, s, escritor, "musubi_fleet_rename",
		map[string]any{"device": "pc-gio", "nuevo": "davantis-1", "confirmar": true})
	if e == nil || e.Code != codeUnauthorized {
		t.Fatalf("un writer con exec pudo renombrar: %+v", e)
	}
}

// renombrarOk llama a la tool y falla el test si devuelve error. Nombre propio y no `mustCall`
// —que ya existe en methods_codegraph_test.go— porque un helper genérico compartido entre dos
// suites que no se conocen es una colisión esperando pasar; ésta la cazó el compilador.
func renombrarOk(t *testing.T, s *McpServer, p *Principal, args map[string]any) interface{} {
	t.Helper()
	res, e := callAsPrincipal(t, s, p, "musubi_fleet_rename", args)
	if e != nil {
		t.Fatalf("musubi_fleet_rename: %+v", e)
	}
	return res
}
