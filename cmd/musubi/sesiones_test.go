package main

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"musubi/internal/transcripts"
)

// LAS PRUEBAS DE `musubi sesiones` CORREN CONTRA UN CLAUDE_CONFIG_DIR TEMPORAL, NUNCA CONTRA EL
// ~/.claude DE VERDAD: borrar es borrar.

const (
	idA = "11111111-1111-4111-8111-111111111111"
	idB = "22222222-2222-4222-8222-222222222222"
	idC = "33333333-3333-4333-8333-333333333333"
)

var hace40Dias = time.Now().Add(-40 * 24 * time.Hour)

// configDePrueba arma un CLAUDE_CONFIG_DIR vacío y lo deja puesto mientras dura la prueba.
func configDePrueba(t *testing.T) (config, projects string) {
	t.Helper()
	config = t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", config)
	projects = filepath.Join(config, "projects")
	if err := os.MkdirAll(projects, 0o755); err != nil {
		t.Fatal(err)
	}
	return config, projects
}

// escribir crea un archivo de n bytes, con las carpetas que falten.
func escribir(t *testing.T, ruta string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruta, bytes.Repeat([]byte("x"), n), 0o644); err != nil {
		t.Fatal(err)
	}
}

// envejecer le pone `mod` a todo lo que hay bajo `raiz`, carpetas incluidas. Las carpetas van al
// final y de abajo hacia arriba: crear algo adentro les mueve la fecha.
func envejecer(t *testing.T, raiz string, mod time.Time) {
	t.Helper()
	var dirs []string
	err := filepath.WalkDir(raiz, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirs = append(dirs, p)
			return nil
		}
		return os.Chtimes(p, mod, mod)
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Chtimes(dirs[i], mod, mod); err != nil {
			t.Fatal(err)
		}
	}
}

// sesionDePrueba crea una sesión: su transcript de `jsonl` bytes y, si `dir` > 0, su carpeta `<id>/`
// con un subagente y un resultado de tool que suman `dir` bytes. Todo con fecha `mod`.
func sesionDePrueba(t *testing.T, projects, carpeta, id string, jsonl, dir int, mod time.Time) {
	t.Helper()
	escribir(t, filepath.Join(projects, carpeta, id+".jsonl"), jsonl)
	if dir > 0 {
		escribir(t, filepath.Join(projects, carpeta, id, "subagents", "agent-a.jsonl"), dir/2)
		escribir(t, filepath.Join(projects, carpeta, id, "tool-results", "r.txt"), dir-dir/2)
		envejecer(t, filepath.Join(projects, carpeta, id), mod)
	}
	envejecer(t, filepath.Join(projects, carpeta, id+".jsonl"), mod)
}

// memoriaDePrueba crea la memoria de Claude Code de una carpeta de proyecto, vieja.
func memoriaDePrueba(t *testing.T, projects, carpeta string) {
	t.Helper()
	escribir(t, filepath.Join(projects, carpeta, "memory", "MEMORY.md"), 100)
	escribir(t, filepath.Join(projects, carpeta, "memory", "nota.md"), 50)
	envejecer(t, filepath.Join(projects, carpeta, "memory"), hace40Dias)
}

func correrSesiones(t *testing.T, entrada string, args ...string) (int, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := sesionesCmd(args, strings.NewReader(entrada), &out, &errOut)
	return code, out.String() + errOut.String()
}

func existe(ruta string) bool {
	_, err := os.Lstat(ruta)
	return err == nil
}

// idsListados son los ids de las filas de un listado, ordenados.
func idsListados(salida string) []string {
	var ids []string
	for _, l := range strings.Split(salida, "\n") {
		if f := strings.Fields(l); len(f) > 0 && esIDDeSesion(f[0]) {
			ids = append(ids, f[0])
		}
	}
	sort.Strings(ids)
	return ids
}

func filaDe(salida, id string) string {
	for _, l := range strings.Split(salida, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), id) {
			return l
		}
	}
	return ""
}

