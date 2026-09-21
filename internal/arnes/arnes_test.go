package arnes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ═════════════════════════════════════════════════════════════════════════════════════════════
// EL ARNÉS QUE AUDITA GUARDAS TIENE QUE SER AUDITADO POR UNA GUARDA
//
// Este paquete existe para cazar guardas que quedan verdes sobre su propio defecto. Si el lector
// tiene un agujero, el resultado es el peor de todos: un censo que dice «la deuda es N» cuando es
// más grande, y una cobertura que dice «mecanizado» sobre sabotajes que no se aplican. Un agujero
// en el enumerador se ve IDÉNTICO a un árbol sano.
//
// Ya pasó dos veces escribiendo esto, y las dos quedan clavadas abajo:
//
//  1. LA ORACIÓN QUE SE ENVUELVE. La primera versión contó 780 anclas donde hay 774: las 6 de más
//     eran menciones a mitad de oración que cayeron al empezar el renglón porque el comentario se
//     envolvió ahí. Una deuda inflada es tan inútil como una subcontada.
//
//  2. EL CORTE POR BYTE. El impresor cortaba la prosa con `prosa[:110]` y partía un carácter
//     acentuado al medio: el listado salía con UTF-8 inválido y la comparación contra el grep
//     inventó CINCO anclas perdidas que no existían. Casi acuso al lector de un agujero que
//     estaba en el impresor.
//
// LA TABLA DE ABAJO ES LA FORMA QUE ESTE REPO YA PROBÓ (`TestCadaSabotajeDelCiCae`): un control
// sano primero, y después cada sabotaje como un cambio de UNA cosa que tiene que caer. En memoria,
// sin tocar el árbol, así que corre en milisegundos y entra en CI.

// ── 1 · EL LECTOR RECONOCE LAS FORMAS QUE EL ÁRBOL YA ESCRIBIÓ ─────────────────────────────────
//
// Las 30 formas no se derivan del árbol A PROPÓSITO: derivarlas de lo que el árbol tiene hoy
// dejaría a esta guarda midiéndose contra el mismo texto que el lector ya acepta, o sea espejo.
// Son un HECHO DEL MUNDO —así escribe este repo sus sabotajes— y por eso van clavadas.
//
// Sabotaje que la hace fallar: exigirle a esAncla la frase canónica.
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="if !strings.HasPrefix(bajo, \"sabotaje\") {"
// arnes: a="if !strings.HasPrefix(bajo, \"sabotaje que la hace fallar\") {"
func TestElLectorReconoceLasTreintaFormasDelAncla(t *testing.T) {
	formas := []string{
		"Sabotaje que la hace fallar: X",
		"Sabotaje: X",
		"Sabotaje que la pone roja: X",
		"Sabotaje que lo hace fallar: X",
		"Sabotaje visto rojo: X",
		"Sabotaje que la hace fallar (VERIFICADO): X",
		"Sabotaje verificado que la pone en rojo: X",
		"Sabotajes que la ponen en rojo (verificados): X",
		"Sabotaje verificado: X",
		"Sabotajes: X",
		"Sabotaje medido: X",
		"Sabotaje que todavía vale: X",
		"Sabotajes MEDIDOS el 2026-09-05: X",
		"Sabotaje medido, con los dos cambios plausibles a la vez: X",
		"Sabotaje que la hace fallar, corriendo esta prueba en Linux: X",
		// LA FORMA EN MAYÚSCULAS, que es la que descubrió esta prueba: hay 42 anclas escritas así
		// y ni el grep canónico ni el cabo A123 las ven. El registro contaba 396 sobre 774.
		"SABOTAJE: X",
		"SABOTAJES CORRIDOS: X",
		"SABOTAJES CORRIDOS, LOS TRES: X",
		"SABOTAJE QUE LA PONE EN ROJO (verificado): X",
		"SABOTAJE QUE LA HACE FALLAR, verificado y corrido: X",
		"SABOTAJE QUE TIENE QUE ROMPERLO: X",
		"SABOTAJES CORRIDOS, Y UNO SALIÓ FALSO — QUEDA ESCRITO PORQUE ENSEÑA: X",
		// Con viñeta, que es como quedan adentro de una lista.
		"· Sabotaje: X",
		"- Sabotaje que la hace fallar: X",
		"* Sabotaje: X",
	}
	for _, f := range formas {
		// anterior="" = arranque de grupo, que es donde vive un encabezado.
		if !esAncla(f, "") {
			t.Errorf("el lector NO reconoce esta forma de ancla, así que su deuda quedaría sin contar: %q", f)
		}
	}
}

// ── 2 · Y NO CUENTA LO QUE NO PROMETE NADA ─────────────────────────────────────────────────────
//
// Sabotaje que la hace fallar: hacer que esAncla acepte cualquier línea que empiece con la palabra
// sin mirar si la anterior cerró oración.
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="\treturn cierraOracion(anterior)"
// arnes: a="\treturn true"
func TestElLectorNoCuentaUnaMencionAMitadDeOracion(t *testing.T) {
	casos := []struct {
		nombre   string
		texto    string
		anterior string
	}{
		{
			// EL CASO MEDIDO, de internal/mcp/consentimiento_matriz_test.go:254. El ancla de
			// verdad está cuatro líneas más abajo; ésta es el final de una oración que se envolvió.
			"la oración que se envuelve",
			"sabotaje: cambiar `ShellSinPermiso` por `ShellAbriendo` en ResponderConsentimientoDeShell —o",
			"La matriz mide que se PREGUNTE. Que la respuesta llegue y mande es otra cosa, y lo descubrió un",
		},
		{
			"otra envuelta, de despliegue_lectura_filtrada_test.go:31",
			"sabotaje sale verde.",
			"sostiene la inversión. Sin ella la lista vuelve a crecer sola, y nadie se entera hasta que un",
		},
		{
			"la palabra a mitad de línea no abre nada",
			"Es la falla 2 de `deploy/pruebas/sabotaje.sh` y la forma dominante del día.",
			"",
		},
		{
			"sin dos puntos no hay encabezado",
			"Sabotaje aplicado y restaurado con cp",
			"",
		},
	}
	for _, c := range casos {
		if esAncla(c.texto, c.anterior) {
			t.Errorf("%s: el lector contó como ancla algo que no promete un sabotaje, y eso INFLA la deuda: %q",
				c.nombre, c.texto)
		}
	}

	// LA OTRA DIRECCIÓN, y sin esto la prueba de arriba se satisface con un `return false`: la
	// misma línea en minúscula SÍ es un ancla cuando la anterior cerró oración.
	if !esAncla("sabotaje: cambiar X por Y", "Lo que sigue es el defecto.") {
		t.Error("un ancla en minúscula después de una oración cerrada TIENE que contar: si no, una " +
			"escrita así queda muda justo donde nadie la revisaría")
	}
	if !esAncla("sabotaje: cambiar X por Y", "════════════════════") {
		t.Error("una regla de separación cierra sección: lo que sigue arranca de cero")
	}
}

