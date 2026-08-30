package fleet

// Pruebas de la CRONOLOGÍA (fase 5 · S11): la línea de tiempo de una máquina.
//
// Lo que se custodia acá es el DOMINIO: qué se puede ver, qué se cuenta y en qué ventana. La
// compuerta contra un principal de verdad vive en internal/mcp/fleet_cronologia_test.go.

import (
	"strings"
	"testing"
	"time"
)

// Todo tipo de hecho MOSTRABLE tiene capacidad Y plano, y el que no es mostrable no tiene ninguna
// de las dos. Es el guard de completitud del enum: un tipo nuevo sin clasificar queda invisible,
// no público, y esta prueba obliga a que esa decisión sea explícita.
//
// Sabotaje: agregarle un `case` a CapDeHecho para HechoSinClasificar → falla acá.
// Sabotaje: agregar un tipo a TiposDeHecho sin su case → falla acá.
func TestCadaTipoDeHechoMostrableTieneCapacidadYPlano(t *testing.T) {
	for _, tipo := range TiposDeHecho {
		capacidad, tieneCap := CapDeHecho(tipo)
		plano, tienePlano := PlanoDeHecho(tipo)

		if tipo == HechoSinClasificar {
			if tieneCap || tienePlano {
				t.Errorf("%q NO puede ser mostrable: cap=%q(%v) plano=%q(%v)", tipo, capacidad, tieneCap, plano, tienePlano)
			}
			continue
		}
		if !tieneCap {
			t.Errorf("el tipo %q no tiene capacidad asociada: quedaría invisible para todos", tipo)
		}
		if !tienePlano {
			t.Errorf("el tipo %q no tiene plano: la respuesta lo dibujaría sin decir qué es", tipo)
		}
		if capacidad == "" || plano == "" {
			t.Errorf("el tipo %q declara tener cap/plano pero vienen vacíos", tipo)
		}
	}
}

// Una operación interna que esta versión NO conoce no se le muestra a nadie, ni siquiera a quien
// tenga todas las capacidades. El default es no mostrar.
//
// Sabotaje: en TipoDeArgv, devolver HechoComando en vez de HechoSinClasificar para lo desconocido
// → una operación nueva del canal se le mostraría a todo el que pueda ejecutar, revelando el
// plano al que pertenece antes de que nadie haya decidido quién puede verla.
func TestUnaOperacionInternaDesconocidaNoSeLeMuestraANadie(t *testing.T) {
	tipo := TipoDeArgv([]string{"musubi:todavia-no-existe", "x"})
	if tipo != HechoSinClasificar {
		t.Fatalf("una operación interna desconocida se clasificó como %q; tiene que ser %q", tipo, HechoSinClasificar)
	}
	if _, mostrable := CapDeHecho(tipo); mostrable {
		t.Error("lo desconocido resultó mostrable: el default tiene que ser NO mostrar")
	}
}

// Las cuatro operaciones internas que el cerebro encola HOY sí están clasificadas, y cada una en
// el plano que le corresponde. Sin esto, el fail-closed de arriba escondería la mitad de la
// cronología real y nadie lo notaría hasta usarla.
func TestLasOperacionesInternasDeHoyEstanClasificadas(t *testing.T) {
	casos := map[string]struct {
		tipo TipoDeHecho
		cap  Cap
	}{
		OpPantalla:  {HechoCanalPantalla, CapScreenView},
		OpAvisar:    {HechoCanalPantalla, CapScreenView},
		OpPreguntar: {HechoCanalPantalla, CapScreenView},
		OpShell:     {HechoCanalShell, CapShell},
	}
	for op, quiero := range casos {
		tipo := TipoDeArgv([]string{op, "id"})
		if tipo != quiero.tipo {
			t.Errorf("%s se clasificó como %q, esperaba %q", op, tipo, quiero.tipo)
		}
		capacidad, mostrable := CapDeHecho(tipo)
		if !mostrable || capacidad != quiero.cap {
			t.Errorf("%s pide %q (mostrable=%v), esperaba %q", op, capacidad, mostrable, quiero.cap)
		}
		if plano, _ := PlanoDeHecho(tipo); plano != PlanoEntrar {
			t.Errorf("%s cayó en el plano %q: las operaciones internas son del plano de ENTRAR", op, plano)
		}
	}
	// Y un comando de verdad NO es una operación interna.
	if tipo := TipoDeArgv([]string{"systemctl", "restart", "nginx"}); tipo != HechoComando {
		t.Errorf("un comando del host se clasificó como %q", tipo)
	}
}