func TestSesionesListaConElTamanoDeSuCarpeta(t *testing.T) {
	_, projects := configDePrueba(t)
	// A: 1 MiB de transcript y 1,5 MiB en su carpeta. B: sin carpeta.
	sesionDePrueba(t, projects, "-proy-a", idA, 1<<20, 3<<19, hace40Dias)
	sesionDePrueba(t, projects, "-proy-b", idB, 1<<20, 0, hace40Dias)
	// Lo que hay al lado y NO es una sesión, medido en la máquina real.
	memoriaDePrueba(t, projects, "-proy-a")
	escribir(t, filepath.Join(projects, "-proy-a", "MEMORY.md"), 10)
	escribir(t, filepath.Join(projects, "-proy-a", "herramientas", "x.sql"), 10)

	ss, err := leerSesiones(projects, []string{"-proy-a", "-proy-b"})
	if err != nil {
		t.Fatal(err)
	}
	por := map[string]sesionCC{}
	for _, s := range ss {
		por[s.ID] = s
	}
	if len(ss) != 2 {
		t.Fatalf("leerSesiones encontró %d sesiones, quiero 2 (memory/, MEMORY.md y herramientas/ no son sesiones): %+v", len(ss), ss)
	}
	if got, want := por[idA].Bytes, int64(1<<20+3<<19); got != want {
		t.Errorf("A mide %d bytes; quiero %d: el .jsonl MÁS su carpeta", got, want)
	}
	if got := por[idB].Bytes; got != 1<<20 {
		t.Errorf("B mide %d bytes; quiero %d", got, 1<<20)
	}

	code, salida := correrSesiones(t, "", "--todas")
	if code != 0 {
		t.Fatalf("musubi sesiones --todas salió %d:\n%s", code, salida)
	}
	if got := idsListados(salida); strings.Join(got, ",") != idA+","+idB {
		t.Fatalf("listó %v; quiero A y B:\n%s", got, salida)
	}
	fila := filaDe(salida, idA)
	if !strings.Contains(fila, "2.5 MB") || !strings.Contains(fila, "-proy-a") || !strings.Contains(fila, hace40Dias.Local().Format("2006-01-02")) {
		t.Errorf("la fila de A no dice 2.5 MB, su carpeta y su fecha: %q", fila)
	}
	if !strings.Contains(salida, "Por carpeta de proyecto") {
		t.Errorf("con dos carpetas el listado no da el subtotal por carpeta:\n%s", salida)
	}
}

func TestSesionesSinTodasMiraSoloElProyectoActual(t *testing.T) {
	_, projects := configDePrueba(t)
	t.Chdir(t.TempDir())
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	sesionDePrueba(t, projects, transcripts.CarpetaDeProyecto(wd), idA, 10, 0, hace40Dias)
	sesionDePrueba(t, projects, "-otro-proyecto", idB, 10, 0, hace40Dias)

	if _, salida := correrSesiones(t, ""); strings.Join(idsListados(salida), ",") != idA {
		t.Errorf("sin --todas listó %v; quiero sólo la del proyecto actual:\n%s", idsListados(salida), salida)
	}
	if _, salida := correrSesiones(t, "", "--todas"); len(idsListados(salida)) != 2 {
		t.Errorf("con --todas listó %v; quiero las dos", idsListados(salida))
	}
}

// En Windows la misma ruta deja carpetas `c--…` y `C--…`: son el mismo proyecto. En Linux no.
func TestSesionesVariantesDeMayusculasDeLaCarpeta(t *testing.T) {
	if !mismaCarpeta("c--Proyectos-Musubi", "C--Proyectos-Musubi", true) {
		t.Error("plegando, c--… y C--… tienen que ser la misma carpeta")
	}
	if mismaCarpeta("c--Proyectos-Musubi", "C--Proyectos-Musubi", false) {
		t.Error("sin plegar (Linux, macOS), c--… y C--… son carpetas distintas")
	}
}

