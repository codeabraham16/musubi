package fleet

// muestra.go es QUÉ SE MIDE de una máquina. Dominio puro: describe la medición y su
// serialización, y no sabe leerla de ningún sistema operativo — eso es colector_*.go, detrás de
// build tags.
//
// EL PRINCIPIO QUE GOBIERNA ESTE ARCHIVO: un dato que no se pudo medir viaja como `null`, nunca
// como cero. Un cero inventado es indistinguible de un cero medido, y la diferencia importa
// justo cuando alguien está mirando el panel para entender por qué algo se cayó. Es el mismo
// criterio que ya usa `last_seen: null` para una máquina que nunca latió.

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Muestra es una fotografía del estado de un host.
//
// Los punteros NO son una optimización: son el vocabulario del «no sé». CPUPct es nil en la
// primera muestra —el porcentaje es una derivada y hace falta una lectura anterior— y TempC es
// nil en una máquina sin sensor térmico. Un float64 a secas no sabe decir eso.
// REGLA DE LOS PARES, y hay que respetarla al escribir un colector nuevo: los campos que van en
// pareja (total + usado) SE FIJAN JUNTOS O NO SE FIJA NINGUNO.
//
// No tienen puntero porque el par ya sabe decir «no sé»: PctUsado devuelve nil cuando el total es
// cero, así que dejar los dos en cero significa «esta máquina no reporta memoria». Pero fijar el
// TOTAL sin el USADO produce un 0 % que se lee como «vacío», que es justo la mentira que el resto
// del diseño evita. Pasa de verdad: macOS da hw.memsize por sysctl y no da la memoria usada sin
// mach, así que la tentación de reportar sólo el total es concreta.
type Muestra struct {
	Tomada time.Time `json:"tomada"`

	// CPUPct es el uso promedio del INTERVALO entre esta muestra y la anterior (0-100).
	// nil = desconocido: es la primera lectura y no hay contra qué restar.
	CPUPct *float64 `json:"cpu_pct"`
	NumCPU int      `json:"num_cpu"`

	MemTotal uint64 `json:"mem_total"`
	MemUsada uint64 `json:"mem_usada"`
	// MemLibre es MemFree: la RAM que el kernel no tiene asignada a NADA, ni siquiera a page
	// cache. NO es la contracara de MemUsada —ésa sale de MemAvailable— y las dos difieren en
	// varios GB en cualquier Linux sano: con el fixture real, 1,1 GB contra 3,0 GB. Viaja como
	// puntero porque Windows no la puede dar sin mentir (ullAvailPhys es el análogo de
	// MemAvailable, no de MemFree) y macOS no la puede dar sin mach: nil = «este sistema no la
	// expone», que es distinto de «no le queda nada libre».
	MemLibre  *uint64 `json:"mem_libre"`
	SwapTotal uint64  `json:"swap_total"`
	SwapUsada uint64  `json:"swap_usada"`

	// Disco es el sistema de archivos RAÍZ. No se enumeran todos los puntos de montaje: en una
	// máquina con contenedores son decenas, casi todos irrelevantes, y el agregado del host es
	// la pregunta que se hace primero. El detalle, cuando haya una pregunta que lo pida.
	//
	// SON TRES NÚMEROS Y NO DOS, por la misma razón que `df` muestra tres columnas:
	// **Usado + Disponible ≠ Total**. El kernel reserva ~5 % del filesystem para root, y esa
	// reserva no es ni una cosa ni la otra. Medido en esta máquina: 25,6 GB sobre 502 GB.
	//
	// Cuál mirar depende de la pregunta, y por eso viajan las dos:
	//   - `DiscoUsado` es cuánto ocupan los archivos. Es el número que un operador va a
	//     contrastar contra `df`, y tiene que coincidir o nadie confía en el panel.
	//   - `DiscoDisponible` es cuánto puede escribir todavía una aplicación. Es el número de la
	//     ALERTA: un disco al 95 % con 5 % reservado ya no acepta una escritura más, y el que
	//     avisa de eso es éste.
	DiscoTotal      uint64 `json:"disco_total"`
	DiscoUsado      uint64 `json:"disco_usado"`
	DiscoDisponible uint64 `json:"disco_disponible"`

	// La carga es un concepto de UNIX. Windows no tiene load average —no es que valga cero,
	// es que no existe—, así que estos campos son punteros por la misma razón que CPUPct:
	// un float64 a secas reportaría 0.00 en cada máquina Windows de la flota, indistinguible
	// de una máquina ociosa.
	Load1     *float64 `json:"load1"`
	Load5     *float64 `json:"load5"`
	Load15    *float64 `json:"load15"`
	UptimeSeg uint64   `json:"uptime_seg"`

	// TempC es la primera zona térmica. nil = la máquina no expone ninguna (D3).
	TempC *float64 `json:"temp_c"`

	// NumProcesos son los PROCESOS, no los hilos. La diferencia no es cosmética: el 4º campo de
	// /proc/loadavg («5/1181») cuenta HILOS y da entre 3 y 5 veces más, así que el atajo que está
	// a mano produce un número creíble y equivocado.
	//
	// Es `int` a secas y no puntero por la misma razón que NumCPU: una máquina encendida siempre
	// tiene al menos un proceso, así que el 0 no es ambiguo — significa «no medido», y los
	// consumidores lo traducen a null antes de mostrarlo.
	NumProcesos int `json:"num_procesos"`

	// Alcance son las sondas de alcanzabilidad desde ESTA máquina (A67): «¿llego a ese puerto?».
	//
	// Viaja DENTRO de la muestra y no en una tabla propia porque es exactamente eso: una medición
	// que la máquina toma de su entorno, con la misma frecuencia y el mismo dueño que la CPU o el
	// disco. Y `last_sample` es una columna de TEXTO con este JSON, así que no cuesta esquema.
	//
	// `omitempty` NO es cosmético: una máquina sin destinos configurados no reporta sondas, y eso
	// significa «nadie le pidió que mirara», no «no llega». Una muestra vieja —anterior a este
	// campo— se lee igual, con la lista vacía. Ausente no es falso.
	Alcance []SondaDeAlcance `json:"alcance,omitempty"`
}

