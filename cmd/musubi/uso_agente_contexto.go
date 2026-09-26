package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"musubi/internal/transcripts"
)

// uso_agente_contexto.go implementa `musubi uso-agente --contexto`: el mismo lector de SÓLO
// LECTURA que `uso-agente`, pero la pregunta es otra. No «¿el agente usó Musubi?» sino «lo que
// Musubi le puso en el contexto, ¿se repite, se pierde o descarrila?».
//
// ES EL INSTRUMENTO DEL FRENTE ARRANQUE DE LA OLA 2, y va antes que cualquier arreglo para tener
// el antes y el después con la MISMA regla. Hasta hoy esos números salían de un python suelto
// sobre los transcripts (linea_base.py); un número que sólo sabe sacar un script de una sesión no
// se puede volver a medir dentro de dos semanas.
//
// QUÉ MIDE, sólo en las sesiones PRINCIPALES (las hijas —subagentes y workflows— se cuentan y no
// se miden: lo que se quiere saber es qué ve la terminal del dueño):
//
//   - M1: turnos de CONTINUACIÓN («sigue», «go», «si hazlo») que recibieron «memoria relevante».
//     El recall del turno usa el prompt como consulta, y con «sigue» trae memoria al azar. Va con
//     DOS clasificadores: la lista de transcripts.EsPedidoDeContinuacion, que es la que va a usar
//     la compuerta del hook, y el largo (≤3 palabras), que no depende de ella: medida sólo con la
//     lista, la compuerta se mediría a sí misma.
//   - M1s: avisos del sistema (<task-notification>, <command-…>) que recibieron memoria: el hook
//     los trata como un pedido y busca con el texto del aviso.
//   - M2: compactaciones seguidas de un bloque de Musubi del SessionStart antes del próximo pedido.
//     Hoy el hook de arranque sólo escucha «startup», así que tras compactar Musubi calla.
//   - M3: ids de memoria repetidos DENTRO de una misma ventana de contexto: ya estaban a la vista.
//   - M5: ids de una ventana ANTERIOR que volvieron después de compactar. El resumen de la
//     compactación no guarda la memoria de Musubi, y el delta de la sesión la sigue dando por
//     mostrada: un cero acá es memoria perdida, no memoria ahorrada.
//   - M7: eco. Bloques de Musubi después de una compactación que repiten un pedido textual del
//     dueño. El resumen de Claude Code ya trae «All user messages»: repetirlos es ruido nuevo. La
//     línea con que el corrector de tipeo avisa lo que corrigió (transcripts.PrefijoDeCorreccion)
//     repite un pedazo del prompt a propósito, y se saltea.
//   - Las reanudaciones, por los tramos que Claude Code reescribe al reanudar. Los forks no dejan
//     rastro en el transcript: salen «sin medir», nunca 0.
//
// SIN TRANSCRIPTS EN LA VENTANA EL ALCANCE SALE «SIN MEDIR» Y SIN UNA SOLA CLAVE NUMÉRICA, como en
// `uso-agente`: un cero que significa «no sé» se lee igual que «medí y no hubo».

// Topes del detector de eco (M7). Un pedido de menos de minimoDeEco runas no se busca: «deploy
// central» puede aparecer en una nota por su cuenta, y contarlo sería eco inventado. De los que sí,
// se buscan las primeras largoDeEco runas, con el texto en minúsculas y los espacios colapsados: un
// bloque que repitiera el pedido truncado o redactado igual lo empieza igual.
const (
	minimoDeEco = 20
	largoDeEco  = 40
)

// motivoForkSinMedir es por qué los forks no tienen número.
const motivoForkSinMedir = "sin medir: un fork no deja rastro en el transcript (ni el source del SessionStart ni uuids compartidos entre archivos)"

// ConteoDeTurnos es cuántos turnos de un tipo hubo y cuántos recibieron «memoria relevante».
type ConteoDeTurnos struct {
	Turnos     int `json:"turnos"`
	ConMemoria int `json:"con_memoria_relevante"`
	IDs        int `json:"ids_de_memoria"`
}

