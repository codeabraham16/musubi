package transcripts

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Ventana es el intervalo de días pedido. `hasta` es EXCLUSIVO: el día siguiente al pedido a las
// 00:00, así `--hasta 2026-09-20` incluye todo el 20.
//
// LOS DÍAS SON UTC, como los timestamps de los transcripts. Interpretarlos en la hora local haría
// que la misma línea de comandos midiera ventanas distintas en davantis-1 y en la laptop.
type Ventana struct {
	desde, hasta       time.Time
	hayDesde, hayHasta bool
}

// ParsearVentana lee `--desde` y `--hasta` (AAAA-MM-DD, vacíos = sin límite).
func ParsearVentana(desde, hasta string) (Ventana, error) {
	var v Ventana
	if desde != "" {
		t, err := time.Parse("2006-01-02", desde)
		if err != nil {
			return v, fmt.Errorf("--desde %q no es una fecha AAAA-MM-DD", desde)
		}
		v.desde, v.hayDesde = t, true
	}
	if hasta != "" {
		t, err := time.Parse("2006-01-02", hasta)
		if err != nil {
			return v, fmt.Errorf("--hasta %q no es una fecha AAAA-MM-DD", hasta)
		}
		v.hasta, v.hayHasta = t.AddDate(0, 0, 1), true
	}
	if v.hayDesde && v.hayHasta && !v.desde.Before(v.hasta) {
		return v, fmt.Errorf("--hasta %s es anterior a --desde %s: la ventana queda vacía", hasta, desde)
	}
	return v, nil
}

// Desde devuelve el primer día pedido como AAAA-MM-DD, o "" si no hay.
func (v Ventana) Desde() string {
	if !v.hayDesde {
		return ""
	}
	return v.desde.Format("2006-01-02")
}

// Hasta devuelve el último día pedido (inclusive) como AAAA-MM-DD, o "" si no hay.
func (v Ventana) Hasta() string {
	if !v.hayHasta {
		return ""
	}
	return v.hasta.AddDate(0, 0, -1).Format("2006-01-02")
}

// Ubicar dice si un registro con ese timestamp CUENTA (cae adentro de la ventana) y si ya quedó
// DESPUÉS de ella. Un registro de antes de la ventana no cuenta pero sí alimenta el estado —una
// tool cargada ayer sigue cargada hoy—; uno de después no hace nada.
func (v Ventana) Ubicar(ts string) (cuenta, despues bool) {
	if !v.hayDesde && !v.hayHasta {
		return true, false
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return false, false
	}
	if v.hayHasta && !t.Before(v.hasta) {
		return false, true
	}
	if v.hayDesde && t.Before(v.desde) {
		return false, false
	}
	return true, false
}

// EsDeSubagente dice si un transcript es de una sesión hija —un subagente o un hijo de workflow—:
// vive bajo una carpeta `subagents/`. Sirve con la ruta relativa que da Recorrer y con la ruta
// absoluta que un hook recibe en `transcript_path`.
//
// ES LA SEÑAL MEDIDA, NO LA DEL ENTORNO. Un hijo de workflow hereda CLAUDE_CODE_ENTRYPOINT y su
// transcript dice el mismo `entrypoint` que la sesión interactiva: esas dos no distinguen nada. La
// carpeta sí: medido el 2026-09-26, los 286 transcripts de primer nivel tienen `isSidechain` en
// false y los 4.144 de `subagents/` que lo traen, en true.
func EsDeSubagente(ruta string) bool {
	for _, parte := range strings.Split(filepath.ToSlash(ruta), "/") {
		if parte == "subagents" {
			return true
		}
	}
	return false
}

