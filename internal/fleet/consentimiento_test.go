package fleet

// consentimiento_test.go custodia el eje que decide qué se le debe a la persona que está EN la
// máquina. Todos sus modos de fallo terminan en el mismo lugar: una sesión que se abre en
// silencio cuando alguien había pedido que no.

import "testing"

// GANA LA MÁS RESTRICTIVA, Y NO LA MÁS ESPECÍFICA.
//
// Es la decisión central del eje y la que se rompe sola si alguien la piensa como una cascada
// tipo CSS —donde lo específico pisa a lo general—. Con cascada, poner `libre` en UNA máquina
// alcanzaría para anular un `pide` puesto en el proyecto entero, y el agujero se abriría desde
// el lado que menos se audita: la fila de un dispositivo.
//
// Acá es un MÁXIMO: una máquina puede endurecer lo que el proyecto dijo, nunca aflojarlo.
//
// Sabotaje que la hace fallar: hacer que la última fuente pise a las anteriores.
func TestGanaLaFuenteMasRestrictivaYNoLaMasEspecifica(t *testing.T) {
	casos := []struct {
		nombre  string
		fuentes []Consentimiento
		quiero  Consentimiento
	}{
		{"el proyecto endurece y la máquina afloja", []Consentimiento{ConsentimientoPide, ConsentimientoLibre}, ConsentimientoPide},
		{"la máquina endurece y el proyecto afloja", []Consentimiento{ConsentimientoLibre, ConsentimientoPide}, ConsentimientoPide},
		{"prohibido gana a todo", []Consentimiento{ConsentimientoLibre, ConsentimientoProhibido, ConsentimientoAvisa}, ConsentimientoProhibido},
		{"todas libres, queda libre", []Consentimiento{ConsentimientoLibre, ConsentimientoLibre}, ConsentimientoLibre},
		{"sin fuentes, el default", nil, ConsentimientoPorDefecto},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := ResolverConsentimiento(c.fuentes...); got != c.quiero {
				t.Errorf("resolvió %q, se esperaba %q", got, c.quiero)
			}
		})
	}

	// Y el orden de los argumentos NO puede cambiar el resultado: si lo cambiara, sería una
	// cascada disfrazada de máximo.
	a := ResolverConsentimiento(ConsentimientoPide, ConsentimientoLibre, ConsentimientoAvisa)
	b := ResolverConsentimiento(ConsentimientoAvisa, ConsentimientoLibre, ConsentimientoPide)
	if a != b {
		t.Errorf("el orden de las fuentes cambió el resultado: %q contra %q", a, b)
	}
}

// UN VALOR INVÁLIDO CAE EN EL DEFAULT, NUNCA EN «libre».
//
// Es el modo de fallo real de un eje configurado a mano: `Pide` con mayúscula, `ask` en inglés,
// una coma de más que deja el campo vacío. Si un valor que no se entiende se tomara como `libre`
// —o se ignorara, que es lo mismo— un typo abriría sesiones sin avisar, y la configuración se
// vería puesta. Es exactamente la clase de fallo que este track viene persiguiendo: verde por el
// motivo equivocado.
//
// Sabotaje que la hace fallar: devolver `nivel[c]` directo (que da 0, o sea `libre`, para
// cualquier cosa que no esté en el mapa).
func TestUnValorIlegibleNoAbreLaPuerta(t *testing.T) {
	basura := []Consentimiento{"", "Pide", "ask", "PROHIBIDO", "sí", "libre "}
	for _, b := range basura {
		t.Run(string(b), func(t *testing.T) {
			if b.Valido() {
				t.Fatalf("%q se aceptó como válido", b)
			}
			got := ResolverConsentimiento(b)
			if got == ConsentimientoLibre {
				t.Errorf("%q se resolvió a `libre`: un typo abriría la sesión sin avisar", b)
			}
			if got != ConsentimientoPorDefecto {
				t.Errorf("%q se resolvió a %q, se esperaba el default %q", b, got, ConsentimientoPorDefecto)
			}
			// Y combinado con algo estricto, no puede aflojarlo.
			if ResolverConsentimiento(ConsentimientoProhibido, b) != ConsentimientoProhibido {
				t.Errorf("%q aflojó un `prohibido`", b)
			}
		})
	}
}

// EL DEFAULT ES `avisa`, Y NI `libre` NI `pide`.
//
// Las dos alternativas fallan por motivos opuestos y los dos importan.
//
// Con `libre`: agregar una máquina la deja sin ninguna protección para quien la usa, y quien la
// agregó no tuvo que decidirlo. La ausencia de configuración no puede ser la opción menos segura.
//
// Con `pide`: cada alta produce sesiones que no se abren por algo que nadie configuró, y eso
// enseña a poner `libre` en todos lados para que deje de molestar. Un default demasiado estricto
// termina en menos seguridad, no en más.
//
// Sabotaje que la hace fallar: mover ConsentimientoPorDefecto a `libre` o a `pide`.
func TestElDefaultAvisaYNoBloquea(t *testing.T) {
	d := ConsentimientoPorDefecto
	if !d.AvisaAlUsuario() {
		t.Error("el default no avisa: una máquina recién dada de alta se puede mirar en silencio")
	}
	if d.PideAprobacion() {
		t.Error("el default pide aprobación: cada alta produce sesiones trabadas por algo que nadie configuró, " +
			"y eso enseña a poner `libre` en todos lados")
	}
	if d.Bloquea() {
		t.Error("el default bloquea")
	}
}

