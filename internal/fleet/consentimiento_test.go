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
// arnes: archivo="internal/fleet/consentimiento.go"
// arnes: de="\t\tres = MasRestrictivo(res, f)"
// arnes: a="\t\tres = f"
//
// Sabotaje que la hace fallar: bajarle el nivel a `prohibido` hasta que empate con `avisa` — el
// candado del dueño deja de ganarle a `pide`.
// arnes: archivo="internal/fleet/consentimiento.go"
// arnes: de="\tConsentimientoProhibido: 3,"
// arnes: a="\tConsentimientoProhibido: 1,"
func TestGanaLaFuenteMasRestrictivaYNoLaMasEspecifica(t *testing.T) {
	casos := []struct {
		nombre  string
		fuentes []Consentimiento
		quiero  Consentimiento
	}{
		{"el proyecto endurece y la máquina afloja", []Consentimiento{ConsentimientoPide, ConsentimientoLibre}, ConsentimientoPide},
		{"la máquina endurece y el proyecto afloja", []Consentimiento{ConsentimientoLibre, ConsentimientoPide}, ConsentimientoPide},
		{"prohibido gana a todo", []Consentimiento{ConsentimientoLibre, ConsentimientoProhibido, ConsentimientoAvisa}, ConsentimientoProhibido},
		// LOS DOS PELDAÑOS DE ARRIBA, QUE ESTABAN SIN CLAVAR. La fila de acá arriba sólo le pide a
		// `prohibido` que le gane a `libre` y a `avisa`; nunca lo enfrenta a `pide`. Medido el
		// 2026-09-21: con `nivel[prohibido] = 1` —o intercambiando los niveles de `pide` y
		// `prohibido`— esta guarda quedaba VERDE mientras el candado del dueño perdía contra un
		// `pide`, y el paquete entero también. Las dos órdenes, porque un máximo que depende del
		// orden es una cascada disfrazada.
		{"el candado del dueño gana a `pide`", []Consentimiento{ConsentimientoPide, ConsentimientoProhibido}, ConsentimientoProhibido},
		{"y gana también si viene primero", []Consentimiento{ConsentimientoProhibido, ConsentimientoPide}, ConsentimientoProhibido},
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
	// Y EL VALOR VA CLAVADO, porque `a == b` sola no dice nada: la satisface cualquier función
	// constante. Con `nivel[prohibido] = 1` las dos daban `pide` y esta comprobación seguía en
	// verde — una simetría no clava un valor.
	if a != ConsentimientoPide {
		t.Errorf("las dos órdenes coinciden en %q y el máximo de {pide, libre, avisa} es `pide`: "+
			"coinciden, pero en el valor equivocado", a)
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
// Sabotaje que la hace fallar: que un valor ilegible caiga en `libre` y no en el default, dentro
// de ResolverConsentimiento.
//
// LA PROSA DECÍA OTRA COSA Y ESTABA MAL — medido el 2026-09-19, al mecanizarla. Nombraba
// «devolver `nivel[c]` directo» en `nivelDe`, y ese corte deja esta guarda ENTERA EN VERDE: los 7
// subtests pasan. Ningún camino de esta prueba llega a `nivelDe` con basura — `ResolverConsentimiento`
// normaliza la fuente ilegible ANTES de combinar, y `Valido()` pregunta por el mapa sin pasar por
// ahí. Una promesa de sabotaje apuntada a un lugar que la guarda no toca es un veredicto que nunca
// se podía cobrar, y mientras siguiera siendo prosa nadie iba a enterarse.
// arnes: archivo="internal/fleet/consentimiento.go"
// arnes: de="\t\t\tf = ConsentimientoPorDefecto"
// arnes: a="\t\t\tf = ConsentimientoLibre"
// LA COLISIÓN ES CON EL ANCLA DE ABAJO, DE ESTA MISMA PRUEBA, y está contestada porque se
// corrieron las dos: aquélla cae en «"" se resolvió a `libre`» (el ilegible aterriza en el grado
// más flojo) y la de abajo en «`libre` junto a "" resolvió "libre"» (el ilegible desaparece en vez
// de aportar el default). Son dos defectos distintos sobre las mismas cuatro líneas, y el aviso va
// acá porque es el `a` de ESTA directiva el que le rompe el ancla a la otra.
// arnes: colision_ok="TestUnValorIlegibleNoAbreLaPuerta"
//
// Sabotaje que la hace fallar: saltear el ilegible cuando NO es la primera fuente — deja de
// aportar el default y la fuente más floja decide sola.
// arnes: archivo="internal/fleet/consentimiento.go"
// arnes: de="\t\tif !f.Valido() {\n\t\t\tf = ConsentimientoPorDefecto\n\t\t}\n\t\tif !visto {"
// arnes: a="\t\tif !f.Valido() {\n\t\t\tf = ConsentimientoPorDefecto\n\t\t\tif visto {\n\t\t\t\tcontinue\n\t\t\t}\n\t\t}\n\t\tif !visto {"
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
			// Y COMBINADO CON ALGO MÁS FLOJO QUE EL DEFAULT, TIENE QUE ENDURECERLO. Es el único
			// caso donde la basura DECIDE, y era el que faltaba: arriba la basura va sola —donde
			// es la primera fuente y sí se normaliza— o contra `prohibido`, que gana igual.
			// Medido el 2026-09-21: salteando el ilegible cuando no es la primera fuente, esta
			// guarda quedaba verde y `libre + typo` resolvía `libre`. Es exactamente lo que el
			// comentario de esa línea advierte en prosa: «un typo en la fuente más restrictiva
			// desaparece sin dejar rastro».
			if got := ResolverConsentimiento(ConsentimientoLibre, b); got != ConsentimientoPorDefecto {
				t.Errorf("`libre` junto a %q resolvió %q: el ilegible tiene que aportar el default, "+
					"y en vez de eso desapareció", b, got)
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
// arnes: archivo="internal/fleet/consentimiento.go"
// arnes: de="const ConsentimientoPorDefecto = ConsentimientoAvisa"
// arnes: a="const ConsentimientoPorDefecto = ConsentimientoLibre"
//
// Sabotaje que la hace fallar: dejar la CONSTANTE intacta y hacer que lo no declarado se normalice
// a `libre` — el default de facto deja de coincidir con la etiqueta.
// arnes: archivo="internal/fleet/consentimiento.go"
// arnes: de="func normalizar(c Consentimiento) Consentimiento {\n\tif c.Valido() {\n\t\treturn c\n\t}\n\treturn ConsentimientoPorDefecto\n}"
// arnes: a="func normalizar(c Consentimiento) Consentimiento {\n\tif c.Valido() {\n\t\treturn c\n\t}\n\tif c == \"\" {\n\t\treturn ConsentimientoLibre\n\t}\n\treturn ConsentimientoPorDefecto\n}"
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

	// EL DEFAULT NO ACTÚA COMO CONSTANTE: ACTÚA CUANDO `normalizar` CONVIERTE LO NO DECLARADO.
	//
	// Las tres preguntas de arriba se le hacen a la etiqueta. Medido el 2026-09-21: dejando la
	// constante intacta y haciendo que `normalizar("")` devuelva `libre`, esta guarda quedaba
	// VERDE y el default de facto —el que le toca a la máquina que no declaró nada— pasaba a ser
	// `libre`: se la mira en silencio. La guarda leía la etiqueta; el default vive en la función.
	if got := normalizar(""); got != ConsentimientoPorDefecto {
		t.Errorf("lo NO DECLARADO se normalizó a %q y no al default %q: la constante dice una cosa "+
			"y la máquina recién dada de alta recibe otra", got, ConsentimientoPorDefecto)
	}
	if sinDeclarar := Consentimiento(""); !sinDeclarar.AvisaAlUsuario() {
		t.Error("una máquina que no declaró nada no avisa: se la puede mirar en silencio sin que " +
			"nadie haya elegido eso")
	}
}

// PEDIR ES AVISAR Y ALGO MÁS.
//
// Si `pide` no contara como aviso, el camino de la notificación se saltearía justo en el caso
// más sensible —donde hay una persona real que tiene que decidir— y quedaría un diálogo que
// aparece sin que nada le haya dicho a nadie que iba a aparecer.
//
// Sabotaje que la hace fallar: comparar por igualdad con `avisa` en vez de por nivel.
// arnes: archivo="internal/fleet/consentimiento.go"
// arnes: de="func (c Consentimiento) AvisaAlUsuario() bool { return nivelDe(c) >= nivel[ConsentimientoAvisa] }"
// arnes: a="func (c Consentimiento) AvisaAlUsuario() bool { return normalizar(c) == ConsentimientoAvisa }"
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
// arnes: archivo="internal/fleet/consentimiento.go"
// arnes: de="\tvar res Consentimiento\n\tvisto := false"
// arnes: a="\tres := ConsentimientoPorDefecto\n\tvisto := true"
//
// Sabotaje que la hace fallar: preguntar por la ORTOGRAFÍA nil del slice en vez de por «no vi
// ninguna fuente» — el vacío no-nil deja de recibir el default.
// arnes: archivo="internal/fleet/consentimiento.go"
// arnes: de="\tif !visto {\n\t\treturn ConsentimientoPorDefecto\n\t}"
// arnes: a="\tif fuentes == nil {\n\t\treturn ConsentimientoPorDefecto\n\t}"
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
	// LA AUSENCIA TIENE DOS ORTOGRAFÍAS Y ACÁ SE PREGUNTAN LAS DOS. `ResolverConsentimiento()` sin
	// argumentos pasa un slice NIL; un llamador que junte las fuentes declaradas pasa uno VACÍO
	// pero no nil. Medido el 2026-09-21: cambiando el cierre a `if fuentes == nil` esta guarda
	// quedaba verde —contesta bien la única ortografía que probaba— y el vacío devolvía `""`, que
	// ni siquiera es un grado válido: la basura se escapaba del resolvedor.
	vacias := make([]Consentimiento, 0, 3)
	if got := ResolverConsentimiento(vacias...); got != ConsentimientoPorDefecto {
		t.Errorf("un slice VACÍO pero no nil resolvió %q, se esperaba el default: «nadie dijo nada» "+
			"no puede depender de cómo se escribió la lista", got)
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
// arnes: archivo="internal/fleet/device.go"
// arnes: de="\treturn otorgada == CapScreen && pedida == CapScreenView"
// arnes: a="\treturn (otorgada == CapScreen && pedida == CapScreenView) || (otorgada == CapScreenView && pedida == CapScreen)"
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

// `pide` EN UNA MÁQUINA SIN NADIE A QUIEN PREGUNTARLE SE CIERRA, NO SE ABRE.
//
// Un servidor sin escritorio, un contenedor, una máquina en la pantalla de bloqueo: no hay dónde
// dibujar el diálogo ni quién lo conteste. Hay que decidir qué hacer con una promesa que el
// sistema no puede cumplir, y la salida cómoda —abrir igual, «nadie puede negarse»— es
// exactamente al revés de lo que alguien quiso decir al escribir `pide`.
//
// Quien lo escribió pidió que nadie entre sin permiso. Si el permiso no se puede pedir, no se
// entra. Degradar hacia abajo convertiría la configuración MÁS ESTRICTA en la MÁS PERMISIVA justo
// en las máquinas donde nadie está mirando — que son las que más se parecen a un servidor de
// producción.
//
// Sabotaje que la hace fallar: degradar `pide` a `libre` (o a `avisa`) cuando no se puede
// preguntar.
// arnes: archivo="internal/fleet/consentimiento.go"
// arnes: de="\t\treturn ConsentimientoProhibido"
// arnes: a="\t\treturn ConsentimientoLibre"
func TestPedirDondeNadiePuedeContestarCierraLaPuerta(t *testing.T) {
	if got := ConsentimientoPide.AplicarACapacidadDePreguntar(false); got != ConsentimientoProhibido {
		t.Errorf("`pide` sin interlocutor quedó en %q: la configuración más estricta se volvió la más permisiva", got)
	}
	if got := ConsentimientoPide.AplicarACapacidadDePreguntar(true); got != ConsentimientoPide {
		t.Errorf("`pide` con interlocutor quedó en %q", got)
	}

	// `avisa` NO se degrada: se puede dejar constancia aunque no haya nadie leyendo en ese
	// momento. Avisar no necesita interlocutor; preguntar sí. Degradarlo cerraría el acceso a
	// todos los servidores de la flota por un default que no bloquea nada.
	if got := ConsentimientoAvisa.AplicarACapacidadDePreguntar(false); got != ConsentimientoAvisa {
		t.Errorf("`avisa` se degradó a %q sin interlocutor: dejar constancia no necesita quien conteste", got)
	}
	// Y los otros dos no se mueven en ningún caso.
	for _, c := range []Consentimiento{ConsentimientoLibre, ConsentimientoProhibido} {
		for _, puede := range []bool{true, false} {
			if got := c.AplicarACapacidadDePreguntar(puede); got != c {
				t.Errorf("%q con puedePreguntar=%v quedó en %q", c, puede, got)
			}
		}
	}
	// Un valor ilegible pasa por el default antes de decidir: sin esto, la basura eludiría la
	// degradación por el camino de atrás.
	if got := Consentimiento("Pide").AplicarACapacidadDePreguntar(false); got != ConsentimientoPorDefecto {
		t.Errorf("un valor ilegible resolvió %q en vez del default", got)
	}
}

// TestLasTresPreguntasSeContestanParaLosCuatroGrados — LA MATRIZ, Y NO LAS CELDAS DE SIEMPRE.
//
// Los tres predicados son toda la superficie por la que el resto del árbol lee el eje, y las
// guardas que los tocan preguntan celdas sueltas: `TestPedirImplicaAvisar` verifica que `libre` y
// `avisa` NO piden aprobación, y nunca que `pide` SÍ; `TestPedirDondeNadiePuedeContestar…` usa
// `Bloquea` sin comprobar nunca que `prohibido` bloquee.
//
// Medido el 2026-09-21: `PideAprobacion` comparando contra `prohibido`, y `Bloquea` devolviendo
// siempre falso, dejaban las dos guardas en VERDE y `./internal/fleet` entero también. Los cazaban
// pruebas de `internal/mcp` —cinco, en el caso de `Bloquea`— y eso no alcanza: un invariante que
// cae de rebote no está guardado. El día que esas pruebas cambien su fixture, la detección se
// evapora y nada lo dice.
//
// LOS DOCE VALORES VAN CLAVADOS, uno por uno. Derivarlos de `nivelDe` o de `normalizar` —«avisa es
// nivel ≥ 1»— dejaría esta tabla midiéndose contra la misma escala que custodia: una mutación del
// mapa movería los dos lados y la guarda la certificaría sana.
//
// Sabotaje que la hace fallar: que `PideAprobacion` compare contra `prohibido` en vez de `pide`.
// arnes: archivo="internal/fleet/consentimiento.go"
// arnes: de="func (c Consentimiento) PideAprobacion() bool { return normalizar(c) == ConsentimientoPide }"
// arnes: a="func (c Consentimiento) PideAprobacion() bool { return normalizar(c) == ConsentimientoProhibido }"
//
// Sabotaje que la hace fallar: que `Bloquea` pregunte por un nivel MAYOR que el máximo, o sea
// nunca.
// arnes: archivo="internal/fleet/consentimiento.go"
// arnes: de="func (c Consentimiento) Bloquea() bool { return normalizar(c) == ConsentimientoProhibido }"
// arnes: a="func (c Consentimiento) Bloquea() bool { return nivelDe(c) > nivel[ConsentimientoProhibido] }"
func TestLasTresPreguntasSeContestanParaLosCuatroGrados(t *testing.T) {
	casos := []struct {
		grado   Consentimiento
		avisa   bool
		pide    bool
		bloquea bool
	}{
		{ConsentimientoLibre, false, false, false},
		{ConsentimientoAvisa, true, false, false},
		{ConsentimientoPide, true, true, false},
		{ConsentimientoProhibido, true, false, true},
	}
	if len(casos) != len(nivel) {
		t.Fatalf("la tabla cubre %d grados y el dominio tiene %d: un grado nuevo sin fila acá entra "+
			"al árbol sin que nadie diga qué contestan sus tres preguntas", len(casos), len(nivel))
	}
	for _, c := range casos {
		t.Run(string(c.grado), func(t *testing.T) {
			if got := c.grado.AvisaAlUsuario(); got != c.avisa {
				t.Errorf("AvisaAlUsuario() = %v y la regla dice %v", got, c.avisa)
			}
			if got := c.grado.PideAprobacion(); got != c.pide {
				t.Errorf("PideAprobacion() = %v y la regla dice %v: `pide` es el ÚNICO grado que "+
					"promete que alguien acepte antes de que pase algo", got, c.pide)
			}
			if got := c.grado.Bloquea(); got != c.bloquea {
				t.Errorf("Bloquea() = %v y la regla dice %v: `prohibido` es el candado del dueño, y "+
					"un candado que no cierra es decoración", got, c.bloquea)
			}
		})
	}
}

// LA IMPLICACIÓN VALE TAMBIÉN EN LA MATRIZ DEL APARATO, Y OLVIDARLO LE SACA ALGO QUE SIEMPRE PUDO.
//
// Lo cazó una prueba existente al partir `screen`, y el modo de fallo es de los peores: silencioso
// y retroactivo. Una máquina enrolada con `caps: ["screen"]` NO tiene `screen:view` en su lista,
// así que una comparación por igualdad la daba por incapaz de mirar la pantalla que ya deja
// controlar. En producción eso vaciaba la bitácora de sesiones de TODAS las máquinas existentes
// —ninguna declara la capacidad nueva— sin un solo error.
//
// La lección general: partir una capacidad en dos exige propagar la implicación a los DOS ejes,
// el de la credencial y el del aparato. Arreglar sólo uno deja el otro mintiendo.
//
// Sabotaje que la hace fallar: volver `Permite` a comparar con `==`.
// arnes: archivo="internal/fleet/device.go"
// arnes: de="\t\tif Implica(tiene, c) {"
// arnes: a="\t\tif tiene == c {"
func TestElAparatoQueAdmiteControlarAdmiteMirar(t *testing.T) {
	// Enrolada a la vieja usanza: sólo `screen`, que es como están TODAS las filas existentes.
	d := Device{Tier: TierAgente, Caps: []Cap{CapMetrics, CapScreen}}

	if !d.Permite(CapScreen) {
		t.Fatal("no admite lo que declara")
	}
	if !d.Permite(CapScreenView) {
		t.Error("una máquina que admite CONTROLAR la pantalla no admite MIRARLA: partir la " +
			"capacidad le sacó, sin avisar, algo que siempre pudo")
	}
	// Y al revés no: declarar sólo `screen:view` no habilita controlar.
	soloMirar := Device{Tier: TierAgente, Caps: []Cap{CapScreenView}}
	if soloMirar.Permite(CapScreen) {
		t.Error("una máquina que sólo admite mirar admite controlar: la implicación se volvió simétrica")
	}
	if !soloMirar.Permite(CapScreenView) {
		t.Error("no admite lo que declara")
	}
	// Y el tier sigue mandando: un Tier B con `screen` escrito a mano no admite ninguna de las dos.
	tierB := Device{Tier: TierProtocolo, Caps: []Cap{CapScreen, CapScreenView}}
	if tierB.Permite(CapScreen) || tierB.Permite(CapScreenView) {
		t.Error("un Tier B admitió pantalla: la implicación eludió la matriz del tier (A4)")
	}
	// Una máquina revocada no admite nada, implicación incluida (C6).
	revocada := Device{Tier: TierAgente, Caps: []Cap{CapScreen}, Revoked: true}
	if revocada.Permite(CapScreenView) {
		t.Error("una máquina revocada admitió mirar por la puerta de la implicación")
	}
}