// CarpetaDeProyecto es el nombre de la carpeta donde Claude Code guarda los transcripts de una ruta
// de trabajo: cada carácter que no es letra ni dígito ASCII pasa a `-`. Es un hecho del formato de
// OTRO programa, medido en las 49 carpetas de esta máquina: `/home/davantis/.cache/x` da
// `-home-davantis--cache-x`, y el `_` de `wf_31193ce6` también pasa a `-`.
func CarpetaDeProyecto(ruta string) string {
	var b strings.Builder
	for _, r := range ruta {
		if r < 128 && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	return b.String()
}

// ExclusionesPorDefecto son las carpetas de proyecto de la carpeta TEMPORAL del sistema: la de
// `os.TempDir()` y todas las que cuelgan de ella.
//
// UNA SESIÓN QUE TRABAJA EN UNA CARPETA TEMPORAL ES UN EXPERIMENTO, no trabajo: una prueba de
// conducta con `claude -p` en un mktemp, que por diseño arranca vacía. Medida junto a las reales las
// contamina, y en la dirección que más engaña: el 2026-09-25, las invocaciones de skills y las
// llamadas sin ToolSearch que el medidor contaba venían TODAS de carpetas de experimento, y en los
// proyectos reales eran cero.
//
// SE DERIVA DE `os.TempDir()` y no se escribe `-tmp-*`: en Windows la temporal es
// `C:\Users\…\Temp` y en macOS cuelga de `/var/folders`, y un patrón clavado al Linux de hoy no
// excluiría nada allá. Se prueba también la ruta con los enlaces resueltos, porque la sesión guarda
// la carpeta como la ve el proceso: en macOS `/var` es un enlace a `/private/var`.
//
// Los experimentos que corren FUERA de la temporal no se pueden reconocer desde acá: van con
// `--excluir`.
func ExclusionesPorDefecto() []string {
	var out []string
	visto := map[string]bool{}
	tmp := filepath.Clean(os.TempDir())
	rutas := []string{tmp}
	if real, err := filepath.EvalSymlinks(tmp); err == nil {
		rutas = append(rutas, real)
	}
	for _, r := range rutas {
		base := CarpetaDeProyecto(r)
		for _, patron := range []string{base, base + "-*"} {
			if !visto[patron] {
				visto[patron] = true
				out = append(out, patron)
			}
		}
	}
	return out
}

// Excluida dice si una carpeta de proyecto cae en alguna exclusión. Los patrones ya se validaron al
// leer los argumentos: acá un error de patrón no puede pasar, y si pasara no excluye.
func Excluida(nombre string, exclusiones []string) bool {
	for _, patron := range exclusiones {
		if ok, _ := path.Match(patron, nombre); ok {
			return true
		}
	}
	return false
}

// Archivo es un transcript que Recorrer encontró.
type Archivo struct {
	Ruta string // la ruta para abrirlo
	Rel  string // relativa a la carpeta recorrida
	// Subagente dice si es de una sesión hija (EsDeSubagente). Cada medidor decide qué hace con
	// ellos: `uso-agente` los mide aparte y `--contexto` no los mide.
	Subagente bool
}

// Recorrido cuenta lo que Recorrer dejó afuera, para que el informe lo diga: un número medido sin
// decir qué quedó afuera no se puede comparar con otro.
type Recorrido struct {
	Archivos          int // .jsonl que son transcripts (el journal no cuenta)
	JournalExcluidos  int
	SalteadosPorFecha int
	CarpetasExcluidas int
}

// Recorrer camina `dir` y llama a fn con cada transcript que puede tener registros en la ventana.
// No escribe nada.
//
// `exclusiones` se comparan contra el nombre de cada carpeta de proyecto —las hijas DIRECTAS de
// `dir`—, y una que cae se saltea entera y se cuenta. Sólo las hijas directas: más abajo están las
// carpetas de cada sesión, que se llaman con un uuid, y un patrón pensado para proyectos no tiene
// nada que decir de ellas.
func Recorrer(dir string, v Ventana, exclusiones []string, fn func(Archivo) error) (Recorrido, error) {
	var rec Recorrido
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return rec, fmt.Errorf("no puedo leer la carpeta de transcripts %s: %v", dir, err)
	}
	err := filepath.WalkDir(dir, func(ruta string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// La carpeta pedida no se excluye a sí misma. Con `--dir .` su nombre es «.», que un
			// `--excluir '*'` alcanza, y el informe diría que no hubo nada que medir.
			if ruta == dir {
				return nil
			}
			if filepath.Dir(ruta) == filepath.Clean(dir) && Excluida(d.Name(), exclusiones) {
				rec.CarpetasExcluidas++
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		if d.Name() == JournalDeWorkflow {
			rec.JournalExcluidos++
			return nil
		}
		rec.Archivos++
		// UN ARCHIVO QUE NO CAMBIÓ DESDE ANTES DE LA VENTANA NO PUEDE TENER REGISTROS ADENTRO DE ELLA:
		// cada registro se escribe en o después de su timestamp, y una reescritura al reanudar
		// también mueve el mtime. Saltearlo no cambia el resultado y ahorra leer gigas.
		if v.hayDesde {
			if info, ierr := d.Info(); ierr == nil && info.ModTime().Before(v.desde) {
				rec.SalteadosPorFecha++
				return nil
			}
		}
		rel, _ := filepath.Rel(dir, ruta)
		return fn(Archivo{Ruta: ruta, Rel: rel, Subagente: EsDeSubagente(rel)})
	})
	return rec, err
}
