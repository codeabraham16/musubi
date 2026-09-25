package memory

// cronologia_bordes_test.go ata la semiapertura de la ventana a LO QUE DECIDE en producción: las
// consultas. Auditoría A131, tema T8.
//
// La guarda de la semiapertura (fleet.TestLaVentanaEsSemiabierta) prueba fleet.Ventana.Contiene, y
// Contiene NO TIENE NINGÚN LLAMADOR DE PRODUCCIÓN — ni hoy ni en ningún commit anterior. Lo que
// decide qué entra en la línea de tiempo es el `>= ? AND < ?` de cinco consultas escritas a mano:
// tres en cronologia.go (comandos, pantallas, shells) y dos en contexto.go (observaciones y
// código). Ninguna prueba ponía una fila EXACTAMENTE en el `hasta`, así que un `creada <= ?` en la
// de pantallas dejaba una sesión del borde en las DOS ventanas de un mosaico, con fleet, memory y
// mcp en verde (C2-m3).
//
// NO SE EXTRAJO UN HELPER DEL PREDICADO, y es a propósito: cinco directivas del arnés ya anclan en
// el texto literal de esos WHERE, y moverlo las dejaría apuntando al vacío por un beneficio que
// esta prueba ya da — cualquier `<=` en cualquiera de las cinco se pone rojo acá.

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// instantesDeBorde salen de las PUNTAS de la ventana: cada una con su vecino inmediato de cada lado
// a la resolución de la tabla (el segundo), el medio, y uno lejos de cada lado. No hay un lado sin
// mirar, que es por donde se coló C2-m1.
func instantesDeBorde(v fleet.Ventana) []time.Time {
	out := []time.Time{v.Desde.Add(-time.Hour), v.Desde.Add(v.Duracion() / 2), v.Hasta.Add(time.Hour)}
	for _, b := range []time.Time{v.Desde, v.Hasta} {
		out = append(out, b.Add(-time.Second), b, b.Add(time.Second))
	}
	return out
}

// sembrado es una fila puesta en un instante conocido, y cómo reconocerla en la respuesta.
type sembrado struct {
	ref    string
	cuando time.Time
	tipo   string // lo que la fila tiene que volver diciendo que es ("" = no aplica)
}

// sembradorDeHecho pone UNA fila del tipo pedido en `cuando` y devuelve su referencia.
type sembradorDeHecho func(t *testing.T, e *DbEngine, d fleet.Device, cuando time.Time) string

// sembradoresPorTipo tiene una entrada por CADA tipo de hecho, porque CronologiaDeDevice los saca de
// tablas y ramas distintas. Se recorre contra fleet.TiposDeHecho —el enum entero, que es lo que
// cierra el conjunto—, así que un tipo nuevo sin sembrador pone esta prueba en rojo en vez de
// quedar con su borde sin mirar.
func sembradoresPorTipo() map[fleet.TipoDeHecho]sembradorDeHecho {
	comando := func(argv []string, clase fleet.TipoDeHecho) sembradorDeHecho {
		return func(t *testing.T, e *DbEngine, d fleet.Device, cuando time.Time) string {
			t.Helper()
			c, err := e.EncolarComando(fleet.Comando{
				DeviceID: d.ID, ProjectID: "infra", Principal: "gio", Creado: cuando,
				Argv: argv, Timeout: 30 * time.Second, Clasificacion: clase,
			})
			if err != nil {
				t.Fatalf("EncolarComando(%v): %v", argv, err)
			}
			return c.ID
		}
	}
	return map[fleet.TipoDeHecho]sembradorDeHecho{
		fleet.HechoComando:       comando([]string{"systemctl", "restart", "nginx"}, ""),
		fleet.HechoCanalPantalla: comando([]string{fleet.OpPantalla, "ses-1", "secreto"}, ""),
		fleet.HechoCanalShell:    comando([]string{fleet.OpShell, "ses-2"}, ""),
		fleet.HechoCanalExec:     comando([]string{fleet.OpAvisar, "texto"}, fleet.HechoCanalExec),
		fleet.HechoSinClasificar: comando([]string{"musubi:todavia-no-existe", "x"}, ""),
		fleet.HechoPantalla: func(t *testing.T, e *DbEngine, d fleet.Device, cuando time.Time) string {
			t.Helper()
			s, err := e.AbrirSesionPantalla(fleet.SesionPantalla{DeviceID: d.ID, ProjectID: "infra", Principal: "gio", Creada: cuando})
			if err != nil {
				t.Fatalf("AbrirSesionPantalla: %v", err)
			}
			return s.ID
		},
		fleet.HechoShell: func(t *testing.T, e *DbEngine, d fleet.Device, cuando time.Time) string {
			t.Helper()
			s, err := e.AbrirSesionShell(fleet.SesionShell{DeviceID: d.ID, ProjectID: "infra", Principal: "gio", Creada: cuando})
			if err != nil {
				t.Fatalf("AbrirSesionShell: %v", err)
			}
			return s.ID
		},
	}
}

