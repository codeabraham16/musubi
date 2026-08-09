package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// skillmd.go — EXPORTACIÓN AL FORMATO SKILL.md, el que el agente SÍ lee.
//
// POR QUÉ EXISTE ESTE ARCHIVO. Medido: `musubi_resolve_skills` tuvo CERO llamadas en 30 días en el
// ledger local y en el central; ningún hook menciona skills; `.claude/skills/` estaba vacío mientras
// `.musubi/skills/` tenía 11. El arsenal existía, estaba validado y federado — y nada lo aplicaba.
//
// El formato se copió de una skill que YA FUNCIONA en esta máquina, no de la documentación:
// frontmatter con `name` y `description`, y la convención real de meter el CUÁNDO adentro de la
// descripción, porque es lo único que el consumidor lee para decidir si carga la skill.
//
// LA FRONTERA MODEL-FREE SIGUE INTACTA: Musubi GENERA el artefacto de forma determinista; quién
// decide cargarlo es el consumidor, con su propio mecanismo. La inferencia queda del otro lado.

// frasesDeAlcance traduce el vocabulario cerrado de AppliesTo a la frase que va en la descripción.
// Es una tabla y no una plantilla armada al vuelo: el texto que decide si una skill se activa no
// puede depender de una heurística de formateo.
var frasesDeAlcance = map[string]string{
	FasePlanificar:  "estés planificando, antes de tocar código",
	FaseImplementar: "estés implementando un cambio",
	FaseRevisar:     "estés cerrando o revisando un cambio",
	TareaAuditar:    "te pidan auditar un codebase o un área",
	TareaOrquestar:  "la tarea sea grande y paralelizable",
}

// CuandoUsarla devuelve la cláusula que dice CUÁNDO aplica la skill, o "" si no hay con qué.
//
// EL ORDEN DE LAS FUENTES NO ES ARBITRARIO: AppliesTo es vocabulario cerrado y traduce a una frase
// exacta; AlwaysBecause es prosa que el autor escribió a propósito para justificar su '*'; los globs
// son el último recurso, mecánico. Nunca se inventa: si no hay ninguna de las tres, devuelve vacío y
// el validador ya avisa por su lado (desc_no_trigger).
func CuandoUsarla(sk Skill) string {
	texto, _ := cuandoConFuente(sk)
	return texto
}

// fuenteDelCuando dice de dónde salió la cláusula. Importa porque el CONECTOR cambia según la
// fuente, y meter el conector equivocado produce una frase rota — cosa que ningún test cazó y que
// sólo se vio mirando la salida real del export.
type fuenteDelCuando int

const (
	sinCuando fuenteDelCuando = iota
	desdeAppliesTo
	desdeAlwaysBecause
	desdeGlobs
)

func cuandoConFuente(sk Skill) (string, fuenteDelCuando) {
	if frases := frasesDeAppliesTo(sk.AppliesTo); len(frases) > 0 {
		return strings.Join(frases, ", o cuando "), desdeAppliesTo
	}
	if s := strings.TrimSpace(sk.AlwaysBecause); s != "" {
		return s, desdeAlwaysBecause
	}
	if exts := extensionesDe(sk.Triggers); len(exts) > 0 {
		return "toques archivos " + strings.Join(exts, ", "), desdeGlobs
	}
	return "", sinCuando
}

func frasesDeAppliesTo(aplica []string) []string {
	var out []string
	for _, a := range aplica {
		if f, ok := frasesDeAlcance[a]; ok {
			out = append(out, f)
		}
	}
	sort.Strings(out) // determinista: el mismo arsenal produce el mismo archivo
	return out
}

