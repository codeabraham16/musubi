package memory

import (
	"fmt"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// UNA FILA SIN FECHA DE ENTREGA LEGIBLE SE LEE SIN FECHA, POR CADA PUERTA, Y NUNCA SALE PERDIDA POR
// ESO.
//
// La guarda vieja (TestUnEntregadoSinFechaDeEntregaNoEsPerdido, internal/fleet) arma el Comando a
// mano con `Entregado` en el cero de Go, o sea que prueba la regla de Perdido sobre un dato que ya
// llega sano. La auditoría A131 (C4-m9) hizo que escanearComando «rellenara» un `entregado` NULL con
// `creado` —una fila vieja sin fecha de entrega salía perdida por cualquier superficie— y fleet,
// memory y mcp quedaron verdes: la única fábrica de un fleet.Comando a partir de una fila no la
// miraba nadie.
//
// Acá la fila se escribe DIRECTO en device_commands —como la dejaría una versión anterior, una
// migración o una mano— para cada estado de fleet.EstadosDeComando y cada forma que puede tener la
// columna: NULL, vacía, en blanco, ilegible, el cero de Go escrito, y dos fechas legibles como
// control. Y se lee por las tres puertas que la leen: ComandoPorID, BitacoraDeComandos y
// CronologiaDeDevice. Sin fecha legible `Entregado` queda en cero y la fila muestra lo que el dominio
// deriva de un comando SIN entrega; con fecha, esa fecha y lo que el dominio deriva de ella.
//
// EXPOSICIÓN medida por la auditoría: cero. En el cerebro, 0 de 12.821 filas tienen estado
// `entregado` sin fecha, y el único escritor (TomarComandos) pone estado y fecha en el mismo UPDATE
// desde el primer commit de la flota. Lo que cuida esto es la próxima fila escrita por otro camino.
//
// Sabotaje: que escanearComando rellene la entrega ausente con `creado` (C4-m9) → una fila vieja sin
// fecha de entrega sale perdida.
// arnes: archivo="internal/memory/comandos.go"
// arnes: de="\tif entregado.Valid {\n\t\tif t, ok := parseObsTime(entregado.String); ok {\n\t\t\tc.Entregado = t\n\t\t}\n\t}"
// arnes: a="\tif t, ok := parseObsTime(entregado.String); ok && entregado.Valid {\n\t\tc.Entregado = t\n\t} else if c.Estado == fleet.EstadoEntregado {\n\t\tc.Entregado = c.Creado\n\t}"
func TestUnaFilaSinFechaDeEntregaLegibleSeLeeSinFechaPorCadaPuerta(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "pc-gio")
	ahora := time.Now().UTC().Truncate(time.Second)
	// MÁS VIEJA QUE CUALQUIER COTA y adentro de la ventana más larga de la cronología: si alguien
	// rellenara la entrega con `creado`, la fila saldría perdida.
	creado := ahora.Add(-20 * 24 * time.Hour)
	fecha := creado.Add(time.Minute)

	formas := []struct {
		nombre string
		valor  any // lo que se escribe en la columna; nil es NULL
		fecha  time.Time
	}{
		{"NULL", nil, time.Time{}},
		{"vacía", "", time.Time{}},
		{"en blanco", "   ", time.Time{}},
		{"ilegible", "no-es-una-fecha", time.Time{}},
		{"el cero de Go escrito", time.Time{}.Format(time.RFC3339), time.Time{}},
		{"RFC3339", fecha.Format(time.RFC3339), fecha},
		{"el formato de SQLite", fecha.Format("2006-01-02 15:04:05"), fecha},
	}

	type fila struct {
		id, caso string
		estado   fleet.EstadoComando
		fecha    time.Time
		quiero   fleet.EstadoComando
	}
	var filas []fila
	perdidasConFecha, entregadasSinFecha := 0, 0
	for _, estado := range fleet.EstadosDeComando {
		for _, f := range formas {
			caso := fmt.Sprintf("estado %q con `entregado` %s", estado, f.nombre)
			c, err := e.EncolarComando(fleet.Comando{
				DeviceID: d.ID, ProjectID: "casa", Principal: "gio", Creado: creado,
				Argv: []string{"echo", caso}, Timeout: 30 * time.Second,
			})
			if err != nil {
				t.Fatalf("%s: %v", caso, err)
			}
			if _, err := e.db.Exec(`UPDATE device_commands SET estado = ?, entregado = ? WHERE id = ?`,
				string(estado), f.valor, c.ID); err != nil {
				t.Fatalf("%s: %v", caso, err)
			}
			// LA REFERENCIA es el dominio sobre el comando que la fila DE VERDAD describe: con la fecha que
			// tiene, o sin ninguna.
			quiero := fleet.Comando{Estado: estado, Creado: creado, Entregado: f.fecha}.EstadoActual(ahora)
			filas = append(filas, fila{id: c.ID, caso: caso, estado: estado, fecha: f.fecha, quiero: quiero})
			if !f.fecha.IsZero() && quiero == fleet.EstadoPerdido && estado == fleet.EstadoEntregado {
				perdidasConFecha++
			}
			if f.fecha.IsZero() && estado == fleet.EstadoEntregado {
				entregadasSinFecha++
			}
		}
	}
	// EL PISO: el control positivo (una fila con fecha que SÍ está perdida) y el caso del hallazgo
	// (una entregada sin fecha). Sin el primero, una puerta que nunca derivara `perdido` pasaría; sin el
	// segundo, la tabla no miraría la fila que C4-m9 rompía.
	if perdidasConFecha == 0 || entregadasSinFecha == 0 {
		t.Fatalf("la tabla tiene %d filas perdidas con fecha y %d entregadas sin fecha: no mide lo que dice",
			perdidasConFecha, entregadasSinFecha)
	}

	revisarComando := func(puerta string, f fila, c fleet.Comando) {
		t.Helper()
		if c.Estado != f.estado {
			t.Errorf("%s, %s: se leyó el estado %q; la siembra no quedó como dice", puerta, f.caso, c.Estado)
			return
		}
		if f.fecha.IsZero() && !c.Entregado.IsZero() {
			t.Errorf("%s, %s: la fila se leyó con una entrega a las %s, y la columna no tiene ninguna fecha "+
				"legible. Una fecha inventada es un comando que alguien «se llevó» cuando no se sabe si pasó",
				puerta, f.caso, c.Entregado.Format(time.RFC3339))
		}
		if !f.fecha.IsZero() && !c.Entregado.Equal(f.fecha) {
			t.Errorf("%s, %s: se leyó la entrega %s y la columna dice %s", puerta, f.caso,
				c.Entregado.Format(time.RFC3339), f.fecha.Format(time.RFC3339))
		}
		if got := c.EstadoActual(ahora); got != f.quiero {
			t.Errorf("%s, %s: se muestra %q y el dominio deriva %q del comando que la fila describe. Un dato "+
				"ausente no es un comando muerto", puerta, f.caso, got, f.quiero)
		}
	}

	for _, f := range filas {
		c, ok, err := e.ComandoPorID(f.id)
		if err != nil || !ok {
			t.Fatalf("ComandoPorID, %s: ok=%v err=%v", f.caso, ok, err)
		}
		revisarComando("ComandoPorID", f, c)
	}

	bitacora, err := e.BitacoraDeComandos("casa", d.ID, 4*len(filas))
	if err != nil {
		t.Fatal(err)
	}
	porID := map[string]fleet.Comando{}
	for _, c := range bitacora {
		porID[c.ID] = c
	}
	for _, f := range filas {
		c, esta := porID[f.id]
		if !esta {
			t.Errorf("BitacoraDeComandos no devolvió %s: la prueba no midió esa puerta", f.caso)
			continue
		}
		revisarComando("BitacoraDeComandos", f, c)
	}

	hechos, _, err := e.CronologiaDeDevice("casa", d.ID, fleet.VentanaHasta(ahora.Add(time.Minute), fleet.VentanaMax), 4*len(filas), ahora)
	if err != nil {
		t.Fatal(err)
	}
	enCronologia := map[string]string{}
	for _, h := range hechos {
		enCronologia[h.Referencia] = h.Estado
	}
	for _, f := range filas {
		got, esta := enCronologia[f.id]
		if !esta {
			t.Errorf("CronologiaDeDevice no devolvió %s: la prueba no midió esa puerta", f.caso)
			continue
		}
		if got != string(f.quiero) {
			t.Errorf("CronologiaDeDevice, %s: el hecho dice %q y el dominio deriva %q del comando que la fila "+
				"describe. Un dato ausente no es un comando muerto", f.caso, got, f.quiero)
		}
	}
}
