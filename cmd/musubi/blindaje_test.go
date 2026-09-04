package main

// Pruebas de A54: el agente declara lo que va a tocar y el despliegue lo verifica.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sinCLI y conPodman son las dos máquinas que importan: una sin contenedores y una con.
func sinCLI(string) bool      { return false }
func conPodman(c string) bool { return c == "podman" }

// LA DECLARACIÓN SIGUE A LO QUE ESTA MÁQUINA VA A HACER, no a todo lo imaginable.
//
// Una necesidad de más es tan mala como una de menos: manda a alguien a abrir un permiso que
// nadie usa, y un blindaje con agujeros que nadie necesita deja de leerse. Es la MISMA señal que
// decide si el trabajo corre (`hayEnPath`, el mismo criterio que enumerarFuente) la que decide si
// su blindaje importa.
//
// Sabotaje que la hace fallar: declarar las rutas de contenedores sin consultar hayCLI.
func TestSinPodmanNiDockerElAgenteNoPideRutasDeContenedores(t *testing.T) {
	ns := necesidadesDelAgente("/home/musubi", "/run/user/1000", sinCLI)
	for _, n := range ns {
		if strings.Contains(n.Trabajo, "contenedores") {
			t.Errorf("declaró %q para %q en una máquina sin podman ni docker", n.Ruta, n.Trabajo)
		}
	}
	// Y el latido SÍ, siempre: el token se escribe en su lugar cuando se rota.
	if len(ns) != 1 || ns[0].Trabajo != "latido" {
		t.Fatalf("se esperaba sólo la necesidad del latido, hubo %d: %+v", len(ns), ns)
	}

	// Con podman aparecen, y son las mismas TRES que el drop-in de producción ya concede.
	conPod := necesidadesDelAgente("/home/musubi", "/run/user/1000", conPodman)
	quiero := map[string]bool{
		"/home/musubi/.local/share/containers": false,
		"/run/user/1000/containers":            false,
		"/run/user/1000/libpod":                false,
	}
	for _, n := range conPod {
		if _, es := quiero[n.Ruta]; es {
			quiero[n.Ruta] = true
		}
	}
	for ruta, hay := range quiero {
		if !hay {
			t.Errorf("con podman instalado no se declaró %q, que `podman ps` abre en escritura", ruta)
		}
	}
}

// LAS RUTAS DE /run SON OPCIONALES Y LAS DE $HOME NO, y la diferencia es load-bearing.
//
// /run/user/1000 lo crea logind, y en un arranque la unidad puede llegar antes que
// user@1000.service. Un `ReadWritePaths` sobre una ruta que no existe hace FALLAR el arranque de
// la unidad entera — el agente no arrancaría en vez de arrancar sin contenedores. Por eso la
// directiva lleva el guion, y la declaración tiene que saberlo o la generaría mal.
//
// Sabotaje que la hace fallar: sacar `Opcional: true` de las rutas de runtimeDir.
// Sabotaje que la hace fallar: emitir la directiva sin el `-`.
func TestLasRutasDeRuntimeSonOpcionalesYSuDirectivaLlevaGuion(t *testing.T) {
	for _, n := range necesidadesDelAgente("/home/musubi", "/run/user/1000", conPodman) {
		enRuntime := strings.HasPrefix(n.Ruta, "/run/")
		if enRuntime && !n.Opcional {
			t.Errorf("%q no está declarada opcional: si logind no llegó, la unidad NO ARRANCA", n.Ruta)
		}
		if !enRuntime && n.Opcional {
			t.Errorf("%q está declarada opcional y vive en $HOME: ahí la ausencia es una falla real", n.Ruta)
		}
		// La directiva y el campo tienen que contar la misma historia. Que se separen es cómo un
		// arreglo copiado y pegado deja de arrancar la unidad seis meses después.
		conGuion := strings.HasPrefix(n.Directiva, "ReadWritePaths=-")
		if conGuion != n.Opcional {
			t.Errorf("%q: Opcional=%v pero la directiva es %q — el guion y el campo se separaron",
				n.Ruta, n.Opcional, n.Directiva)
		}
	}
}