// La contraseña de una sesión de pantalla NUNCA sale por el argv, en ninguna superficie.
//
// Sabotaje: que ArgvDeBitacora devuelva `argv` tal cual → falla acá, y la cronología entregaría
// contraseñas de sesión a quien pueda leerla.
func TestElArgvDeBitacoraNuncaLlevaLaContrasena(t *testing.T) {
	const secreto = "ContraseñaDeSesión123"
	limpio := ArgvDeBitacora([]string{OpPantalla, "ses-42", secreto, "30m0s"})
	if strings.Contains(strings.Join(limpio, " "), secreto) {
		t.Fatalf("la contraseña sobrevivió al saneo: %v", limpio)
	}
	// El id se conserva a propósito: es con lo que se cruza contra la bitácora de pantalla.
	if len(limpio) < 2 || limpio[1] != "ses-42" {
		t.Errorf("se perdió el id de sesión, que es lo único que sirve para cruzar: %v", limpio)
	}
	// Un comando común no se toca.
	crudo := []string{"journalctl", "-u", "nginx"}
	if got := ArgvDeBitacora(crudo); strings.Join(got, " ") != strings.Join(crudo, " ") {
		t.Errorf("un comando común no se debe tocar: %v", got)
	}
	// El hecho construido tampoco lo lleva: es la puerta por la que pasa TODA la cronología.
	h := HechoDeComando(Comando{Argv: []string{OpPantalla, "ses-42", secreto}}, "pc")
	if strings.Contains(strings.Join(h.Argv, " "), secreto) {
		t.Fatalf("HechoDeComando dejó pasar la contraseña: %v", h.Argv)
	}
}

// La ventana es SEMIABIERTA: incluye el comienzo y excluye el final. Dos ventanas consecutivas no
// cuentan dos veces el hecho del borde.
//
// Sabotaje: usar `!t.After(v.Hasta)` en Contiene → el hecho de las 12:00 cae en las dos ventanas
// y sumar los dos tramos da un total que no existe.
func TestLaVentanaEsSemiabierta(t *testing.T) {
	base := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	manana := Ventana{Desde: base.Add(-12 * time.Hour), Hasta: base}
	tarde := Ventana{Desde: base, Hasta: base.Add(12 * time.Hour)}

	if !tarde.Contiene(base) {
		t.Error("el instante del borde tiene que caer en la ventana que EMPIEZA ahí")
	}
	if manana.Contiene(base) {
		t.Error("el instante del borde NO puede caer también en la ventana que TERMINA ahí")
	}
	if manana.Contiene(base.Add(-13 * time.Hour)) {
		t.Error("un instante anterior al `desde` no puede caer adentro")
	}
}

