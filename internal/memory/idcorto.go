package memory

// IDCorto es la forma CORTA con que las superficies del hook nombran una observación en «[id:…]».
// Un id con forma de UUID canónico (8-4-4-4-12 hex minúscula) se reduce a sus primeros 8 hex;
// cualquier otra forma —los ids «commit-…», los ids cortos de ejemplo, un id ya acortado— viaja
// verbatim.
//
// POR QUÉ. El «[id:…]» es overhead PURO del envoltorio del hook: NO cuenta contra el presupuesto del
// recall (que se gasta en el contenido del gist, no en el id que se agrega después), así que un UUID
// de 36 caracteres paga ~7 tokens por viñeta en CADA turno sin aportarle significado al modelo. 8 hex
// (32 bits) alcanzan para nombrar la observación dentro del acervo acotado del proyecto.
//
// CONTRATO CON EL RESOLVEDOR. El agente devuelve este id a musubi_memory_expand. Acortar acá SÓLO es
// correcto porque hydrateByIDs (expand.go) resuelve una ref corta además del id exacto, y usa ESTE
// mismo código (esRefCorta) para reconocerla: emisor y resolvedor no pueden divergir. Si el extremo
// del resolvedor dejara de aceptar prefijos, el agente no podría hidratar y esto hay que revertirlo.
//
// FASE. Esta es la mitad EMISORA del contrato y DESDE LA FASE 2 la llama producción: formatGists y
// formatDeltaGists (cmd/musubi/turn.go) acortan el id de cada viñeta del envoltorio del hook. El
// resolvedor (esRefCorta + hydrateByIDs) viajó PRIMERO —fase 1, release 0.144.0— y se desplegó en
// todos los procesos que hidratan (daemon por sesión y cerebro central) ANTES de encender esto; ese
// orden importa, porque al revés —un hook nuevo que acorta contra un resolvedor viejo— el viejo
// responde vacío sin error en cada ref y el agente pierde la hidratación en silencio.
func IDCorto(id string) string {
	if esUUIDCanonico(id) {
		return id[:8]
	}
	return id
}

// esRefCorta dice si un token que llega a la hidratación es una REF CORTA —los 8 hex que emite
// IDCorto— y no un id completo. Es exactamente 8 hex en minúscula; nada más entra por el camino de
// prefijo. OJO: un id entero que YA mida 8 hex (p. ej. un id de prueba «deadbeef») también cae por el
// prefijo, y entonces trae su propia fila MÁS cualquier UUID que empiece con esos 8 hex —un
// superconjunto, no la fila exacta a secas—. Sigue latente: ningún id canónico mide 8 hex (los UUID
// miden 36, los demás son «commit-…»), y la fase 2 emite refs cortas pero NO guarda ids de 8 hex en la
// base (son prefijos de UUID que ya existen), así que nadie pide una fila con ese id. Si algún día un
// consumidor creara ids de 8 hex, hay que revisitarlo.
func esRefCorta(s string) bool {
	return len(s) == 8 && hexMinuscula(s)
}

// EsRefCorta expone esRefCorta al límite del tool (internal/mcp). save_observation con id= la usa para
// RECHAZAR un id corto: resolverlo convertiría un alta en sobrescritura de otra fila, y no resolverlo
// insertaría un duplicado de 8 hex (ver el handler). promote/discard, en cambio, SÍ resuelven el corto
// con ResolverRefCortaCtx porque ahí el intent es inequívoco. La forma canónica vive en esRefCorta: el
// emisor (IDCorto), el resolvedor (hydrateByIDs, ResolverRefCortaCtx) y este rechazo comparten una.
func EsRefCorta(s string) bool {
	return esRefCorta(s)
}

// esUUIDCanonico reconoce SÓLO un UUID canónico en minúscula (8-4-4-4-12, guiones en 8/13/18/23 y el
// resto hex). Estricto a propósito: lo que no es exactamente esto no se acorta y viaja verbatim, que
// es el lado seguro —el id entero siempre resuelve por el camino exacto—.
func esUUIDCanonico(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				return false
			}
		}
	}
	return true
}

// hexMinuscula dice si s es no vacío y sólo dígitos hex en minúscula (0-9a-f).
func hexMinuscula(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
