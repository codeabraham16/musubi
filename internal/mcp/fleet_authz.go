package mcp

// fleet_authz.go responde UNA pregunta: ¿puede ESTA persona pedirle ESTO a ESTA máquina?
// Track «Control de flota», slice S3. Es la compuerta, y se construyó antes que aquello que
// compuerta (exec es S5, pantalla S6) a propósito: al revés, entre la ejecución remota y su
// autorización siempre hay una release de por medio.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA RESPUESTA ES UNA CONJUNCIÓN DE TRES LADOS, Y NINGUNO PUEDE SUPLIR A OTRO
//
//	1. TENENCIA   — la máquina tiene que estar dentro del alcance de la credencial (su proyecto,
//	                o cualquiera si es read=all). El eje que ya existía para la memoria.
//	2. CONCESIÓN  — la persona tiene que tener ESA capacidad sobre ESA máquina, declarada en
//	                principals.yaml. El eje NUEVO. Ausente ⇒ nada.
//	3. EL APARATO — la máquina tiene que poder honrarla: su tier la admite, la tiene concedida y
//	                no está revocada. Lo que S1 dejó en fleet.Device.Permite.
//
// El orden importa poco; lo que importa es que las tres son necesarias. La tentación de
// simplificar "si es admin, que pueda todo" es exactamente el puente de privilegio que el track
// evita desde el proposal: administrar la memoria del equipo NO puede convertirse, de rebote, en
// root sobre toda la flota. Esa asimetría —el rol no otorga flota, la ausencia no significa
// todas— es el diseño, no un descuido.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"musubi/internal/fleet"
)

// comodinFlota es el selector "todas las máquinas de mi alcance". Explícito a propósito: hay que
// escribirlo. Un default que significara "todas" convertiría un registro a medio llenar en una
// concesión total, y los registros a medio llenar existen.
const comodinFlota = "*"

// PuedeSobreDevice es la compuerta. Las tres condiciones, en orden de costo.
func PuedeSobreDevice(p *Principal, d fleet.Device, c fleet.Cap) bool {
	// 3. EL APARATO primero porque es puro y barato, y porque su respuesta no depende de quién
	// pregunte: si la máquina no puede, no hay credencial que lo arregle. Acá vive también el
	// kill-switch de S2 — un device revocado no admite nada de nadie (C6).
	if !d.Permite(c) {
		return false
	}
	// stdio local (sin principal): confianza local, acceso pleno. Misma regla que canCall,
	// isAdmin y writeOriginFor. Es la vía de arranque —alguien tiene que poder otorgar la
	// primera capacidad— y está probada explícitamente para que sea una decisión visible.
	if p == nil {
		return true
	}
	// 1. TENENCIA: el grant no es una puerta lateral al aislamiento por proyecto. Se aplica
	// DESPUÉS, nunca en su lugar. Nombrar una máquina ajena en principals.yaml no la alcanza.
	if !alcanzaElProyecto(p, d.ProjectID) {
		return false
	}
	// 2. CONCESIÓN.
	return tieneGrant(p, c, d.Name)
}

// PuedeVerHistorialDeDevice responde una pregunta DISTINTA de PuedeSobreDevice: ¿esta credencial
// puede ver lo que quedó REGISTRADO de una máquina que ya no está en la flota? (A51)
//
// Es PuedeSobreDevice con el kill-switch de la revocación levantado y NADA MÁS aflojado: misma
// tenencia, misma concesión, mismo tier. Quien podía ver los servicios de esa máquina mientras
// vivía los sigue viendo; nadie más. Se implementa limpiando el flag y delegando —no repitiendo
// la cadena— para que no puedan desincronizarse: una regla nueva en PuedeSobreDevice aplica acá
// sola, que es justo lo que una copia no garantiza.
//
// POR QUÉ NO ES «ADMIN Y LISTO»: eso derivaría una capacidad de flota del ROL, que es exactamente
// lo que C1 prohíbe en todo este eje. Un admin sin concesión `metrics` no ve el estado de una
// máquina viva; no hay razón para que vea el de una muerta.
//
// SÓLO PARA LEER LO YA ESCRITO. La revocación sigue siendo absoluta para todo lo que TOQUE la
// máquina: exec, pantalla y shell pasan por PuedeSobreDevice y ahí el kill-switch manda. El
// motivo de que las filas existan es la auditoría —la migración 36 y RevocarServiciosDeDevice las
// conservan a propósito—, y una auditoría que nadie puede leer no es una auditoría.
func PuedeVerHistorialDeDevice(p *Principal, d fleet.Device, c fleet.Cap) bool {
	d.Revoked = false // copia por valor: no toca el device del que llama
	return PuedeSobreDevice(p, d, c)
}

// alcanzaElProyecto dice si la credencial ve ese proyecto. read=all ve todos (sala de mando,
// cabina); el resto, sólo el suyo. Es el mismo criterio que recallScopeFor usa para la memoria:
// una sola definición de "qué alcanzo", no dos que se puedan desincronizar.
func alcanzaElProyecto(p *Principal, projectID string) bool {
	if read, _ := p.caps(); read == ReadAll {
		return true
	}
	return p.ProjectID != "" && p.ProjectID == projectID
}

