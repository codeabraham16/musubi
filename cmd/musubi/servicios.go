package main

// servicios.go es la mitad de A42 que NO depende del sistema operativo: qué se reporta, en qué
// orden, y qué pasa cuando no entra todo.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ EL ORDEN IMPORTA MÁS QUE LA LISTA
//
// El cerebro PODA POR AUSENCIA: lo que esta máquina deja de reportar se da de baja. Y rechaza el
// lote entero si pasa de `fleet.ServiciosPorLatido`, en vez de truncarlo — decisión suya y
// correcta, porque un truncado silencioso del otro lado sería una baja silenciosa de la cola.
//
// Las dos cosas juntas significan que el recorte lo tiene que hacer ACÁ, y que tiene que ser
// ESTABLE: si dos latidos seguidos eligen distintos 64 de los mismos 80 servicios, la diferencia
// se da de baja y se vuelve a dar de alta cada pocos segundos. El inventario latiría y el panel
// mostraría bajas que no ocurrieron. Por eso el orden es determinista —prioridad, después nombre—
// y nunca depende de en qué orden los devolvió el sistema.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// QUÉ SE REPORTA, Y POR QUÉ NO TODO
//
// Una máquina tiene cientos de units. Reportar «las primeras 64» no informa nada. Se reporta lo
// que alguien DECIDIÓ que corra acá (habilitado) más lo que está roto (fallado): «declarado y
// detenido» es la fila que uno quiere ver, y «deshabilitado e inactivo» es ruido.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"musubi/internal/fleet"
)

// enumerarServicios lo implementa cada OS. Devuelve TODO lo que encontró, sin recortar: el
// recorte y el orden son de acá, para que las cuatro implementaciones no lo repitan mal.
//
// Es `var` para que las pruebas lo apunten a un doble: la parte que habla con systemd o con el
// SCM de Windows es la única que no se puede verificar sin esa máquina.
var enumerarServicios = enumerarServiciosDelSistema

// prioridadDeReporte ordena por lo que un operador quiere ver primero cuando no entra todo.
//
// Fallado antes que corriendo NO es cosmético: si una máquina tiene 80 servicios y sólo entran
// 64, el que se cae afuera tiene que ser el que anda bien, jamás el que está roto.
func prioridadDeReporte(r fleet.ReporteServicio) int {
	switch r.Salud.Estado {
	case fleet.EstadoFallado:
		return 0
	case fleet.EstadoDetenido:
		return 1
	case fleet.EstadoCorriendo:
		return 2
	default:
		return 3
	}
}

// serviciosParaElLatido arma la lista definitiva: valida, deduplica, ordena y recorta.
//
// Devuelve también cuántos quedaron afuera, porque un recorte que no se cuenta es un recorte que
// nadie arregla.
func serviciosParaElLatido(crudos []fleet.ReporteServicio) (lista []fleet.ReporteServicio, afuera int) {
	vistos := make(map[string]bool, len(crudos))
	for _, r := range crudos {
		r = fleet.RecortarReporte(r)
		if !fleet.NombreDeServicioValido(r.Nombre) {
			continue
		}
		// El mismo nombre dos veces rompería la poda: el cerebro guarda uno y el otro queda como
		// ausente. Pasa de verdad — un `docker.service` de systemd y un contenedor llamado igual.
		if vistos[strings.ToLower(r.Nombre)] {
			continue
		}
		vistos[strings.ToLower(r.Nombre)] = true
		lista = append(lista, r)
	}

	sort.SliceStable(lista, func(i, j int) bool {
		pi, pj := prioridadDeReporte(lista[i]), prioridadDeReporte(lista[j])
		if pi != pj {
			return pi < pj
		}
		return lista[i].Nombre < lista[j].Nombre
	})

	if len(lista) > fleet.ServiciosPorLatido {
		afuera = len(lista) - fleet.ServiciosPorLatido
		lista = lista[:fleet.ServiciosPorLatido]
	}
	return lista, afuera
}

// ────────────────────────────────────────────────────────────────────────────────────────────
// EL INVENTARIO NO VIAJA EN CADA LATIDO, Y NO ES UNA OPTIMIZACIÓN
//
// La primera versión lo colgaba de todos los latidos y rompió una guarda que ya existía:
// «un latido SIN muestra tiene que ser sólo el autorreporte, decenas de bytes». Pasó a mandar
// 7.180. Con el intervalo más corto de la flota (10 s) son 7 KB cada diez segundos POR MÁQUINA,
// y del otro lado una escritura del inventario entero con su poda cada vez.
//
// La telemetría cambia todo el tiempo; el inventario casi nunca. Así que se manda cuando CAMBIÓ,
// y además cada `intervaloInventarioCompleto` aunque no haya cambiado — ese piso no es paranoia:
// el cerebro guarda `last_report` cuando llega un reporte, y sin él un inventario estable se
// vería cada vez más viejo hasta parecer abandonado.
//
// Que no se mande NO borra nada: la poda del cerebro sólo corre cuando llega una lista.
// Sale del dominio (fleet.InventarioCada) y no de un número acá: el cerebro mide la frescura
// contra la misma constante, y dos números separados se separan.
const intervaloInventarioCompleto = fleet.InventarioCada

var ultimoInventario struct {
	sync.Mutex
	huella  string
	enviado time.Time
}

