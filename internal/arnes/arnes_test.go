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
	t.Run("conserva el modo del archivo", func(t *testing.T) {
		ruta := escribir("ejecutable.sh", "#!/bin/sh\necho uno\n")
		if err := os.Chmod(ruta, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := Aplicar(ruta, "uno", "dos"); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(ruta)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o755 {
			t.Errorf("el modo pasó de 0755 a %04o: un sabotaje que le saca el bit de ejecución a "+
				"un guion rompe la corrida SIGUIENTE, y el síntoma no apunta acá", got)
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
// Sabotaje que la hace fallar: hacer que ArchivosDePrueba filtre por un patrón que no matchea
// nada, p. ej. `*_prueba.go`.
// arnes: archivo="internal/arnes/arnes.go"
// arnes: de="\"ls-files\", \"-z\", \"*_test.go\""
// arnes: a="\"ls-files\", \"-z\", \"*_prueba.go\""
func TestElLectorNoPuedeDevolverCeroSobreElArbolDeVerdad(t *testing.T) {
	raiz := filepath.Join("..", "..")
	archivos, err := ArchivosDePrueba(raiz)
	if err != nil {
		t.Fatal(err)
	}
	if len(archivos) < 400 {
		t.Fatalf("git listó %d `_test.go` y el árbol tiene más de 600: este enumerador está mirando "+
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
