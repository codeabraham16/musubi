package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"

	"musubi/internal/transcripts"
)

// `musubi sesiones` lista las sesiones de Claude Code guardadas en disco y borra las que la persona
// elige. Es lo único de la CLI que borra transcripts, y lo hace bajo tres protecciones que NO tienen
// bandera para saltearlas: no toca `memory/` (la memoria automática de Claude Code), no toca nada
// fuera de `<config>/projects` y no toca una sesión que puede estar abierta —modificada en los
// últimos 15 minutos o registrada por Claude Code como abierta—, que puede ser ESTA misma.
//
// EL FORMATO ES DE OTRO PROGRAMA. Medido en davantis-1 el 2026-10-14 (22 carpetas de proyecto, 30
// sesiones de primer nivel):
//
//   - Una sesión es `<config>/projects/<carpeta>/<id>.jsonl`, con `<id>` un uuid, y puede tener al
//     lado una carpeta `<id>/` con los transcripts de sus subagentes y los resultados de sus tools.
//     Las dos cosas son la sesión. `<config>` es CLAUDE_CONFIG_DIR o `~/.claude` (dirConfigDeClaude).
//   - En la carpeta de proyecto hay además cosas que NO son sesiones: `memory/`, un `MEMORY.md`
//     suelto y una carpeta de trabajo con otro nombre, las tres medidas. Por eso una sesión es SÓLO
//     un `<uuid>.jsonl`, y lo único que este comando borra es ese archivo y su `<uuid>/`.
//   - Hay `<uuid>/` sin su `.jsonl` (6 medidas, 0,4 MB entre todas): no son sesiones que se puedan
//     abrir, y este comando no las lista ni las borra.
//   - Claude Code registra cada proceso abierto en `<config>/sessions/<pid>.json`, con el
//     `sessionId` de su sesión: los 6 registros medidos apuntaban a un transcript existente.
//   - «Archivada» no está en el disco: ningún registro ni clave de los 30 transcripts habla de
//     archivar, y lo único que lo nombra son banderas de `~/.claude.json` sobre las sesiones
//     remotas. Por eso el listado no lo muestra.

// ventanaDeActividad es cuánto tiene que llevar quieta una sesión para poder borrarla. Una sesión
// abierta escribe su transcript (o el de un subagente, en su `<id>/`) a cada rato; una tocada adentro
// de la ventana puede ser la que está corriendo, INCLUSO LA MISMA QUE CORRE ESTE COMANDO.
const ventanaDeActividad = 15 * time.Minute

// carpetaDeMemoria es la memoria automática de Claude Code, adentro de cada carpeta de proyecto.
const carpetaDeMemoria = "memory"

// maxDiasViejas acota --viejas: más de un siglo no filtra nada y desborda la cuenta en nanosegundos.
const maxDiasViejas = 36500

