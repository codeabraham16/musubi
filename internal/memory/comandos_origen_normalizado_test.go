package memory

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"musubi/internal/fleet"
	"musubi/internal/fleet/fleettest"
)

// origenesDelEnum son los orígenes que OrigenValido conserva: el enum entero, tal como lo exporta
// fleet.OrigenesDeComando. Son el control positivo de las dos pruebas de abajo. Acá NO se copian:
// hasta la revisión de A131 (tema T9) era una lista escrita a mano, y un origen nuevo se habría
// decidido en fleet sin que las puertas de escritura y de lectura lo recorrieran. La decisión sobre
// el enum —y el cierre de la lista contra el bloque const— vive en internal/fleet
// (TestElOrigenEsUnaListaBlancaCerradaContraSuEnum).
func origenesDelEnum(t *testing.T) []fleet.OrigenComando {
	t.Helper()
	// EL PISO: una lista recortada dejaría a las dos pruebas sin recorrer lo que falta, en verde.
	if len(fleet.OrigenesDeComando) < 3 {
		t.Fatalf("fleet.OrigenesDeComando trae %d orígenes (%q) y cuando se escribió esta prueba eran 3: "+
			"las puertas de escritura y de lectura quedan sin medir para los que faltan",
			len(fleet.OrigenesDeComando), fleet.OrigenesDeComando)
	}
	return fleet.OrigenesDeComando
}