// ── 3 · LA DIRECTIVA LA LEE EL SCANNER DE GO, ASÍ QUE EL ESCAPADO NO ES NUESTRO PROBLEMA ───────
//
// Sabotaje que la hace fallar: reemplazar el `strconv.Unquote` por un parser propio de comillas
// —`strings.Trim(lit, "\"")`, que es lo que uno escribe a mano— y los cinco casos con escapes y el
// literal crudo se caen.
//
// EL SABOTAJE LLEVA UN `_ = strconv.Quote` PEGADO, Y ESO NO ES ADORNO. La primera versión sacaba
// el `strconv.Unquote` a secas y era el ÚNICO uso del import: el paquete quedaba con `imported and
// not used` y el arnés informó «EL SABOTAJE ROMPE LA COMPILACIÓN, así que no prueba nada». Es la
// clase estructural que A107 midió el 2026-09-05 y que explica los 12 sabotajes que habían quedado
// sin veredicto: cuando la guarda es el único lector de un import o de una variable, el sabotaje
// LITERAL no puede compilar nunca, y un rojo por build roto se lee igual que un rojo por guarda
// que funciona. La salida es mantener el import vivo y cambiar sólo la conducta.
//
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="\t\t\tv, err := strconv.Unquote(lit)"
// arnes: a="\t\t\tv, err := strings.Trim(lit, string('\"')), error(nil)\n\t\t\t_ = strconv.Quote"
func TestLaDirectivaAceptaTodasLasFormasDeLiteralDeGo(t *testing.T) {
	casos := []struct {
		nombre  string
		payload string
		clave   string
		espera  string
	}{
		{"comillas dobles", ` de="hola"`, "de", "hola"},
		{"escape de comilla adentro", ` de="dice \"hola\""`, "de", `dice "hola"`},
		{"barra invertida", ` de="c:\\temp"`, "de", `c:\temp`},
		{"salto de línea escapado", ` de="a\nb"`, "de", "a\nb"},
		{"tabulación escapada", ` de="\tif x {"`, "de", "\tif x {"},
		// EL LITERAL CRUDO ES EL QUE HACE FALTA DE VERDAD: el texto a sabotear trae comillas
		// dobles todo el tiempo (`rpcErrorf(code, "…")`), y con backticks no hay que escapar nada.
		{"literal crudo con backtick", " de=`if s != \"\" {`", "de", `if s != "" {`},
		{"vacío es válido en `a`: borra", ` a=""`, "a", ""},
		{"multilínea", " archivo=\"x.go\"\n de=\"y\"", "de", "y"},
	}
	for _, c := range casos {
		campos, quejas := camposDe(c.payload)
		if len(quejas) > 0 {
			t.Errorf("%s: la directiva %q no se pudo leer: %v", c.nombre, c.payload, quejas)
			continue
		}
		if got := campos[c.clave]; got != c.espera {
			t.Errorf("%s: `%s` leyó %q y esperaba %q", c.nombre, c.clave, got, c.espera)
		}
	}
}

// ── 4 · Y DENUNCIA LO QUE NO ENTIENDE, EN VEZ DE SALTEARLO ─────────────────────────────────────
//
// ES LA LECCIÓN MÁS CARA DE ESTA SEMANA Y ES DE LA MISMA FAMILIA: la guarda del candado que
// escribí ayer descartaba SIETE tools en silencio porque su extractor no sabía leer
// `handler: s.envoltorio(s.toolX)`. Una de esas siete era el caso de 8 segundos. Un lector que
// saltea lo que no entiende produce el mismo verde que un árbol sano.
//
// Sabotaje que la hace fallar: en camposDe, cambiar el `default` que se queja por un `continue`.
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="\t\tdefault:\n\t\t\tquejas = append(quejas, fmt.Sprintf(\"no esperaba %q en la directiva\", lit))"
// arnes: a="\t\tdefault:\n\t\t\tcontinue"
func TestLaDirectivaIlegibleSeDenunciaYNoSeSaltea(t *testing.T) {
	casos := []struct {
		nombre  string
		payload string
		enQueja string
	}{
		{"clave desconocida", ` archivoo="x.go"`, "clave desconocida"},
		{"valor sin comillas", ` de=hola`, "literal de string de Go"},
		{"falta el igual", ` de "hola"`, "esperaba `=`"},
		{"se corta", ` de=`, "se corta"},
		{"clave repetida", ` de="a" de="b"`, "está dos veces"},
		// ESTE CASO LO AGREGÓ EL ARNÉS, Y ES LA PRIMERA GUARDA HUECA QUE ENCONTRÓ. El sabotaje
		// declarado acá arriba —cambiar el `default` que se queja por un `continue`— dejaba esta
		// prueba en VERDE, porque los cinco casos de arriba caen todos en una rama ANTERIOR
		// (esperaValor, esperaIgual, clave desconocida) y ninguno llegaba al `default`. O sea que
		// la rama que atrapa un token inesperado no estaba cubierta por nada.
		{"operador suelto", ` de="a" + "b"`, "no esperaba"},
	}
	for _, c := range casos {
		_, quejas := camposDe(c.payload)
		if len(quejas) == 0 {
			t.Errorf("%s: %q se leyó sin una queja. Una directiva que no se entiende y no se "+
				"denuncia se cuenta como mecanizada y nadie la corre nunca.", c.nombre, c.payload)
			continue
		}
		if !strings.Contains(strings.Join(quejas, " | "), c.enQueja) {
			t.Errorf("%s: la queja no nombra el problema (%q): %v", c.nombre, c.enQueja, quejas)
		}
	}
}

// ── 5 · LA UNICIDAD DEL `de` ES LA GUARDA QUE IMPIDE EL SABOTAJE FANTASMA ──────────────────────
//
// Un sabotaje que NO SE APLICA y una guarda que NO CUBRE dan exactamente el mismo verde. Este repo
// lo midió: «el `-run` del sabotaje escrito a mano» tiene tres caras y la tercera es que el
// sabotaje no se aplique. Con `de` ambiguo tampoco se sabe qué se tocó.
//
// Sabotaje que la hace fallar: aflojar la exigencia a `n < 1`.
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="\tif n := strings.Count(string(b), de); n != 1 {"
// arnes: a="\tif n := strings.Count(string(b), de); n < 1 {"
func TestAplicarExigeQueElTextoSeaUnico(t *testing.T) {
	dir := t.TempDir()
	escribir := func(nombre, contenido string) string {
		ruta := filepath.Join(dir, nombre)
		if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
			t.Fatal(err)
		}
		return ruta
	}

	t.Run("cero veces es un error, no un no-op", func(t *testing.T) {
		ruta := escribir("cero.go", "package p\n")
		err := Aplicar(ruta, "no existe", "x")
		if err == nil {
			t.Fatal("Aplicar aceptó un `de` que no está: el sabotaje no se aplicaría y el verde " +
				"que venga después se leería como «la guarda cubre»")
		}
		if !strings.Contains(err.Error(), "0 veces") {
			t.Errorf("el error no dice cuántas veces apareció: %v", err)
		}
	})

	t.Run("dos veces es un error: no se sabe cuál se tocó", func(t *testing.T) {
		ruta := escribir("dos.go", "package p\nvar a = 1\nvar b = 1\n")
		if err := Aplicar(ruta, "= 1", "= 2"); err == nil {
			t.Fatal("Aplicar aceptó un `de` ambiguo")
		}
	})

	t.Run("una vez se aplica", func(t *testing.T) {
		ruta := escribir("una.go", "package p\nvar a = 1\n")
		if err := Aplicar(ruta, "= 1", "= 2"); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(ruta)
		if !strings.Contains(string(b), "= 2") {
			t.Errorf("no se aplicó: %q", b)
		}
	})

	// EL MODO SE CONSERVA, Y NO ES UN DETALLE: `cp` sobre un archivo que existe no lo toca, así
	// que un reemplazo que REESCRIBE el archivo le deja el modo del umask. Pasó en este repo: el
	// guion quedó sin bit de ejecución y la corrida siguiente murió con `Permission denied`, que
	// manda a mirar cualquier cosa menos acá.
	//
	// SE COMPARA EL MODO CONTRA SÍ MISMO, NO CONTRA 0755, y eso lo enseñó el CI de Windows. La
	// primera versión clavaba el `0o755` y allá se puso roja: en Windows `Chmod` sólo mueve el bit
	// de sólo-lectura y `Stat` contesta 0666, así que la prueba acusaba a `Aplicar` de un defecto
	// que era un HECHO DEL SISTEMA OPERATIVO. Escribir a mano el valor esperado en vez de medir la
	// línea de base es la misma familia que «un derivado escrito a mano es una copia», dada vuelta.
	//
	// Lo que `Aplicar` promete no es «el archivo queda en 0755»: es «el modo que tenía es el modo
	// que queda». Ése es el invariante y es cierto en los tres sistemas.
	t.Run("conserva el modo del archivo", func(t *testing.T) {
		ruta := escribir("ejecutable.sh", "#!/bin/sh\necho uno\n")
		if err := os.Chmod(ruta, 0o755); err != nil {
			t.Fatal(err)
		}
		previo, err := os.Stat(ruta)
		if err != nil {
			t.Fatal(err)
		}
		antes := previo.Mode().Perm()

		if err := Aplicar(ruta, "uno", "dos"); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(ruta)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != antes {
			t.Errorf("el modo pasó de %04o a %04o: un sabotaje que le saca el bit de ejecución a "+
				"un guion rompe la corrida SIGUIENTE, y el síntoma no apunta acá", antes, got)
		}
		// Y SE DICE CUÁNDO EL CASO QUEDÓ DÉBIL. Si el sistema no llevó el bit de ejecución, este
		// subtest comparó 0666 contra 0666: comprobó que el modo no cambia, pero NO el bit que
		// motivó la guarda. Un verde que no midió lo que dice tiene que decirlo en vez de callarse.
		if antes&0o111 == 0 {
			t.Logf("este sistema no lleva bit de ejecución (tras Chmod 0755 el archivo quedó en %04o): "+
				"acá se comprobó que el modo no cambia, NO que sobreviva el bit de ejecución", antes)
		}
	})
}