var (
	reIDDeSesion = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	reElemento   = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

// esIDDeSesion dice si un texto es un id de sesión de Claude Code: un uuid entero. Un prefijo no
// alcanza, a propósito: con borrar, «el que empieza así» es una adivinanza.
func esIDDeSesion(s string) bool { return reIDDeSesion.MatchString(s) }

// elementoSimple dice si un nombre es UN elemento de ruta y nada más: sólo letras y dígitos ASCII,
// `-` y `_`. Así no entran `..`, un separador de cualquier sistema, el `:` de una unidad o de un
// flujo de Windows, ni los puntos y espacios finales que Windows recorta y que harían de dos nombres
// distintos el mismo archivo. Las carpetas de proyecto ya tienen esa forma (CarpetaDeProyecto) y los
// ids de sesión son uuid.
func elementoSimple(s string) bool { return reElemento.MatchString(s) }

// esMemoria dice si un nombre es la memoria de Claude Code. Sin mayúsculas: en Windows y en macOS
// `MEMORY` y `memory` son la misma carpeta.
func esMemoria(nombre string) bool { return strings.EqualFold(nombre, carpetaDeMemoria) }

// sesionCC es una sesión de Claude Code en disco.
type sesionCC struct {
	ID      string
	Carpeta string    // la carpeta de proyecto: hija directa de projects
	Jsonl   string    // el transcript
	Dir     string    // su `<id>/`, "" si no tiene
	Mod     time.Time // lo más nuevo entre el .jsonl y TODO lo que hay en `<id>/`
	Bytes   int64     // el .jsonl más `<id>/` entero
	// Enlaces cuenta lo que en `<id>/` no es ni archivo ni carpeta: enlaces simbólicos y junctions.
	// Lo que hay del otro lado no es de la sesión, y una sesión con enlaces no se borra.
	Enlaces int
	// Problema es lo que impidió medirla entera. Una sesión que no se pudo medir no se borra.
	Problema string
	Titulo   string
}

// entornoSesiones es dónde mira el comando.
type entornoSesiones struct {
	config, projects string
	wd               string // elige el proyecto cuando no se pide --todas
	ahora            time.Time
}

func entornoDeSesiones() (entornoSesiones, error) {
	config, err := dirConfigDeClaude()
	if err != nil {
		return entornoSesiones{}, fmt.Errorf("no sé dónde está la configuración de Claude Code: %w", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		return entornoSesiones{}, fmt.Errorf("no sé en qué carpeta estoy: %w", err)
	}
	return entornoSesiones{config: config, projects: filepath.Join(config, "projects"), wd: wd, ahora: time.Now()}, nil
}

// alcanceSesiones son las carpetas de proyecto que se miran.
type alcanceSesiones struct {
	todas    bool
	buscada  string   // la carpeta del proyecto actual, cuando no es --todas
	carpetas []string // las que existen y se miran
	enlaces  int      // carpetas de proyecto que son enlaces: no se miran
}

// alcanceDe elige las carpetas de proyecto. Sin --todas es la del proyecto actual con sus variantes:
// en Windows la misma ruta aparece como `c--…` y como `C--…` según quién abrió la sesión, y las dos
// son el mismo proyecto. En Linux y macOS las rutas distinguen mayúsculas y no se pliegan.
func alcanceDe(projects, wd string, todas bool) (alcanceSesiones, error) {
	alc := alcanceSesiones{todas: todas}
	if !todas {
		alc.buscada = transcripts.CarpetaDeProyecto(wd)
	}
	ents, err := os.ReadDir(projects)
	if err != nil {
		return alc, fmt.Errorf("no puedo leer la carpeta de sesiones de Claude Code %s: %w", projects, err)
	}
	plegar := runtime.GOOS == "windows"
	for _, e := range ents {
		if !todas && !mismaCarpeta(e.Name(), alc.buscada, plegar) {
			continue
		}
		switch {
		case e.IsDir():
			alc.carpetas = append(alc.carpetas, e.Name())
		case e.Type()&fs.ModeType != 0:
			// Un enlace o una junction con forma de carpeta de proyecto: lo que hay del otro lado no
			// está en este `projects`, y lo que no está acá no se mira ni se borra.
			alc.enlaces++
		}
	}
	return alc, nil
}

func mismaCarpeta(nombre, buscada string, plegar bool) bool {
	if plegar {
		return strings.EqualFold(nombre, buscada)
	}
	return nombre == buscada
}

// medirSesion lee del disco UNA sesión: tamaño, última modificación y lo que impediría borrarla. No
// sigue enlaces: los cuenta. Devuelve error sólo si el `.jsonl` no está.
func medirSesion(projects, carpeta, id string) (sesionCC, error) {
	s := sesionCC{ID: id, Carpeta: carpeta, Jsonl: filepath.Join(projects, carpeta, id+".jsonl")}
	info, err := os.Lstat(s.Jsonl)
	if err != nil {
		return s, err
	}
	if !info.Mode().IsRegular() {
		s.Problema = "el .jsonl no es un archivo común (¿un enlace?)"
		return s, nil
	}
	s.Bytes, s.Mod = info.Size(), info.ModTime()
	dir := filepath.Join(projects, carpeta, id)
	dinfo, err := os.Lstat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return s, nil
	case err != nil:
		s.Problema = err.Error()
		return s, nil
	case !dinfo.IsDir():
		s.Enlaces++
		return s, nil
	}
	s.Dir = dir
	// WalkDir no entra en un enlace ni en una junction: los ve como entradas que no son carpeta.
	werr := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		if fi.ModTime().After(s.Mod) {
			s.Mod = fi.ModTime()
		}
		switch {
		case d.IsDir():
		case d.Type().IsRegular():
			s.Bytes += fi.Size()
		default:
			s.Enlaces++
		}
		return nil
	})
	if werr != nil {
		s.Problema = werr.Error()
	}
	return s, nil
}

