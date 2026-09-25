package memory

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// origenesDelEnum son los orígenes que OrigenValido conserva. Son el control positivo de las dos
// pruebas de abajo; la decisión sobre el enum —y su cierre contra el bloque const— vive en
// internal/fleet (TestElOrigenEsUnaListaBlancaCerradaContraSuEnum).
var origenesDelEnum = []fleet.OrigenComando{fleet.OrigenPersona, fleet.OrigenPolitica, fleet.OrigenDesconocido}

// origenesRaros deriva del enum los valores que se le parecen sin serlo, y suma dos que no se
// parecen a nada. Es el mismo corpus que la prueba del dominio, derivado de la misma forma.
func origenesRaros() []fleet.OrigenComando {
	enum := map[fleet.OrigenComando]bool{}
	for _, o := range origenesDelEnum {
		enum[o] = true
	}
	vistos := map[fleet.OrigenComando]bool{}
	var out []fleet.OrigenComando
	sumar := func(o fleet.OrigenComando) {
		if !enum[o] && !vistos[o] {
			vistos[o] = true
			out = append(out, o)
		}
	}
	for _, o := range origenesDelEnum {
		s := string(o)
		sumar(fleet.OrigenComando(strings.ToUpper(s)))
		sumar(fleet.OrigenComando(s + " "))
		sumar(fleet.OrigenComando(" " + s))
		sumar(fleet.OrigenComando(s + "x"))
	}
	sumar("cron")
	sumar("robot")
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// LO QUE QUEDA EN LA TABLA ES UN ORIGEN DEL ENUM, VENGA LO QUE VENGA POR LA PUERTA DE ESCRITURA.
//
// La guarda vieja (TestUnOrigenRaroSeGuardaComoDesconocido, internal/fleet) prueba la FUNCIÓN
// OrigenValido; la promesa de su nombre es sobre lo que se GUARDA. La auditoría A131 (C3-m8) sacó la
// normalización de EncolarComando y fleet, memory y mcp quedaron verdes: un `"cron"` quedaba crudo
// en device_commands.origen, y lo tapaba la normalización al LEER. Acá se lee la columna CRUDA, así
// que la lectura no puede tapar nada.
//
// EXPOSICIÓN medida por la auditoría: cero. Las 12.821 filas tienen el origen vacío, `persona` o
// `politica`, y los siete llamadores de EncolarComando pasan una constante. El hueco era para el
// próximo llamador.
//
// Sabotaje: sacar la normalización al escribir (C3-m8) → el origen raro queda crudo en la tabla.
// arnes: archivo="internal/memory/comandos.go"
// arnes: de="\tc.Origen = fleet.OrigenValido(c.Origen)\n"
// arnes: a=""
func TestLoQueQuedaEnLaTablaEsUnOrigenDelEnum(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "pc-gio")
	raros := origenesRaros()
	// EL PISO: sin raros, esto sólo mediría el camino feliz.
	if len(raros) < 10 {
		t.Fatalf("sólo %d orígenes raros (%q): la derivación se rompió", len(raros), raros)
	}
	for _, o := range append(append([]fleet.OrigenComando{}, origenesDelEnum...), raros...) {
		c, err := e.EncolarComando(fleet.Comando{
			DeviceID: d.ID, ProjectID: "casa", Principal: "gio", Creado: time.Now().UTC(),
			Argv: []string{"uptime"}, Timeout: 30 * time.Second, Origen: o,
		})
		if err != nil {
			t.Fatalf("encolar con origen %q: %v", o, err)
		}
		var crudo string
		if err := e.db.QueryRow(`SELECT origen FROM device_commands WHERE id = ?`, c.ID).Scan(&crudo); err != nil {
			t.Fatal(err)
		}
		quiero := fleet.OrigenValido(o)
		if crudo != string(quiero) {
			t.Errorf("con origen %q la tabla guardó %q y tenía que guardar %q: una categoría que ninguna "+
				"superficie sabe dibujar queda escrita, y sólo la tapa quien se acuerde de normalizar al leer", o, crudo, quiero)
		}
		if c.Origen != quiero {
			t.Errorf("con origen %q EncolarComando devolvió %q y guardó %q: el llamador y la tabla cuentan dos historias", o, c.Origen, crudo)
		}
	}
	// Y lo de afuera es DESCONOCIDO, no «lo que OrigenValido diga»: esto no puede quedar verde porque
	// alguien le haya aflojado la mano a la función.
	for _, r := range raros {
		if fleet.OrigenValido(r) != fleet.OrigenDesconocido {
			t.Errorf("OrigenValido(%q) no es desconocido: la comparación de arriba no probó la regla", r)
		}
	}
}