// PEDIR ES AVISAR Y ALGO MÁS.
//
// Si `pide` no contara como aviso, el camino de la notificación se saltearía justo en el caso
// más sensible —donde hay una persona real que tiene que decidir— y quedaría un diálogo que
// aparece sin que nada le haya dicho a nadie que iba a aparecer.
//
// Sabotaje que la hace fallar: comparar por igualdad con `avisa` en vez de por nivel.
func TestPedirImplicaAvisar(t *testing.T) {
	if !ConsentimientoPide.AvisaAlUsuario() {
		t.Error("`pide` no avisa")
	}
	if !ConsentimientoProhibido.AvisaAlUsuario() {
		t.Error("`prohibido` no avisa: aunque no se abra, la máquina tiene que poder dejar constancia del intento")
	}
	if ConsentimientoLibre.AvisaAlUsuario() {
		t.Error("`libre` avisa: entonces no es libre")
	}
	if ConsentimientoLibre.PideAprobacion() || ConsentimientoAvisa.PideAprobacion() {
		t.Error("un grado que no es `pide` está pidiendo aprobación")
	}
}

// `libre` TIENE QUE SER ALCANZABLE, Y NO LO ERA.
//
// Esta prueba nació de que la de arriba falló contra la primera versión: `ResolverConsentimiento`
// arrancaba en el default y tomaba el máximo, con lo cual `avisa` quedaba de PISO y `libre` no se
// podía obtener ni declarándolo en todas las fuentes. El comentario de la función decía lo
// contrario, así que el código y su documentación discrepaban en silencio.
//
// Se deja como prueba propia y no como un caso más: la forma de romperlo —un acumulador que
// arranca en el default— es tan natural que va a volver.
//
// Sabotaje que la hace fallar: inicializar el acumulador en ConsentimientoPorDefecto.
func TestLibreEsAlcanzableCuandoTodasLasFuentesLoDicen(t *testing.T) {
	if got := ResolverConsentimiento(ConsentimientoLibre); got != ConsentimientoLibre {
		t.Errorf("una sola fuente `libre` resolvió %q: el default está actuando de piso", got)
	}
	if got := ResolverConsentimiento(ConsentimientoLibre, ConsentimientoLibre, ConsentimientoLibre); got != ConsentimientoLibre {
		t.Errorf("tres fuentes `libre` resolvieron %q", got)
	}
	// Y la ausencia sigue dando el default: es lo que distingue «nadie dijo nada» de «todos
	// dijeron libre», que son dos cosas distintas y tienen que resolverse distinto.
	if got := ResolverConsentimiento(); got != ConsentimientoPorDefecto {
		t.Errorf("sin fuentes resolvió %q, se esperaba el default", got)
	}
}

// MIRAR NO ES CONTROLAR, Y CONTROLAR SÍ ES MIRAR.
//
// La implicación es asimétrica a propósito: quien mueve el mouse ya está viendo la pantalla, así
// que exigirle además un `screen:view` explícito sería una trampa de configuración —el permiso
// concedido y la acción negada—. Al revés no: si `screen:view` alcanzara para controlar, la
// capacidad nueva no acotaría nada y sería decoración.
//
// Sabotaje que la hace fallar: hacer Implica simétrica.
func TestMirarNoEsControlarYControlarSiEsMirar(t *testing.T) {
	if !Implica(CapScreen, CapScreenView) {
		t.Error("quien controla no puede mirar: la capacidad concedida y la acción negada")
	}
	if Implica(CapScreenView, CapScreen) {
		t.Error("mirar alcanza para controlar: entonces partir la capacidad no acotó nada")
	}
	// Y ninguna otra implicación se coló: cada capacidad se implica sólo a sí misma.
	todas := []Cap{CapMetrics, CapExec, CapScreen, CapScreenView, CapShell}
	for _, a := range todas {
		for _, b := range todas {
			esperado := a == b || (a == CapScreen && b == CapScreenView)
			if Implica(a, b) != esperado {
				t.Errorf("Implica(%q, %q) = %v, se esperaba %v", a, b, Implica(a, b), esperado)
			}
		}
	}
	// Un Tier B no tiene pantalla de ninguna clase: partir la capacidad no puede haberle
	// agregado una por la puerta de atrás.
	if TierAdmite(TierProtocolo, CapScreenView) {
		t.Error("un Tier B admite `screen:view`: un switch por SNMP no tiene framebuffer que mirar")
	}
	if !TierAdmite(TierMovil, CapScreenView) || !TierAdmite(TierAgente, CapScreenView) {
		t.Error("un tier con pantalla no admite mirarla")
	}
}