// leerSesiones mide las sesiones de las carpetas dadas, de la más vieja a la más nueva. Sólo un
// `<uuid>.jsonl` es una sesión: `memory/`, un `MEMORY.md` suelto o una carpeta de trabajo no lo son.
func leerSesiones(projects string, carpetas []string) ([]sesionCC, error) {
	var out []sesionCC
	for _, c := range carpetas {
		ents, err := os.ReadDir(filepath.Join(projects, c))
		if err != nil {
			return nil, err
		}
		for _, e := range ents {
			id, esJsonl := strings.CutSuffix(e.Name(), ".jsonl")
			if e.IsDir() || !esJsonl || !esIDDeSesion(id) {
				continue
			}
			s, err := medirSesion(projects, c, id)
			if err != nil {
				continue // se fue entre el listado y la medición: Claude Code también limpia
			}
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Mod.Before(out[j].Mod) })
	return out, nil
}

// sesionesAbiertas lee el registro de procesos de Claude Code (`<config>/sessions/<pid>.json`) y
// devuelve los ids de las sesiones abiertas. Un registro ilegible no se puede atribuir a ninguna
// sesión y se ignora: la ventana de actividad sigue cubriendo a la que está escribiendo.
func sesionesAbiertas(config string) map[string]bool {
	abiertas := map[string]bool{}
	dir := filepath.Join(config, "sessions")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return abiertas
	}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var r struct {
			SessionID string `json:"sessionId"`
		}
		if json.Unmarshal(b, &r) == nil && r.SessionID != "" {
			abiertas[strings.ToLower(r.SessionID)] = true
		}
	}
	return abiertas
}

// protegida dice por qué una sesión NO se puede borrar ("" si se puede), y si el motivo es que puede
// estar en uso, que es lo que el listado muestra como «posiblemente activa». Es la ÚNICA definición:
// la usan el listado, el plan y borrarSesion, que la vuelve a preguntar contra el disco justo antes
// de borrar.
func protegida(s sesionCC, ahora time.Time, abiertas map[string]bool) (motivo string, activa bool) {
	if abiertas[strings.ToLower(s.ID)] {
		return "Claude Code la registra como abierta (sessions/<pid>.json)", true
	}
	if ahora.Sub(s.Mod) < ventanaDeActividad {
		return fmt.Sprintf("se modificó %s: puede ser la que está corriendo, incluso esta misma", haceCuanto(ahora.Sub(s.Mod))), true
	}
	if s.Problema != "" {
		return "no la pude medir entera: " + s.Problema, false
	}
	if s.Enlaces > 0 {
		return fmt.Sprintf("tiene %d enlace(s) en su carpeta: no borro a través de un enlace", s.Enlaces), false
	}
	return "", false
}

// borrarSesion borra UNA sesión: su `.jsonl` y su `<id>/`. Es lo ÚNICO de este comando que borra, y
// no confía en el plan que vio la persona: vuelve a medir la sesión y a leer el registro de Claude
// Code justo antes, porque entre el plan y el «s» pueden pasar minutos, y una sesión vieja retomada
// en ese rato vuelve a estar viva. Devuelve los bytes liberados.
func borrarSesion(config, carpeta, id string) (int64, error) {
	projects := filepath.Join(config, "projects")
	// Nada fuera de projects: carpeta e id son UN elemento cada uno, así que lo que se borra queda
	// exactamente dos niveles debajo de projects; y la carpeta de proyecto tiene que ser una carpeta
	// de este disco, no un enlace a otro lado.
	if !elementoSimple(carpeta) || !elementoSimple(id) {
		return 0, fmt.Errorf("%q / %q no nombra una sesión adentro de %s", carpeta, id, projects)
	}
	// La memoria de Claude Code vive al lado de las sesiones con un nombre de la misma forma: un
	// `memory.jsonl` plantado al lado haría de `memory/` «la carpeta de la sesión».
	if esMemoria(id) {
		return 0, fmt.Errorf("%s es la memoria de Claude Code: no se borra nunca", filepath.Join(projects, carpeta, id))
	}
	if fi, err := os.Lstat(filepath.Join(projects, carpeta)); err != nil || !fi.IsDir() {
		return 0, fmt.Errorf("%s no es una carpeta de este disco (¿un enlace?): no borro adentro", filepath.Join(projects, carpeta))
	}
	s, err := medirSesion(projects, carpeta, id)
	if err != nil {
		return 0, err
	}
	if motivo, _ := protegida(s, time.Now(), sesionesAbiertas(config)); motivo != "" {
		return 0, errors.New(motivo)
	}
	// El transcript primero: si Windows no lo deja borrar porque otro proceso lo tiene abierto, la
	// sesión queda entera, con su carpeta.
	if err := os.Remove(s.Jsonl); err != nil {
		return 0, err
	}
	if s.Dir != "" {
		if err := os.RemoveAll(s.Dir); err != nil {
			return 0, fmt.Errorf("borré %s pero quedó %s: %w", filepath.Base(s.Jsonl), s.Dir, err)
		}
	}
	return s.Bytes, nil
}