// La ventana se lleva a la granularidad del ALMACENAMIENTO —el segundo— redondeando hacia AFUERA,
// y una punta sin fracción no se mueve.
//
// Sabotaje: truncar las dos puntas hacia abajo → una ventana que termina «ahora» excluye lo que
// acaba de pasar, y quien reinicia un servicio y entra a mirar ve la cronología vacía. Fue un bug
// real de esta misma tanda, encontrado por el control POSITIVO del barrido de aislamiento.
func TestLaVentanaSeNormalizaHaciaAfuera(t *testing.T) {
	// Punta de arriba con fracción: se redondea hacia ARRIBA, así entra el hecho de ese segundo.
	ahora := time.Date(2026, 8, 29, 22, 29, 58, 700_000_000, time.UTC)
	v := Ventana{Desde: ahora.Add(-time.Hour), Hasta: ahora}.Normalizada()
	hecho := time.Date(2026, 8, 29, 22, 29, 58, 0, time.UTC) // como lo guarda RFC3339: sin fracción
	if !v.Contiene(hecho) {
		t.Fatalf("el hecho de %s quedó afuera de una ventana que termina en %s", hecho.Format(time.RFC3339Nano), v.Hasta.Format(time.RFC3339Nano))
	}

	// Punta de abajo con fracción: se redondea hacia ABAJO, así no se pierde el hecho del borde.
	v2 := Ventana{Desde: ahora, Hasta: ahora.Add(time.Hour)}.Normalizada()
	if !v2.Contiene(hecho) {
		t.Errorf("el hecho de %s quedó afuera de una ventana que empieza en %s", hecho.Format(time.RFC3339Nano), v2.Desde.Format(time.RFC3339Nano))
	}

	// Sin fracción NO se mueve, y eso es lo que conserva el mosaico.
	entera := Ventana{
		Desde: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC),
		Hasta: time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC),
	}
	if n := entera.Normalizada(); !n.Desde.Equal(entera.Desde) || !n.Hasta.Equal(entera.Hasta) {
		t.Errorf("una ventana en segundos enteros no se debe mover: %s→%s quedó %s→%s",
			entera.Desde, entera.Hasta, n.Desde, n.Hasta)
	}
}

// Una ventana mal armada NO se convierte en «traeme todo». Fail-closed.
//
// Sabotaje: que Valida devuelva nil siempre → un `desde` vacío consultaría desde el año cero.
func TestUnaVentanaInvalidaNoSeConvierteEnTraemeTodo(t *testing.T) {
	ahora := time.Now().UTC()
	casos := map[string]Ventana{
		"sin desde":      {Hasta: ahora},
		"sin hasta":      {Desde: ahora.Add(-time.Hour)},
		"vacía":          {},
		"al revés":       {Desde: ahora, Hasta: ahora.Add(-time.Hour)},
		"de largo cero":  {Desde: ahora, Hasta: ahora},
		"más del máximo": {Desde: ahora.Add(-VentanaMax - time.Hour), Hasta: ahora},
	}
	for nombre, v := range casos {
		if err := v.Valida(); err == nil {
			t.Errorf("la ventana %q pasó la validación y no debería", nombre)
		}
	}
	buena := VentanaHasta(ahora, 6*time.Hour)
	if err := buena.Valida(); err != nil {
		t.Errorf("una ventana buena fue rechazada: %v", err)
	}
	if d := buena.Duracion(); d != 6*time.Hour {
		t.Errorf("duración = %s, esperaba 6h", d)
	}
}

// VentanaHasta acota sola: pedir más del máximo da el máximo, no un error ni una ventana enorme.
func TestVentanaHastaAplicaLosDefaults(t *testing.T) {
	ahora := time.Now().UTC()
	if d := VentanaHasta(ahora, 0).Duracion(); d != VentanaDefault {
		t.Errorf("sin duración, esperaba el default %s, obtuve %s", VentanaDefault, d)
	}
	if d := VentanaHasta(ahora, 365*24*time.Hour).Duracion(); d != VentanaMax {
		t.Errorf("pedir de más tiene que dar el máximo %s, obtuve %s", VentanaMax, d)
	}
}

// El orden es del más nuevo al más viejo, y el desempate es ESTABLE: dos hechos del mismo instante
// —lo que pasa cuando se abre una pantalla, porque la sesión y su comando de canal se escriben
// juntos— tienen que salir siempre en el mismo orden.
//
// Sabotaje: sacar el desempate por referencia → el orden de esos dos depende del orden de lectura
// y la lista se reordena sola entre llamadas.
func TestOrdenarHechosEsEstableYDelMasNuevoAlMasViejo(t *testing.T) {
	t0 := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	mismo := t0.Add(time.Hour)
	arme := func() []Hecho {
		return []Hecho{
			{Cuando: t0, Referencia: "viejo"},
			{Cuando: mismo, Referencia: "zzz"},
			{Cuando: mismo, Referencia: "aaa"},
			{Cuando: t0.Add(2 * time.Hour), Referencia: "nuevo"},
		}
	}
	a, b := arme(), arme()
	OrdenarHechos(a)
	OrdenarHechos(b)

	quiero := []string{"nuevo", "aaa", "zzz", "viejo"}
	for i := range quiero {
		if a[i].Referencia != quiero[i] {
			t.Fatalf("orden = %s en la posición %d, esperaba %s", a[i].Referencia, i, quiero[i])
		}
		if b[i].Referencia != a[i].Referencia {
			t.Fatalf("dos ordenamientos de la misma lista dieron distinto en la posición %d", i)
		}
	}
}