// lectorDeVentana siembra una fila por instante y devuelve lo sembrado y lo que volvió (ref → tipo).
type lectorDeVentana func(t *testing.T, e *DbEngine, v fleet.Ventana, instantes []time.Time) ([]sembrado, map[string]string)

// lectoresDeVentana tiene una entrada por cada método del motor que recibe una fleet.Ventana.
func lectoresDeVentana() map[string]lectorDeVentana {
	return map[string]lectorDeVentana{
		"CronologiaDeDevice": func(t *testing.T, e *DbEngine, v fleet.Ventana, instantes []time.Time) ([]sembrado, map[string]string) {
			d, _ := altaDePrueba(t, e, "infra", "pc-borde")
			sembradores := sembradoresPorTipo()
			for _, tipo := range fleet.TiposDeHecho {
				if sembradores[tipo] == nil {
					t.Errorf("el tipo de hecho %q no tiene sembrador en esta prueba: su borde de ventana no lo mira nadie", tipo)
				}
			}
			var out []sembrado
			for tipo, sembrar := range sembradores {
				for _, cuando := range instantes {
					out = append(out, sembrado{ref: sembrar(t, e, d, cuando), cuando: cuando, tipo: string(tipo)})
				}
			}
			hechos, truncado, err := e.CronologiaDeDevice("infra", d.ID, v, 1000, v.Hasta.Add(48*time.Hour))
			if err != nil {
				t.Fatalf("CronologiaDeDevice: %v", err)
			}
			if truncado {
				t.Fatalf("el tope cortó: la prueba no puede distinguir «afuera de la ventana» de «afuera del tope»")
			}
			volvieron := map[string]string{}
			for _, h := range hechos {
				volvieron[h.Referencia] = string(h.Tipo)
			}
			return out, volvieron
		},
		"ObservacionesEnVentana": func(t *testing.T, e *DbEngine, v fleet.Ventana, instantes []time.Time) ([]sembrado, map[string]string) {
			var out []sembrado
			for i, cuando := range instantes {
				id := fmt.Sprintf("obs-borde-%d", i)
				if err := e.SaveObservationTypedFrom("infra", "", id, "infra/borde",
					fmt.Sprintf("MARCABORDE %d", i), 1.0, "semantic", "local", nil); err != nil {
					t.Fatal(err)
				}
				// LA HORA SE FIJA CON UN UPDATE DIRECTO: la escritura normal pone CURRENT_TIMESTAMP, y
				// un borde que depende de a qué hora corre la prueba no es un borde.
				if _, err := e.db.Exec(`UPDATE observations SET created_at = ? WHERE id = ?`,
					cuando.UTC().Format(formatoDeMemoria), id); err != nil {
					t.Fatal(err)
				}
				out = append(out, sembrado{ref: id, cuando: cuando})
			}
			obs, err := e.ObservacionesEnVentana(context.Background(), v, 1000)
			if err != nil {
				t.Fatalf("ObservacionesEnVentana: %v", err)
			}
			volvieron := map[string]string{}
			for _, o := range obs {
				volvieron[o.ID] = ""
			}
			return out, volvieron
		},
		"CodigoTocadoEnVentana": func(t *testing.T, e *DbEngine, v fleet.Ventana, instantes []time.Time) ([]sembrado, map[string]string) {
			var out []sembrado
			for i, cuando := range instantes {
				path := fmt.Sprintf("infra/borde-%d.go", i)
				if _, err := e.db.Exec(`INSERT INTO code_memory (path, gist, symbols, fingerprint, tokens, project_id, updated_at)
					VALUES (?, 'g', '', 'h', 1, 'infra', ?)`, path, cuando.UTC().Format(formatoDeMemoria)); err != nil {
					t.Fatal(err)
				}
				out = append(out, sembrado{ref: path, cuando: cuando})
			}
			archivos, err := e.CodigoTocadoEnVentana(context.Background(), v, 1000)
			if err != nil {
				t.Fatalf("CodigoTocadoEnVentana: %v", err)
			}
			volvieron := map[string]string{}
			for _, a := range archivos {
				volvieron[a.Path] = ""
			}
			return out, volvieron
		},
	}
}