// CADA NECESIDAD LLEVA SU SÍNTOMA, y es el campo por el que este archivo existe.
//
// A54 costó dos días porque el fallo NO dijo «permiso denegado» en ningún lado: fue un `podman ps`
// saliendo con código 1. La ruta se puede deducir de un strace; el síntoma es lo que alguien tiene
// delante DOS DÍAS ANTES de pensar en correr un strace.
//
// Sabotaje que la hace fallar: dejar `Sintoma` vacío en cualquier necesidad.
func TestCadaNecesidadDiceComoSeVeSuFalloYComoSeArregla(t *testing.T) {
	ns := necesidadesDelAgente("/home/musubi", "/run/user/1000", conPodman)
	if len(ns) < 4 {
		t.Fatalf("la declaración quedó en %d necesidades: la prueba no está mirando lo que cree", len(ns))
	}
	for _, n := range ns {
		if strings.TrimSpace(n.Sintoma) == "" {
			t.Errorf("%q no dice cómo se ve su fallo: sin eso el verificador informa una ruta y "+
				"nadie la relaciona con el problema que está mirando", n.Ruta)
		}
		if !strings.Contains(n.Directiva, n.Ruta) {
			t.Errorf("%q: la directiva %q no nombra la ruta, así que no se puede copiar y pegar",
				n.Ruta, n.Directiva)
		}
		if strings.TrimSpace(n.Trabajo) == "" {
			t.Errorf("%q no dice qué trabajo la pide: sin eso nadie puede decidir si sacarla", n.Ruta)
		}
	}
}

// «FALTA» Y «NO SE PUEDE» SON DOS FALLAS DISTINTAS CON ARREGLOS DISTINTOS.
//
// Un mkdir contra una línea de systemd. Mezclarlas es EXACTAMENTE el error que A54 documenta —un
// problema de montaje presentándose como otra cosa— y un verificador que le eche la culpa al
// blindaje cuando la ruta no existe manda a alguien a editar una unidad que está bien.
//
// Sabotaje que la hace fallar: que Estado() devuelva estadoBloqueada cuando !Existe.
func TestElVerificadorNoLeEchaLaCulpaAlBlindajeCuandoLaRutaNoExiste(t *testing.T) {
	base := t.TempDir()
	falta := filepath.Join(base, "no-existe")

	rs := revisarBlindaje([]necesidad{
		{Trabajo: "latido", Ruta: falta, Acceso: accesoEscritura,
			Sintoma: "x", Directiva: "ReadWritePaths=" + falta},
	})
	if len(rs) != 1 {
		t.Fatalf("revisarBlindaje devolvió %d revisiones", len(rs))
	}
	if e := rs[0].Estado(); e != estadoAusente {
		t.Fatalf("una ruta inexistente dio estado %q; tiene que ser %q", e, estadoAusente)
	}
	texto, fallas := informeDeBlindaje(rs)
	if fallas != 1 {
		t.Errorf("una ruta requerida que falta tiene que contar como falla (contó %d)", fallas)
	}
	if !strings.Contains(texto, "mkdir -p") {
		t.Errorf("el informe no dice el arreglo real (un mkdir).\n%s", texto)
	}
	if strings.Contains(texto, "el confinamiento no deja tocar") {
		t.Errorf("el informe le echó la culpa al blindaje por una ruta que NO EXISTE: eso manda "+
			"a alguien a editar una unidad que está bien.\n%s", texto)
	}
	// Y avisa del borde que hace fallar el arranque, porque el arreglo obvio —agregar la ruta al
	// ReadWritePaths— es el que rompe la unidad si la ruta sigue sin existir.
	if !strings.Contains(texto, "FALLAR el arranque") {
		t.Errorf("el informe no avisa que un ReadWritePaths sobre una ruta ausente tumba la unidad.\n%s", texto)
	}
}