// ── 6 · EL CENSO SOBRE UN ÁRBOL SINTÉTICO: LAS TRES CATEGORÍAS Y NADA MÁS ──────────────────────
//
// Sabotaje que la hace fallar: en directivaDe, devolver la directiva sin exigir `archivo` ni `de`.
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="\tif d.Archivo == \"\" {"
// arnes: a="\tif false {"
func TestElCensoPartaLasAnclasEnMecanizadaExentaOPendiente(t *testing.T) {
	dir := t.TempDir()
	fuente := "package p\n" +
		"\n" +
		"// Sabotaje que la hace fallar: cambiar uno por dos.\n" +
		"// arnes: archivo=\"prod.go\" de=\"uno\" a=\"dos\"\n" +
		"func TestMecanizada(t *testing.T) {}\n" +
		"\n" +
		"// Sabotaje: borrar la guarda entera.\n" +
		"// arnes: no_mecanizable=\"la guarda es el único lector de `vivo`: borrarla deja `declared and not used` y no compila\"\n" +
		"func TestExenta(t *testing.T) {}\n" +
		"\n" +
		"// Sabotaje que la pone roja: algo que nadie mecanizó todavía.\n" +
		"func TestPendiente(t *testing.T) {}\n" +
		"\n" +
		"// Sabotaje: una directiva que no se puede leer.\n" +
		"// arnes: archivo=\"prod.go\" de=hola\n" +
		"func TestIlegible(t *testing.T) {}\n" +
		"\n" +
		// ESTE CASO LO AGREGÓ EL ARNÉS, Y ES LA SEGUNDA GUARDA HUECA QUE ENCONTRÓ. El sabotaje
		// declarado arriba —cambiar `if d.Archivo == ""` por `if false`— dejaba esta prueba en
		// VERDE, porque las cuatro directivas sintéticas traían `archivo=` y ninguna ejercitaba
		// la exigencia. La guarda pedía un campo obligatorio que nada comprobaba.
		"// Sabotaje: una directiva sin el archivo a sabotear.\n" +
		"// arnes: de=\"uno\" a=\"dos\"\n" +
		"func TestSinArchivo(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(dir, "algo_test.go"), []byte(fuente), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prod.go"), []byte("package p\nvar x = \"uno\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := censarArchivo(dir, "algo_test.go")
	if err != nil {
		t.Fatal(err)
	}
	anclas, quejas, sinUbicar := r.anclas, r.quejas, r.sinUbicar
	if len(sinUbicar) != 0 {
		t.Errorf("el lector no pudo colocar %d ancla/s que vio en el texto: %v", len(sinUbicar), sinUbicar)
	}
	if len(anclas) != 5 {
		t.Fatalf("esperaba 5 anclas y encontré %d: %+v", len(anclas), anclas)
	}
	if len(quejas) == 0 {
		t.Error("la directiva `de=hola` (sin comillas) tenía que producir una queja")
	}
	// EL DENOMINADOR: cinco funciones Test en el fuente sintético, las cinco con ancla en su doc.
	// Sin esto, el conteo que este censo publica —774 anclas sobre 3.219 pruebas— no tiene guarda,
	// y es justo el número que otra sesión tuvo que corregirme.
	if r.funcionesTest != 5 || r.pruebasConAncla != 5 {
		t.Errorf("contó %d funciones Test y %d con ancla; el fuente tiene 5 y 5",
			r.funcionesTest, r.pruebasConAncla)
	}

	// LA PRUEBA SE DERIVA DEL AST, no se escribe a mano: es lo que hace que mover una prueba no
	// deje la directiva apuntando a un nombre que ya no existe.
	porNombre := map[string]Ancla{}
	for _, a := range anclas {
		porNombre[a.Prueba] = a
	}
	mec, ok := porNombre["TestMecanizada"]
	if !ok || mec.Directiva == nil {
		t.Fatal("la primera ancla tenía que quedar mecanizada y pegada a TestMecanizada")
	}
	if mec.Directiva.Prueba != "TestMecanizada" {
		t.Errorf("la prueba no se derivó del AST: %q", mec.Directiva.Prueba)
	}
	if mec.Directiva.Paquete != "./." {
		t.Errorf("el paquete se deriva del directorio del archivo de prueba; salió %q", mec.Directiva.Paquete)
	}
	if ex := porNombre["TestExenta"]; ex.NoMecanizable == "" || ex.Directiva != nil {
		t.Errorf("la segunda tenía que quedar EXENTA con su motivo, y salió %+v", ex)
	}
	if pe := porNombre["TestPendiente"]; pe.Directiva != nil || pe.NoMecanizable != "" {
		t.Errorf("la tercera tenía que quedar PENDIENTE (prosa y nada más), y salió %+v", pe)
	}
	// Y LA QUE NO DICE QUÉ ARCHIVO SABOTEAR TIENE QUE QUEDAR ROTA, NO MECANIZADA: sin `archivo`
	// no hay nada que tocar, y contarla como cubierta sube la cobertura con un sabotaje que no
	// existe.
	sa := porNombre["TestSinArchivo"]
	if len(sa.Quejas) == 0 {
		t.Error("una directiva sin `archivo=` pasó sin queja: el corredor no sabría qué sabotear")
	} else if !strings.Contains(strings.Join(sa.Quejas, " | "), "archivo") {
		t.Errorf("la queja no nombra el campo que falta: %v", sa.Quejas)
	}
	if (Censo{Anclas: []Ancla{sa}}).Mecanizadas() != nil {
		t.Error("una directiva con quejas se contó como MECANIZADA: así la cobertura sube con " +
			"sabotajes que nadie puede correr")
	}

	// Y VALIDAR TIENE QUE ACEPTAR LA QUE APUNTA BIEN.
	if males := Validar(Censo{Raiz: dir, Anclas: []Ancla{mec}}); len(males) != 0 {
		t.Errorf("Validar rechazó una directiva que apunta a un literal que existe y es único: %v", males)
	}
	// …Y RECHAZAR LA QUE DEJÓ DE APUNTAR. Es la otra dirección, y es la que impide que el corpus
	// se podra: 774 sabotajes no se pueden CORRER en CI, pero sus anclas se pueden comprobar.
	rota := mec
	d := *rota.Directiva
	d.De = "esto ya no está en prod.go"
	rota.Directiva = &d
	if males := Validar(Censo{Raiz: dir, Anclas: []Ancla{rota}}); len(males) == 0 {
		t.Error("Validar aceptó una directiva cuyo `de` ya no existe: ese sabotaje NO SE APLICARÍA " +
			"y su verde se leería como «la guarda cubre»")
	}
}

