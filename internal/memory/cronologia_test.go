package memory

// Pruebas de la CRONOLOGÍA a nivel ALMACENAMIENTO (fase 5 · S11).
//
// Existen aparte de las de la tool porque custodian dos cosas que la tool no puede: que la ventana
// viaje en el `WHERE` y que las fechas se lleven a la granularidad de la tabla ANTES de formatear.
// Las dos viven en este archivo y las dos tienen una copia de la guarda un nivel más arriba — que
// es lo correcto, porque CronologiaDeDevice es parte de la interfaz del motor y la puede llamar
// alguien que no pase por la tool.

import (
	"strings"
	"testing"
	"time"

	"musubi/internal/fleet"
)

func sembrarCronologia(t *testing.T, e *DbEngine, proyecto, nombre string) fleet.Device {
	t.Helper()
	d, _ := altaDePrueba(t, e, proyecto, nombre)
	if _, err := e.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: proyecto, Principal: "gio",
		Argv: []string{"echo", "MARCAAHORA"}, Timeout: 30 * time.Second,
	}); err != nil {
		t.Fatal(err)
	}
	return d
}

// UNA VENTANA QUE TERMINA «AHORA» INCLUYE LO QUE ACABA DE PASAR, aunque el instante traiga
// fracción de segundo y la tabla guarde sólo segundos.
//
// Sabotaje: sacar `v = v.Normalizada()` de CronologiaDeDevice → el comando encolado en este mismo
// segundo queda afuera, porque `Format(time.RFC3339)` tira la fracción y el borde superior es
// abierto. Es el bug que encontró el control POSITIVO del barrido de aislamiento.
func TestLaCronologiaIncluyeLoQueAcabaDePasar(t *testing.T) {
	e := newTestEngine(t)
	d := sembrarCronologia(t, e, "infra", "pc")

	// `ahora` con fracción: es lo que devuelve time.Now() en la vida real.
	ahora := time.Now().UTC()
	if ahora.Nanosecond() == 0 {
		ahora = ahora.Add(700 * time.Millisecond) // que la fracción exista sí o sí
	}
	v := fleet.Ventana{Desde: ahora.Add(-time.Hour), Hasta: ahora}
	hechos, _, err := e.CronologiaDeDevice("infra", d.ID, v, 50, ahora)
	if err != nil {
		t.Fatalf("CronologiaDeDevice: %v", err)
	}
	if len(hechos) == 0 {
		t.Fatal("lo que acaba de pasar quedó afuera de una ventana que termina ahora")
	}
	if !strings.Contains(strings.Join(hechos[0].Argv, " "), "MARCAAHORA") {
		t.Errorf("el hecho que volvió no es el que se sembró: %v", hechos[0])
	}
	if hechos[0].Device != "pc" {
		t.Errorf("el nombre de la máquina no se resolvió: %q", hechos[0].Device)
	}
}

// LA VENTANA VA EN EL `WHERE`. Con el filtro después del tope, un hecho viejo tapado por ruido
// nuevo no aparece nunca y el vacío se lee como «no pasó nada».
//
// Sabotaje: quitar `AND creado >= ? AND creado < ?` de hechosDeComandos y filtrar en Go → esta
// prueba falla con `limite: 2` y tres hechos nuevos encima del viejo.
func TestLaVentanaViajaEnLaConsultaYNoEnGo(t *testing.T) {
	e := newTestEngine(t)
	d := sembrarCronologia(t, e, "infra", "pc")
	viejo := time.Now().UTC().Add(-72 * time.Hour)
	if _, err := e.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: "infra", Principal: "gio", Creado: viejo,
		Argv: []string{"echo", "MARCAVIEJA"}, Timeout: 30 * time.Second,
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := e.EncolarComando(fleet.Comando{
			DeviceID: d.ID, ProjectID: "infra", Principal: "gio",
			Argv: []string{"echo", "ruido"}, Timeout: 30 * time.Second,
		}); err != nil {
			t.Fatal(err)
		}
	}
	ahora := time.Now().UTC()
	v := fleet.Ventana{Desde: viejo.Add(-time.Hour), Hasta: viejo.Add(time.Hour)}
	hechos, _, err := e.CronologiaDeDevice("infra", d.ID, v, 2, ahora)
	if err != nil {
		t.Fatalf("CronologiaDeDevice: %v", err)
	}
	if len(hechos) != 1 {
		t.Fatalf("esperaba 1 hecho en la ventana vieja, obtuve %d: %v", len(hechos), hechos)
	}
	if !strings.Contains(strings.Join(hechos[0].Argv, " "), "MARCAVIEJA") {
		t.Errorf("el hecho de la ventana vieja no volvió: %v", hechos[0])
	}
}

// LA MÁQUINA AJENA NO ENTRA, aunque se pida su id: el `project_id` está en las tres consultas.
//
// Sabotaje: sacar `project_id = ?` del WHERE → pedir la cronología con el id de una máquina de
// otro tenant la devuelve entera. El id de un device es un uuid, pero un uuid filtrado en una
// bitácora anterior no puede convertirse en la llave de otro tenant.
func TestLaCronologiaNoCruzaTenants(t *testing.T) {
	e := newTestEngine(t)
	ajena := sembrarCronologia(t, e, "otro-tenant", "pc-ajena")

	ahora := time.Now().UTC()
	v := fleet.VentanaHasta(ahora, fleet.VentanaDefault)
	hechos, _, err := e.CronologiaDeDevice("infra", ajena.ID, v, 50, ahora)
	if err != nil {
		t.Fatalf("CronologiaDeDevice: %v", err)
	}
	if len(hechos) != 0 {
		t.Fatalf("FUGA cross-tenant: se devolvieron %d hechos de una máquina de otro proyecto: %v", len(hechos), hechos)
	}
	// Control positivo: con su proyecto de verdad, el dato SÍ está. Sin esto la prueba pasaría
	// igual si la cronología estuviera rota y no devolviera nada nunca.
	suyos, _, err := e.CronologiaDeDevice("otro-tenant", ajena.ID, v, 50, ahora)
	if err != nil || len(suyos) == 0 {
		t.Fatalf("el control positivo falló: %d hechos, err=%v", len(suyos), err)
	}
}

// Una ventana inválida devuelve ERROR y no una lista. Fail-closed también acá: si el motor
// tragara una ventana vacía, cualquier llamador que no valide barrería la tabla entera.
func TestLaCronologiaRechazaUnaVentanaInvalida(t *testing.T) {
	e := newTestEngine(t)
	d := sembrarCronologia(t, e, "infra", "pc")
	ahora := time.Now().UTC()
	if _, _, err := e.CronologiaDeDevice("infra", d.ID, fleet.Ventana{}, 50, ahora); err == nil {
		t.Error("una ventana vacía tiene que dar error, no barrer la tabla")
	}
	if _, _, err := e.CronologiaDeDevice("infra", d.ID, fleet.Ventana{Desde: ahora, Hasta: ahora.Add(-time.Hour)}, 50, ahora); err == nil {
		t.Error("una ventana al revés tiene que dar error")
	}
}