// tieneGrant busca la capacidad en la concesión del principal y evalúa el selector.
//
// Nil map ⇒ false, que es el caso más importante de todos: un principal sin sección `fleet:`
// —incluido un admin con write=any— no puede nada (C1).
func tieneGrant(p *Principal, c fleet.Cap, nombreDevice string) bool {
	// SE RECORREN TODAS LAS CONCESIONES, no sólo la de la capacidad pedida, porque una puede
	// IMPLICAR a otra: quien tiene `screen` (controlar) puede mirar, y pedirle además un
	// `screen:view` explícito sería una trampa de configuración — el permiso estaría concedido y
	// la acción se negaría.
	//
	// El recorrido es sobre lo OTORGADO y no sobre una lista de implicaciones al revés, así que
	// una capacidad nueva que no implique nada sigue funcionando sin tocar esto.
	for otorgada, selectores := range p.Fleet {
		if !fleet.Implica(otorgada, c) {
			continue
		}
		for _, selector := range selectores {
			if selector == comodinFlota || selector == nombreDevice {
				return true
			}
		}
	}
	return false
}

// capsQuePuede devuelve la INTERSECCIÓN real: qué puede ejercer este principal sobre esta
// máquina, ahora. Es lo que el inventario muestra en `puedo` (C8).
//
// Se recorre la matriz del tier y no las caps declaradas del device para que el resultado siga
// el orden canónico (metrics < exec < screen:view < screen < shell) sin depender de cómo se
// escribió la fila.
func capsQuePuede(p *Principal, d fleet.Device) []fleet.Cap {
	todas := []fleet.Cap{fleet.CapMetrics, fleet.CapExec, fleet.CapScreenView, fleet.CapScreen, fleet.CapShell}
	out := make([]fleet.Cap, 0, len(todas))
	for _, c := range todas {
		if PuedeSobreDevice(p, d, c) {
			out = append(out, c)
		}
	}
	return out
}

// puedeOtorgar decide si un principal puede CONCEDER una capacidad a una máquina que se está
// dando de alta (C7).
//
// Exige el COMODÍN, no "tenerla en alguna máquina", y la diferencia cierra un escalamiento real
// y corto: alguien con `exec: ["pc-gio", "nas"]` da de alta una tercera máquina con exec y acaba
// de ampliarse el alcance sin que nadie lo autorice. Una máquina recién nacida no figura en
// ninguna lista de nombres, así que el único criterio honesto es «¿la tendrías igual?» — y sólo
// el comodín responde que sí.
func puedeOtorgar(p *Principal, c fleet.Cap) bool {
	if p == nil {
		return true // stdio local: confianza local, igual que arriba
	}
	for _, selector := range p.Fleet[c] {
		if selector == comodinFlota {
			return true
		}
	}
	return false
}

// ── El cuarto lado: QUÉ comando (S10) ───────────────────────────────────────────────────────
//
// La compuerta de arriba responde «¿podés ejecutar en esta máquina?». Esta responde «¿podés
// ejecutar ESTO?», y son dos permisos distintos: la diferencia entre poder reiniciar un servicio
// y poder hacer cualquier cosa como quien corra el agente.
//
// Se aplica DESPUÉS y jamás en lugar de PuedeSobreDevice. No otorga nada — sólo recorta.

// argvPermitido dice si la allowlist de este principal deja pasar este comando en esta máquina.
//
// LA PRECEDENCIA, que es donde vive el error caro:
//
//  1. Sin sección          ⇒ SIN RESTRICCIÓN. `exec` sigue significando lo de siempre.
//  2. Entrada para la máquina ⇒ manda, aunque esté vacía (vacía = cero comandos).
//  3. Si no, la entrada "*"    ⇒ el techo general.
//  4. Con sección y sin ninguna de las dos ⇒ NADA. La sección es el opt-in y, una vez adentro,
//     es exhaustiva: una máquina que nadie nombró no queda con permiso de todo por descuido.
//
// El paso 4 es el que hace que agregar una máquina a la flota NO le abra exec irrestricto a
// alguien que creía tener una allowlist. Que el default de "no lo pensé" sea "no puede" es la
// única forma en que una allowlist a medio escribir falla del lado correcto.
func argvPermitido(p *Principal, d fleet.Device, argv []string) bool {
	if p == nil {
		return true // stdio local: misma confianza local que PuedeSobreDevice
	}
	if p.ExecAllow == nil {
		return true // 1
	}
	if lista, hay := p.ExecAllow[d.Name]; hay {
		return fleet.PermiteArgv(lista, argv) // 2
	}
	if lista, hay := p.ExecAllow[comodinFlota]; hay {
		return fleet.PermiteArgv(lista, argv) // 3
	}
	return false // 4
}

// comandosPermitidos devuelve la allowlist EFECTIVA sobre una máquina, para mostrarla en el
// inventario. nil ⇒ sin restricción, que es distinto de una lista vacía (⇒ nada permitido) y el
// inventario tiene que poder decir la diferencia: si las dibujara igual, nadie podría distinguir
// «puede todo» de «no puede nada» mirando la misma celda.
func comandosPermitidos(p *Principal, d fleet.Device) ([]string, bool) {
	if p == nil || p.ExecAllow == nil {
		return nil, false
	}
	if lista, hay := p.ExecAllow[d.Name]; hay {
		return lista, true
	}
	if lista, hay := p.ExecAllow[comodinFlota]; hay {
		return lista, true
	}
	return []string{}, true
}