// origenesRaros son los valores que se le parecen al enum sin serlo, y dos que no se parecen a nada.
// Los deriva fleettest.OrigenesParecidos, la misma fuente que usa la prueba del dominio
// (internal/fleet): hasta la revisión de T9 cada una tenía su copia de la derivación.
func origenesRaros(t *testing.T) []fleet.OrigenComando {
	t.Helper()
	var valores []string
	for _, o := range origenesDelEnum(t) {
		valores = append(valores, string(o))
	}
	var out []fleet.OrigenComando
	for _, r := range fleettest.OrigenesParecidos(valores) {
		out = append(out, fleet.OrigenComando(r))
	}
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
	raros := origenesRaros(t)
	// EL PISO: sin raros, esto sólo mediría el camino feliz.
	if len(raros) < 10 {
		t.Fatalf("sólo %d orígenes raros (%q): la derivación se rompió", len(raros), raros)
	}
	for _, o := range append(append([]fleet.OrigenComando{}, origenesDelEnum(t)...), raros...) {
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
// escritura no corre, así que no puede tapar nada— y se lee por cada puerta que convierte una fila
// de device_commands en un comando.
//
// LAS PUERTAS SALEN DEL FUENTE (puertasDeLecturaDeComandos): toda función exportada del paquete que
// llega a escanearComando. Hasta la segunda revisión de T9 eran cinco escritas a mano; ahora la tabla
// de abajo dice sólo CÓMO se lee cada una, y una puerta nueva sin su lectura pone esto en rojo. Cada
// puerta lee su propia máquina: las que entregan cambian el estado de lo que tocan, y compartir
// filas haría que el orden de la tabla decidiera qué ve cada una.
//
// EXPOSICIÓN medida por la auditoría: cero filas hoy (user_version 57, 12.821 filas, 0 fuera del
// enum). Aparecería por una escritura a mano, por un downgrade después de una versión que escriba una
// categoría nueva, o por una migración que llene la columna.
//
// Sabotaje: sacar la normalización al leer (C3-m9) → el origen raro llega crudo a todas las puertas.
// arnes: archivo="internal/memory/comandos.go"
// arnes: de="\tc.Origen = fleet.OrigenValido(fleet.OrigenComando(origen))"
// arnes: a="\tc.Origen = fleet.OrigenComando(origen)"
// Sabotaje: una puerta de lectura nueva (otra consulta que convierte filas en comandos) → nadie la
// recorre con un origen raro, y con la lista a mano esto seguía verde.
// arnes: archivo="internal/memory/comandos.go"
// arnes: de="// ComandoPorID devuelve un comando. Lo usa la espera acotada de musubi_fleet_exec.\n"
// arnes: a="// UltimoComando devuelve el último comando encolado para una máquina.\nfunc (e *DbEngine) UltimoComando(deviceID string) (fleet.Comando, error) {\n\treturn escanearComando(e.db.QueryRow(`SELECT `+columnasComando+` FROM device_commands WHERE device_id = ? ORDER BY creado DESC LIMIT 1`, deviceID))\n}\n\n// ComandoPorID devuelve un comando. Lo usa la espera acotada de musubi_fleet_exec.\n"
func TestUnOrigenRaroEnLaTablaSeLeeDesconocidoPorCadaPuerta(t *testing.T) {
	e := newTestEngine(t)
	ahora := time.Now().UTC()

	// sembrar deja en `device` una fila por origen, con el origen escrito CRUDO por SQL, y devuelve
	// qué origen tiene que leerse de cada una.
	sembrar := func(device fleet.Device) map[string]fleet.OrigenComando {
		t.Helper()
		quiero := map[string]fleet.OrigenComando{}
		for _, o := range append(append([]fleet.OrigenComando{}, origenesDelEnum(t)...), origenesRaros(t)...) {
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

	// CÓMO se lee cada puerta, dada su máquina sembrada: qué origen devolvió por id. QUÉ puertas hay
	// no lo decide esta tabla.
	type lectura func(d fleet.Device, ids []string) (map[string]fleet.OrigenComando, error)
	comandos := func(cs []fleet.Comando, err error) (map[string]fleet.OrigenComando, error) {
		out := map[string]fleet.OrigenComando{}
		for _, c := range cs {
			out[c.ID] = c.Origen
		}
		return out, err
	}
	leerPor := map[string]lectura{
		"DbEngine.ComandoPorID": func(_ fleet.Device, ids []string) (map[string]fleet.OrigenComando, error) {
			out := map[string]fleet.OrigenComando{}
			for _, id := range ids {
				c, ok, err := e.ComandoPorID(id)
				if err != nil {
					return nil, err
				}
				if ok {
					out[id] = c.Origen
				}
			}
			return out, nil
		},
		"DbEngine.BitacoraDeComandos": func(d fleet.Device, _ []string) (map[string]fleet.OrigenComando, error) {
			return comandos(e.BitacoraDeComandos(d.ProjectID, d.ID, 200))
		},
		"DbEngine.CronologiaDeDevice": func(d fleet.Device, _ []string) (map[string]fleet.OrigenComando, error) {
			hechos, _, err := e.CronologiaDeDevice(d.ProjectID, d.ID, fleet.VentanaHasta(ahora, time.Hour), 300, ahora)
			out := map[string]fleet.OrigenComando{}
			for _, h := range hechos {
				out[h.Referencia] = h.Origen
			}
			return out, err
		},
		"DbEngine.TomarComandos": func(d fleet.Device, _ []string) (map[string]fleet.OrigenComando, error) {
			return comandos(e.TomarComandos(d.ID, ahora, fleet.ColaMaxPorDevice))
		},
		"DbEngine.LatirYTomarComandos": func(d fleet.Device, _ []string) (map[string]fleet.OrigenComando, error) {
			_, cs, err := e.LatirYTomarComandos(d.ID, ahora, "", fleet.ColaMaxPorDevice)
			return comandos(cs, err)
		},
	}

	puertas := puertasDeLecturaDeComandos(t)
	nombres := make([]string, 0, len(puertas))
	for p := range puertas {
		nombres = append(nombres, p)
	}
	sort.Strings(nombres)
	for i, p := range nombres {
		leer, ok := leerPor[p]
		if !ok {
			t.Errorf("%s convierte filas de device_commands en comandos (%s) y esta prueba no sabe leerla: "+
				"sumale cómo en leerPor, o un origen raro puede llegar crudo por ahí sin que nadie lo mida",
				p, strings.Join(puertas[p], " → "))
			continue
		}
		d, _ := altaDePrueba(t, e, "casa", fmt.Sprintf("pc-puerta-%d", i))
		quiero := sembrar(d)
		ids := make([]string, 0, len(quiero))
		for id := range quiero {
			ids = append(ids, id)
		}
		leidos, err := leer(d, ids)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		revisar(p, quiero, leidos)
	}
	for p := range leerPor {
		if _, ok := puertas[p]; !ok {
			t.Errorf("la prueba sabe leer %s y el fuente dice que ya no llega a escanearComando: o dejó de ser "+
				"una puerta de lectura, o cambió de nombre y la nueva quedó sin medir", p)
		}
	}
}