// UNA RUTA OPCIONAL QUE NO EXISTE NO ES UNA FALLA. /run/user/1000/libpod no existe hasta que
// podman corre por primera vez, y exigirlo pintaría de rojo una máquina sana — que es cómo se le
// enseña a alguien a ignorar un verificador.
//
// Pero si EXISTE y no se alcanza, sí lo es: lo opcional era el existir, nunca el poder.
//
// Sabotaje que la hace fallar: devolver estadoOK siempre que Opcional sea true.
func TestLoOpcionalEsElExistirYNuncaElPoder(t *testing.T) {
	base := t.TempDir()
	ausente := necesidad{Trabajo: "t", Ruta: filepath.Join(base, "nunca"), Acceso: accesoEscritura,
		Opcional: true, Sintoma: "x", Directiva: "ReadWritePaths=-x"}
	if e := revisarBlindaje([]necesidad{ausente})[0].Estado(); e != estadoOK {
		t.Errorf("una ruta opcional ausente dio %q: una máquina sana no puede salir en rojo", e)
	}

	// La misma ruta, pero existiendo y sin permiso de escritura.
	cerrada := filepath.Join(base, "cerrada")
	if err := os.Mkdir(cerrada, 0o500); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() == 0 {
		t.Skip("como root los permisos del inodo no frenan nada; este borde se prueba sin privilegios")
	}
	r := revisarBlindaje([]necesidad{{Trabajo: "t", Ruta: cerrada, Acceso: accesoEscritura,
		Opcional: true, Sintoma: "x", Directiva: "ReadWritePaths=-" + cerrada}})[0]
	if r.Estado() != estadoBloqueada {
		t.Errorf("una ruta opcional que EXISTE y no se puede escribir dio %q: lo opcional era el "+
			"existir, no el poder", r.Estado())
	}
}

// LA ESCRITURA SE PRUEBA ESCRIBIENDO, y no mirando permisos.
//
// El confinamiento de systemd es un MONTAJE de sólo lectura: los bits del inodo siguen diciendo
// que se puede, y `access(W_OK)` no ve un bind mount read-only. El único chequeo que detecta un
// ReadWritePaths que falta es crear un archivo y borrarlo — que es lo que `podman ps` hace y lo
// que fallaba. Medido contra el servidor real: con ProtectHome=read-only las tres rutas dieron
// «read-only file system» y el informe nombró las tres líneas exactas del drop-in.
//
// Y NO PUEDE DEJAR BASURA: corre en máquinas ajenas, adentro del store de podman.
//
// Sabotaje que la hace fallar: quitar el os.Remove del archivo temporal.
func TestProbarLaEscrituraNoDejaNadaAtras(t *testing.T) {
	dir := t.TempDir()
	antes, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	ok, motivo := probarAcceso(dir, accesoEscritura, true)
	if !ok {
		t.Fatalf("un directorio recién creado dio no-escribible: %s", motivo)
	}
	despues, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(despues) != len(antes) {
		var nombres []string
		for _, e := range despues {
			nombres = append(nombres, e.Name())
		}
		t.Errorf("la revisión dejó %d archivo(s) en el directorio ajeno que estaba probando: %v",
			len(despues)-len(antes), nombres)
	}
}

// ── La guarda estructural: lo que el agente DECLARA contra lo que la unidad CONCEDE ──────────

// unidadYDropIns junta la unidad base y todos sus drop-ins, que es como los lee systemd.
func unidadYDropIns(t *testing.T) string {
	t.Helper()
	rutas, err := filepath.Glob(filepath.Join("..", "..", "deploy", "systemd", "musubi-agente*"))
	if err != nil {
		t.Fatal(err)
	}
	// Si el glob deja de encontrar los archivos —se movieron, se renombraron— la prueba pasaría
	// VACÍA y en verde, que es el modo de fallo más peligroso que puede tener un barrido.
	if len(rutas) < 2 {
		t.Fatalf("se encontraron %d archivos de la unidad del agente en deploy/systemd/; "+
			"la guarda no está mirando donde cree (%v)", len(rutas), rutas)
	}
	var b strings.Builder
	for _, r := range rutas {
		crudo, err := os.ReadFile(r)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(crudo)
		b.WriteString("\n")
	}
	return b.String()
}

