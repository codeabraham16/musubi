package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TODO `ssh` DE `deploy/` LLEVA `-n`, SALVO LOS QUE PIDEN stdin A PROPÓSITO — Y ESOS SE DECLARAN.
//
// EL DEFECTO QUE ESTA GUARDA EXISTE PARA IMPEDIR, MEDIDO EL 2026-09-08.
//
// `verificar-despliegue.sh` compara los «guiones derivados» —los archivos que se instalan en el
// servidor— adentro de un bucle que lee su lista de un heredoc:
//
//	while IFS='|' read -r rel destino; do
//	    SHA_ALLA="$(sha_alla "$destino")"   # -> corre_alla -> ssh
//	done <<GUIONES
//
// `ssh` sin `-n` LEE stdin, y ahí stdin es el heredoc. El primer `ssh` se comía el resto de la
// lista, el `read` no encontraba más renglones y el bucle TERMINABA. No fallaba: terminaba. Sin
// línea roja, sin amarilla, sin verde — sin línea. Las últimas 3 corridas compararon
// `/usr/local/bin/musubi-backup` y NUNCA `/usr/local/sbin/redesplegar-cerebro.sh`, que es
// exactamente el archivo cuya deriva FUE A111. El agujero que A111 cerró quedó otra vez sin
// vigilancia y el informe se veía igual que siempre.
//
// POR QUÉ LA GUARDA ES DE LA CLASE Y NO DEL CASO. El bug estaba en UNA función, pero el `ssh` sin
// `-n` es inofensivo hasta el día que alguien mueve la llamada adentro de un bucle que lee. Una
// guarda sobre `corre_alla` habría quedado verde el día que el defecto reaparece dos funciones más
// abajo — que es la forma exacta de «la lección aprendida de un lado y no del hermano». Por eso se
// barren TODOS los `.sh` de `deploy/`.
//
// POR QUÉ NO SE PUEDE SATISFACER CON UN COMENTARIO. A107 midió que 7 de 46 guardas de grep de este
// repo quedaban verdes porque el texto que buscaban vivía en un comentario, en un mensaje de error
// o en el vecino. Acá las líneas de comentario se descartan ANTES de mirar, y el `-n` se exige en
// la posición de opción de un `ssh` que está en posición de COMANDO — no alcanza con que la letra
// aparezca en el renglón.
//
// Sabotajes que la ponen en rojo (verificados):
//   - sacarle el `-n` a `corre_alla` en `verificar-despliegue.sh` (el defecto original);
//   - sacarle el `-n` a la espera de red de `comparar-y-latir.sh`;
//   - agregar un `ssh` nuevo sin `-n` en cualquier `.sh` de `deploy/`;
//   - poner el `-n` sólo adentro de un comentario que lo nombre.
func TestTodoSSHDeDespliegueLlevaMenosNSalvoLosQueDeclaranUsarStdin(t *testing.T) {
	// LAS EXCEPCIONES SE DECLARAN ACÁ, UNA POR UNA, CON SU RAZÓN. Una excepción por REGLA
	// («si tiene un pipe, pasa») le daría vía libre a un pipe accidental, que es un defecto tan
	// silencioso como el que esto viene a cerrar: el que escribe el pipe cree que manda datos y
	// en realidad se está comiendo el stdin de un bucle.
	consumenStdinAProposito := map[string]string{
		"deploy/verificar-despliegue.sh:config_curl | ssh":                                                "le pipea la config de curl al servidor",
		"deploy/comparar-y-latir.sh:printf '%s' \"$PAYLOAD\" | ssh":                                       "le pipea el sobre OTLP del latido",
		"deploy/verificar-cobertura.sh:ssh -o BatchMode=yes -o ConnectTimeout=10 \"$SSH_HOST\" 'bash -s'": "le manda el guion por stdin con `< \"$TMP/consulta.sh\"`",
	}

	raiz := filepath.Join("..", "..", "deploy")
	// `ssh` en posición de COMANDO: al principio del renglón, después de un `|`, `;`, `&&`, `||`,
	// `$(`, `"$(`, o de una palabra clave de shell (`until`, `while`, `if`, `then`, `do`, `!`).
	enPosicionDeComando := regexp.MustCompile(`(^|[|;&(]|\$\(|\b(?:until|while|if|then|do|else|elif)\s+|!\s+)\s*ssh\s`)
	llevaMenosN := regexp.MustCompile(`\bssh\s+(-[a-zA-Z]*\s+)*-n\b|\bssh\s+-[a-zA-Z]*n`)

	var faltantes []string
	revisados := 0

	err := filepath.Walk(raiz, func(ruta string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(ruta, ".sh") {
			return err
		}
		crudo, err := os.ReadFile(ruta)
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(filepath.Join("deploy", strings.TrimPrefix(filepath.ToSlash(ruta), filepath.ToSlash(raiz)+"/")))

		for i, linea := range strings.Split(string(crudo), "\n") {
			// LOS COMENTARIOS SE DESCARTAN ANTES DE MIRAR NADA. Sin esto, la guarda se satisface
			// con un `# ojo: acá va ssh -n` y castiga un comentario que explique el defecto.
			sinComentario := linea
			if idx := indiceDeComentario(linea); idx >= 0 {
				sinComentario = linea[:idx]
			}
			if !enPosicionDeComando.MatchString(sinComentario) {
				continue
			}
			revisados++
			if llevaMenosN.MatchString(sinComentario) {
				continue
			}
			exceptuado := false
			for clave := range consumenStdinAProposito {
				partes := strings.SplitN(clave, ":", 2)
				if len(partes) == 2 && partes[0] == rel && strings.Contains(sinComentario, partes[1]) {
					exceptuado = true
					break
				}
			}
			if !exceptuado {
				faltantes = append(faltantes, rel+":"+strconv.Itoa(i+1)+"  "+strings.TrimSpace(linea))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("no se pudo barrer %s: %v", raiz, err)
	}

	// SIN ESTO LA GUARDA SE APAGA SOLA. Si mañana cambia la ruta, el `Walk` no encuentra un solo
	// `.sh`, `faltantes` queda vacío y la prueba pasa en VERDE sin haber mirado nada — que es
	// precisamente el modo de falla que este archivo entero viene a denunciar.
	if revisados < 5 {
		t.Fatalf("sólo se encontraron %d invocaciones de ssh en %s; se esperaban al menos 5. "+
			"O se movieron los guiones, o el barrido dejó de mirar: un verde acá no significaría nada", revisados, raiz)
	}

	if len(faltantes) > 0 {
		t.Errorf("hay %d invocación(es) de `ssh` sin `-n` en deploy/ y sin declararse en "+
			"`consumenStdinAProposito`:\n  %s\n\n"+
			"`ssh` sin `-n` LEE stdin. Adentro de un `while read ... done <<HEREDOC` se come la "+
			"lista y el bucle TERMINA sin imprimir una sola línea — pasó el 2026-09-08 y dejó "+
			"`/usr/local/sbin/redesplegar-cerebro.sh` (el archivo de A111) sin comparar durante 3 "+
			"corridas, con el informe viéndose igual.\n"+
			"Si esta llamada SÍ necesita stdin, agregala al mapa con su razón; si no, poné `-n`.",
			len(faltantes), strings.Join(faltantes, "\n  "))
	}
}

// indiceDeComentario devuelve dónde empieza el comentario de la línea, o -1. Respeta las comillas:
// un `#` adentro de un string no abre un comentario, y varios mensajes de este repo los llevan.
func indiceDeComentario(linea string) int {
	var comillaSimple, comillaDoble bool
	for i := 0; i < len(linea); i++ {
		switch linea[i] {
		case '\\':
			i++
		case '\'':
			if !comillaDoble {
				comillaSimple = !comillaSimple
			}
		case '"':
			if !comillaSimple {
				comillaDoble = !comillaDoble
			}
		case '#':
			if !comillaSimple && !comillaDoble && (i == 0 || linea[i-1] == ' ' || linea[i-1] == '\t') {
				return i
			}
		}
	}
	return -1
}