// huellaDelInventario resume lo que importa para decidir si cambió: los nombres y sus estados.
//
// NO entra el PID ni el detalle a propósito. Un servicio que se reinicia cambia de pid cada vez,
// y meterlo en la huella haría que «cambió» sea verdad siempre — que es exactamente el problema
// que esto viene a resolver.
func huellaDelInventario(lista []fleet.ReporteServicio) string {
	h := sha256.New()
	for _, r := range lista {
		fmt.Fprintf(h, "%s\x00%s\x00%s\n", r.Nombre, r.Clase, r.Salud.Estado)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// serviciosDelLatido es lo que llama el agente. Nunca devuelve error: que falte el inventario no
// puede impedir que la máquina lata — un agente que se calla porque no pudo listar sus units es
// una máquina que figura muerta por un motivo que no tiene nada que ver.
//
// Devuelve nil cuando el inventario no cambió y todavía no toca el reenvío periódico.
func serviciosDelLatido() []fleet.ReporteServicio {
	crudos, err := enumerarServicios()
	if err != nil {
		avisarUnaVez("servicios-enumerar", "no se pudieron enumerar los servicios de esta máquina: %v", err)
		return nil
	}
	lista, afuera := serviciosParaElLatido(crudos)
	if afuera > 0 {
		// Una vez por vida del proceso: el latido corre cada pocos segundos y un aviso por latido
		// deja de leerse, que es lo mismo que no avisar.
		avisarUnaVez("servicios-recortados",
			"esta máquina tiene %d servicios y el latido lleva %d: %d quedaron afuera. Se priorizan los fallados y los detenidos; los que sobran son los que están corriendo bien",
			len(crudos), len(lista), afuera)
	}

	huella := huellaDelInventario(lista)
	ultimoInventario.Lock()
	defer ultimoInventario.Unlock()
	if huella == ultimoInventario.huella && time.Since(ultimoInventario.enviado) < intervaloInventarioCompleto {
		return nil
	}
	ultimoInventario.huella = huella
	ultimoInventario.enviado = time.Now()
	return lista
}

// salidaDeComando corre un comando y devuelve su salida. Los enumeradores la usan en vez de
// hablar con una API: systemd, el SCM y launchd tienen todos una herramienta de línea de comandos
// estable, y usarla evita una dependencia por sistema operativo.
func salidaDeComando(nombre string, args ...string) (string, error) {
	b, err := ejecutarParaEnumerar(nombre, args...)
	if err != nil {
		return "", fmt.Errorf("%s: %w", nombre, err)
	}
	return string(b), nil
}

// ────────────────────────────────────────────────────────────────────────────────────────────
// UN INVENTARIO PARCIAL NO ES MENOS INFORMACIÓN: ES UNA AFIRMACIÓN FALSA
//
// Acá había una `avisarDeEnumeracionParcial` que avisaba de la fuente rota y mandaba igual lo que
// las demás hubieran conseguido, con este razonamiento escrito: «perder el inventario entero
// porque una de las dos fuentes falló sería cambiar información parcial por ninguna».
//
// Es falso, y se pagó en producción. El cerebro PODA POR AUSENCIA: la lista no dice «encontré
// esto», dice «esto es lo que corre acá». Un `podman ps` que falla no le saca 18 filas al mensaje
// — le agrega la afirmación de que esos 18 contenedores dejaron de existir. La poda los dio de
// baja y los 18 quedaron invisibles.
//
// Así que la regla es: el inventario se manda COMPLETO o NO SE MANDA. Que no se mande no borra
// nada —la poda sólo corre cuando llega una lista— y no es silencioso: los servicios que ya
// estaban dejan de recibir noticias, se ponen `fresco: false` y salta `ServicioSinNoticias`.
// «No pude armar un inventario confiable» tiene que verse; parchearlo con uno incompleto es
// justamente lo que lo escondía.
//
// enumerarFuente separa los TRES desenlaces que la versión anterior mezclaba en dos:
//
//	("", false, nil)  la herramienta no está instalada — normal, no es una falla; se saltea
//	("", true,  err)  está y falló — no hay inventario confiable; el llamador aborta
//	(salida, true, nil) anduvo
//
// La distinción es `exec.ErrNotFound` y no un `LookPath` previo: entre mirar si está y correrlo
// hay una carrera, y una fuente que desaparece justo en el medio caería del lado equivocado.
func enumerarFuente(cli string, args ...string) (salida string, hayFuente bool, err error) {
	salida, err = salidaDeComando(cli, args...)
	switch {
	case err == nil:
		return salida, true, nil
	case errors.Is(err, exec.ErrNotFound):
		return "", false, nil
	default:
		// EL MENSAJE TIENE QUE NOMBRAR AL SOSPECHOSO QUE NADIE MIRA (A54). Un `podman ps` que
		// sale con código 1 no dice «permiso denegado» en ningún lado, y la unidad de systemd
		// —que está bien escrita para el agente que describe— es lo último en lo que alguien
		// piensa. Costó dos días la primera vez. El error apunta al verificador en vez de
		// repetir acá una lista de rutas que se quedaría vieja.
		return "", true, fmt.Errorf("%s está instalado y no se pudo consultar: %w"+
			" · si esto corre bajo systemd, revisá el blindaje de la unidad con"+
			" `musubi agent --revisar-blindaje`: enumerar contenedores NO es una lectura"+
			" y ProtectHome/ProtectSystem lo prohíben sin un drop-in", cli, err)
	}
}