// MedidaM1 separa los turnos humanos por los dos clasificadores de continuación, con los
// sustantivos (según la lista) como referencia.
type MedidaM1 struct {
	PorLista    ConteoDeTurnos `json:"por_lista"`
	PorLargo    ConteoDeTurnos `json:"por_largo_hasta_3_palabras"`
	Sustantivos ConteoDeTurnos `json:"sustantivos_referencia"`
}

// MedidaM2 cuenta las compactaciones y cuántas siguió un bloque de Musubi del SessionStart.
type MedidaM2 struct {
	Compactaciones int `json:"compactaciones"`
	ConArranque    int `json:"seguidas_de_arranque_de_musubi"`
}

// MedidaM3 cuenta los ids inyectados y cuántos ya estaban en la ventana.
type MedidaM3 struct {
	IDs       int `json:"ids_inyectados"`
	Repetidos int `json:"repetidos_en_la_ventana"`
}

// MedidaM5 cuenta, en las ventanas que abre una compactación, los ids distintos inyectados y
// cuántos ya se habían inyectado en una ventana anterior de la misma sesión.
type MedidaM5 struct {
	IDs                    int `json:"ids_tras_compactar"`
	Reinyectados           int `json:"reinyectados_de_ventanas_anteriores"`
	VentanasConReinyeccion int `json:"compactaciones_con_reinyeccion"`
}

// MedidaM7 cuenta los bloques de Musubi posteriores a una compactación y cuántos repiten un pedido.
type MedidaM7 struct {
	Bloques int `json:"bloques_tras_compactar"`
	ConEco  int `json:"con_eco_de_un_pedido"`
}

// Reanudaciones es lo que el transcript deja ver de /resume y de un fork.
type Reanudaciones struct {
	Resume int    `json:"resume"`
	Fork   string `json:"fork"`
}

// MedicionContexto son los números de un alcance medido.
type MedicionContexto struct {
	Transcripts      int            `json:"transcripts"`
	PromptsPorOrigen map[string]int `json:"prompts_por_origen"`

	M1            MedidaM1       `json:"m1_continuacion_con_memoria"`
	M1s           ConteoDeTurnos `json:"m1s_avisos_del_sistema_con_memoria"`
	M2            MedidaM2       `json:"m2_compactacion_con_arranque"`
	M3            MedidaM3       `json:"m3_repetidos_en_la_ventana"`
	M5            MedidaM5       `json:"m5_reinyectados_tras_compactar"`
	M7            MedidaM7       `json:"m7_eco_tras_compactar"`
	Reanudaciones Reanudaciones  `json:"reanudaciones"`
}

// AlcanceContexto es la medición de las sesiones principales. El puntero nil es «sin medir»: ver
// AlcanceUso, que tiene el mismo porqué.
type AlcanceContexto struct {
	Estado string `json:"estado"`
	Motivo string `json:"motivo,omitempty"`
	*MedicionContexto
}

// InformeContexto es lo que `--contexto` mide, entero, y la forma del `--json`.
type InformeContexto struct {
	Dir                 string   `json:"dir"`
	Desde               string   `json:"desde,omitempty"`
	Hasta               string   `json:"hasta,omitempty"`
	Archivos            int      `json:"archivos_jsonl"`
	JournalExcluidos    int      `json:"journal_excluidos"`
	SalteadosPorFecha   int      `json:"salteados_por_fecha"`
	LineasIlegibles     int      `json:"lineas_ilegibles"`
	Exclusiones         []string `json:"exclusiones"`
	CarpetasExcluidas   int      `json:"carpetas_excluidas"`
	SubagentesNoMedidos int      `json:"subagentes_no_medidos"`

	Principal AlcanceContexto `json:"principal"`
}

// alcanceContexto decide si el alcance se midió. Es el ÚNICO lugar que lo decide.
func alcanceContexto(m *MedicionContexto) AlcanceContexto {
	if m.Transcripts == 0 {
		return AlcanceContexto{Estado: estadoSinMedir,
			Motivo: "ningún transcript de sesión principal tiene turnos en la ventana"}
	}
	return AlcanceContexto{Estado: estadoMedido, MedicionContexto: m}
}