// ── musubi sesiones ───────────────────────────────────────────────────────────────────────────────

func runSesiones(args []string) {
	os.Exit(sesionesCmd(args, os.Stdin, os.Stdout, os.Stderr))
}

// sesionesCmd es el comando entero con la entrada y las salidas inyectadas, para probarlo sin
// procesos. Devuelve el código de salida: 0 hecho, 1 no se pudo o no se hizo (incluido el «no» a la
// confirmación), 2 argumentos que no sirven.
func sesionesCmd(args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) > 0 && args[0] == "borrar" {
		return sesionesBorrar(args[1:], in, out, errOut)
	}
	return sesionesListar(args, out, errOut)
}

// parsearIntercalado acepta banderas y argumentos en cualquier orden: `borrar ID --si` y `borrar --si
// ID` dicen lo mismo. El paquete flag corta en el primer argumento que no es bandera, y un `--si`
// escrito después del id quedaría como otro id.
func parsearIntercalado(fl *flag.FlagSet, args []string) ([]string, error) {
	var sueltos []string
	for {
		if err := fl.Parse(args); err != nil {
			return nil, err
		}
		args = fl.Args()
		if len(args) == 0 {
			return sueltos, nil
		}
		sueltos = append(sueltos, args[0])
		args = args[1:]
	}
}

func validarViejas(n int, errOut io.Writer, pre string) bool {
	if n < 0 || n > maxDiasViejas {
		fmt.Fprintf(errOut, "%s --viejas va en días, de 1 a %d\n", pre, maxDiasViejas)
		return false
	}
	return true
}

// filtrarViejas deja las sesiones que nadie tocó en los últimos `dias` días: ni el transcript ni nada
// de su `<id>/`, porque un subagente que escribe ahí es la sesión trabajando.
func filtrarViejas(ss []sesionCC, ahora time.Time, dias int) []sesionCC {
	limite := time.Duration(dias) * 24 * time.Hour
	var out []sesionCC
	for _, s := range ss {
		if ahora.Sub(s.Mod) >= limite {
			out = append(out, s)
		}
	}
	return out
}

func sesionesListar(args []string, out, errOut io.Writer) int {
	const pre = "musubi sesiones:"
	fl := flag.NewFlagSet("sesiones", flag.ContinueOnError)
	fl.SetOutput(errOut)
	todas := fl.Bool("todas", false, "las de todos los proyectos, no sólo las del actual")
	viejas := fl.Int("viejas", 0, "sólo las que no cambiaron hace N días o más")
	titulos := fl.Bool("titulos", false, "agregar el título de cada sesión (lee los transcripts enteros)")
	sueltos, err := parsearIntercalado(fl, args)
	if err != nil {
		return 2
	}
	if len(sueltos) > 0 {
		fmt.Fprintf(errOut, "%s argumento de más: %q (para borrar: musubi sesiones borrar <id>…)\n", pre, sueltos[0])
		return 2
	}
	if !validarViejas(*viejas, errOut, pre) {
		return 2
	}
	env, err := entornoDeSesiones()
	if err != nil {
		fmt.Fprintln(errOut, pre, err)
		return 1
	}
	alc, err := alcanceDe(env.projects, env.wd, *todas)
	if err != nil {
		fmt.Fprintln(errOut, pre, err)
		return 1
	}
	ss, err := leerSesiones(env.projects, alc.carpetas)
	if err != nil {
		fmt.Fprintln(errOut, pre, err)
		return 1
	}
	if *viejas > 0 {
		ss = filtrarViejas(ss, env.ahora, *viejas)
	}
	if *titulos {
		for i := range ss {
			if t, err := tituloDeSesion(ss[i].Jsonl); err != nil {
				ss[i].Titulo = "(no pude leer el título: " + err.Error() + ")"
			} else {
				ss[i].Titulo = t
			}
		}
	}
	imprimirSesiones(out, env, alc, ss, sesionesAbiertas(env.config), *viejas)
	return 0
}