// ESTA ES LA MITAD DE A54 QUE FALTABA.
//
// El agente ganó una capacidad (A42, enumerar contenedores), el blindaje la prohibía, y NADA lo
// dijo: no hubo «permiso denegado», hubo un `podman ps` con código 1 y el inventario entero
// abortado. La unidad —bien escrita para el agente que describía— nunca fue sospechosa. Dos días.
//
// A partir de acá, una capacidad nueva que necesite una ruta que la unidad no concede rompe la
// SUITE, no la producción. Las rutas de producción se fijan a mano y a propósito: son las que la
// unidad y su drop-in tienen escritas, así que si alguien cambia una de las dos y no la otra,
// esto lo dice.
//
// Sabotaje que la hace fallar: borrar una línea ReadWritePaths del drop-in de contenedores.
// Sabotaje que la hace fallar: agregarle al agente una necesidad nueva sin tocar deploy/systemd/.
func TestElBlindajeDeLaUnidadConcedeLoQueElAgenteDECLARA(t *testing.T) {
	// Los valores de la máquina de producción, que son los que los archivos tienen escritos.
	const homeProd, runtimeProd = "/home/musubi", "/run/user/1000"
	unidad := unidadYDropIns(t)

	ns := necesidadesDelAgente(homeProd, runtimeProd, conPodman)
	if len(ns) < 4 {
		t.Fatalf("la declaración quedó en %d necesidades: la guarda no está mirando lo que cree", len(ns))
	}
	for _, n := range ns {
		if !strings.Contains(unidad, n.Directiva) {
			t.Errorf("el agente DECLARA que va a tocar %q para %q, y ni la unidad ni sus drop-ins\n"+
				"  la conceden. Falta esta línea en deploy/systemd/:\n\n    %s\n\n"+
				"  Sin ella el síntoma en producción es: %s",
				n.Ruta, n.Trabajo, n.Directiva, n.Sintoma)
		}
	}
}

// LO QUE LA UNIDAD CONCEDE Y EL AGENTE NO PIDE TAMBIÉN ES UN HALLAZGO.
//
// Es la dirección opuesta de la guarda de arriba y hace falta igual: un permiso que sobrevive a
// la capacidad que lo justificaba es un agujero que nadie va a cerrar, porque nadie se acuerda de
// para qué estaba. El drop-in de contenedores lo dice en su propio texto: «si el agente deja de
// enumerar contenedores, esta excepción se borra».
//
// Se reporta como aviso y no como error: puede haber una concesión legítima que el agente no
// declare todavía (una capacidad a medio camino), y romper la suite por eso sería enseñar a
// ignorarla. Pero queda escrito en la salida del test.
//
// Sabotaje que la hace fallar: agregar un ReadWritePaths inventado al drop-in.
func TestLaUnidadNoConcedeRutasQueElAgenteNoPide(t *testing.T) {
	const homeProd, runtimeProd = "/home/musubi", "/run/user/1000"
	declaradas := map[string]bool{}
	for _, n := range necesidadesDelAgente(homeProd, runtimeProd, conPodman) {
		declaradas[n.Ruta] = true
	}

	for _, linea := range strings.Split(unidadYDropIns(t), "\n") {
		linea = strings.TrimSpace(linea)
		if strings.HasPrefix(linea, "#") || !strings.HasPrefix(linea, "ReadWritePaths=") {
			continue
		}
		ruta := strings.TrimPrefix(strings.TrimPrefix(linea, "ReadWritePaths="), "-")
		if !declaradas[ruta] {
			t.Errorf("la unidad concede %q y el agente no declara necesitarla.\n"+
				"  Un permiso que sobrevive a la capacidad que lo justificaba es un agujero que\n"+
				"  nadie va a cerrar, porque nadie se acuerda de para qué estaba.\n"+
				"  O se declara en necesidadesDelAgente(), o se borra de deploy/systemd/.", ruta)
		}
	}
}