// La duración dice SI SE SABE. Un 0 devuelto a secas se dibuja como «duró nada» y lo que pasa es
// que sigue en curso — el mismo cero mentiroso que persigue todo el track, en el eje del tiempo.
//
// Sabotaje: devolver sólo la duración → un comando pendiente se dibuja como instantáneo.
func TestLaDuracionDiceSiSeSabe(t *testing.T) {
	inicio := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	if _, hay := (Hecho{Cuando: inicio}).Duracion(); hay {
		t.Error("un hecho sin `termino` no puede reportar duración: todavía está en curso")
	}
	d, hay := Hecho{Cuando: inicio, Termino: inicio.Add(90 * time.Second)}.Duracion()
	if !hay || d != 90*time.Second {
		t.Errorf("duración = %s (%v), esperaba 90s", d, hay)
	}
	// Un `termino` anterior al comienzo es dato corrupto, no una duración negativa.
	if _, hay := (Hecho{Cuando: inicio, Termino: inicio.Add(-time.Minute)}).Duracion(); hay {
		t.Error("un `termino` anterior al comienzo no puede dar una duración")
	}
}

// Lo que la cronología NO vio se declara, y la lista no puede quedar vacía: una respuesta sin
// huecos declarados se lee como «esto es todo lo que pasó».
//
// Sabotaje: devolver nil desde HuecosDeLaCronologia → falla acá.
func TestLaCronologiaDeclaraLoQueNoVio(t *testing.T) {
	huecos := HuecosDeLaCronologia()
	if len(huecos) == 0 {
		t.Fatal("la cronología tiene que declarar qué NO contiene")
	}
	junto := strings.ToLower(strings.Join(huecos, " | "))
	for _, obligatorio := range []string{"serie temporal", "log", "servicio", "política", "contenido"} {
		if !strings.Contains(junto, obligatorio) {
			t.Errorf("falta declarar el hueco de %q en: %s", obligatorio, junto)
		}
	}
}

// UN COMANDO PENDIENTE Y VIEJO SE MUESTRA `expirado`, aunque la fila diga otra cosa.
//
// `expirado` sólo se ESTAMPA cuando el agente viene a pedir su cola. Si el agente no vuelve
// nunca, nadie estampa nada — y la fila dice `pendiente` para siempre. Medido en producción:
// cincuenta comandos de diez horas con una vida máxima de quince minutos.
//
// Sabotaje: que EstadoActual devuelva `c.Estado` a secas → falla acá.
func TestUnComandoPendienteYViejoSeMuestraExpirado(t *testing.T) {
	ahora := time.Date(2026, 8, 30, 5, 0, 0, 0, time.UTC)

	viejo := Comando{Estado: EstadoPendiente, Creado: ahora.Add(-10 * time.Hour)}
	if got := viejo.EstadoActual(ahora); got != EstadoExpirado {
		t.Errorf("un pendiente de 10 h se muestra %q, esperaba %q", got, EstadoExpirado)
	}

	// EL DE RECIÉN NO VENCE, y el control importa: sin él, un EstadoActual que devolviera
	// `expirado` siempre pasaría la aserción de arriba.
	nuevo := Comando{Estado: EstadoPendiente, Creado: ahora.Add(-time.Minute)}
	if got := nuevo.EstadoActual(ahora); got != EstadoPendiente {
		t.Errorf("un pendiente de 1 min se muestra %q, esperaba %q", got, EstadoPendiente)
	}

	// LO ENTREGADO NO VENCE POR ACÁ. Su reloj es el timeout del comando, no la vida en la cola:
	// marcar expirado a los 15 min haría que un comando legítimo de 9 minutos aparezca muerto
	// mientras corre. Que un `entregado` que nunca reporta se quede así es OTRO agujero (A60).
	corriendo := Comando{Estado: EstadoEntregado, Creado: ahora.Add(-10 * time.Hour)}
	if got := corriendo.EstadoActual(ahora); got != EstadoEntregado {
		t.Errorf("un entregado viejo se muestra %q: su reloj es el timeout, no ComandoVidaMax", got)
	}

	// Y lo terminado no se toca nunca.
	listo := Comando{Estado: EstadoTerminado, Creado: ahora.Add(-10 * time.Hour)}
	if got := listo.EstadoActual(ahora); got != EstadoTerminado {
		t.Errorf("un terminado viejo se muestra %q", got)
	}
}