// medirContexto camina `dir`, lee cada transcript de sesión principal por turnos y arma el
// informe. No escribe nada.
func medirContexto(dir string, v transcripts.Ventana, exclusiones []string) (InformeContexto, error) {
	inf := InformeContexto{Dir: dir, Exclusiones: exclusiones, Desde: v.Desde(), Hasta: v.Hasta()}
	if inf.Exclusiones == nil {
		inf.Exclusiones = []string{}
	}
	m := &MedicionContexto{PromptsPorOrigen: map[string]int{},
		Reanudaciones: Reanudaciones{Fork: motivoForkSinMedir}}
	rec, err := transcripts.Recorrer(dir, v, exclusiones, func(a transcripts.Archivo) error {
		if a.Subagente {
			inf.SubagentesNoMedidos++
			return nil
		}
		s, lerr := transcripts.LeerSesion(a.Ruta)
		inf.LineasIlegibles += s.Ilegibles
		if lerr != nil {
			return lerr
		}
		if medirSesionContexto(s, v, m) {
			m.Transcripts++
		}
		return nil
	})
	inf.Archivos, inf.JournalExcluidos = rec.Archivos, rec.JournalExcluidos
	inf.SalteadosPorFecha, inf.CarpetasExcluidas = rec.SalteadosPorFecha, rec.CarpetasExcluidas
	if err != nil {
		return inf, err
	}
	inf.Principal = alcanceContexto(m)
	return inf, nil
}

// medirSesionContexto suma una sesión a la medición y dice si tuvo algún turno en la ventana.
//
// LO DE ANTES DE LA VENTANA NO CUENTA PERO ALIMENTA EL ESTADO: los ids que ya estaban en la
// ventana de contexto y los pedidos que el dueño ya había escrito siguen ahí aunque sean de ayer.
func medirSesionContexto(s transcripts.Sesion, v transcripts.Ventana, m *MedicionContexto) bool {
	enVentana := false
	ventana := -1
	enLaVentana := map[string]bool{} // ids inyectados en la ventana de contexto en curso
	antes := map[string]bool{}       // ids inyectados en ventanas anteriores de la sesión
	var pedidos []string             // comienzos de los pedidos sustantivos, para el eco
	conReinyeccion := map[int]bool{}
	esperandoArranque := false

	for _, t := range s.Turnos {
		cuenta, despues := v.Ubicar(t.Timestamp)
		if despues {
			break
		}
		if cuenta {
			enVentana = true
		}
		if t.Ventana != ventana {
			// Lo que se inyectó en la ventana que se cierra pasa a «antes»: el resumen lo tapó.
			for id := range enLaVentana {
				antes[id] = true
			}
			enLaVentana = map[string]bool{}
			ventana = t.Ventana
		}
		if t.Compactacion && cuenta {
			m.M2.Compactaciones++
			esperandoArranque = true
		}
		if t.Origen == transcripts.OrigenHumano {
			esperandoArranque = false // llegó el pedido siguiente y el arranque no habló
		}

		memoria, idsMemoria := false, 0
		for _, in := range t.Inyecciones {
			if !in.DeMusubi() || (in.Evento != transcripts.EventoTurno && in.Evento != transcripts.EventoArranque) {
				continue
			}
			if in.Evento == transcripts.EventoArranque && esperandoArranque {
				m.M2.ConArranque++
				esperandoArranque = false
			}
			ids := transcripts.IDsDeMemoria(in.Texto)
			if in.Evento == transcripts.EventoTurno && in.Tiene("memoria relevante") {
				memoria = true
				idsMemoria += len(ids)
			}
			for _, id := range ids {
				if cuenta {
					m.M3.IDs++
					if enLaVentana[id] {
						m.M3.Repetidos++
					}
				}
				if !enLaVentana[id] && t.Ventana > 0 && cuenta {
					m.M5.IDs++
					if antes[id] {
						m.M5.Reinyectados++
						conReinyeccion[t.Ventana] = true
					}
				}
				enLaVentana[id] = true
			}
			if t.Ventana > 0 && cuenta {
				m.M7.Bloques++
				if repiteUnPedido(in.Texto, pedidos) {
					m.M7.ConEco++
				}
			}
		}

		if cuenta && t.Origen != transcripts.OrigenApertura {
			m.PromptsPorOrigen[string(t.Origen)]++
		}
		if cuenta {
			sumarTurno := func(c *ConteoDeTurnos) {
				c.Turnos++
				if memoria {
					c.ConMemoria++
					c.IDs += idsMemoria
				}
			}
			switch t.Origen {
			case transcripts.OrigenHumano:
				if transcripts.EsPedidoDeContinuacion(t.Prompt) {
					sumarTurno(&m.M1.PorLista)
				} else {
					sumarTurno(&m.M1.Sustantivos)
				}
				if esPromptCorto(t.Prompt) {
					sumarTurno(&m.M1.PorLargo)
				}
			case transcripts.OrigenSistema:
				sumarTurno(&m.M1s)
			}
		}
		// El pedido entra DESPUÉS de mirar sus propias inyecciones: el eco es repetir un pedido
		// VIEJO, y el bloque de este mismo turno puede nombrar lo que se acaba de pedir.
		if t.Origen == transcripts.OrigenHumano && !transcripts.EsPedidoDeContinuacion(t.Prompt) {
			if p := []rune(normalizarParaEco(t.Prompt)); len(p) >= minimoDeEco {
				if len(p) > largoDeEco {
					p = p[:largoDeEco]
				}
				pedidos = append(pedidos, string(p))
			}
		}
	}
	m.M5.VentanasConReinyeccion += len(conReinyeccion)
	for _, ts := range s.Reanudaciones {
		if cuenta, _ := v.Ubicar(ts); cuenta {
			m.Reanudaciones.Resume++
		}
	}
	return enVentana
}