// ── 7 · UNA EXENCIÓN SIN MOTIVO ES UN `skip` ───────────────────────────────────────────────────
//
// `no_mecanizable` es la única salida del censo, así que es la única forma de vaciarlo. El motivo
// escrito es lo que obliga a defenderla — un allowlist sin razones se llena solo.
//
// Sabotaje que la hace fallar: bajar el mínimo a 0.
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="if len(strings.TrimSpace(motivo)) < 20 {"
// arnes: a="if len(strings.TrimSpace(motivo)) < 0 {"
func TestUnaExencionSinMotivoNoVale(t *testing.T) {
	lineas := []lineaCom{
		{linea: 1, texto: "Sabotaje: X", cruda: " Sabotaje: X"},
		{linea: 2, texto: `arnes: no_mecanizable="porque"`, cruda: ` arnes: no_mecanizable="porque"`},
	}
	_, motivo, quejas := directivaDe(lineas, "x_test.go", "TestX")
	if motivo == "" {
		t.Fatal("no leyó la exención")
	}
	if len(quejas) == 0 {
		t.Error("una exención con un motivo de seis letras pasó sin queja: así se vacía el censo")
	}
}

// ── 8 · EL CONTROL DE «MEDÍ ALGO»: SOBRE EL ÁRBOL DE VERDAD, NO PUEDE DAR CERO ─────────────────
//
// Un cero acá no es «el árbol no promete sabotajes»: es «este lector no miró nada», y son cosas
// opuestas que salen por la misma puerta. Todo lo que cuelgue del censo daría verde.
//
// Sabotaje que la hace fallar: hacer que ArchivosDelCorpus filtre por un patrón que no matchea
// nada, p. ej. `*_prueba.go`.
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="\"ls-files\", \"-z\", \"*.go\""
// arnes: a="\"ls-files\", \"-z\", \"*_prueba.go\""
// arnes: colision_ok="TestElCorpusIncluyeElCodigoDeProduccion"
func TestElLectorNoPuedeDevolverCeroSobreElArbolDeVerdad(t *testing.T) {
	raiz := filepath.Join("..", "..")
	archivos, err := ArchivosDelCorpus(raiz)
	if err != nil {
		t.Fatal(err)
	}
	if len(archivos) < 400 {
		t.Fatalf("git listó %d `.go` y el árbol tiene más de mil: este enumerador está mirando "+
			"otra cosa, y un censo chico se lee igual que una deuda chica", len(archivos))
	}
	c, err := Censar(raiz)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Anclas) == 0 {
		t.Fatal("cero anclas sobre el árbol real: el árbol promete cientos, así que esto es el " +
			"lector sin mirar nada")
	}
	// EL AGUJERO DEL LECTOR SE DENUNCIA ACÁ Y NO SE RESTA EN SILENCIO.
	if len(c.SinUbicar) > 0 {
		t.Errorf("el lector vio %d ancla/s en el texto crudo y no pudo colocarlas en el AST: %v",
			len(c.SinUbicar), c.SinUbicar)
	}
}

// ── 8b · EL CORPUS NO ES «LOS `_test.go`»: TAMBIÉN ES EL CÓDIGO DE PRODUCCIÓN ──────────────────
//
// SU HERMANA DE ARRIBA PREGUNTA «¿MEDÍ ALGO?» Y ÉSTA PREGUNTA «¿MEDÍ TODO?», que no es lo mismo y
// por eso son dos. Con el enumerador acotado a `*_test.go` la de arriba seguía verde —675 archivos
// son muchos más que 400— mientras el número que el censo publica dejaba de ser una propiedad del
// árbol para ser una de los `_test.go`. Medido: `cmd/musubi/precheck.go` llevaba dos anclas
// invisibles, y una de ellas era la única promesa de `TestPrecheckNoAbreLaBaseSiNoLeToca`.
//
// LA COMPROBACIÓN NO NOMBRA UN ARCHIVO. Pedir «que aparezca precheck.go» ataría esta guarda a que
// ese archivo conserve su ancla: el día que alguien la mueva, la guarda se pondría roja por una
// mudanza y no por un agujero. Pregunta por la FORMA —que el censo haya colocado al menos un ancla
// fuera de un `_test.go`— que es lo que el enumerador decide.
//
// Sabotaje que la hace fallar: volver a acotar el enumerador a `*_test.go`.
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="\"ls-files\", \"-z\", \"*.go\""
// arnes: a="\"ls-files\", \"-z\", \"*_test.go\""
// arnes: colision_ok="TestElLectorNoPuedeDevolverCeroSobreElArbolDeVerdad"
func TestElCorpusIncluyeElCodigoDeProduccion(t *testing.T) {
	raiz := filepath.Join("..", "..")
	archivos, err := ArchivosDelCorpus(raiz)
	if err != nil {
		t.Fatal(err)
	}
	produccion := 0
	for _, rel := range archivos {
		if !strings.HasSuffix(rel, "_test.go") {
			produccion++
		}
	}
	if produccion == 0 {
		t.Fatalf("de %d archivos enumerados, CERO son de producción: el censo estaría midiendo los "+
			"`_test.go` y publicando el número como si fuera del árbol", len(archivos))
	}

	c, err := Censar(raiz)
	if err != nil {
		t.Fatal(err)
	}
	fuera := 0
	for _, a := range c.Anclas {
		if !strings.HasSuffix(a.Archivo, "_test.go") {
			fuera++
		}
	}
	if fuera == 0 {
		t.Errorf("el censo enumeró %d archivos de producción y no colocó NI UN ancla en ellos. O el "+
			"árbol dejó de prometer sabotajes fuera de las pruebas —y entonces esta guarda sobra y "+
			"se saca a mano— o el lector los está mirando sin leerlos, que se ve igual.", produccion)
	}
}

// ── 8c · UN ANCLA PEGADA A UN HELPER NO HEREDA EL NOMBRE DEL HELPER ────────────────────────────
//
// La queja que cubre este caso dice «el ancla no está pegada a un `func Test…`», y durante toda su
// vida fue mentira: el mapa de pruebas aceptaba CUALQUIER función con doc, así que un ancla sobre
// un helper no se quedaba sin nombre —se quedaba con el nombre EQUIVOCADO—, y la queja no se
// disparaba nunca. Un `-run ^nombreDelHelper$` no matchea ninguna prueba: el sabotaje sale «sin
// veredicto», que es el desenlace que este arnés existe para sacar.
//
// El árbol tiene dos anclas exactamente ahí —`cmd/musubi/agent_test.go:488` y
// `internal/mcp/despliegue_alertas_test.go:459`— y las dos se salvan porque declaran su
// `prueba="…"` a mano. O sea que hoy no hay daño, y lo único que sostenía eso era la costumbre de
// quien las escribió.
//
// Sabotaje que la hace fallar: sacar el `if !esPrueba { continue }` de `censarArchivo` → el helper
// vuelve a prestarle su nombre al ancla.
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="\t\tif !esPrueba {\n\t\t\tcontinue\n\t\t}\n"
// arnes: a=""
func TestUnAnclaPegadaAUnHelperNoHeredaSuNombre(t *testing.T) {
	raiz := t.TempDir()
	rel := "x_test.go"
	// EL FIXTURE SE ARMA CONCATENANDO Y NO CON UN LITERAL CRUDO. Con backticks, las líneas
	// `// Sabotaje…` quedan FÍSICAS en este archivo: el control de `SinUbicar` las lee del texto
	// crudo, no las encuentra en el AST —viven adentro de un string— y denuncia un agujero del
	// lector que no existe. Ya pasó al escribir esta guarda.
	cuerpo := "package p\n\n" +
		"// ayudante arma el fixture.\n" +
		"//\n" +
		"// Sabotaje que la hace fallar: romper el fixture.\n" +
		"func ayudante() int { return 1 }\n\n" +
		"// TestDeVerdad mide algo.\n" +
		"//\n" +
		"// Sabotaje que la hace fallar: sacar la comprobación.\n" +
		"func TestDeVerdad() {}\n"
	if err := os.WriteFile(filepath.Join(raiz, rel), []byte(cuerpo), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := censarArchivo(raiz, rel)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.anclas) != 2 {
		t.Fatalf("esperaba las 2 anclas del fixture, vinieron %d: sin las dos no se puede comparar "+
			"el caso malo contra su control", len(r.anclas))
	}
	// EL CONTROL VA JUNTO AL CASO: si la derivación dejara de funcionar del TODO, el caso malo
	// daría verde por la razón equivocada y esta guarda no lo notaría.
	//
	// Las dos anclas se distinguen por su PROSA y no por su número de línea: agregar un renglón al
	// fixture correría los números y esta guarda pasaría a mirar el ancla que no es.
	var deHelper, deTest string
	for _, a := range r.anclas {
		switch {
		case strings.Contains(a.Prosa, "romper el fixture"):
			deHelper = a.Prueba
		case strings.Contains(a.Prosa, "sacar la comprobación"):
			deTest = a.Prueba
		}
	}
	if deTest != "TestDeVerdad" {
		t.Errorf("el ancla pegada a `func TestDeVerdad` derivó %q: la derivación que SÍ tiene que "+
			"andar dejó de andar, y sin ella el caso de abajo daría verde sin medir nada", deTest)
	}
	if deHelper != "" {
		t.Errorf("el ancla pegada a `func ayudante` derivó la prueba %q. No es un nombre que falte: "+
			"es uno equivocado, y `-run ^%s$` no matchea nada — el sabotaje saldría «sin veredicto» "+
			"en vez de denunciar que falta `prueba=\"…\"`", deHelper, deHelper)
	}
}

