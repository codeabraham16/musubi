package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// UN COMENTARIO QUE DICE EN PRESENTE QUE UN CABO SIGUE ABIERTO TIENE QUE CITAR UNO QUE LO ESTÉ.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// La regla 6 del registro nombra EXPLÍCITAMENTE este uso: «citalo donde se cierra ("A70 CERRADO",
// "cierra A13"), que es lo que permite seguir la pista desde un comentario del código». O sea que
// las citas desde el código son parte del contrato del registro — y no las miraba nadie.
//
// LO QUE CUESTA UNA CITA VIEJA NO ES LA PROSA. Estos comentarios no describen: RAZONAN. «Hoy no
// explota sólo porque el agente todavía no enumera (A42 abierto)» es un argumento sobre por qué
// una bomba no está armada, y era falso: A42 se cerró y el agente enumera desde hace tiempo. Quien
// leyera ese comentario decidiría que hay margen donde no lo hay.
//
// Se encontraron CUATRO el 2026-09-11 —tres sobre A42 y una sobre A2— y las cuatro afirmaban un
// estado del sistema que había dejado de ser cierto.
//
// SE MIRA LA AFIRMACIÓN EN PRESENTE Y NO CUALQUIER MENCIÓN. Citar un cabo cerrado para contar su
// historia es CORRECTO y es lo que la regla 6 pide: «cuando esto se escribió, A42 estaba abierto»
// es prosa sana. Lo que no puede es decir que sigue abierto.
// ────────────────────────────────────────────────────────────────────────────────────────────

func TestNingunComentarioAfirmaQueUnCaboCerradoSigueAbierto(t *testing.T) {
	vivos := cabosVivosDelRegistro(t)
	if len(vivos) < 20 {
		t.Fatalf("sólo se reconocieron %d cabos vivos en ABIERTO.md y son bastantes más: cambió el "+
			"formato del registro y esta guarda está en verde por no haber podido leer nada", len(vivos))
	}

	// LA AFIRMACIÓN EN PRESENTE, Y NADA MÁS. La primera versión también aceptaba un `abierto)`
	// suelto, y se disparó tres veces sobre prosa CORRECTA: «cuando esto se escribió, A42 estaba
	// abierto» es exactamente lo que la regla 6 pide que se escriba. Una guarda que castiga la
	// forma buena manda a borrar la historia, que es lo contrario de lo que hace falta.
	afirma := regexp.MustCompile(`\b([AB][0-9]{1,3})\b[^.\n]{0,40}(sigue abierto|está abierto|todavía abierto)|` +
		`(sigue abierto|está abierto|todavía abierto)[^.\n]{0,40}\b([AB][0-9]{1,3})\b`)

	// Y LA LÍNEA NO PUEDE ESTAR CORRIGIÉNDOSE A SÍ MISMA. `musubi-alerts-flota.yml` dice «eso
	// sigue abierto (A2)» y sigue, en el MISMO renglón, «y eso dejó de ser cierto el día que el
	// agente…»: está citando una creencia vieja para desmentirla. Leer sólo los 40 caracteres de
	// alrededor no ve la desmentida.
	desmiente := regexp.MustCompile(`dejó de ser cierto|ya no lo est|cuando esto se escribió|estaba abierto|en su momento|se cerró`)

	revisados, citas := 0, 0
	raiz := filepath.Join("..", "..")
	for _, dir := range []string{"internal", "cmd", "deploy"} {
		_ = filepath.Walk(filepath.Join(raiz, dir), func(ruta string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			switch filepath.Ext(ruta) {
			case ".go", ".sh", ".yml":
			default:
				return nil
			}
			crudo, err := os.ReadFile(ruta)
			if err != nil {
				return nil
			}
			revisados++
			for n, linea := range strings.Split(string(crudo), "\n") {
				d := strings.TrimSpace(linea)
				// SÓLO COMENTARIOS: un literal de código que contenga «abierto» no es una
				// afirmación sobre el registro.
				if !strings.HasPrefix(d, "//") && !strings.HasPrefix(d, "#") {
					continue
				}
				if desmiente.MatchString(d) {
					continue
				}
				for _, m := range afirma.FindAllStringSubmatch(d, -1) {
					id := m[1]
					if id == "" {
						id = m[4]
					}
					if id == "" {
						continue
					}
					citas++
					if vivos[id] {
						continue
					}
					rel, _ := filepath.Rel(raiz, ruta)
					t.Errorf("%s:%d afirma en presente que **%s** sigue abierto, y no está en la "+
						"tabla viva de ABIERTO.md.\n    %s\n"+
						"  Estos comentarios no describen: RAZONAN. Una cita vieja hace que quien la "+
						"lea decida que hay margen donde no lo hay.\n"+
						"  Contar la historia está BIEN —«cuando esto se escribió, %s estaba "+
						"abierto»—; lo que no puede es decir que sigue abierto.", rel, n+1, id, d, id)
				}
			}
			return nil
		})
	}
	if revisados < 100 {
		t.Fatalf("sólo se revisaron %d archivos y son muchos más: cambió dónde viven y esta guarda "+
			"está en verde sin mirar nada", revisados)
	}
	t.Logf("%d archivo(s) revisados, %d afirmación(es) de cabo abierto, todas sobre cabos vivos", revisados, citas)
}

// cabosVivosDelRegistro devuelve los números que HOY tienen una fila en las dos tablas del
// registro. Se leen del archivo y no de una lista: una lista sería una copia.
func cabosVivosDelRegistro(t *testing.T) map[string]bool {
	t.Helper()
	crudo, err := os.ReadFile(filepath.Join("..", "..", "specs", "control-de-flota", "ABIERTO.md"))
	if err != nil {
		t.Fatalf("no pude leer ABIERTO.md: %v", err)
	}
	// SÓLO LA PARTE VIVA: la sección 3 es el cementerio, y sus filas NO cuentan como abiertas.
	texto := string(crudo)
	if i := strings.Index(texto, "\n## 3 · Cerrado en este track"); i > 0 {
		texto = texto[:i]
	}
	fila := regexp.MustCompile(`(?m)^\|\s*([AB][0-9]{1,3})\s*\|`)
	vivos := map[string]bool{}
	for _, m := range fila.FindAllStringSubmatch(texto, -1) {
		vivos[m[1]] = true
	}
	return vivos
}