func TestSesionesViejasFiltraPorLaUltimaModificacion(t *testing.T) {
	_, projects := configDePrueba(t)
	hace2Dias := time.Now().Add(-2 * 24 * time.Hour)
	sesionDePrueba(t, projects, "-p", idA, 10, 10, hace40Dias)
	sesionDePrueba(t, projects, "-p", idB, 10, 0, hace2Dias)
	// C: el transcript es viejo, pero un subagente escribió en su carpeta hace dos días.
	sesionDePrueba(t, projects, "-p", idC, 10, 10, hace40Dias)
	if err := os.Chtimes(filepath.Join(projects, "-p", idC, "subagents", "agent-a.jsonl"), hace2Dias, hace2Dias); err != nil {
		t.Fatal(err)
	}

	code, salida := correrSesiones(t, "", "--todas", "--viejas", "30")
	if code != 0 {
		t.Fatalf("salió %d:\n%s", code, salida)
	}
	if got := idsListados(salida); strings.Join(got, ",") != idA {
		t.Errorf("--viejas 30 listó %v; quiero sólo A (C tiene su carpeta tocada hace 2 días):\n%s", got, salida)
	}
	if got := idsListados(salida); len(got) == 1 && !strings.Contains(salida, "sin cambios hace 30 día(s)") {
		t.Errorf("el listado no dice qué filtro aplicó:\n%s", salida)
	}
	for _, mal := range []string{"-1", "100000", "x"} {
		if code, _ := correrSesiones(t, "", "--viejas", mal); code != 2 {
			t.Errorf("--viejas %s salió %d; quiero 2", mal, code)
		}
	}
}

func TestSesionesBorrarConSiExigeUnaSemana(t *testing.T) {
	_, projects := configDePrueba(t)
	sesionDePrueba(t, projects, "-p", idA, 10, 10, hace40Dias)
	jsonl := filepath.Join(projects, "-p", idA+".jsonl")
	// Con --si, menos de una semana se rechaza entero, aunque la sesión elegida cumpla los días.
	for _, n := range []string{"1", "6"} {
		code, salida := correrSesiones(t, "", "borrar", "--viejas", n, "--todas", "--si")
		if code != 2 {
			t.Errorf("--viejas %s --si salió %d; quiero 2:\n%s", n, code, salida)
		}
		if !existe(jsonl) {
			t.Fatalf("--viejas %s --si borró la sesión:\n%s", n, salida)
		}
	}
	// Sin --si, un día sigue valiendo: muestra el plan, pregunta y sin «s» no borra.
	if code, salida := correrSesiones(t, "", "borrar", "--viejas", "1", "--todas"); code != 1 || !existe(jsonl) {
		t.Errorf("--viejas 1 sin --si salió %d (quiero 1, sin borrar):\n%s", code, salida)
	}
	// Desde una semana, --si borra sin preguntar.
	if code, salida := correrSesiones(t, "", "borrar", "--viejas", "7", "--todas", "--si"); code != 0 || existe(jsonl) {
		t.Errorf("--viejas 7 --si salió %d y la sesión sigue en disco=%v:\n%s", code, existe(jsonl), salida)
	}
}

func TestSesionesBorrarSinSiYConNoNoBorraNada(t *testing.T) {
	_, projects := configDePrueba(t)
	sesionDePrueba(t, projects, "-p", idA, 10, 10, hace40Dias)
	jsonl, dir := filepath.Join(projects, "-p", idA+".jsonl"), filepath.Join(projects, "-p", idA)
	// «n», la entrada vacía (nadie mirando) y cualquier otra cosa son no.
	for _, entrada := range []string{"n\n", "", "quizás\n", "N\n"} {
		code, salida := correrSesiones(t, entrada, "borrar", idA)
		if code != 1 {
			t.Errorf("entrada %q: salió %d; quiero 1 (no se hizo)", entrada, code)
		}
		if !existe(jsonl) || !existe(dir) {
			t.Fatalf("entrada %q: se borró la sesión sin un «s»:\n%s", entrada, salida)
		}
		if !strings.Contains(salida, "Voy a borrar 1 sesión(es)") || !strings.Contains(salida, "No se borró nada") {
			t.Errorf("entrada %q: no mostró el plan o no dijo que no borró:\n%s", entrada, salida)
		}
	}
}