// ── 9 · LOS SABOTAJES QUE SE PISAN ENTRE SÍ ────────────────────────────────────────────────────
//
// POR QUÉ EXISTE, Y CÓMO APARECIÓ. La guarda del censo declaraba un sabotaje contra la unicidad de
// `Aplicar`, y esa guarda NO LLAMA a `Aplicar`. Daba ROJO igual en las siete corridas que se
// hicieron: al aplicarse, el sabotaje cambiaba una línea que OTRAS DOS directivas usan como su
// `de`, esas dos dejaban de apuntar, y `Validar` —que la guarda SÍ llama— denunciaba eso. La prueba
// caía por el daño al corpus y no por el defecto declarado.
//
// Fue un rojo real por el motivo equivocado, con el archivo correcto, la prueba correcta y una
// línea de fallo correcta: ninguna de las comprobaciones de `revisarElRojo` lo podía ver. Y
// sobrevivió porque un verde inesperado hace preguntar y un rojo esperado no — lo encontró un
// refutador ajeno que estaba midiendo otra cosa.
//
// LA COMPROBACIÓN NO ENUMERA FORMAS: simula el reemplazo y pregunta a quién le rompió el ancla.
//
// Sabotaje que la hace fallar: en `Colisiones`, cambiar `if len(anclas) < 2 {` por `if true {` →
// nunca compara ningún par y devuelve la lista vacía siempre, que es un cero que significa «no
// miré».
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="\t\tif len(anclas) < 2 {"
// arnes: a="\t\tif true {"
// arnes: prueba="TestSeDenuncianLosSabotajesQueSePisanEntreSi"
func TestSeDenuncianLosSabotajesQueSePisanEntreSi(t *testing.T) {
	raiz := t.TempDir()
	escribir := func(rel, cuerpo string) {
		ruta := filepath.Join(raiz, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, []byte(cuerpo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	escribir("prod.go", "package p\n\nfunc f() {\n\tif n != 1 {\n\t\treturn\n\t}\n}\n")

	censo := func(anclas ...Ancla) Censo {
		c := Censo{Raiz: raiz}
		c.Anclas = append(c.Anclas, anclas...)
		return c
	}
	ancla := func(arch string, lin int, de, a string) Ancla {
		return Ancla{Archivo: arch, Linea: lin, Directiva: &Directiva{Archivo: "prod.go", De: de, A: a}}
	}

	t.Run("dos directivas sobre la MISMA línea se denuncian, en los dos sentidos", func(t *testing.T) {
		c := censo(
			ancla("uno_test.go", 10, "\tif n != 1 {", "\tif n != 2 {"),
			ancla("otro_test.go", 20, "\tif n != 1 {", "\tif false {"),
		)
		males := Colisiones(c)
		if len(males) != 2 {
			t.Fatalf("esperaba 2 denuncias (una por sentido), vinieron %d:\n  %s",
				len(males), strings.Join(males, "\n  "))
		}
		junto := strings.Join(males, "\n")
		for _, quiero := range []string{"uno_test.go:10", "otro_test.go:20"} {
			if !strings.Contains(junto, quiero) {
				t.Errorf("la denuncia no nombra a %s, así que no se puede ir a mirar:\n%s", quiero, junto)
			}
		}
	})

	t.Run("una directiva cuyo `a` CONTIENE al `de` no pisa a nadie", func(t *testing.T) {
		// Es el caso de «agregar una línea»: el ancla del otro sigue estando entera.
		c := censo(
			ancla("uno_test.go", 10, "\tif n != 1 {", "\tif n != 1 {\n\t\t_ = 0"),
			ancla("otro_test.go", 20, "\tif n != 1 {", "\tif false {"),
		)
		if males := Colisiones(c); len(males) != 1 {
			t.Errorf("esperaba 1 (sólo el sentido que SÍ pisa), vinieron %d:\n  %s",
				len(males), strings.Join(males, "\n  "))
		}
	})

	t.Run("directivas sobre archivos distintos nunca se pisan", func(t *testing.T) {
		escribir("otro.go", "package p\n\nfunc g() {\n\tif n != 1 {\n\t}\n}\n")
		c := censo(ancla("uno_test.go", 10, "\tif n != 1 {", "\tif n != 2 {"))
		c.Anclas[0].Directiva.Archivo = "prod.go"
		c.Anclas = append(c.Anclas, Ancla{Archivo: "otro_test.go", Linea: 20,
			Directiva: &Directiva{Archivo: "otro.go", De: "\tif n != 1 {", A: "\tif false {"}})
		if males := Colisiones(c); len(males) != 0 {
			t.Errorf("dos archivos distintos no se pisan, vinieron %d:\n  %s",
				len(males), strings.Join(males, "\n  "))
		}
	})

	t.Run("una sola directiva en el archivo no puede pisar a nadie", func(t *testing.T) {
		c := censo(ancla("uno_test.go", 10, "\tif n != 1 {", "\tif n != 2 {"))
		if males := Colisiones(c); len(males) != 0 {
			t.Errorf("con una sola no hay par: %v", males)
		}
	})
}

// ── 10 · UNA DIRECTIVA QUE NO CUELGA DE NINGÚN ANCLA ───────────────────────────────────────────
//
// POR QUÉ EXISTE. Otra sesión escribió SEIS directivas —válidas, con `de`/`a` únicos y anclaje
// correcto— y este lector no las vio nunca. Ni como rotas: el censo dio el MISMO número que sin
// ellas, `-validar` contestó «✓ las 76 apuntan a un literal único», y las seis no se contaron, no
// se validaron y no se corrieron. Sus bloques empezaban con «MECANIZADA. El sabotaje es…»: la
// palabra está, pero no al empezar la línea, así que no había ancla de la cual colgarlas.
//
// HABÍA CATEGORÍA PARA «DIRECTIVA ILEGIBLE» Y NO PARA ÉSTA, y la huérfana es la peor de las dos:
// la rota avisa, y el que escribió la huérfana cree que la cobertura subió. Iba a abrir un PR
// diciendo «seis mecanizadas» con seis que no corrían.
//
// Y no me podía pasar a mí: mis directivas nacieron pegadas a anclas que ya existían. Es la
// lección aprendida de un lado y no del hermano, en el instrumento que existe para cazar eso.
//
// Sabotaje que la hace fallar: en censarArchivo, no marcar `consumidas[m.linea]` → todas las
// directivas del árbol pasan a contarse como huérfanas y el control de abajo cae.
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="\t\t\t\t\tconsumidas[m.linea] = true"
// arnes: a="\t\t\t\t\t_ = m"
// arnes: prueba="TestUnaDirectivaSinAnclaSeDenunciaYNoSePierde"
func TestUnaDirectivaSinAnclaSeDenunciaYNoSePierde(t *testing.T) {
	raiz := t.TempDir()
	escribir := func(rel, cuerpo string) {
		if err := os.WriteFile(filepath.Join(raiz, rel), []byte(cuerpo), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("la huerfana se denuncia con archivo y linea", func(t *testing.T) {
		escribir("h_test.go", "package p\n\n"+
			"// MECANIZADA. Esta línea NO empieza con la palabra canónica, así que no es un ancla.\n"+
			"// arnes: archivo=\"p.go\"\n"+
			"// arnes: de=\"uno\"\n"+
			"// arnes: a=\"dos\"\n"+
			"func TestAlgo(t *testing.T) {}\n")
		r, err := censarArchivo(raiz, "h_test.go")
		if err != nil {
			t.Fatal(err)
		}
		c := Censo{Raiz: raiz, Anclas: r.anclas, Huerfanas: r.huerfanas}
		if len(c.Huerfanas) != 3 {
			t.Fatalf("esperaba 3 líneas huérfanas denunciadas, vinieron %d: %v", len(c.Huerfanas), c.Huerfanas)
		}
		junto := strings.Join(c.Huerfanas, "\n")
		if !strings.Contains(junto, "h_test.go:4") {
			t.Errorf("la denuncia no lleva archivo:línea, así que no se puede ir a mirar:\n%s", junto)
		}
		// Y NO SE CUENTAN COMO COBERTURA, que es la mitad que importa.
		if n := len(c.Mecanizadas()); n != 0 {
			t.Errorf("una directiva huérfana se contó como mecanizada (%d): la cobertura subiría con "+
				"sabotajes que nadie corre", n)
		}
	})

	t.Run("con su ancla arriba deja de ser huerfana", func(t *testing.T) {
		escribir("s_test.go", "package p\n\n"+
			"// Sabotaje que la hace fallar: cambiar uno por dos.\n"+
			"// arnes: archivo=\"p.go\"\n"+
			"// arnes: de=\"uno\"\n"+
			"// arnes: a=\"dos\"\n"+
			"func TestAlgo(t *testing.T) {}\n")
		r, err := censarArchivo(raiz, "s_test.go")
		if err != nil {
			t.Fatal(err)
		}
		c := Censo{Raiz: raiz, Anclas: r.anclas, Huerfanas: r.huerfanas}
		// EL CONTROL DE QUE LA COMPROBACIÓN NO SEA UN «SIEMPRE HUÉRFANA»: sin este caso, marcar
		// todo como huérfano pasaría el subtest de arriba y dejaría la herramienta gritando
		// siempre, que es la otra forma de no medir.
		if len(c.Huerfanas) != 0 {
			t.Errorf("una directiva CON su ancla se denunció como huérfana: %v", c.Huerfanas)
		}
		if n := len(c.Mecanizadas()); n != 1 {
			t.Errorf("esperaba 1 mecanizada, vinieron %d", n)
		}
	})
}

// ── 11 · `tags` Y `env`: EL ENTORNO QUE LA PRUEBA NECESITA PARA EXISTIR ───────────────────────
//
// Sin estas dos claves el corredor sólo sabe correr `go test` PELADO, y una prueba detrás de un
// build tag o de un `t.Skip(os.Getenv(X) == "")` sale «sin veredicto». Medido el 2026-09-12 sobre
// el árbol entero: 85 corridas y 2 sin veredicto, las dos por eso. Ninguna era un hueco —las
// verifiqué a mano— pero verificar a mano no escala y lo que se mide a mano se deja de medir.
//
// LO QUE SE EXIGE ACÁ NO ES QUE FUNCIONEN: ES QUE UN ERROR DE ESCRITURA NO SE VUELVA SILENCIO.
// `tags="tree!sitter"` se lo come el toolchain, y `env="FOO=/x"` —la confusión natural, porque
// parece que fuera a SETEARLA— nunca va a estar, así que la directiva diagnosticaría «falta
// FOO=/x» para siempre. Las dos terminan en «sin veredicto», o sea en el desenlace exacto que
// estas claves existen para sacar: el defecto se disfraza de la enfermedad que cura.
//
// Sabotaje que la hace fallar: en `directivaDe`, sacar el bucle que valida `d.Env` —el
// `for _, e := range d.Env`— y con él la queja de `env`.
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="\t\tif !esNombreDeVariable(e) {"
// arnes: a="\t\tif false {"
// arnes: arreglo_de="\t\tif !esNombreDeVariable(e) {"
// arnes: arreglo_a="\t\tif !esIdentificadorDeBuild(e) {"
func TestTagsYEnvSeLeenYUnErrorDeEscrituraNoSeVuelveSilencio(t *testing.T) {
	leer := func(pares ...string) (*Directiva, []string) {
		lineas := []lineaCom{{linea: 1, texto: "Sabotaje: X", cruda: " Sabotaje: X"}}
		for i, p := range pares {
			lineas = append(lineas, lineaCom{linea: i + 2, texto: "arnes: " + p, cruda: " arnes: " + p})
		}
		d, _, quejas := directivaDe(lineas, "internal/x/x_test.go", "TestX")
		return d, quejas
	}
	base := []string{`archivo="x.go"`, `de="viejo"`, `a="nuevo"`}
	con := func(extra ...string) (*Directiva, []string) {
		return leer(append(append([]string{}, base...), extra...)...)
	}

	t.Run("los tags se separan por espacios y viajan como lista de GOFLAGS", func(t *testing.T) {
		// Se escriben como se escriben en la línea de comandos (`-tags 'a b'`) y se guardan como
		// los quiere GOFLAGS (`-tags=a,b`). Traducir acá es lo que evita que cada sitio de uso
		// invente su propia conversión.
		d, quejas := con(`tags="treesitter grammar_subset"`)
		if len(quejas) > 0 {
			t.Fatalf("tags bien escritos se quejaron: %v", quejas)
		}
		if d.Tags != "treesitter,grammar_subset" {
			t.Errorf("Tags = %q, esperaba %q", d.Tags, "treesitter,grammar_subset")
		}
	})

	t.Run("env declara NOMBRES, y un `NOMBRE=valor` se denuncia", func(t *testing.T) {
		d, quejas := con(`env="MUSUBI_SPM_TESTDATA OTRA"`)
		if len(quejas) > 0 {
			t.Fatalf("env bien escrito se quejó: %v", quejas)
		}
		if len(d.Env) != 2 || d.Env[0] != "MUSUBI_SPM_TESTDATA" || d.Env[1] != "OTRA" {
			t.Errorf("Env = %v, esperaba [MUSUBI_SPM_TESTDATA OTRA]", d.Env)
		}
		// LA MITAD QUE IMPORTA: escribirle un valor tiene que ser un rojo del censo, no un
		// diagnóstico eterno de «falta FOO=/x».
		_, quejas = con(`env="FOO=/x"`)
		if len(quejas) == 0 {
			t.Error("`env=\"FOO=/x\"` pasó sin queja: esa directiva no mediría NUNCA, " +
				"y su «sin veredicto» se leería como un límite de la herramienta")
		}
	})

	t.Run("un tag que el toolchain rechaza se denuncia acá, no noventa segundos después", func(t *testing.T) {
		if _, quejas := con(`tags="tree!sitter"`); len(quejas) == 0 {
			t.Error("un tag con `!` pasó sin queja: el toolchain lo rechaza en medio de la corrida")
		}
	})

	t.Run("CONTROL: sin declarar ninguna de las dos, la directiva sigue siendo válida", func(t *testing.T) {
		// Las 83 directivas que no necesitan entorno no tienen que escribir nada. Si esto se
		// pusiera rojo, la guarda estaría exigiendo las claves nuevas en todo el árbol.
		d, quejas := con()
		if len(quejas) > 0 {
			t.Fatalf("una directiva sin `tags` ni `env` se quejó: %v", quejas)
		}
		if d.Tags != "" || len(d.Env) != 0 {
			t.Errorf("sin declarar nada quedó Tags=%q Env=%v", d.Tags, d.Env)
		}
	})
}

// ── 12 · `colision_ok`: CONTESTAR UN AVISO SIN APAGAR EL DETECTOR ─────────────────────────────
//
// El aviso de `Colisiones` es una pregunta legítima —con el mismo `de`, o son dos guardas o es una
// contada dos veces— y a veces la respuesta es «dos». Ahí el aviso es CIERTO y no se puede apagar:
// sale en cada corrida, para siempre. Un aviso verdadero que nadie puede contestar entrena a
// ignorar los avisos, que es la misma familia que un cero que significa «no sé».
//
// LO PELIGROSO DE UNA CLAVE QUE SILENCIA ES QUE SILENCIE DE MÁS, así que la clave no dice «ya lo
// miré»: dice A QUIÉN. Una respuesta que nombra a un tercero no tapa esta colisión, y una que no
// se pisa con nadie se denuncia como rancia — porque «esto ya se miró» sobre algo que nadie miró
// es peor que el aviso que reemplaza.
//
// Sabotaje que la hace fallar: en `Colisiones`, que alcance con tener la clave puesta sin exigir
// que nombre a la prueba que se pisa → cualquier respuesta tapa cualquier colisión.
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="\t\t\t\tif x.Directiva.ContestaA(y.Directiva.Prueba) {"
// arnes: a="\t\t\t\tif x.Directiva.ColisionOk != \"\" {"
// arnes: prueba="TestUnaColisionContestadaSeCallaYUnaRespuestaRanciaSeDenuncia"
func TestUnaColisionContestadaSeCallaYUnaRespuestaRanciaSeDenuncia(t *testing.T) {
	raiz := t.TempDir()
	ruta := filepath.Join(raiz, "prod.go")
	if err := os.WriteFile(ruta, []byte("package p\n\nfunc f() {\n\tif n != 1 {\n\t\treturn\n\t}\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Las dos se pisan de verdad: el `a` de cada una destruye el `de` de la otra.
	const literal = "\tif n != 1 {"
	una := func(colisionOk string) Ancla {
		return Ancla{Archivo: "uno_test.go", Linea: 10, Directiva: &Directiva{
			Archivo: "prod.go", De: literal, A: "\tif n != 2 {",
			Prueba: "TestUna", ColisionOk: colisionOk,
		}}
	}
	otra := func(colisionOk string) Ancla {
		return Ancla{Archivo: "otro_test.go", Linea: 20, Directiva: &Directiva{
			Archivo: "prod.go", De: literal, A: "\tif false {",
			Prueba: "TestOtra", ColisionOk: colisionOk,
		}}
	}
	males := func(anclas ...Ancla) []string {
		return Colisiones(Censo{Raiz: raiz, Anclas: anclas})
	}

	t.Run("CONTROL: sin la clave, las dos direcciones se denuncian", func(t *testing.T) {
		// Si esto no diera 2, nada de lo que sigue significaría algo: un «no se denunció» sobre un
		// detector muerto se lee igual que sobre una colisión contestada.
		if m := males(una(""), otra("")); len(m) != 2 {
			t.Fatalf("esperaba 2 denuncias, vinieron %d:\n  %s", len(m), strings.Join(m, "\n  "))
		}
	})

	t.Run("la clave tapa SÓLO su dirección", func(t *testing.T) {
		// Contestar de un lado no contesta del otro: cada guarda declara lo que su autor miró.
		m := males(una("TestOtra"), otra(""))
		if len(m) != 1 {
			t.Fatalf("esperaba 1 (la dirección sin contestar), vinieron %d:\n  %s",
				len(m), strings.Join(m, "\n  "))
		}
		if !strings.Contains(m[0], "otro_test.go:20") {
			t.Errorf("la que quedó no es la que falta contestar:\n  %s", m[0])
		}
	})

	t.Run("contestadas las dos, el aviso se calla", func(t *testing.T) {
		if m := males(una("TestOtra"), otra("TestUna")); len(m) != 0 {
			t.Errorf("las dos contestaron y siguió avisando:\n  %s", strings.Join(m, "\n  "))
		}
	})

	t.Run("una respuesta que nombra a OTRO no tapa esta colisión", func(t *testing.T) {
		// LA MITAD QUE IMPIDE QUE LA CLAVE SEA UN INTERRUPTOR. Si alcanzara con tenerla puesta,
		// `colision_ok="TestLoQueSea"` apagaría un aviso que nadie miró.
		m := males(una("TestUnTercero"), otra("TestUna"))
		if len(m) == 0 {
			t.Fatal("una respuesta que no nombra a la prueba que se pisa tapó la colisión: " +
				"la clave dejó de ser una respuesta y pasó a ser un interruptor")
		}
		junto := strings.Join(m, "\n")
		if !strings.Contains(junto, "uno_test.go:10") {
			t.Errorf("la denuncia no nombra el sitio que contestó mal:\n%s", junto)
		}
		// Y ADEMÁS SE DICE QUE LA RESPUESTA NO CONTESTA NADA: sin eso, el autor ve el aviso, cree
		// que su clave no llegó a leerse, y la escribe de nuevo igual de mal.
		if !strings.Contains(junto, "TestUnTercero") {
			t.Errorf("no se dice a quién decía contestarle, así que no se puede corregir:\n%s", junto)
		}
		// TIENEN QUE SALIR LAS DOS QUEJAS, Y NO ALCANZA CON CONTAR. Acá había una guarda hueca, y
		// se destapó al cambiar el rastreo de `usadas` de por-sitio a por-nombre (2026-09-21): con
		// el sabotaje puesto —«alcanza con tener la clave para tapar cualquier colisión»— la
		// colisión se callaba Y la respuesta pasaba a ser rancia, o sea UNA queja en vez de UNA
		// queja, del mismo tamaño y nombrando los mismos textos. Las tres aserciones de arriba
		// seguían verdes porque la queja RANCIA también nombra `uno_test.go:10` y `TestUnTercero`.
		// Es la forma de siempre: preguntar por un texto que está, sin preguntar QUIÉN lo puso.
		//
		// Lo que decide es que la COLISIÓN siga denunciada además de la respuesta rancia.
		var colision, rancia int
		for _, q := range m {
			if strings.Contains(q, "pisa a") {
				colision++
			}
			if strings.Contains(q, "rancia") {
				rancia++
			}
		}
		if colision != 1 || rancia != 1 {
			t.Errorf("esperaba UNA denuncia de colisión (la que la clave no contestó) y UNA de "+
				"respuesta rancia; vinieron %d y %d:\n%s", colision, rancia, junto)
		}
	})

	t.Run("una respuesta a una colisión que ya no existe se denuncia como rancia", func(t *testing.T) {
		// Pasa cuando la otra guarda se reescribe o se va. La clave queda, nadie la relee, y lo que
		// fue una medición se vuelve una afirmación heredada.
		// LAS DOS AGREGAN EN VEZ DE REEMPLAZAR, así que sus `a` CONTIENEN al `de` y ninguna rompe el
		// ancla de la otra: no hay colisión que contestar. Lo que queda es la clave sola.
		sinPisar := func(arch string, lin int, prueba, colisionOk string) Ancla {
			return Ancla{Archivo: arch, Linea: lin, Directiva: &Directiva{
				Archivo: "prod.go", De: literal, A: literal + "\n\t\t_ = 0",
				Prueba: prueba, ColisionOk: colisionOk,
			}}
		}
		m := males(sinPisar("uno_test.go", 10, "TestUna", "TestOtra"),
			sinPisar("otro_test.go", 20, "TestOtra", ""))
		if len(m) != 1 {
			t.Fatalf("esperaba 1 (la respuesta rancia), vinieron %d:\n  %s",
				len(m), strings.Join(m, "\n  "))
		}
		if !strings.Contains(m[0], "uno_test.go:10") || !strings.Contains(m[0], "TestOtra") {
			t.Errorf("la denuncia no dice qué sitio ni a quién decía contestarle:\n  %s", m[0])
		}
	})
}

// TestUnColisionOkConNumeroDeLineaSeRechaza fija la forma de la respuesta.
//
// La manera natural de señalar el otro sitio es `archivo:línea`, y es justo la que se pudre: se
// midió pasando. La respuesta en prosa que #494 dejó escrita nombraba `colector_test.go:152`, y el
// PROPIO PR que la escribió insertó catorce líneas de comentario arriba y movió esa ancla a la 166.
// La respuesta quedó apuntando a otro lado el día que aterrizó, y nada se puso rojo.
//
// Sabotaje que la hace fallar: en `directivaDe`, sacar la exigencia de que `colision_ok` sea un
// nombre de prueba → un `archivo:línea` pasa y vuelve a pudrirse solo.
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="\t\tif !esNombreDePrueba(nombre) {"
// arnes: a="\t\tif false {"
// arnes: prueba="TestUnColisionOkConNumeroDeLineaSeRechaza"
func TestUnColisionOkConNumeroDeLineaSeRechaza(t *testing.T) {
	leer := func(extra string) []string {
		lineas := []lineaCom{{linea: 1, texto: "Sabotaje: X", cruda: " Sabotaje: X"}}
		for i, p := range []string{`archivo="x.go"`, `de="viejo"`, `a="nuevo"`, extra} {
			if p == "" {
				continue
			}
			lineas = append(lineas, lineaCom{linea: i + 2, texto: "arnes: " + p, cruda: " arnes: " + p})
		}
		_, _, quejas := directivaDe(lineas, "internal/x/x_test.go", "TestX")
		return quejas
	}

	t.Run("un archivo:linea se rechaza", func(t *testing.T) {
		if q := leer(`colision_ok="internal/fleet/colector_test.go:152"`); len(q) == 0 {
			t.Error("pasó un `archivo:línea`: esa respuesta apunta a otro lado en cuanto alguien " +
				"agrega una línea arriba, y nada se pondría rojo")
		}
	})

	t.Run("un nombre de prueba se acepta", func(t *testing.T) {
		// LA MITAD QUE IMPIDE QUE ESTO SEA UN «SIEMPRE RECHAZA», que dejaría la clave inutilizable.
		if q := leer(`colision_ok="TestUnaLecturaIncompletaNoInventaNumeros"`); len(q) != 0 {
			t.Errorf("un nombre de prueba legítimo se rechazó: %v", q)
		}
	})

	t.Run("CONTROL: sin la clave la directiva sigue siendo válida", func(t *testing.T) {
		// Las 88 directivas que no tienen colisión no tienen que escribir nada.
		if q := leer(""); len(q) != 0 {
			t.Errorf("una directiva sin `colision_ok` se quejó: %v", q)
		}
	})
}

// UN ANCLA SE PISA CON VARIAS, Y CON UN SOLO NOMBRE LA RESPUESTA ERA IMPOSIBLE DE DAR.
//
// `colision_ok` nació llevando UN nombre, y mientras las colisiones venían de a pares alcanzaba.
// Medido el 2026-09-21 mecanizando setenta y ocho anclas de una vez, deja de alcanzar: el `WHERE`
// de la cronología lo cubren TRES guardas —la ventana del dominio, la ventana del envoltorio y el
// tenant— y el UPDATE de `servicios.go`, CUATRO. Contestarle a una dejaba a las otras gritando
// para siempre, que es justo el «aviso verdadero que nadie puede contestar» que la clave vino a
// evitar: cinco pares quedaron sin respuesta por el techo de la clave y no por descuido.
//
// LO QUE ESTA PRUEBA CUIDA NO ES QUE ACEPTE VARIOS SINO QUE LOS CUENTE POR SEPARADO. El riesgo del
// arreglo cómodo es marcar «usada» la declaración entera en cuanto UNO de los nombres se pisa de
// verdad: ahí un nombre rancio viaja escondido detrás de sus hermanos y nadie lo relee nunca, que
// es peor que el aviso original — un «esto ya se miró» sobre algo que nadie miró.
//
// Sabotaje que la hace fallar: marcar `usadas[sitio]` en vez de `usadas[sitio+"\x00"+nombre]`.
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="\t\t\t\t\tusadas[sitio+\"\\x00\"+y.Directiva.Prueba] = true"
// arnes: a="\t\t\t\t\tusadas[sitio] = true"
func TestUnColisionOkContestaAVariasYCadaNombreSeCuentaSolo(t *testing.T) {
	raiz := t.TempDir()
	ruta := filepath.Join(raiz, "prod.go")
	if err := os.WriteFile(ruta, []byte("package p\n\nfunc f() {\n\tif n != 1 {\n\t\treturn\n\t}\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Las TRES se pisan entre sí: cada `a` destruye el `de` de las otras dos.
	const literal = "\tif n != 1 {"
	ancla := func(archivo, prueba, a, colisionOk string) Ancla {
		return Ancla{Archivo: archivo, Linea: 10, Directiva: &Directiva{
			Archivo: "prod.go", De: literal, A: a, Prueba: prueba, ColisionOk: colisionOk,
		}}
	}
	males := func(anclas ...Ancla) []string {
		return Colisiones(Censo{Raiz: raiz, Anclas: anclas})
	}

	// CONTROL DE «EL DETECTOR ESTÁ VIVO». Sin esto, un cero más abajo no distingue «contestada» de
	// «no se miró nada»: tres anclas que se pisan de a pares dan seis denuncias, dos por par.
	t.Run("CONTROL: sin ninguna clave, las tres se denuncian en las dos direcciones", func(t *testing.T) {
		m := males(
			ancla("a_test.go", "TestA", "\tif n != 2 {", ""),
			ancla("b_test.go", "TestB", "\tif false {", ""),
			ancla("c_test.go", "TestC", "\tif n > 9 {", ""),
		)
		if len(m) != 6 {
			t.Fatalf("esperaba 6 denuncias, vinieron %d:\n  %s", len(m), strings.Join(m, "\n  "))
		}
	})

	t.Run("un solo nombre NO alcanza cuando se pisa con dos", func(t *testing.T) {
		// A contesta sólo a B: la dirección A→C queda viva, que es correcto y es el techo viejo.
		m := males(
			ancla("a_test.go", "TestA", "\tif n != 2 {", "TestB"),
			ancla("b_test.go", "TestB", "\tif false {", "TestA TestC"),
			ancla("c_test.go", "TestC", "\tif n > 9 {", "TestA TestB"),
		)
		if len(m) != 1 {
			t.Fatalf("esperaba 1 denuncia (la dirección A→C, sin contestar), vinieron %d:\n  %s",
				len(m), strings.Join(m, "\n  "))
		}
		if !strings.Contains(m[0], "a_test.go") || !strings.Contains(m[0], "TestC") {
			t.Errorf("la denuncia que queda tiene que ser A→C y dice: %s", m[0])
		}
	})

	t.Run("con los tres nombres puestos, el aviso se calla entero", func(t *testing.T) {
		m := males(
			ancla("a_test.go", "TestA", "\tif n != 2 {", "TestB TestC"),
			ancla("b_test.go", "TestB", "\tif false {", "TestA TestC"),
			ancla("c_test.go", "TestC", "\tif n > 9 {", "TestA TestB"),
		)
		if len(m) != 0 {
			t.Fatalf("con las tres respuestas puestas no tendría que quedar nada, vinieron %d:\n  %s",
				len(m), strings.Join(m, "\n  "))
		}
	})

	t.Run("UN NOMBRE RANCIO NO SE ESCONDE DETRÁS DE SUS HERMANOS", func(t *testing.T) {
		// Ésta es la que decide. `TestA` contesta a `TestB` —que se pisa de verdad— y de paso
		// nombra a `TestFantasma`, que no existe. Si la declaración se marcara «usada» entera, el
		// fantasma pasaría sin que nadie lo relea.
		m := males(
			ancla("a_test.go", "TestA", "\tif n != 2 {", "TestB TestFantasma"),
			ancla("b_test.go", "TestB", "\tif false {", "TestA"),
		)
		if len(m) != 1 {
			t.Fatalf("esperaba 1 denuncia (la rancia), vinieron %d:\n  %s", len(m), strings.Join(m, "\n  "))
		}
		if !strings.Contains(m[0], "rancia") || !strings.Contains(m[0], "TestFantasma") {
			t.Errorf("la denuncia tiene que nombrar al fantasma rancio y dice: %s", m[0])
		}
	})
}
