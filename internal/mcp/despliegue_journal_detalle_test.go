package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// EL DETALLE DEL INFORME LO IMPRIME LA SHELL, NO UN PROCESO EFÍMERO — O NO LLEGA AL JOURNAL.
//
// EL DEFECTO, MEDIDO EL 2026-09-08, Y LO QUE COSTÓ.
//
// `verificar-despliegue.sh` imprimía el detalle de cada hallazgo con cinco variantes de
//
//	printf '%s\n' "$faltan" | sed 's/^/      falta: /'
//
// En una terminal se ve perfecto. Bajo systemd NO LLEGA: journald resuelve a qué unidad pertenece
// cada línea leyendo `/proc/<pid>/cgroup` CUANDO LA RECIBE, y `sed` —un proceso de pipeline que
// vive milisegundos— ya murió. Los campos vuelven vacíos y la línea queda huérfana.
//
// Medido sobre el journal real de `musubi-comparar.service`:
//
//	journalctl --user -u musubi-comparar.service | grep -cE 'falta:|firing:|down:|sobra:'  ->   0
//	journalctl --user                            | grep -cE 'falta:|firing:|down:|sobra:'  ->  42
//
// Y los metadatos de una de esas 42: `_COMM=sed`, `_SYSTEMD_UNIT` AUSENTE, `_SYSTEMD_USER_UNIT`
// AUSENTE, `_SYSTEMD_CGROUP` AUSENTE. Sólo sobrevive `SYSLOG_IDENTIFIER`, que va en el fd del
// stream y no se resuelve desde `/proc`.
//
// Verificado con las dos formas EN LA MISMA unidad transitoria y la misma corrida:
//
//	con -u : sólo aparece la línea escrita por el builtin (PID de bash)
//	sin -u : aparecen las dos (la de `sed` con su propio PID)
//
// POR QUÉ NO ES COSMÉTICO. El unit file documenta `journalctl --user -u musubi-comparar.service`
// como LA forma de leer esto. Quien la seguía veía «desplegado A MEDIAS: faltan 1 de 28» y NUNCA
// el nombre; veía «disparadas ahora:» seguido de nada. El titular llegaba y lo accionable no, así
// que para saber qué hacer había que volver a correr el guion A MANO — el paso manual que este
// guion existe para eliminar. La regla que faltaba era `InventarioDeServiciosIncompleto` y estuvo
// TRES DÍAS escrita en el journal sin que la lectura documentada pudiera mostrarla.
//
// LA REGLA, EN UNA LÍNEA: si el stdout de un pipeline no se captura con `$(...)`, va al journal —
// y entonces lo tiene que escribir bash (un builtin), no un proceso que muere enseguida.
//
// Sabotajes que la ponen en rojo (verificados):
//   - volver cualquiera de los cinco sitios a `printf ... | sed 's/^/...'`;
//   - agregar un `echo ... | awk ...` nuevo a nivel de sentencia en cualquiera de los dos guiones;
//   - emitir un prefijo de detalle sin pasar por `detalle`.
func TestElDetalleDelInformeLoEscribeLaShellYNoUnProcesoEfimero(t *testing.T) {
	// LOS DOS GUIONES QUE CORREN BAJO LA UNIDAD. `comparar-y-latir.sh` es el ExecStart y
	// `verificar-despliegue.sh` es lo que invoca; los dos escriben al journal por el mismo fd.
	guiones := []string{
		filepath.Join("..", "..", "deploy", "comparar-y-latir.sh"),
		filepath.Join("..", "..", "deploy", "verificar-despliegue.sh"),
	}

	// (A) Un pipeline A NIVEL DE SENTENCIA hacia un filtro de texto externo. «A nivel de
	// sentencia» = su stdout NO se captura, o sea que va derecho al journal. Si estuviera dentro
	// de `$(...)` la salida se captura y el proceso efímero no escribe a ningún lado.
	emiteYPipea := regexp.MustCompile(`^\s*(printf|echo|cat|comm)\b[^|]*\|\s*(sed|awk|tr|tail|head|cut|sort|uniq|column|fold|nl|rev|paste)\b`)
	esAsignacion := regexp.MustCompile(`^\s*[A-Za-z_][A-Za-z0-9_]*=`)

	// (B) Los prefijos del detalle sólo se emiten por `detalle`.
	prefijos := []string{"falta: ", "down: ", "firing: ", "sobra: "}

	var ofensores []string
	lineasMiradas := 0

	for _, g := range guiones {
		crudo, err := os.ReadFile(g)
		if err != nil {
			t.Fatalf("no se pudo leer %s: %v", g, err)
		}
		rel := filepath.ToSlash(filepath.Join("deploy", filepath.Base(g)))

		for i, linea := range strings.Split(string(crudo), "\n") {
			// Los comentarios se descartan ANTES de mirar: sin esto, el comentario que EXPLICA
			// el defecto (y que cita el `printf ... | sed` textual) pondría la guarda en rojo
			// sobre el archivo ya arreglado — el otro lado del error de A107, que cuesta igual.
			codigo := linea
			if idx := indiceDeComentario(linea); idx >= 0 {
				codigo = linea[:idx]
			}
			if strings.TrimSpace(codigo) == "" {
				continue
			}
			lineasMiradas++

			// (A)
			if !esAsignacion.MatchString(codigo) {
				antesDelPipe := codigo
				if j := strings.Index(codigo, "|"); j >= 0 {
					antesDelPipe = codigo[:j]
				}
				// Si hay `$(` antes del pipe, la salida se captura: no llega al journal.
				if !strings.Contains(antesDelPipe, "$(") && emiteYPipea.MatchString(codigo) {
					ofensores = append(ofensores, rel+":"+strconv.Itoa(i+1)+
						"  [pipeline al journal]  "+strings.TrimSpace(linea))
				}
			}

			// (B)
			for _, p := range prefijos {
				if strings.Contains(codigo, p) && !strings.Contains(codigo, "detalle") {
					ofensores = append(ofensores, rel+":"+strconv.Itoa(i+1)+
						"  [prefijo sin `detalle`]  "+strings.TrimSpace(linea))
					break
				}
			}
		}
	}

	// SIN ESTO LA GUARDA SE APAGA SOLA: si los guiones se mueven o se renombran, `ofensores`
	// queda vacío y esto pasa en VERDE sin haber mirado nada — que es la misma clase de defecto
	// que el archivo entero denuncia, un piso más arriba.
	if lineasMiradas < 300 {
		t.Fatalf("sólo se miraron %d líneas de código entre los dos guiones; se esperaban más de "+
			"300. Se movieron o el barrido dejó de mirar: un verde acá no significaría nada", lineasMiradas)
	}

	if len(ofensores) > 0 {
		t.Errorf("hay %d línea(s) que mandarían salida al journal desde un proceso efímero:\n  %s\n\n"+
			"journald resuelve la unidad de cada línea leyendo `/proc/<pid>/cgroup` al recibirla, y "+
			"un `sed`/`awk` de pipeline ya murió: la línea queda SIN `_SYSTEMD_UNIT` y NO aparece "+
			"bajo `journalctl -u`, que es la lectura que el unit file documenta.\n"+
			"Medido el 2026-09-08: 0 líneas de detalle con `-u` contra 42 sin `-u`, y por eso el "+
			"nombre de la regla que faltaba (`InventarioDeServiciosIncompleto`) estuvo tres días "+
			"invisible.\n"+
			"Usá `detalle '<prefijo>' \"$var\"`: imprime con `printf`, que es un BUILTIN, así que "+
			"escribe bash — que vive toda la corrida y sí se puede resolver.",
			len(ofensores), strings.Join(ofensores, "\n  "))
	}
}
