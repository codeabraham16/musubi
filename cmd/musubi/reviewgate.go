package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"musubi/internal/receipt"
)

// reviewgate.go implementa EL GATE DE REVISIÓN POST-APPLY: el aviso, una vez por
// sesión, de que se acumuló trabajo sin que nadie lo revisara.
//
// EL PROBLEMA QUE RESUELVE. El repo tiene el mecanismo de revisión (la skill
// adversarial-review, musubi_debate) y tiene la autoridad (el recibo de RDD), pero
// nada CONECTA el momento en que hay algo que revisar con el momento en que se
// revisa. El gate del recibo vive en el pre-push: avisa cuando ya se terminó de
// trabajar y lo único que uno quiere es entregar, que es el peor momento posible
// para pedir una revisión. El resultado medido es el modo de falla dominante del
// proyecto: se construye y no se enciende.
//
// QUÉ HACE. Le pregunta a GIT —no al modelo, no a una heurística— cuánto trabajo de
// producción hay encima del último commit; si cruza un umbral y no hay recibo que
// cubra ese estado exacto, inyecta un bloque que nombra la skill y el comando.
//
// QUÉ NO HACE, A PROPÓSITO. No bloquea nada y no decide si el cambio está bien. No
// puede: eso es juicio, y el juicio se delega (regla 3 del repo). Lo único que
// aporta es LA OPORTUNIDAD — poner la pregunta cuando todavía es barata de contestar.
// Que el modelo la tome no se puede garantizar desde acá.
//
// FALLA ABIERTO, SIEMPRE. Fuera de un repo git, con git colgado, sin memoria o con
// el interruptor apagado devuelve "" y la sesión sigue igual (regla 2 del repo).

// metaReviewGateInjected marca la sesión en la que ya se avisó. Lleva el session_id
// como sufijo por el mismo motivo que el recordatorio de captura: sin él, el estado
// SANGRA entre sesiones y una sesión nueva nace creyendo que ya avisó.
const metaReviewGateInjected = "loop_reviewgate_injected"

const (
	// Umbrales. Cualquiera de los dos alcanza: dos archivos de producción tocados ya
	// es un cambio con superficie, y cuarenta líneas en UN archivo también. Exigir las
	// dos condiciones a la vez dejaría pasar justo esos dos casos.
	reviewGateUmbralArchivos = 2
	reviewGateUmbralLineas   = 40

	// Techo de espera de git. El hook tiene 10 s de presupuesto total; dos comandos a
	// 2 s dejan margen de sobra. Si git se cuelga (un lock, un repo en red), el gate
	// calla en vez de frenar la sesión.
	reviewGateTimeout = 2 * time.Second

	// Cotas de lo que se lee del disco para contar líneas de archivos sin trackear.
	// Sin ellas, un node_modules sin ignorar convertiría un aviso en un escaneo.
	reviewGateMaxUntracked = 200
	reviewGateMaxBytes     = 1 << 20 // 1 MB
)

// trabajoSinRevisar es cuánto hay encima del último commit, contando SOLO producción.
type trabajoSinRevisar struct {
	archivos int
	lineas   int
}

// cruzaUmbral decide si lo acumulado amerita el aviso. Es pura y está separada de la
// medición a propósito: así la política se testea sin repositorio y sin git.
func (t trabajoSinRevisar) cruzaUmbral() bool {
	return t.archivos >= reviewGateUmbralArchivos || t.lineas >= reviewGateUmbralLineas
}

// gateProbe es lo que el gate necesita de git. Está detrás de una interfaz para que
// los tests midan la POLÍTICA sin depender del estado real del árbol —que cambia
// mientras los tests corren, y con él el resultado.
type gateProbe interface {
	// pendiente cuenta el trabajo de producción sin commitear. Es la consulta BARATA:
	// --numstat resume el diff en tres columnas por archivo en vez de traerlo entero.
	pendiente() (trabajoSinRevisar, error)
	// huella deriva la huella del árbol, la misma que usa el recibo. Es la consulta
	// CARA (trae el diff completo): sólo se pide cuando ya se sabe que hay un recibo
	// aprobado que podría llegar a cubrirla.
	huella() (string, error)
}

// gitGateProbe es la sonda real: le pregunta a git en el workspace.
type gitGateProbe struct{ root string }