// Colector lee el estado del host. Es un seam por sistema operativo: la implementación real vive
// en colector_linux.go y el resto de los OS reciben un stub que FALLA en vez de mentir (D4).
//
// Tomar guarda estado entre llamadas (la lectura anterior de CPU), así que un Colector es de un
// solo dueño y no es seguro compartirlo entre goroutines.
type Colector interface {
	Tomar() (Muestra, error)
}

// ErrSinColector lo devuelve el stub de los OS todavía no soportados. Es un error y no una
// muestra de ceros a propósito (D4): un panel que pinta 0 % de CPU en todos los Windows se cree
// y no se arregla; uno que dice «esta máquina no reporta métricas» se arregla.
var ErrSinColector = fmt.Errorf("todavía no hay colector de métricas para este sistema operativo")

// MuestraMaxBytes es el tope del JSON de una muestra en el cable (D6).
//
// Una Muestra serializada son ~300 bytes. 4 KiB deja lugar de sobra para crecer y sigue siendo
// ridículamente chico comparado con el tope general del transporte (4 MiB). El tope existe
// porque el agente corre en la superficie MÁS expuesta de la flota: un cuerpo sin acotar es un
// DoS con forma de telemetría, y el techo del transporte es demasiado alto para esta puerta.
const MuestraMaxBytes = 4 << 10

// PctUsado devuelve el porcentaje usado de un par (usado, total), o nil si total es 0.
// Centralizado para que el panel, el CLI y las pruebas no dividan cada uno por su cuenta —
// y para que nadie divida por cero.
func PctUsado(usado, total uint64) *float64 {
	if total == 0 {
		return nil
	}
	p := float64(usado) / float64(total) * 100
	return &p
}