func TestSesionesBorrarConSBorraElJsonlYSuCarpeta(t *testing.T) {
	_, projects := configDePrueba(t)
	sesionDePrueba(t, projects, "-p", idA, 10, 10, hace40Dias)
	sesionDePrueba(t, projects, "-p", idB, 10, 10, hace40Dias)
	memoriaDePrueba(t, projects, "-p")

	code, salida := correrSesiones(t, "s\n", "borrar", idA)
	if code != 0 {
		t.Fatalf("salió %d:\n%s", code, salida)
	}
	if existe(filepath.Join(projects, "-p", idA+".jsonl")) || existe(filepath.Join(projects, "-p", idA)) {
		t.Errorf("con «s» quedó el .jsonl o la carpeta de A:\n%s", salida)
	}
	if !existe(filepath.Join(projects, "-p", idB+".jsonl")) || !existe(filepath.Join(projects, "-p", idB)) {
		t.Errorf("se borró B, que nadie pidió")
	}
	if !existe(filepath.Join(projects, "-p", "memory", "MEMORY.md")) {
		t.Errorf("se tocó memory/")
	}
	if !strings.Contains(salida, "Borradas 1 de 1") {
		t.Errorf("no informó lo borrado:\n%s", salida)
	}
	// `--si` DESPUÉS del id es una bandera, no otro id.
	if code, salida := correrSesiones(t, "", "borrar", idB, "--si"); code != 0 || existe(filepath.Join(projects, "-p", idB+".jsonl")) {
		t.Errorf("borrar <id> --si salió %d y B existe=%v:\n%s", code, existe(filepath.Join(projects, "-p", idB+".jsonl")), salida)
	}
}

// memory/ es la memoria de Claude Code y queda intacta SIEMPRE: con el filtro más ancho, nombrada
// por la CLI y nombrada por debajo de la CLI, con un `memory.jsonl` plantado que la haría pasar por
// la carpeta de una sesión.
func TestSesionesMemoryQuedaIntactaSiempre(t *testing.T) {
	config, projects := configDePrueba(t)
	sesionDePrueba(t, projects, "-p", idA, 10, 10, hace40Dias)
	memoriaDePrueba(t, projects, "-p")
	for _, r := range []string{"memory.jsonl", "MEMORY.md", "herramientas/x.sql"} {
		escribir(t, filepath.Join(projects, "-p", filepath.FromSlash(r)), 10)
	}
	envejecer(t, filepath.Join(projects, "-p"), hace40Dias)
	intacta := func(cuando string) {
		t.Helper()
		for _, r := range []string{"memory/MEMORY.md", "memory/nota.md", "memory.jsonl", "MEMORY.md", "herramientas/x.sql"} {
			if !existe(filepath.Join(projects, "-p", filepath.FromSlash(r))) {
				t.Fatalf("%s: se borró %s", cuando, r)
			}
		}
	}

	code, salida := correrSesiones(t, "", "borrar", "--viejas", "7", "--todas", "--si")
	if code != 0 || existe(filepath.Join(projects, "-p", idA+".jsonl")) {
		t.Fatalf("borrar --viejas 7 --todas --si salió %d y A existe=%v:\n%s", code, existe(filepath.Join(projects, "-p", idA+".jsonl")), salida)
	}
	intacta("borrar --viejas 7 --todas --si")

	for _, id := range []string{"memory", "MEMORY", "memory.jsonl"} {
		if code, salida := correrSesiones(t, "s\n", "borrar", id, "--si"); code != 2 {
			t.Errorf("borrar %s salió %d; quiero 2:\n%s", id, code, salida)
		}
		intacta("borrar " + id)
	}

	// borrarSesion es la que borra, y se niega sola: aunque alguien la llame sin pasar por la CLI.
	for _, id := range []string{"memory", "Memory"} {
		if _, err := borrarSesion(config, "-p", id); err == nil {
			t.Errorf("borrarSesion(%q) no se negó", id)
		}
		intacta("borrarSesion(" + id + ")")
	}
}

