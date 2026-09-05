package memory

// Pruebas de la persistencia de la bitácora de comandos. Lo que se custodia acá es lo que la
// TABLA guarda, no lo que las superficies muestran — para eso están las pruebas de internal/mcp.

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// Contraseña SINTÉTICA: no es de ninguna sesión real, sólo tiene que ser una cadena que no pueda
// aparecer en la fila por casualidad.
const claveDePruebaA74 = "clave-sintetica-A74-no-real"

func argvCrudoEnLaBase(t *testing.T, e *DbEngine, id string) string {
	t.Helper()
	var argv string
	if err := e.db.QueryRow(`SELECT argv FROM device_commands WHERE id = ?`, id).Scan(&argv); err != nil {
		t.Fatalf("leer el argv crudo de %q: %v", id, err)
	}
	return argv
}

// A74: LA CONTRASEÑA DE PANTALLA NO PUEDE QUEDAR EN LA TABLA UNA VEZ ENTREGADA. Las superficies de
// lectura ya la tapaban con ArgvDeBitacora, pero la fila la guardaba cruda; una sesión dura horas
// y el argv crudo deja de servir en el instante en que el agente lo toma. El agente, eso sí, TIENE
// que recibirla: es la única forma de que llegue a la máquina.
//
// Sabotaje que la hace fallar: quitar la llamada a taparArgvConSecreto del lazo de entrega.
func TestAlEntregarUnaPantallaLaContrasenaSeTapaEnLaBase(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "pc-gio")
	ahora := time.Now().UTC()

	c, err := e.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: "casa", Principal: "gio", Creado: ahora,
		Argv:    []string{fleet.OpPantalla, "sesion-1", claveDePruebaA74, "30m0s"},
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Antes de entregarse la fila la lleva: no hay otra forma de que el agente la reciba.
	if !strings.Contains(argvCrudoEnLaBase(t, e, c.ID), claveDePruebaA74) {
		t.Fatal("la prueba no prueba nada: la contraseña ni siquiera llegó a la cola")
	}

	entregados, err := e.TomarComandos(d.ID, ahora, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entregados) != 1 || !reflect.DeepEqual(entregados[0].Argv, c.Argv) {
		t.Fatalf("el agente tiene que recibir el argv CRUDO, con la contraseña: %v", entregados)
	}

	crudo := argvCrudoEnLaBase(t, e, c.ID)
	if strings.Contains(crudo, claveDePruebaA74) {
		t.Fatalf("LA CONTRASEÑA DE PANTALLA SIGUE EN CLARO EN device_commands.argv tras entregarse: %s", crudo)
	}
	// Tapar no es borrar: el id de sesión se conserva porque es lo que usa el cierre de la sesión
	// (marcarSesionSiEsDePantalla lee argv[0] y argv[1]) y lo que cruza las dos bitácoras.
	quiero := fleet.ArgvDeBitacora(c.Argv)
	if got := fleet.ArgvDesdeTexto(crudo); !reflect.DeepEqual(got, quiero) {
		t.Errorf("la fila entregada tiene que quedar en la forma de ArgvDeBitacora %v, quedó %v", quiero, got)
	}
	leido, existe, err := e.ComandoPorID(c.ID)
	if err != nil || !existe {
		t.Fatalf("ComandoPorID: %v / %v", existe, err)
	}
	if leido.Estado != fleet.EstadoEntregado || len(leido.Argv) < 2 || leido.Argv[0] != fleet.OpPantalla || leido.Argv[1] != "sesion-1" {
		t.Errorf("el comando tapado tiene que seguir siendo reconocible como pantalla de sesion-1 y entregado: %+v", leido)
	}
}

// EL CONTROL: tapar es sólo para lo que lleva secreto. Un comando común conserva su argv intacto
// después de entregarse — la bitácora tiene que decir QUÉ se corrió, y eso es el argv.
//
// Sabotaje: ampliar la guarda de taparArgvConSecreto a cualquier argv → falla acá.
func TestUnComandoComunConservaSuArgvTrasEntregarse(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "pc-gio")
	ahora := time.Now().UTC()

	argv := []string{"ls", "-la", "/var/log"}
	c, err := e.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: "casa", Principal: "gio", Creado: ahora,
		Argv: argv, Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.TomarComandos(d.ID, ahora, 10); err != nil {
		t.Fatal(err)
	}
	leido, _, err := e.ComandoPorID(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if leido.Estado != fleet.EstadoEntregado || !reflect.DeepEqual(leido.Argv, argv) {
		t.Fatalf("un comando común tiene que quedar entregado con su argv intacto, quedó %v (%s)", leido.Argv, leido.Estado)
	}
}

// Una pantalla que VENCE sin entregarse también lleva la contraseña encima, y ya no le va a
// servir a nadie: se tapa en el mismo barrido que la vence.
//
// Sabotaje: quitar taparPantallasPendientesVencidas de TomarComandos → falla acá.
func TestUnaPantallaVencidaSinEntregarTambienSeTapa(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "pc-gio")
	ahora := time.Now().UTC()

	vieja, err := e.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: "casa", Principal: "gio", Creado: ahora.Add(-2 * fleet.ComandoVidaMax),
		Argv:    []string{fleet.OpPantalla, "sesion-vieja", claveDePruebaA74, "30m0s"},
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	entregados, err := e.TomarComandos(d.ID, ahora, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entregados) != 0 {
		t.Fatalf("una pantalla vencida no se entrega: %v", entregados)
	}
	crudo := argvCrudoEnLaBase(t, e, vieja.ID)
	if strings.Contains(crudo, claveDePruebaA74) {
		t.Fatalf("la pantalla venció y la contraseña sigue en claro en la fila: %s", crudo)
	}
	if leido, _, _ := e.ComandoPorID(vieja.ID); leido.Estado != fleet.EstadoExpirado || len(leido.Argv) < 2 || leido.Argv[1] != "sesion-vieja" {
		t.Errorf("la fila tiene que quedar expirada y reconocible como pantalla de sesion-vieja: %+v", leido)
	}
}