// extensionesDe saca los globs de archivo REALES (descarta el '*', que no dice nada).
func extensionesDe(triggers []string) []string {
	var out []string
	for _, t := range triggers {
		if t == "*" || strings.TrimSpace(t) == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}

// DescripcionParaAgente arma la descripción que va al frontmatter: el CUÁNDO primero, porque es lo
// que el consumidor usa para elegir, y después el QUÉ.
//
// Si la descripción original YA dice cuándo, se devuelve tal cual. Anteponerle otro «Usá cuando…»
// daría una frase rota y —peor— sería el sistema pisando lo que una persona ya escribió bien.
func DescripcionParaAgente(sk Skill) string {
	desc := strings.TrimSpace(sk.Description)
	if containsAny(strings.ToLower(desc), triggerClauses) {
		return desc
	}
	cuando, fuente := cuandoConFuente(sk)
	if cuando == "" {
		return desc
	}
	// EL CONECTOR DEPENDE DE LA FUENTE, y esto salió de mirar la salida real del export:
	//
	// `applies_to` y los globs traducen a frases escritas PARA seguir a «Usá cuando» («estés
	// planificando…», «toques archivos *.go»). `always_because`, en cambio, es prosa que el autor
	// escribió para explicarle a UNA PERSONA por qué su skill lleva '*' («se activa al cerrar un
	// cambio de riesgo», «gobierna el flujo completo de un cambio»). Anteponerle «Usá cuando»
	// produce «Usá cuando gobierna el flujo completo», que está roto — y una descripción rota es
	// exactamente lo único que decide si la skill se activa.
	clausula := "Usá cuando " + cuando + "."
	if fuente == desdeAlwaysBecause {
		clausula = "Cuándo: " + strings.TrimRight(cuando, ".") + "."
	}
	if desc == "" {
		return clausula
	}
	return clausula + " " + desc
}

// ASkillMD serializa la skill al formato SKILL.md. El checksum va en metadata para que el escritor
// pueda reconocer después si el archivo sigue tal como Musubi lo escribió.
//
// Se arma a mano y no con yaml.Marshal porque el frontmatter tiene un orden que importa para quien
// lo lee (name, description, después la procedencia) y Marshal no garantiza otra cosa que el orden
// de declaración de un struct que acá no existe.
func ASkillMD(sk Skill, checksum string) (string, error) {
	if strings.TrimSpace(sk.Name) == "" {
		return "", fmt.Errorf("skill sin nombre: no se puede exportar")
	}
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", sk.Name)
	fmt.Fprintf(&b, "description: %s\n", citarYAML(DescripcionParaAgente(sk)))
	b.WriteString("metadata:\n")
	b.WriteString("  generated_by: musubi\n")
	fmt.Fprintf(&b, "  musubi_checksum: %s\n", checksum)
	if sk.Source != "" {
		fmt.Fprintf(&b, "  source: %s\n", citarYAML(sk.Source))
	}
	if len(sk.Capabilities) > 0 {
		// Las capabilities son binarios que tienen que estar en el PATH. El resolvedor de Musubi las
		// verifica; el consumidor de SKILL.md no sabe de ellas, así que van declaradas para que al
		// menos una persona pueda leer por qué la skill no le funciona.
		fmt.Fprintf(&b, "  requires: %s\n", citarYAML(strings.Join(sk.Capabilities, ", ")))
	}
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimRight(sk.Rules, "\n"))
	b.WriteString("\n")
	return b.String(), nil
}

// citarYAML pone el valor entre comillas dobles y escapa lo mínimo. Las descripciones traen dos
// puntos y comillas simples seguido, y sin comillas el YAML se rompe o —peor— parsea distinto.
func citarYAML(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", " ")
	return `"` + s + `"`
}

// ChecksumSkillMD es la huella del contenido CANÓNICO (el que se generaría con el checksum vacío).
// Mismo truco que skillContentChecksum para las YAML: el campo no puede influir en su propio valor.
func ChecksumSkillMD(sk Skill) (string, error) {
	canon, err := ASkillMD(sk, "")
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(strings.ReplaceAll(canon, "\r", "")))
	return hex.EncodeToString(sum[:]), nil
}

// ChecksumDeSKILLMD extrae el checksum que declara un archivo ya escrito, o "" si no tiene.
// Un archivo sin checksum NO es de Musubi: puede ser una skill que el usuario instaló a mano con el
// mismo nombre, y adoptarla sería reclamar trabajo ajeno.
func ChecksumDeSKILLMD(contenido []byte) string {
	lineas := strings.Split(strings.ReplaceAll(string(contenido), "\r", ""), "\n")
	for i, linea := range lineas {
		l := strings.TrimSpace(linea)
		if i > 0 && l == "---" {
			return "" // se cerró el frontmatter sin checksum: el cuerpo no cuenta
		}
		if strings.HasPrefix(l, "musubi_checksum:") {
			return strings.TrimSpace(strings.TrimPrefix(l, "musubi_checksum:"))
		}
	}
	return ""
}

// SigueIntacto indica si el archivo en disco es EXACTAMENTE lo que Musubi escribió: su checksum
// declarado tiene que coincidir con la huella de su propio contenido con ese campo vaciado.
//
// SAFETY (regla de oro, igual que managedSkillAction): ante la mínima duda, preservar.
func SigueIntacto(contenido []byte) bool {
	declarado := ChecksumDeSKILLMD(contenido)
	if declarado == "" {
		return false
	}
	limpio := strings.ReplaceAll(string(contenido), "\r", "")
	vaciado := strings.Replace(limpio, "musubi_checksum: "+declarado, "musubi_checksum: ", 1)
	sum := sha256.Sum256([]byte(vaciado))
	return hex.EncodeToString(sum[:]) == declarado
}