func TestSesionesNoBorraUnaSesionDeHaceCincoMinutosNiConSi(t *testing.T) {
	config, projects := configDePrueba(t)
	sesionDePrueba(t, projects, "-p", idA, 10, 10, time.Now().Add(-5*time.Minute))
	jsonl := filepath.Join(projects, "-p", idA+".jsonl")

	code, salida := correrSesiones(t, "", "borrar", idA, "--si")
	if code != 1 || !existe(jsonl) || !existe(filepath.Join(projects, "-p", idA)) {
		t.Fatalf("una sesión de hace 5 minutos: salió %d y existe=%v; no se tiene que borrar ni con --si:\n%s", code, existe(jsonl), salida)
	}
	if !strings.Contains(salida, "puede ser la que está corriendo") {
		t.Errorf("no dijo por qué no la borra:\n%s", salida)
	}
	if _, salida := correrSesiones(t, "", "--todas"); !strings.Contains(salida, "POSIBLEMENTE ACTIVA") {
		t.Errorf("el listado no la marca como posiblemente activa:\n%s", salida)
	}
	if _, err := borrarSesion(config, "-p", idA); err == nil || !existe(jsonl) {
		t.Errorf("borrarSesion la borró sin pasar por la CLI (err=%v)", err)
	}

	// La actividad cuenta en su carpeta: transcript viejo, subagente escribiendo hace 5 minutos.
	sesionDePrueba(t, projects, "-p", idB, 10, 10, hace40Dias)
	hace5 := time.Now().Add(-5 * time.Minute)
	if err := os.Chtimes(filepath.Join(projects, "-p", idB, "subagents", "agent-a.jsonl"), hace5, hace5); err != nil {
		t.Fatal(err)
	}
	if code, salida := correrSesiones(t, "", "borrar", idB, "--si"); code != 1 || !existe(filepath.Join(projects, "-p", idB+".jsonl")) {
		t.Errorf("con un subagente activo hace 5 minutos la sesión se borró (salió %d):\n%s", code, salida)
	}
}