// UNA FILA CON UN ORIGEN RARO SE LEE DESCONOCIDO POR CADA PUERTA DE LECTURA.
//
// La guarda vieja probaba la FUNCIÓN, y la normalización al escribir tapaba la de leer: la auditoría
// A131 (C3-m9) sacó la de escanearComando y fleet, memory y mcp quedaron verdes. Una fila escrita a
// mano, por una versión futura o por una migración, con `origen = 'cron'`, llegaba así a la
// bitácora, a la cronología y al agente. Acá la fila se siembra con SQL directo —la puerta de
// escritura no corre, así que no puede tapar nada— y se lee por las CINCO puertas que convierten una
// fila de device_commands en un comando: las cinco que llaman a escanearComando.
//
// EXPOSICIÓN medida por la auditoría: cero filas hoy (user_version 57, 12.821 filas, 0 fuera del
// enum). Aparecería por una escritura a mano, por un downgrade después de una versión que escriba una
// categoría nueva, o por una migración que llene la columna.
//
// Sabotaje: sacar la normalización al leer (C3-m9) → el origen raro llega crudo a las cinco puertas.
// arnes: archivo="internal/memory/comandos.go"
// arnes: de="\tc.Origen = fleet.OrigenValido(fleet.OrigenComando(origen))"
// arnes: a="\tc.Origen = fleet.OrigenComando(origen)"
func TestUnOrigenRaroEnLaTablaSeLeeDesconocidoPorCadaPuerta(t *testing.T) {
	e := newTestEngine(t)
	ahora := time.Now().UTC()

	// sembrar deja en `device` una fila por origen, con el origen escrito CRUDO por SQL, y devuelve
	// qué origen tiene que leerse de cada una.
	sembrar := func(device fleet.Device) map[string]fleet.OrigenComando {
		t.Helper()
		quiero := map[string]fleet.OrigenComando{}
		for _, o := range append(append([]fleet.OrigenComando{}, origenesDelEnum...), origenesRaros()...) {
			c, err := e.EncolarComando(fleet.Comando{
				DeviceID: device.ID, ProjectID: device.ProjectID, Principal: "gio", Creado: ahora.Add(-time.Minute),
				Argv: []string{"uptime"}, Timeout: 30 * time.Second, Origen: fleet.OrigenPersona,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := e.db.Exec(`UPDATE device_commands SET origen = ? WHERE id = ?`, string(o), c.ID); err != nil {
				t.Fatal(err)
			}
			quiero[c.ID] = fleet.OrigenValido(o)
		}
		return quiero
	}
	revisar := func(puerta string, quiero map[string]fleet.OrigenComando, leidos map[string]fleet.OrigenComando) {
		t.Helper()
		for id, o := range quiero {
			got, ok := leidos[id]
			if !ok {
				t.Errorf("%s no devolvió la fila %s: la prueba no midió esa puerta", puerta, id)
				continue
			}
			if got != o {
				t.Errorf("%s leyó origen %q y tenía que leer %q: lo que no está en el enum llega como una "+
					"categoría que ninguna superficie sabe dibujar", puerta, got, o)
			}
		}
	}

	// Las tres puertas que sólo leen comparten máquina.
	lectura, _ := altaDePrueba(t, e, "casa", "pc-lectura")
	quiero := sembrar(lectura)
	porID := map[string]fleet.OrigenComando{}
	for id := range quiero {
		c, ok, err := e.ComandoPorID(id)
		if err != nil || !ok {
			t.Fatalf("ComandoPorID(%s): ok=%v err=%v", id, ok, err)
		}
		porID[id] = c.Origen
	}
	revisar("ComandoPorID", quiero, porID)

	bitacora, err := e.BitacoraDeComandos("casa", lectura.ID, 200)
	if err != nil {
		t.Fatal(err)
	}
	enBitacora := map[string]fleet.OrigenComando{}
	for _, c := range bitacora {
		enBitacora[c.ID] = c.Origen
	}
	revisar("BitacoraDeComandos", quiero, enBitacora)

	hechos, _, err := e.CronologiaDeDevice("casa", lectura.ID, fleet.VentanaHasta(ahora, time.Hour), 300, ahora)
	if err != nil {
		t.Fatal(err)
	}
	enCronologia := map[string]fleet.OrigenComando{}
	for _, h := range hechos {
		enCronologia[h.Referencia] = h.Origen
	}
	revisar("CronologiaDeDevice", quiero, enCronologia)

	// Las dos que ENTREGAN cambian el estado de lo que tocan: cada una con su máquina.
	tomas := []struct {
		nombre string
		tomar  func(id string) ([]fleet.Comando, error)
	}{
		{"TomarComandos", func(id string) ([]fleet.Comando, error) { return e.TomarComandos(id, ahora, fleet.ColaMaxPorDevice) }},
		{"LatirYTomarComandos", func(id string) ([]fleet.Comando, error) {
			_, cs, err := e.LatirYTomarComandos(id, ahora, "", fleet.ColaMaxPorDevice)
			return cs, err
		}},
	}
	for i, toma := range tomas {
		d, _ := altaDePrueba(t, e, "casa", fmt.Sprintf("pc-toma-%d", i))
		quiero := sembrar(d)
		entregados, err := toma.tomar(d.ID)
		if err != nil {
			t.Fatalf("%s: %v", toma.nombre, err)
		}
		enToma := map[string]fleet.OrigenComando{}
		for _, c := range entregados {
			enToma[c.ID] = c.Origen
		}
		revisar(toma.nombre, quiero, enToma)
	}
}