func imprimirSesiones(w io.Writer, env entornoSesiones, alc alcanceSesiones, ss []sesionCC, abiertas map[string]bool, viejas int) {
	if alc.todas {
		fmt.Fprintln(w, "Sesiones de Claude Code — todos los proyectos")
		fmt.Fprintf(w, "  carpeta : %s\n", env.projects)
	} else {
		fmt.Fprintf(w, "Sesiones de Claude Code — este proyecto (%s)\n", env.wd)
		fmt.Fprintf(w, "  carpeta : %s\n", filepath.Join(env.projects, alc.buscada))
	}
	if viejas > 0 {
		fmt.Fprintf(w, "  filtro  : sin cambios hace %d día(s) o más (el .jsonl y su carpeta)\n", viejas)
	}
	if alc.enlaces > 0 {
		fmt.Fprintf(w, "  enlaces : %d carpeta(s) de proyecto son enlaces: no se miran ni se borran\n", alc.enlaces)
	}
	if len(ss) == 0 {
		fmt.Fprintln(w, "\n  No hay sesiones que mostrar.")
		return
	}
	type subtotal struct {
		carpeta string
		n       int
		bytes   int64
	}
	var total int64
	activas, otras := 0, 0
	porCarpeta := map[string]*subtotal{}
	fmt.Fprintf(w, "\n  %-36s  %-16s  %-14s  %10s  %s\n", "ID", "MODIFICADA", "", "TAMAÑO", "CARPETA")
	for _, s := range ss {
		fmt.Fprintf(w, "  %-36s  %-16s  %-14s  %10s  %s\n", s.ID, s.Mod.Local().Format("2006-01-02 15:04"),
			haceCuanto(env.ahora.Sub(s.Mod)), megas(s.Bytes), s.Carpeta)
		if motivo, activa := protegida(s, env.ahora, abiertas); activa {
			activas++
			fmt.Fprintf(w, "      POSIBLEMENTE ACTIVA: %s\n", motivo)
		} else if motivo != "" {
			otras++
			fmt.Fprintf(w, "      NO SE PUEDE BORRAR: %s\n", motivo)
		}
		if s.Titulo != "" {
			fmt.Fprintf(w, "      «%s»\n", s.Titulo)
		}
		total += s.Bytes
		st := porCarpeta[s.Carpeta]
		if st == nil {
			st = &subtotal{carpeta: s.Carpeta}
			porCarpeta[s.Carpeta] = st
		}
		st.n++
		st.bytes += s.Bytes
	}
	fmt.Fprintf(w, "\n  %d sesión(es) · %s · %d posiblemente activa(s)", len(ss), megas(total), activas)
	if otras > 0 {
		fmt.Fprintf(w, " · %d que no se puede(n) borrar por otro motivo", otras)
	}
	fmt.Fprintln(w)
	if len(porCarpeta) > 1 {
		subtotales := make([]*subtotal, 0, len(porCarpeta))
		for _, st := range porCarpeta {
			subtotales = append(subtotales, st)
		}
		sort.Slice(subtotales, func(i, j int) bool {
			if subtotales[i].bytes != subtotales[j].bytes {
				return subtotales[i].bytes > subtotales[j].bytes
			}
			return subtotales[i].carpeta < subtotales[j].carpeta
		})
		temporales := transcripts.ExclusionesPorDefecto()
		fmt.Fprintln(w, "\n  Por carpeta de proyecto:")
		for _, st := range subtotales {
			nota := ""
			if transcripts.Excluida(st.carpeta, temporales) {
				nota = "  (carpeta temporal: experimentos)"
			}
			fmt.Fprintf(w, "    %-50s %4d sesión(es) %12s%s\n", st.carpeta, st.n, megas(st.bytes), nota)
		}
	}
	fmt.Fprintln(w, "\n  Para borrar: musubi sesiones borrar <id>…  o  musubi sesiones borrar --viejas N [--todas]")
	fmt.Fprintln(w, "  Muestra el plan y pregunta. memory/, lo de fuera de projects y las posiblemente activas no se tocan nunca.")
}

