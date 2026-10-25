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
// FASE. Esta es la mitad EMISORA del contrato y hoy NO la llama producción: el hook sigue emitiendo el
// id entero. El resolvedor (esRefCorta + hydrateByIDs) viaja PRIMERO y se despliega en todos los
// procesos que hidratan —daemon por sesión y cerebro central—; recién cuando esté en todos lados se
// enciende este emisor, en una segunda fase. Al revés —un hook nuevo que acorte contra un resolvedor
// viejo— el resolvedor viejo responde vacío sin error en cada ref y el agente pierde la hidratación.
// idcorto_test.go la ejercita, así que `deadcode -test` la ve viva aunque aún no tenga caller de prod.
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
// superconjunto, no la fila exacta a secas—. Hoy es latente: ningún id canónico mide 8 hex (los UUID
// miden 36, los demás son «commit-…»), así que nadie pide uno. Si en fase 2 algún consumidor pudiera
// crear ids de 8 hex, hay que revisitarlo.
func esRefCorta(s string) bool {
	return len(s) == 8 && hexMinuscula(s)
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