func (g gitGateProbe) pendiente() (trabajoSinRevisar, error) {
	ctx, cancel := context.WithTimeout(context.Background(), reviewGateTimeout)
	defer cancel()
	// core.quotePath=false evita que git escape los nombres no-ASCII en octal, lo que
	// rompería la clasificación por extensión.
	numstat, err := gitOut(ctx, g.root, "-c", "core.quotePath=false", "diff", "HEAD", "--numstat")
	if err != nil {
		return trabajoSinRevisar{}, err
	}
	unt, err := gitOut(ctx, g.root, "-c", "core.quotePath=false", "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return trabajoSinRevisar{}, err
	}
	t := contarNumstat(numstat)
	u := contarUntracked(g.root, unt)
	return trabajoSinRevisar{archivos: t.archivos + u.archivos, lineas: t.lineas + u.lineas}, nil
}

func (g gitGateProbe) huella() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), reviewGateTimeout)
	defer cancel()
	fp, _, err := treeFingerprint(ctx, g.root)
	return fp, err
}

// esProduccion clasifica una ruta como trabajo que amerita revisión adversarial.
//
// Quedan afuera los tests, la documentación y las imágenes. NO es que no importen:
// es que el gate mide RIESGO, y un test o un README que cambian solos no lo tienen.
// Incluirlos haría que escribir documentación dispare el aviso, y un aviso que salta
// cuando no corresponde se aprende a ignorar — que es exactamente cómo muere un gate.
func esProduccion(ruta string) bool {
	p := strings.ToLower(strings.TrimSpace(ruta))
	if p == "" {
		return false
	}
	if strings.HasSuffix(p, "_test.go") {
		return false
	}
	switch filepath.Ext(p) {
	case ".md", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".ico", ".bmp":
		return false
	}
	return true
}

// rutaDeNumstat normaliza la ruta de una línea de --numstat, que para un renombre no
// es una ruta sino una notación con las dos.
//
// MEDIDO, porque la intuición se equivoca acá: en la forma simple ("viejo => nuevo")
// separar NO hace falta — el nombre nuevo va último, así que la última extensión ya es
// la suya. Lo que sí hace falta es la llave: en la forma con llaves de git,
// "docs/{notas.md => otras.md}" tiene extensión ".md}" —con la llave pegada—, no
// coincide con ninguna de la lista de exclusión y un DOC renombrado terminaría contando
// como producción. Lo mismo con "cmd/{a_test.go => b_test.go}", que deja de terminar en
// "_test.go". Las dos cosas quedan porque juntas cubren las dos formas sin adivinar cuál
// vino.
func rutaDeNumstat(campo string) string {
	if i := strings.LastIndex(campo, " => "); i >= 0 {
		campo = campo[i+len(" => "):]
	}
	return strings.TrimRight(campo, "}")
}

// contarNumstat parsea la salida de `git diff HEAD --numstat`, con el formato
// "<agregadas>\t<borradas>\t<ruta>". Un archivo binario trae "-" en las dos primeras
// columnas: cuenta como archivo pero no suma líneas, que es lo correcto — no tiene
// líneas que leer.
func contarNumstat(salida string) trabajoSinRevisar {
	var t trabajoSinRevisar
	for _, ln := range strings.Split(salida, "\n") {
		ln = strings.TrimRight(ln, "\r")
		if strings.TrimSpace(ln) == "" {
			continue
		}
		campos := strings.Split(ln, "\t")
		if len(campos) < 3 {
			continue
		}
		if !esProduccion(rutaDeNumstat(campos[len(campos)-1])) {
			continue
		}
		t.archivos++
		if n, err := strconv.Atoi(strings.TrimSpace(campos[0])); err == nil {
			t.lineas += n
		}
		if n, err := strconv.Atoi(strings.TrimSpace(campos[1])); err == nil {
			t.lineas += n
		}
	}
	return t
}

// contarUntracked cuenta los archivos que todavía no están en ningún diff. Sin esto
// el gate tendría el mismo agujero que tenía el recibo antes de incluirlos: un
// archivo nuevo entero es trabajo 100 % sin revisar y no aparecería por ningún lado.
//
// Las líneas se cuentan LEYENDO el archivo, porque git todavía no las sabe. La
// lectura está acotada —cantidad de archivos y tamaño— para que el costo del gate no
// dependa de lo que haya tirado suelto en el árbol.
func contarUntracked(root, salida string) trabajoSinRevisar {
	var t trabajoSinRevisar
	leidos := 0
	for _, ruta := range strings.Split(salida, "\n") {
		ruta = strings.TrimSpace(strings.TrimRight(ruta, "\r"))
		if !esProduccion(ruta) {
			continue
		}
		t.archivos++
		if leidos >= reviewGateMaxUntracked {
			continue
		}
		leidos++
		t.lineas += lineasDeArchivo(filepath.Join(root, ruta))
	}
	return t
}