type sesionSalteada struct {
	s      sesionCC
	motivo string
}

func sesionesBorrar(args []string, in io.Reader, out, errOut io.Writer) int {
	const pre = "musubi sesiones borrar:"
	fl := flag.NewFlagSet("sesiones borrar", flag.ContinueOnError)
	fl.SetOutput(errOut)
	viejas := fl.Int("viejas", 0, "elegir las que no cambiaron hace N días o más")
	todas := fl.Bool("todas", false, "con --viejas: de todos los proyectos, no sólo del actual")
	si := fl.Bool("si", false, "no preguntar: borrar lo que muestra el plan")
	ids, err := parsearIntercalado(fl, args)
	if err != nil {
		return 2
	}
	if !validarViejas(*viejas, errOut, pre) {
		return 2
	}
	switch {
	case len(ids) == 0 && *viejas == 0:
		fmt.Fprintln(errOut, pre, "decime qué borrar: uno o más ids de los que lista `musubi sesiones`, o --viejas N")
		return 2
	case len(ids) > 0 && *viejas > 0:
		fmt.Fprintln(errOut, pre, "elegí por id o por --viejas, no las dos cosas a la vez")
		return 2
	}
	// Selección: el id tiene que ser un uuid entero ANTES de mirar el disco. Es la puerta que deja
	// afuera a `..`, a los separadores, a `memory` y a los prefijos.
	for _, id := range ids {
		if !esIDDeSesion(id) {
			fmt.Fprintf(errOut, "%s %q no es un id de sesión: tiene que ser el uuid entero, como lo lista `musubi sesiones`\n", pre, id)
			return 2
		}
	}
	env, err := entornoDeSesiones()
	if err != nil {
		fmt.Fprintln(errOut, pre, err)
		return 1
	}
	var elegidas []sesionCC
	if len(ids) > 0 {
		// Un id nombra UNA sesión sin ambigüedad: se busca en todos los proyectos, como la muestra
		// `musubi sesiones --todas`.
		alc, err := alcanceDe(env.projects, env.wd, true)
		if err != nil {
			fmt.Fprintln(errOut, pre, err)
			return 1
		}
		todasLas, err := leerSesiones(env.projects, alc.carpetas)
		if err != nil {
			fmt.Fprintln(errOut, pre, err)
			return 1
		}
		var faltan []string
		vista := map[string]bool{}
		for _, id := range ids {
			hallada := false
			for _, s := range todasLas {
				if !strings.EqualFold(s.ID, id) {
					continue
				}
				hallada = true
				if !vista[s.Jsonl] {
					vista[s.Jsonl] = true
					elegidas = append(elegidas, s)
				}
			}
			if !hallada {
				faltan = append(faltan, id)
			}
		}
		if len(faltan) > 0 {
			fmt.Fprintf(errOut, "%s no encontré %s en %s. No se borró nada.\n", pre, strings.Join(faltan, ", "), env.projects)
			return 1
		}
	} else {
		alc, err := alcanceDe(env.projects, env.wd, *todas)
		if err != nil {
			fmt.Fprintln(errOut, pre, err)
			return 1
		}
		ss, err := leerSesiones(env.projects, alc.carpetas)
		if err != nil {
			fmt.Fprintln(errOut, pre, err)
			return 1
		}
		if elegidas = filtrarViejas(ss, env.ahora, *viejas); len(elegidas) == 0 {
			donde := "este proyecto"
			if *todas {
				donde = "ningún proyecto"
			}
			fmt.Fprintf(out, "No hay sesiones sin cambios hace %d día(s) o más en %s. No se borró nada.\n", *viejas, donde)
			return 0
		}
	}

	abiertas := sesionesAbiertas(env.config)
	var aBorrar []sesionCC
	var salteadas []sesionSalteada
	for _, s := range elegidas {
		if motivo, _ := protegida(s, env.ahora, abiertas); motivo != "" {
			salteadas = append(salteadas, sesionSalteada{s, motivo})
			continue
		}
		aBorrar = append(aBorrar, s)
	}
	imprimirPlan(out, env, aBorrar, salteadas)
	if len(aBorrar) == 0 {
		fmt.Fprintln(out, "Nada que borrar: todas las elegidas están protegidas.")
		return 1
	}

	// Confirmación: sin --si, sólo un «s» explícito sigue.
	if *si {
		fmt.Fprintln(out, "--si: borro sin preguntar.")
	} else {
		fmt.Fprintf(out, "¿Borro %d sesión(es) y libero %s? [s/N] ", len(aBorrar), megas(sumarBytes(aBorrar)))
		if !confirmar(in) {
			fmt.Fprintln(out, "\nNo se borró nada.")
			return 1
		}
	}
	var liberados int64
	fallidas := 0
	for _, s := range aBorrar {
		n, err := borrarSesion(env.config, s.Carpeta, s.ID)
		if err != nil {
			fallidas++
			fmt.Fprintf(out, "  NO borrada %s (%s): %v\n", s.ID, s.Carpeta, err)
			continue
		}
		liberados += n
		fmt.Fprintf(out, "  borrada    %s (%s) · %s\n", s.ID, s.Carpeta, megas(n))
	}
	fmt.Fprintf(out, "Borradas %d de %d sesión(es) · %s liberados.\n", len(aBorrar)-fallidas, len(aBorrar), megas(liberados))
	if fallidas > 0 || len(salteadas) > 0 {
		return 1
	}
	return 0
}