// SYSTEMD NO EXPORTA XDG_RUNTIME_DIR EN UNA UNIDAD DE SISTEMA CON `User=`.
//
// Medido contra el agente real en musubi-server: el proceso tiene HOME y USER, y NO tiene
// XDG_RUNTIME_DIR — systemd sólo la exporta en unidades de USUARIO. Podman lo sabe y cae a
// /run/user/<uid>, que es por qué encuentra sus locks igual y por qué el drop-in tuvo que
// conceder esas rutas.
//
// Leyendo sólo la variable, el verificador declaraba NADA para /run en la única máquina donde
// importa: salía en verde sin mirar las tres rutas que rompieron A42. Casi repito adentro del
// arreglo el hueco silencioso que el arreglo existe para cerrar.
//
// Sabotaje que la hace fallar: volver a `os.Getenv("XDG_RUNTIME_DIR")` pelado.
func TestSinXDGRuntimeDirElVerificadorNoSeQuedaCiego(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	d := dirDeRuntime()
	// SIN UID POSITIVO NO HAY STORE ROOTLESS, y son DOS los casos: root (uid 0, no existe
	// /run/user/0) y una máquina sin el concepto de uid, donde os.Getuid() devuelve -1. El
	// segundo lo destapó `test-cross` en Windows: con `== 0` la prueba caía en la rama de abajo
	// y exigía un fallback /run/user/<uid> que ahí no significa nada. Devolver "" es lo correcto
	// en los dos, y así lo dice.
	if os.Getuid() <= 0 {
		if d != "" {
			t.Errorf("sin uid positivo devolvió %q; no hay /run/user/<uid> y el store rootless no aplica", d)
		}
		return
	}
	if d == "" {
		t.Fatal("sin XDG_RUNTIME_DIR devolvió vacío: las rutas de /run no se declaran y el " +
			"verificador sale en verde sin mirar las que rompieron A42")
	}
	if !strings.HasPrefix(d, "/run/user/") {
		t.Errorf("el fallback dio %q; podman usa /run/user/<uid> y hay que buscar donde él busca", d)
	}
	// Y con la variable puesta gana ella: una máquina que la exporta puede tenerla en otro lado.
	t.Setenv("XDG_RUNTIME_DIR", "/tmp/otro-runtime")
	if d := dirDeRuntime(); d != "/tmp/otro-runtime" {
		t.Errorf("con XDG_RUNTIME_DIR puesta devolvió %q: la variable tiene prioridad", d)
	}
}

// FUERA DE LINUX LA HERRAMIENTA NO TIENE NADA QUE DECIR, y decirlo igual sería peor que callarse.
//
// `ReadWritePaths` es una directiva de systemd. En un Windows —donde corren dos de las tres
// máquinas de esta flota— el verificador emitiría `ReadWritePaths=C:\Users\gio\...`: un consejo
// con forma de respuesta que no aplica a nada. Dar una instrucción equivocada con confianza es el
// modo de falla exacto que A54 documenta; repetirlo adentro del arreglo sería el colmo.
//
// La prueba mira la DECLARACIÓN y no la salida, porque la salida se compila para linux acá. Lo
// que se custodia es que la rama exista y que el binario cruce a las tres plataformas.
//
// Sabotaje que la hace fallar: quitar la guarda `runtime.GOOS != "linux"` de
// revisarBlindajeDelAgente.
func TestElVerificadorSeCallaFueraDeLinux(t *testing.T) {
	crudo, err := os.ReadFile("blindaje.go")
	if err != nil {
		t.Fatal(err)
	}
	fuente := string(crudo)
	i := strings.Index(fuente, "func revisarBlindajeDelAgente()")
	if i < 0 {
		t.Fatal("no existe revisarBlindajeDelAgente: la prueba no está mirando nada")
	}
	cuerpo := fuente[i:]
	if fin := strings.Index(cuerpo, "\nfunc "); fin > 0 {
		cuerpo = cuerpo[:fin]
	}
	if !strings.Contains(cuerpo, `runtime.GOOS != "linux"`) {
		t.Errorf("el verificador no se calla fuera de Linux: en un Windows emitiría directivas de "+
			"systemd sobre rutas con backslash, que es un consejo equivocado dicho con confianza.\n%s",
			cuerpo)
	}
	// Y sale con 0: no tener nada que revisar NO es una falla, y devolver 1 pondría en rojo un
	// despliegue sano de Windows para siempre.
	guarda := cuerpo[strings.Index(cuerpo, `runtime.GOOS != "linux"`):]
	if cierre := strings.Index(guarda, "\n\t}"); cierre > 0 {
		guarda = guarda[:cierre]
	}
	if !strings.Contains(guarda, "return 0") {
		t.Errorf("la rama de no-Linux no devuelve 0: un Windows sano quedaría en rojo.\n%s", guarda)
	}
}