// lineasDeArchivo cuenta líneas con cota de tamaño. Devuelve 0 ante cualquier
// problema: el gate prefiere sub-contar —y callarse— a inventar trabajo que no está.
func lineasDeArchivo(ruta string) int {
	st, err := os.Stat(ruta)
	if err != nil || st.IsDir() || st.Size() > reviewGateMaxBytes {
		return 0
	}
	b, err := os.ReadFile(ruta)
	if err != nil {
		return 0
	}
	n := strings.Count(string(b), "\n")
	if n == 0 && len(b) > 0 {
		return 1 // una línea sin salto final sigue siendo una línea
	}
	return n
}

// reviewGateYaRevisado indica si un recibo APROBADO cubre el estado exacto del árbol.
//
// El orden importa y es la optimización que hace barato al gate: primero se mira si
// hay recibo (una lectura de meta) y sólo si lo hay se le pide a git la huella, que
// cuesta el diff completo. En el caso común —nadie emitió recibo todavía— ese diff
// no se pide nunca.
func reviewGateYaRevisado(store turnStore, probe gateProbe) bool {
	raw, ok, err := store.GetMeta(receipt.MetaKey)
	if err != nil || !ok {
		return false
	}
	r := receipt.Decode(raw)
	if r == nil || r.Verdict != receipt.Approved {
		return false
	}
	fp, err := probe.huella()
	if err != nil {
		return false
	}
	return receipt.Check(r, fp).Allowed
}

// buildReviewGate arma el bloque del gate, o "" si no corresponde avisar.
//
// El orden de los cortes es de MÁS BARATO A MÁS CARO, y no es cosmético: esto corre
// en CADA prompt del usuario. Apagar el gate tiene que costar cero —no dos
// subprocesos de git— y una sesión ya avisada tiene que costar una lectura de meta.
//
//  1. el interruptor de apagado    (una variable de entorno)
//  2. ya se avisó en esta sesión   (una lectura de meta)
//  3. cuánto hay sin revisar       (dos comandos de git, salida chica)
//  4. ¿lo cubre un recibo?         (meta y, sólo si hay recibo, el diff completo)
func buildReviewGate(store turnStore, sessionID string, probe gateProbe) string {
	// El interruptor va primero: es el único corte que tiene que ser gratis.
	if os.Getenv("MUSUBI_REVIEW_GATE") == "0" {
		return ""
	}
	if store == nil || probe == nil {
		return ""
	}
	// Una vez por sesión. Repetirlo cada turno lo convertiría en ruido, y el ruido se
	// filtra: un aviso que aparece siempre deja de leerse a los tres turnos.
	clave := metaReviewGateInjected + ":" + sessionID
	if _, ok, _ := store.GetMeta(clave); ok {
		return ""
	}
	t, err := probe.pendiente()
	if err != nil {
		return "" // sin repo, o git no contesta: el gate no es motivo para romper nada
	}
	if !t.cruzaUmbral() {
		return ""
	}
	if reviewGateYaRevisado(store, probe) {
		return ""
	}
	_ = store.SetMeta(clave, "1")
	return fmt.Sprintf("[Musubi — revisión] Hay %s de producción sin revisar encima del último commit (%d líneas) y ningún recibo cubre este estado. "+
		"Antes de seguir apilando, pasalo por la skill adversarial-review: corré VOS las comprobaciones del proyecto (build, vet, go test -count=1) y quedate con la salida real; "+
		"sabotéa cada invariante nuevo y comprobá que se ponga en ROJO; recién ahí abrí el debate con musubi_debate. "+
		"Si sobrevive, sellalo con musubi receipt emit --by <lente> --reason \"<qué se verificó>\" y el aviso se apaga solo. "+
		"Si ahora no es el momento, decilo y seguí — pero que sea una decisión y no un olvido.",
		pluralArchivos(t.archivos), t.lineas)
}

// pluralArchivos evita el "1 archivos" que delata a un mensaje generado.
func pluralArchivos(n int) string {
	if n == 1 {
		return "1 archivo"
	}
	return fmt.Sprintf("%d archivos", n)
}