func imprimirPlan(w io.Writer, env entornoSesiones, aBorrar []sesionCC, salteadas []sesionSalteada) {
	if len(aBorrar) > 0 {
		fmt.Fprintf(w, "Voy a borrar %d sesión(es) de Claude Code, %s en total:\n", len(aBorrar), megas(sumarBytes(aBorrar)))
		for _, s := range aBorrar {
			fmt.Fprintf(w, "  %s  %s  %10s  %s\n", s.ID, s.Mod.Local().Format("2006-01-02 15:04"), megas(s.Bytes), s.Carpeta)
		}
	}
	if len(salteadas) > 0 {
		fmt.Fprintf(w, "No toco %d sesión(es):\n", len(salteadas))
		for _, x := range salteadas {
			fmt.Fprintf(w, "  %s  %s — %s\n", x.s.ID, x.s.Carpeta, x.motivo)
		}
	}
	fmt.Fprintf(w, "De cada sesión se borran su .jsonl y su carpeta <id>/. Nunca se tocan memory/ ni nada fuera de %s.\n", env.projects)
}

// confirmar lee UNA línea y dice si es un sí. Cualquier otra cosa —incluido el fin de la entrada, que
// es lo que llega cuando nadie está mirando— es no.
func confirmar(in io.Reader) bool {
	linea, _ := bufio.NewReader(in).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(linea)) {
	case "s", "si", "sí":
		return true
	}
	return false
}

func sumarBytes(ss []sesionCC) int64 {
	var n int64
	for _, s := range ss {
		n += s.Bytes
	}
	return n
}

func megas(b int64) string { return fmt.Sprintf("%.1f MB", float64(b)/(1<<20)) }

func haceCuanto(d time.Duration) string {
	switch {
	case d < 0:
		return "en el futuro"
	case d < time.Minute:
		return "hace <1 min"
	case d < time.Hour:
		return fmt.Sprintf("hace %d min", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("hace %d h", int(d.Hours()))
	}
	return fmt.Sprintf("hace %d d", int(d.Hours()/24))
}

// tituloDeSesion devuelve el título de una sesión: el puesto a mano si hay, y si no el que le puso
// Claude Code; de cada uno, el último, porque se reescriben a lo largo de la sesión.
func tituloDeSesion(jsonl string) (string, error) {
	var ia, propio string
	_, err := transcripts.Leer(jsonl, func(r *transcripts.Registro) {
		switch {
		case r.Type == transcripts.RegistroTituloPropio && r.CustomTitle != "":
			propio = r.CustomTitle
		case r.Type == transcripts.RegistroTituloIA && r.AITitle != "":
			ia = r.AITitle
		}
	})
	if propio != "" {
		return limpiarTitulo(propio), err
	}
	return limpiarTitulo(ia), err
}

// limpiarTitulo deja un título apto para la terminal: sin caracteres de control ni de formato —un
// ESC adentro de un título es una secuencia que la terminal ejecuta—, en una línea y corto.
func limpiarTitulo(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 80 {
		s = string(r[:79]) + "…"
	}
	return s
}