// Valida chequea que una muestra REPORTADA por un agente sea creíble antes de guardarla.
//
// El agente es un cliente y sus datos son entrada no confiable, aunque su credencial sea válida:
// una máquina comprometida puede reportar un 900 % de CPU para ensuciar un panel o disparar
// alertas. No se «corrige» el valor —eso ocultaría el problema—, se RECHAZA la muestra entera y
// el latido sigue valiendo (D7): estar viva y saber medirse son cosas distintas.
func (m Muestra) Valida() error {
	// SIN `tomada` NO HAY MUESTRA, y era el único campo sin guarda pese a ser el que decide si
	// todo lo demás vale.
	//
	// `SaludServicio.Valida` ya lo exigía con estas mismas palabras; la muestra del HOST no, y
	// son la misma clase de dato entrando por la misma puerta no confiable. Con `tomada` en cero
	// la antigüedad se calcula contra el año 1: `musubi_fleet_metrics` contesta
	// `antiguedad_s: 63882345600` —sin ninguna guarda que lo tape— y el motor de políticas ve una
	// muestra vieja por dos mil años, así que deja de actuar sobre esa máquina en silencio.
	//
	// El exportador SÍ se protege (`!m.Tomada.IsZero()` omite la serie), y esa asimetría es la
	// pista: una superficie lo sabía y las otras dos no. Se rechaza en la puerta, que es donde
	// alcanza con arreglarlo una vez.
	if m.Tomada.IsZero() {
		return errors.New("la muestra no dice cuándo se tomó: sin `tomada` no hay forma de saber si es de hace un minuto o de hace una semana")
	}
	if m.CPUPct != nil && (*m.CPUPct < 0 || *m.CPUPct > 100) {
		return fmt.Errorf("cpu_pct fuera de rango: %v", *m.CPUPct)
	}
	if m.MemUsada > m.MemTotal {
		return fmt.Errorf("mem_usada (%d) supera mem_total (%d)", m.MemUsada, m.MemTotal)
	}
	if m.SwapUsada > m.SwapTotal {
		return fmt.Errorf("swap_usada (%d) supera swap_total (%d)", m.SwapUsada, m.SwapTotal)
	}
	if m.DiscoUsado > m.DiscoTotal {
		return fmt.Errorf("disco_usado (%d) supera disco_total (%d)", m.DiscoUsado, m.DiscoTotal)
	}
	if m.DiscoDisponible > m.DiscoTotal {
		return fmt.Errorf("disco_disponible (%d) supera disco_total (%d)", m.DiscoDisponible, m.DiscoTotal)
	}
	for nombre, l := range map[string]*float64{"load1": m.Load1, "load5": m.Load5, "load15": m.Load15} {
		if l != nil && *l < 0 {
			return fmt.Errorf("%s negativo: %v", nombre, *l)
		}
	}
	// LA ÚNICA REGLA QUE SE PUEDE AFIRMAR DE mem_libre, y conviene escribir la que NO va.
	//
	// `mem_libre <= mem_total - mem_usada` parece obvia y es FALSA: MemUsada sale de
	// MemAvailable, que DESCUENTA los watermarks del kernel, así que en una máquina con poco
	// cache reclamable MemAvailable puede ser MENOR que MemFree y la resta da negativa sin que
	// nada esté mal. Y el rechazo no es barato: una muestra inválida se descarta ENTERA (D7), así
	// que la aserción de más le costaría toda la telemetría a un servidor recién arrancado —
	// justo el que más se mira. Lo único siempre cierto es que la RAM libre no supera la total.
	// SIN el `MemTotal > 0`, que era la guarda original y dejaba el único campo nuevo sin techo
	// justo cuando el total llega en cero. La muestra es entrada NO CONFIABLE —la manda el agente,
	// que puede estar comprometido— y `{"mem_total":0,"mem_libre":9223372036854775808}` pasaba.
	// Con mem_total en 0, «libre» sólo puede ser 0: no hay memoria de la que sobre.
	if m.MemLibre != nil && *m.MemLibre > m.MemTotal {
		return fmt.Errorf("mem_libre (%d) supera mem_total (%d)", *m.MemLibre, m.MemTotal)
	}
	// Tampoco se valida un techo para NumProcesos: un conteo alto es raro, no imposible (un build
	// server con miles de procesos existe), y no vale perder la muestra entera por él.
	if m.NumProcesos < 0 {
		return fmt.Errorf("num_procesos negativo: %d", m.NumProcesos)
	}

	// La regla de los pares, verificada: un total sin su usado produciría un 0 % engañoso.
	if m.MemTotal > 0 && m.MemUsada == 0 {
		return fmt.Errorf("mem_total sin mem_usada: un total sin su usado se lee como 0%% de memoria en uso")
	}
	if m.DiscoTotal > 0 && m.DiscoUsado == 0 {
		return fmt.Errorf("disco_total sin disco_usado: un total sin su usado se lee como disco vacío")
	}
	if m.NumCPU < 0 {
		return fmt.Errorf("num_cpu negativo: %d", m.NumCPU)
	}
	return nil
}

// Serializar lleva la muestra a JSON para guardarla en la fila del dispositivo.
func (m Muestra) Serializar() (string, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("no se pudo serializar la muestra: %w", err)
	}
	return string(b), nil
}

// MuestraDesdeTexto revierte Serializar. Un texto vacío devuelve (nil, nil): «esta máquina
// todavía no reportó», que no es un error sino el estado inicial de toda máquina.
func MuestraDesdeTexto(s string) (*Muestra, error) {
	if s == "" {
		return nil, nil
	}
	var m Muestra
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("muestra guardada ilegible: %w", err)
	}
	return &m, nil
}

// f64 devuelve un puntero a v. Los colectores lo usan para decir «esto SÍ lo medí», frente al nil
// que significa «este sistema operativo no expone esta métrica».
func f64(v float64) *float64 { return &v }

// u64 devuelve un puntero a v. Gemelo de f64 para los contadores de BYTES que un sistema
// operativo puede no exponer: el nil dice «acá no se mide», el puntero a 0 diría «medí y da
// cero», y son cosas distintas.
func u64(v uint64) *uint64 { return &v }