// TODO LECTOR DE VENTANA DEL MOTOR CORTA EXACTAMENTE DONDE CORTA fleet.Ventana.Contiene, con una
// fila de cada tipo en cada borde.
//
// Contiene es el ORÁCULO: la definición de la semiapertura escrita una sola vez. La prueba no
// repite el predicado a mano —eso sería una copia que envejece al lado de las otras cinco—, sino
// que exige que lo que devuelve la consulta coincida con lo que Contiene dice, fila por fila. Si
// alguien cierra una consulta (`<=`), o si alguien le afloja el borde a Contiene, los dos lados se
// separan y esto se pone rojo.
//
// LOS DOS CONJUNTOS SE CIERRAN CONTRA SU FUENTE, no contra una lista: los lectores son los métodos
// del motor que reciben una fleet.Ventana (por reflexión), y los tipos de hecho son
// fleet.TiposDeHecho, que TestTiposDeHechoEsElEnumEntero (fleet) ata al bloque const. Un lector o
// un tipo nuevo sin fila en estas tablas pone la prueba en rojo.
//
// EXPOSICIÓN medida por la auditoría: el estado roto no existe hoy en producción —4 sesiones de
// pantalla en total, ninguna en minuto, hora ni medianoche redondos— y la ventana default no puede
// dispararlo, porque redondea `hasta` al segundo SIGUIENTE. Lo dispara un `hasta` explícito igual
// al segundo de una fila, que es lo que hace un operador cuando copia el `creada` de una sesión
// para investigar. Prometheus no tiene ninguna serie de la cronología: nadie lo habría visto.
//
// Sabotaje: cerrar el borde superior de la consulta de pantallas (`creada <= ?`) → la sesión del
// borde cae en las dos ventanas del mosaico (C2-m3).
// arnes: archivo="internal/memory/cronologia.go"
// arnes: de="FROM screen_sessions\n\t\t  WHERE project_id = ? AND device_id = ? AND creada >= ? AND creada < ?"
// arnes: a="FROM screen_sessions\n\t\t  WHERE project_id = ? AND device_id = ? AND creada >= ? AND creada <= ?"
// Sabotaje: cerrar el borde superior del código tocado (`updated_at <= ?`) → el archivo del borde
// entra en la ventana que TERMINA ahí.
// arnes: archivo="internal/memory/contexto.go"
// arnes: de="\t\tWHERE updated_at >= ? AND updated_at < ? AND "
// arnes: a="\t\tWHERE updated_at >= ? AND updated_at <= ? AND "
func TestLosLectoresDeVentanaCortanComoContiene(t *testing.T) {
	// El conjunto de lectores, DERIVADO del motor.
	ventanaT := reflect.TypeOf(fleet.Ventana{})
	motor := reflect.TypeOf(&DbEngine{})
	var derivados []string
	for i := 0; i < motor.NumMethod(); i++ {
		m := motor.Method(i)
		for j := 0; j < m.Type.NumIn(); j++ {
			if m.Type.In(j) == ventanaT {
				derivados = append(derivados, m.Name)
				break
			}
		}
	}
	sort.Strings(derivados)
	tabla := lectoresDeVentana()
	esDerivado := map[string]bool{}
	for _, nombre := range derivados {
		esDerivado[nombre] = true
		if tabla[nombre] == nil {
			t.Errorf("%s recibe una fleet.Ventana y esta prueba no lo recorre: su borde no lo mira nadie", nombre)
		}
	}
	// Y AL REVÉS, que es además el piso: si la reflexión no encontrara nada, TODAS las filas de la
	// tabla caerían acá en vez de pasar en verde sin haber medido ningún lector.
	for nombre := range tabla {
		if !esDerivado[nombre] {
			t.Errorf("la tabla recorre %s y el motor no tiene un método con ese nombre que reciba una ventana "+
				"(derivados: [%s]): la fila mide algo que no existe", nombre, strings.Join(derivados, ", "))
		}
	}

	// UNA ventana en segundos enteros —Normalizada no la mueve— de dos horas.
	v := fleet.Ventana{
		Desde: time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC),
		Hasta: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
	}
	instantes := instantesDeBorde(v)

	for _, nombre := range derivados {
		lector := tabla[nombre]
		if lector == nil {
			continue
		}
		t.Run(nombre, func(t *testing.T) {
			e := newTestEngine(t)
			sembrados, volvieron := lector(t, e, v, instantes)
			// EL PISO, por tipo: cada uno tiene que tener filas de los DOS lados del corte. Sin esto, un
			// tipo sembrado sólo afuera «coincidiría» con una consulta que no devuelve nada.
			dentro, fuera := map[string]int{}, map[string]int{}
			for _, s := range sembrados {
				tipo, volvio := volvieron[s.ref]
				quiero := v.Contiene(s.cuando)
				if volvio != quiero {
					consecuencia := "se PIERDE: un mosaico de ventanas no la cuenta en ninguna"
					if volvio {
						consecuencia = "entra también en la ventana vecina: un mosaico de ventanas la cuenta DOS veces"
					}
					t.Errorf("%s: la fila %q de %s (tipo %q) volvió=%v y Contiene dice %v — la consulta y la ventana "+
						"no cortan en el mismo lugar, y la fila %s",
						nombre, s.ref, s.cuando.Format(time.RFC3339), s.tipo, volvio, quiero, consecuencia)
				}
				if volvio && tipo != s.tipo {
					t.Errorf("%s: la fila %q volvió como %q y se sembró como %q: el sembrador no mide el tipo que dice",
						nombre, s.ref, tipo, s.tipo)
				}
				if quiero {
					dentro[s.tipo]++
				} else {
					fuera[s.tipo]++
				}
			}
			for _, s := range sembrados {
				if dentro[s.tipo] == 0 || fuera[s.tipo] == 0 {
					t.Fatalf("%s: el tipo %q tiene %d filas adentro y %d afuera: sin los dos lados, la prueba no mide el corte",
						nombre, s.tipo, dentro[s.tipo], fuera[s.tipo])
				}
			}
			if len(sembrados) == 0 {
				t.Fatalf("%s: no se sembró nada: la prueba pasaría sin leer nada", nombre)
			}
		})
	}
}
