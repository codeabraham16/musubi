package mcp

import (
	"strconv"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// TestNingunPlanoLePrometeUnAvisoAQuienNoSabeMostrarlo — LA COLUMNA QUE LE FALTABA A LA MATRIZ.
//
// `TestElEjeDeConsentimientoEsUnaMatrizDeCaminosPorGrados` generaliza sobre los GRADOS y clava el
// otro eje: `maquinaConGradoYToken` pone `puede_preguntar = true` para todas sus filas. Y cubre
// TRES caminos, no cuatro — la política no entra por una tool, así que quedó fuera de esa tabla.
// Las dos cosas juntas son exactamente la forma del defecto que esta guarda cierra.
//
// LO QUE SE MIDIÓ, el 2026-09-21: una política sobre una máquina en `avisa` con
// `puede_preguntar=false` encolaba un `musubi:avisar` igual que con `true`. Los otros tres planos
// lo cazaban con un `case consent.AvisaAlUsuario() && !d.PuedePreguntar:` escrito TRES VECES;
// `aplicarPoliticas` nació sin esa copia. A83 había deducido el ENCOLADO en un embudo y dejado la
// PRECONDICIÓN repartida — el cuarto camino reusó lo deducido y perdió lo copiado.
//
// LA TABLA LLEVA EL CONTROL EN LA MISMA CORRIDA, y no es adorno: una guarda que sólo mira el caso
// `false` daría verde si el aviso dejara de encolarse NUNCA, que es el otro modo de romper este
// eje y se ve idéntico desde afuera.
//
// Sabotaje que la hace fallar: exceptuar al plano de la política en el embudo, que es el defecto
// exacto que existía → sólo su fila se pone roja y las otras tres siguen verdes.
// arnes: archivo="internal/mcp/methods_pantalla.go"
// arnes: de="\tif !d.PuedePreguntar {"
// arnes: a="\tif !d.PuedePreguntar && a.operacion != \"politica\" {"
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// Y EL AVISO QUE SÍ SE ENCOLA DICE EL ORIGEN DE LA ACCIÓN QUE ANUNCIA (A131 · T5)
//
// La fila `true` de cada plano mira ahora también QUIÉN firma el aviso. Estaba escrito a fuego en
// el embudo —`Origen: persona` para los cuatro—, así que el aviso de una política se leía en la
// bitácora como pedido por alguien mientras el comando de al lado decía `automatico: true` (P2-live1,
// un defecto VIVO: bastaba `puede_preguntar=1` y una política que disparara, y `avisa` es el grado
// por defecto). Ninguna guarda lo veía porque en la de la política `puede_preguntar=false` y el
// aviso no se encolaba. La comparación es RELACIONAL —el aviso contra las filas que dejó el mismo
// pedido— y se lee por musubi_fleet_log, la superficie donde el origen se muestra: no clava qué
// origen tiene cada plano, clava que el aviso no contradiga a su acción, en los dos sentidos.
//
// Y LA TABLA RECORRE TODOS LOS PLANOS QUE TIENEN AVISO, leídos del código (planosConAviso): un
// quinto avisoDeAcceso sin su fila acá la pone roja, en vez de nacer sin ninguna de estas dos
// garantías, que es como nació la política (A91).
//
// Exposición medida (auditoría A131): 0 filas en producción. La única política corre sobre una
// máquina con `puede_preguntar=0`; la única máquina que sabe avisar (davantis-1, en `avisa`) no
// está en el alcance de ninguna política.
//
// Sabotaje: que el embudo vuelva a firmar todos los avisos como de una persona (el estado de la base).
//
// Y la otra dirección: pasar el origen por OrigenValido —la misma lista blanca que aplica el
// almacén— es un cambio correcto que esta guarda no puede castigar.
// arnes: archivo="internal/mcp/methods_pantalla.go"
// arnes: de="\t\tOrigen: a.origen,\n"
// arnes: a="\t\tOrigen: fleet.OrigenPersona,\n"
// arnes: arreglo_de="\t\tOrigen: a.origen,\n"
// arnes: arreglo_a="\t\tOrigen: fleet.OrigenValido(a.origen),\n"
//
// Sabotaje: que el aviso de una terminal abierta por una persona se firme como automático.
// arnes: archivo="internal/mcp/methods_pantalla.go"
// arnes: de="\"shell\", fleet.OrigenPersona}"
// arnes: a="\"shell\", fleet.OrigenPolitica}"
func TestNingunPlanoLePrometeUnAvisoAQuienNoSabeMostrarlo(t *testing.T) {
	planos := []string{"pantalla", "shell", "exec", "politica"}
	enLaTabla := map[string]bool{}
	for _, p := range planos {
		enLaTabla[p] = true
	}
	for plano, donde := range planosConAviso(t) {
		if !enLaTabla[plano] {
			t.Errorf("el plano %q le promete un aviso al usuario de la máquina (%s) y esta tabla no lo recorre: "+
				"nadie mide que no se le encole a quien no sabe mostrarlo, ni que diga el origen de lo que anuncia", plano, donde)
		}
		delete(enLaTabla, plano)
	}
	for plano := range enLaTabla {
		t.Errorf("la tabla recorre el plano %q y ningún avisoDeAcceso del paquete lo declara: la fila mide un plano que no existe", plano)
	}
	for _, plano := range planos {
		for _, puede := range []bool{true, false} {
			t.Run(plano+"/puede_preguntar="+strconv.FormatBool(puede), func(t *testing.T) {
				var s *McpServer

				if plano == "politica" {
					var d fleet.Device
					s, d = prepararPolitica(t, politicaDeMemoria(), registroDePrueba(autoHeal()))
					fijarEjeYCapacidad(t, s, d.ID, puede)
					ahora := time.Now()
					latir(t, s, d.ID, muestraSana(95, ahora), ahora) // la condición de la política se cumple
					s.aplicarPoliticas("casa", ahora)
				} else {
					s = newTestServer(t, embedding.NoopProvider{})
					d, p := maquinaConGrado(t, s, fleet.ConsentimientoAvisa)
					fijarEjeYCapacidad(t, s, d.ID, puede)
					switch plano {
					case "pantalla":
						callAsPrincipal(t, s, p, "musubi_fleet_screen", map[string]any{"device": d.Name})
					case "shell":
						callAsPrincipal(t, s, p, "musubi_fleet_shell", map[string]any{"device": d.Name})
					case "exec":
						callAsPrincipal(t, s, p, "musubi_fleet_exec", map[string]any{
							"device": d.Name, "argv": []string{"echo", "hola"}, "no_wait": true})
					}
				}

				avisos := 0
				for _, cmd := range comandosEncolados(t, s) {
					if len(cmd.Argv) > 0 && cmd.Argv[0] == comandoAviso {
						avisos++
					}
				}
				quiero := 0
				if puede {
					quiero = 1
				}
				if avisos != quiero {
					if puede {
						t.Fatalf("el plano %q NO encoló el aviso a una máquina que SÍ sabe mostrarlo (%d avisos): "+
							"sin este control, la fila de abajo daría verde con el eje entero apagado", plano, avisos)
					}
					t.Errorf("el plano %q le encoló %d aviso(s) a una máquina que declara NO saber notificar.\n"+
						"  Prometer una notificación que el agente de esa máquina no sabe dar es lo que este eje\n"+
						"  viene a evitar: una configuración que se ve puesta y no lo está. La precondición vive\n"+
						"  en `encolarAvisoDeAcceso`, que es el ÚNICO sitio del cerebro que encola `OpAvisar`.", plano, avisos)
				}
				if puede {
					exigirQueElAvisoDigaElOrigenDeLaAccion(t, s, plano)
				}
			})
		}
	}
}

// fijarEjeYCapacidad deja la máquina en `avisa` con la capacidad que se le pida.
//
// Va aparte porque los dos caminos de la tabla llegan con la máquina en estados distintos —la de
// la política no pasó por `maquinaConGrado`— y el eje tiene que quedar en el MISMO grado para que
// la comparación entre planos signifique algo.
func fijarEjeYCapacidad(t *testing.T, s *McpServer, deviceID string, puede bool) {
	t.Helper()
	if err := s.engine.FijarCapacidadDePreguntar(deviceID, puede); err != nil {
		t.Fatalf("FijarCapacidadDePreguntar: %v", err)
	}
	if _, err := s.engine.FijarConsentimiento(deviceID, fleet.ConsentimientoAvisa); err != nil {
		t.Fatalf("FijarConsentimiento: %v", err)
	}
	d, _, _ := s.engine.DevicePorID(deviceID)
	if got := d.ConsentimientoEfectivo(); got != fleet.ConsentimientoAvisa {
		t.Fatalf("el grado efectivo quedó en %q y se pidió `avisa`: esta fila no mide lo que dice", got)
	}
	if d.PuedePreguntar != puede {
		t.Fatalf("`puede_preguntar` quedó en %v y se pidió %v", d.PuedePreguntar, puede)
	}
}