func TestSesionesNoBorraUnaSesionRegistradaComoAbierta(t *testing.T) {
	config, projects := configDePrueba(t)
	sesionDePrueba(t, projects, "-p", idA, 10, 10, hace40Dias)
	reg := filepath.Join(config, "sessions", "4242.json")
	escribir(t, reg, 0)
	if err := os.WriteFile(reg, []byte(`{"pid":4242,"sessionId":"`+idA+`","status":"idle"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, salida := correrSesiones(t, "", "borrar", idA, "--si")
	if code != 1 || !existe(filepath.Join(projects, "-p", idA+".jsonl")) {
		t.Fatalf("una sesión registrada como abierta se borró (salió %d):\n%s", code, salida)
	}
	if !strings.Contains(salida, "registra como abierta") {
		t.Errorf("no dijo por qué no la borra:\n%s", salida)
	}
}

// lectorQueToca corre `antes` la primera vez que alguien lee, y recién después contesta: es la
// persona que tarda en decir «s» mientras otra terminal retoma la sesión.
type lectorQueToca struct {
	antes     func()
	respuesta io.Reader
}

func (l *lectorQueToca) Read(p []byte) (int, error) {
	if l.antes != nil {
		l.antes()
		l.antes = nil
	}
	return l.respuesta.Read(p)
}

func TestSesionesBorrarVuelveAMirarDespuesDelSi(t *testing.T) {
	_, projects := configDePrueba(t)
	sesionDePrueba(t, projects, "-p", idA, 10, 10, hace40Dias)
	jsonl := filepath.Join(projects, "-p", idA+".jsonl")
	in := &lectorQueToca{
		antes: func() {
			ahora := time.Now()
			_ = os.Chtimes(jsonl, ahora, ahora)
		},
		respuesta: strings.NewReader("s\n"),
	}
	var out, errOut bytes.Buffer
	code := sesionesCmd([]string{"borrar", idA}, in, &out, &errOut)
	if code != 1 || !existe(jsonl) {
		t.Fatalf("la sesión revivió mientras se esperaba el «s» y se borró igual (salió %d):\n%s%s", code, out.String(), errOut.String())
	}
}

func TestSesionesBorrarRechazaIDsQueNoSonUnaSesion(t *testing.T) {
	config, projects := configDePrueba(t)
	sesionDePrueba(t, projects, "-p", idA, 10, 10, hace40Dias)
	// Lo que un id tramposo podría alcanzar fuera de projects.
	escribir(t, filepath.Join(config, "afuera.jsonl"), 10)
	escribir(t, filepath.Join(config, "afuera", "x"), 10)
	envejecer(t, config, hace40Dias)
	sigue := func(cuando string) {
		t.Helper()
		for _, r := range []string{"afuera.jsonl", "afuera/x", "projects/-p/" + idA + ".jsonl", "projects/-p/" + idA} {
			if !existe(filepath.Join(config, filepath.FromSlash(r))) {
				t.Fatalf("%s: se borró %s", cuando, r)
			}
		}
	}
	for _, id := range []string{"..", "../afuera", `..\afuera`, "../../afuera", "-p/" + idA, `-p\` + idA,
		"C:" + idA, idA[:8], idA + "x", idA + ".jsonl", idA + "/..", "", "."} {
		if code, salida := correrSesiones(t, "s\n", "borrar", id, "--si"); code != 2 {
			t.Errorf("borrar %q salió %d; quiero 2:\n%s", id, code, salida)
		}
		sigue("borrar " + id)
	}
	// Por debajo de la CLI tampoco: ni la carpeta ni el id pueden salir de projects.
	for _, c := range [][2]string{{"..", "afuera"}, {".", idA}, {"-p/..", idA}, {"-p", "../" + idA}, {"-p", idA + `\..`}, {"", idA}} {
		if _, err := borrarSesion(config, c[0], c[1]); err == nil {
			t.Errorf("borrarSesion(%q, %q) no se negó", c[0], c[1])
		}
		sigue("borrarSesion")
	}
}

// Un enlace adentro de la carpeta de una sesión apunta a algo que no es de la sesión: no se borra a
// través de él. En Windows se prueba también con una junction, que no pide privilegios.
func TestSesionesNoBorraATravesDeUnEnlace(t *testing.T) {
	config, projects := configDePrueba(t)
	afuera := filepath.Join(config, "afuera")
	escribir(t, filepath.Join(afuera, "valioso.txt"), 10)
	casos := 0
	for i, crear := range []func(destino, enlace string) error{
		os.Symlink,
		crearJunction, // sin lanzar `cmd /c mklink`: ver sesiones_ntfs_windows.go
	} {
		id := []string{idA, idB}[i]
		sesionDePrueba(t, projects, "-p", id, 10, 10, hace40Dias)
		if err := crear(afuera, filepath.Join(projects, "-p", id, "enlace")); err != nil {
			continue
		}
		casos++
		s, err := medirSesion(projects, "-p", id)
		if err != nil {
			t.Fatal(err)
		}
		// Una hora más tarde, para que la ventana de actividad no tape el motivo: el enlace acaba de
		// nacer y su fecha propia no se puede envejecer.
		if motivo, _ := protegida(s, time.Now().Add(time.Hour), nil); s.Enlaces != 1 || !strings.Contains(motivo, "enlace") {
			t.Errorf("caso %d: medí %d enlace(s) y el motivo es %q; quiero 1 y que lo nombre", i, s.Enlaces, motivo)
		}
		if code, salida := correrSesiones(t, "", "borrar", id, "--si"); code != 1 || !existe(filepath.Join(projects, "-p", id+".jsonl")) {
			t.Errorf("caso %d: la sesión con un enlace se borró (salió %d):\n%s", i, code, salida)
		}
		if !existe(filepath.Join(afuera, "valioso.txt")) {
			t.Fatalf("caso %d: se borró lo que había del otro lado del enlace", i)
		}
	}
	if casos == 0 {
		t.Skip("este sistema no deja crear enlaces ni junctions")
	}
	t.Logf("enlaces probados: %d de 2 (simbólico; junction sólo en Windows)", casos)
}

func TestSesionesTitulosElPuestoAManoGana(t *testing.T) {
	_, projects := configDePrueba(t)
	conPropio := `{"type":"user","uuid":"u1","message":{"content":"hola"}}
{"type":"ai-title","aiTitle":"Titulo viejo","sessionId":"` + idA + `"}
{"type":"custom-title","customTitle":"Mi nombre\u001b[31m rojo","sessionId":"` + idA + `"}
{"type":"ai-title","aiTitle":"Titulo automatico","sessionId":"` + idA + `"}
`
	soloIA := `{"type":"ai-title","aiTitle":"Primero","sessionId":"` + idB + `"}
{"type":"ai-title","aiTitle":"Ultimo","sessionId":"` + idB + `"}
`
	for id, contenido := range map[string]string{idA: conPropio, idB: soloIA} {
		ruta := filepath.Join(projects, "-p", id+".jsonl")
		escribir(t, ruta, 0)
		if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
			t.Fatal(err)
		}
		envejecer(t, ruta, hace40Dias)
	}
	code, salida := correrSesiones(t, "", "--todas", "--titulos")
	if code != 0 {
		t.Fatalf("salió %d:\n%s", code, salida)
	}
	if !strings.Contains(salida, "«Mi nombre [31m rojo»") || strings.Contains(salida, "Titulo automatico") {
		t.Errorf("el título puesto a mano no le ganó al automático, o no se limpió el ESC:\n%q", salida)
	}
	if strings.Contains(salida, "\x1b") {
		t.Errorf("un ESC del título llegó a la terminal")
	}
	if !strings.Contains(salida, "«Ultimo»") || strings.Contains(salida, "«Primero»") {
		t.Errorf("sin título a mano tiene que valer el último automático:\n%s", salida)
	}
}

func TestSesionesBorrarPideQueElegir(t *testing.T) {
	configDePrueba(t)
	for _, args := range [][]string{{"borrar"}, {"borrar", "--si"}, {"borrar", idA, "--viejas", "3"}, {"listar"}} {
		if code, salida := correrSesiones(t, "", args...); code != 2 {
			t.Errorf("%v salió %d; quiero 2:\n%s", args, code, salida)
		}
	}
	if code, salida := correrSesiones(t, "s\n", "borrar", idC, "--si"); code != 1 || !strings.Contains(salida, "no encontré") {
		t.Errorf("un id que no existe salió %d:\n%s", code, salida)
	}
}

// Como TestUsageDocumentsUserCommands, para el comando de este archivo.
func TestUsageDocumentaSesiones(t *testing.T) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	leido := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(r)
		leido <- b
	}()
	printUsage()
	_ = w.Close()
	os.Stdout = orig
	help := string(<-leido)
	for _, cmd := range []string{"sesiones [--todas]", "sesiones borrar"} {
		if !strings.Contains(help, cmd) {
			t.Errorf("printUsage no documenta %q", cmd)
		}
	}
}