// esPromptCorto es el clasificador de continuación INDEPENDIENTE de la lista: tres palabras o
// menos, separadas por espacios. Un prompt sin texto no lo es.
func esPromptCorto(prompt string) bool {
	n := len(strings.Fields(prompt))
	return n > 0 && n <= 3
}

// normalizarParaEco pasa un texto a minúsculas con los espacios colapsados.
func normalizarParaEco(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// repiteUnPedido dice si un bloque de Musubi repite el comienzo de algún pedido anterior. Las
// líneas de corrección de tipeo no cuentan: ver transcripts.PrefijoDeCorreccion.
func repiteUnPedido(texto string, pedidos []string) bool {
	if len(pedidos) == 0 {
		return false
	}
	lineas := strings.Split(texto, "\n")
	quedan := lineas[:0]
	for _, l := range lineas {
		if strings.HasPrefix(strings.TrimSpace(l), transcripts.PrefijoDeCorreccion) {
			continue
		}
		quedan = append(quedan, l)
	}
	cuerpo := normalizarParaEco(strings.Join(quedan, "\n"))
	for _, p := range pedidos {
		if strings.Contains(cuerpo, p) {
			return true
		}
	}
	return false
}

// usoAgenteContexto es la rama `--contexto` del comando, con los argumentos ya leídos.
func usoAgenteContexto(dir string, v transcripts.Ventana, exclusiones []string, comoJSON bool, out, errOut io.Writer) int {
	inf, err := medirContexto(dir, v, exclusiones)
	if err != nil {
		fmt.Fprintf(errOut, "musubi uso-agente --contexto: %v — sin medir\n", err)
		return 1
	}
	if comoJSON {
		b, err := json.MarshalIndent(inf, "", "  ")
		if err != nil {
			fmt.Fprintln(errOut, "musubi uso-agente --contexto: no pude serializar el informe:", err)
			return 1
		}
		fmt.Fprintln(out, string(b))
		return 0
	}
	imprimirContexto(out, inf)
	return 0
}

// fraccion escribe «n/d (x %)», o «0/0 (sin casos)» cuando no hubo nada que contar: un 0 % sin
// casos se leería como «medí y no pasó».
func fraccion(n, d int) string {
	if d == 0 {
		return fmt.Sprintf("%d/%d (sin casos)", n, d)
	}
	return fmt.Sprintf("%d/%d (%.1f %%)", n, d, 100*float64(n)/float64(d))
}

func imprimirContexto(w io.Writer, inf InformeContexto) {
	fmt.Fprintln(w, "Contexto que Musubi le dio al agente — ¿se repite, se pierde o descarrila? (transcripts de Claude Code)")
	fmt.Fprintf(w, "  carpeta  : %s\n", inf.Dir)
	desde, hasta := inf.Desde, inf.Hasta
	if desde == "" {
		desde = "el principio"
	}
	if hasta == "" {
		hasta = "hoy"
	}
	fmt.Fprintf(w, "  ventana  : %s → %s (días UTC, por el timestamp de cada turno)\n", desde, hasta)
	fmt.Fprintf(w, "  archivos : %d .jsonl · %d journal.jsonl excluidos · %d salteados (sin cambios desde antes de la ventana) · %d línea(s) ilegible(s)\n",
		inf.Archivos, inf.JournalExcluidos, inf.SalteadosPorFecha, inf.LineasIlegibles)
	if len(inf.Exclusiones) == 0 {
		fmt.Fprintf(w, "  excluidas: ninguna carpeta (se midió todo lo que hay en la carpeta)\n")
	} else {
		fmt.Fprintf(w, "  excluidas: %d carpeta(s) de proyecto por %s (--incluir-temporales mide las temporales)\n",
			inf.CarpetasExcluidas, strings.Join(inf.Exclusiones, " "))
	}
	fmt.Fprintf(w, "  hijas    : %d transcript(s) de subagentes y workflows, que esta medición no mira\n", inf.SubagentesNoMedidos)

	fmt.Fprintf(w, "\n── Sesiones PRINCIPALES\n")
	a := inf.Principal
	if a.MedicionContexto == nil {
		fmt.Fprintf(w, "  %s — %s\n", a.Estado, a.Motivo)
		return
	}
	m := a.MedicionContexto
	fmt.Fprintf(w, "  transcripts medidos: %d · prompts: %d humanos, %d avisos del sistema, %d internos de Claude Code\n",
		m.Transcripts, m.PromptsPorOrigen[string(transcripts.OrigenHumano)],
		m.PromptsPorOrigen[string(transcripts.OrigenSistema)], m.PromptsPorOrigen[string(transcripts.OrigenInterno)])
	fila := func(etiqueta, valor string) { fmt.Fprintf(w, "  %-62s %s\n", etiqueta, valor) }
	conteo := func(c ConteoDeTurnos) string {
		return fmt.Sprintf("%s · %d ids", fraccion(c.ConMemoria, c.Turnos), c.IDs)
	}
	fila("M1  continuación que recibió «memoria relevante» (meta ≤5 %)", "")
	fila("      por la lista («sigue», «go», «dale»…)", conteo(m.M1.PorLista))
	fila("      por el largo (≤3 palabras)", conteo(m.M1.PorLargo))
	fila("      referencia: pedidos sustantivos", conteo(m.M1.Sustantivos))
	fila("M1s avisos del sistema que recibieron memoria", conteo(m.M1s))
	fila("M2  compactaciones seguidas de un arranque de Musubi (meta ≥95 %)", fraccion(m.M2.ConArranque, m.M2.Compactaciones))
	fila("M3  ids repetidos dentro de una ventana de contexto (meta <1 %)", fraccion(m.M3.Repetidos, m.M3.IDs))
	fila("M5  ids de ventanas anteriores que volvieron tras compactar", fraccion(m.M5.Reinyectados, m.M5.IDs))
	fila("      compactaciones con alguno de vuelta", fraccion(m.M5.VentanasConReinyeccion, m.M2.Compactaciones))
	fila("M7  bloques tras compactar que repiten un pedido (meta 0)", fraccion(m.M7.ConEco, m.M7.Bloques))
	fila("reanudaciones (tramos que Claude Code reescribió al reanudar)", fmt.Sprintf("%d", m.Reanudaciones.Resume))
	fila("forks", m.Reanudaciones.Fork)
}