// EL ORIGEN VACÍO NO ES «PERSONA», y ésa es toda la regla de A59.
//
// Las filas anteriores a la migración 41 no dicen quién las originó. Rellenarlas con `persona`
// haría que cada disparo automático viejo figure como una acción humana — en la cronología de una
// máquina eso es atribuirle a alguien algo que no hizo.
//
// Sabotaje: que `EsAutomatico` devuelva `o != OrigenPersona` (lista NEGRA en vez de blanca) → lo
// desconocido pasa a contarse como automático, que es la mentira simétrica.
func TestUnOrigenDesconocidoNoEsPersonaNiAutomatico(t *testing.T) {
	if OrigenDesconocido.EsAutomatico() {
		t.Error("lo desconocido no puede contarse como automático")
	}
	if OrigenPersona.EsAutomatico() {
		t.Error("una persona no es automática")
	}
	if !OrigenPolitica.EsAutomatico() {
		t.Error("una política SÍ es automática: si no, la columna no distingue nada")
	}
	// Y lo desconocido tampoco es persona: la comparación tiene que poder distinguirlos.
	if OrigenDesconocido == OrigenPersona {
		t.Error("desconocido y persona no pueden ser el mismo valor")
	}
}

// Lo que no está en la lista se guarda como DESCONOCIDO, no como una categoría nueva.
//
// Fail-closed en el borde: un llamador futuro que mande `"cron"` no puede crear un valor que
// ninguna superficie sabe dibujar. Lo desconocido ya tiene significado; lo inventado, no.
//
// Sabotaje: que `OrigenValido` devuelva `o` tal cual → falla acá.
func TestUnOrigenRaroSeGuardaComoDesconocido(t *testing.T) {
	for _, raro := range []OrigenComando{"cron", "PERSONA", "politica ", "robot"} {
		if got := OrigenValido(raro); got != OrigenDesconocido {
			t.Errorf("OrigenValido(%q) = %q, esperaba desconocido", raro, got)
		}
	}
	for _, bueno := range []OrigenComando{OrigenPersona, OrigenPolitica} {
		if got := OrigenValido(bueno); got != bueno {
			t.Errorf("OrigenValido(%q) = %q: los válidos no se tocan", bueno, got)
		}
	}
}

// El hecho de la cronología ARRASTRA el origen del comando. Sin esto la columna existe en la
// tabla y no llega a ninguna superficie — que es el patrón de A58, otra vez.
//
// Sabotaje: no copiar `Origen` en HechoDeComando → falla acá.
func TestElHechoArrastraElOrigenDelComando(t *testing.T) {
	h := HechoDeComando(Comando{Argv: []string{"systemctl", "restart", "nginx"}, Origen: OrigenPolitica}, "pc")
	if h.Origen != OrigenPolitica {
		t.Errorf("el hecho perdió el origen: %q", h.Origen)
	}
	// Una sesión no tiene origen: la abre siempre alguien, y un campo vacío ahí se leería como
	// «no se sabe» cuando sí se sabe.
	if s := HechoDeSesionShell(SesionShell{}, "pc"); s.Origen != OrigenDesconocido {
		t.Errorf("una sesión no debería llevar origen: %q", s.Origen)
	}
}
