package main

// blindaje.go es la mitad que faltaba de A54: el agente DECLARA lo que va a tocar, y el despliegue
// lo verifica ANTES de que producción lo descubra.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA HISTORIA QUE ESTE ARCHIVO EXISTE PARA NO REPETIR
//
// `musubi-agente.service` se blindó para un agente que «sólo lee /proc y habla con loopback»:
// ProtectHome=read-only, ProtectSystem=strict. Era cierto. Después A42 le dio un trabajo nuevo
// —enumerar contenedores— y el blindaje lo prohibía, porque `podman ps` NO es una lectura: abre
// su base de estado en modo escritura y toma media docena de locks.
//
// EL SÍNTOMA NO MENCIONÓ PERMISOS EN NINGÚN LADO. No hubo un «permiso denegado». Hubo un
// `podman ps` saliendo con código 1, el agente abortando el inventario entero, y el cerebro
// podando por ausencia. Costó dos días, y la unidad —que estaba bien escrita para el agente que
// describía— nunca fue sospechosa.
//
// Por eso cada necesidad de acá lleva su SÍNTOMA además de su ruta. La ruta dice qué conceder; el
// síntoma es lo que alguien va a tener delante cuando falte, y es lo único que convierte un
// mensaje de error en un diagnóstico.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// SE DECLARA LO QUE ESTA MÁQUINA VA A HACER, NO TODO LO IMAGINABLE
//
// La lista se arma en tiempo de ejecución y depende de la máquina: si no hay `podman` ni `docker`
// instalados, el agente no va a enumerar contenedores y esas rutas NO son una necesidad. Es la
// MISMA señal que decide si el trabajo corre (`enumerarFuente` y su `hayFuente`) la que decide si
// su blindaje importa — no dos listas que se puedan desincronizar.
//
// Una necesidad de más es tan mala como una de menos: manda a alguien a abrir un permiso que
// nadie usa, y un blindaje con agujeros que nadie necesita deja de leerse.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// acceso es el modo en que el agente va a tocar una ruta. La distinción es la que importa: el
// blindaje de systemd separa leer de escribir, y todo el incidente de A42 fue creer que
// enumerar contenedores era leer.
type acceso string

const (
	accesoLectura   acceso = "lectura"
	accesoEscritura acceso = "escritura"
)

// necesidad es UNA cosa que el agente va a tocar para hacer UN trabajo.
type necesidad struct {
	// Trabajo es la capacidad que la pide, con el nombre que usa quien la lee en el inventario.
	Trabajo string
	Ruta    string
	Acceso  acceso
	// Opcional marca las rutas que pueden NO existir sin que eso sea una falla. Es el mismo `-`
	// de systemd y por el mismo motivo: /run/user/1000 lo crea logind y puede llegar tarde en un
	// arranque. Un ReadWritePaths sobre una ruta ausente hace fallar el arranque de la unidad.
	Opcional bool
	// Sintoma es CÓMO SE VE el fallo cuando esta ruta falta. Es el campo más valioso de la
	// struct: la ruta se puede deducir de un strace, el síntoma es lo que alguien tiene delante
	// dos días antes de pensar en correr un strace.
	Sintoma string
	// Directiva es la línea exacta que lo concede. Un diagnóstico que no termina en algo que se
	// pueda copiar y pegar deja el último tramo —el más fácil de equivocar— en manos de quien
	// menos contexto tiene.
	Directiva string
}

// necesidadesDelAgente arma la lista para ESTA máquina.
//
// `hayCLI` se inyecta para poder probar las dos ramas sin instalar ni desinstalar nada.
func necesidadesDelAgente(home, runtimeDir string, hayCLI func(string) bool) []necesidad {
	var ns []necesidad

	// El token del dispositivo. Se escribe, no sólo se lee: la rotación lo reemplaza en su lugar.
	if home != "" {
		ns = append(ns, necesidad{
			Trabajo: "latido", Acceso: accesoEscritura,
			Ruta:      filepath.Join(home, ".config", "musubi-agente"),
			Sintoma:   "el agente arranca y sale en el acto diciendo que falta la credencial",
			Directiva: "ReadWritePaths=" + filepath.Join(home, ".config", "musubi-agente"),
		})
	}

	// CONTENEDORES: sólo si hay con qué. Sin podman ni docker el agente no los enumera y estas
	// rutas no le hacen falta a nadie.
	if hayCLI("podman") {
		if home != "" {
			ns = append(ns, necesidad{
				Trabajo: "inventario de contenedores", Acceso: accesoEscritura,
				Ruta: filepath.Join(home, ".local", "share", "containers"),
				Sintoma: "`podman ps` sale con código 1 sin decir que es un permiso, y el agente " +
					"NO manda inventario ninguno — ni el de systemd. El cerebro poda por ausencia, " +
					"así que todos los servicios de esta máquina desaparecen del panel a la vez",
				Directiva: "ReadWritePaths=" + filepath.Join(home, ".local", "share", "containers"),
			})
		}
		if runtimeDir != "" {
			for _, sub := range []string{"containers", "libpod"} {
				ns = append(ns, necesidad{
					Trabajo: "inventario de contenedores", Acceso: accesoEscritura,
					Ruta: filepath.Join(runtimeDir, sub), Opcional: true,
					Sintoma: "`podman ps` no puede tomar sus locks de runtime y sale con código 1; " +
						"mismo desenlace que arriba, y encima intermitente: depende de si logind " +
						"llegó antes que esta unidad en el arranque",
					Directiva: "ReadWritePaths=-" + filepath.Join(runtimeDir, sub),
				})
			}
		}
	}
	if hayCLI("docker") {
		// El socket, no un store: el docker de siempre corre como demonio de root y el CLI sólo
		// le habla. Escritura porque un socket se escribe para pedirle algo, aunque lo que se
		// pida sea una lectura.
		ns = append(ns, necesidad{
			Trabajo: "inventario de contenedores", Acceso: accesoEscritura,
			Ruta: "/var/run/docker.sock", Opcional: true,
			Sintoma: "`docker ps` dice que no puede conectar al demonio, y el agente aborta el " +
				"inventario entero igual que con podman",
			Directiva: "ReadWritePaths=-/var/run/docker.sock",
		})
	}

	sort.SliceStable(ns, func(i, j int) bool { return ns[i].Ruta < ns[j].Ruta })
	return ns
}

// revision es el resultado de PROBAR una necesidad. No se deduce del blindaje: se intenta el
// acceso de verdad, en el proceso que corre adentro del confinamiento.
type revision struct {
	necesidad
	Existe  bool
	Alcanza bool
	Motivo  string
}

// estado es el desenlace de una revisión, y son TRES y no dos.
//
// «Falta» y «no se puede» tienen arreglos distintos —un mkdir contra una línea de systemd— y
// mezclarlos es exactamente el error que este archivo existe para no repetir: A54 costó dos días
// porque un problema de montaje se presentó como otra cosa. Un verificador que le eche la culpa
// al blindaje cuando la ruta no existe manda a alguien a editar una unidad que está bien.
type estadoRevision string

const (
	estadoOK        estadoRevision = "ok"
	estadoAusente   estadoRevision = "ausente"
	estadoBloqueada estadoRevision = "bloqueada"
)

// Estado clasifica la revisión.
//
// UNA RUTA OPCIONAL QUE NO EXISTE ESTÁ BIEN. /run/user/1000/libpod no existe hasta que podman
// corre por primera vez, y exigirlo pintaría de rojo una máquina sana. Pero si la ruta EXISTE y
// no se alcanza, eso sí es una falla: lo opcional era el existir, nunca el poder.
func (r revision) Estado() estadoRevision {
	switch {
	case !r.Existe && r.Opcional:
		return estadoOK
	case !r.Existe:
		return estadoAusente
	case !r.Alcanza:
		return estadoBloqueada
	default:
		return estadoOK
	}
}

// revisarBlindaje prueba cada necesidad de verdad.
//
// LA ESCRITURA SE PRUEBA ESCRIBIENDO. No alcanza con `access(W_OK)` ni con mirar los permisos: el
// confinamiento de systemd es un MONTAJE de sólo lectura, y los bits del inodo siguen diciendo que
// se puede. El único chequeo que ve un ReadWritePaths que falta es crear un archivo y borrarlo —
// que es exactamente lo que `podman ps` hace y lo que fallaba.
func revisarBlindaje(ns []necesidad) []revision {
	out := make([]revision, 0, len(ns))
	for _, n := range ns {
		r := revision{necesidad: n}
		info, err := os.Stat(n.Ruta)
		switch {
		case errors.Is(err, os.ErrNotExist):
			r.Motivo = "no existe"
		case err != nil:
			r.Existe = true
			r.Motivo = err.Error()
		default:
			r.Existe = true
			r.Alcanza, r.Motivo = probarAcceso(n.Ruta, n.Acceso, info.IsDir())
		}
		out = append(out, r)
	}
	return out
}

// probarAcceso intenta el acceso pedido y limpia lo que haya creado.
func probarAcceso(ruta string, modo acceso, esDir bool) (bool, string) {
	if modo == accesoLectura {
		f, err := os.Open(ruta)
		if err != nil {
			return false, err.Error()
		}
		_ = f.Close()
		return true, "se puede leer"
	}
	if !esDir {
		// Un socket o un archivo: se abre para escritura SIN truncar ni crear. O_WRONLY sobre un
		// socket unix falla por ser un socket, no por permisos, así que se distingue con O_RDWR y
		// se acepta cualquier error que no sea de permiso.
		f, err := os.OpenFile(ruta, os.O_RDWR, 0)
		if err != nil {
			if os.IsPermission(err) || strings.Contains(err.Error(), "read-only") {
				return false, err.Error()
			}
			// Existe y el error no es de permiso (un socket no se abre así). No es lo que esta
			// revisión busca atrapar y decirlo mal sería peor que no decirlo.
			return true, "existe; el modo no se pudo probar sin efectos (" + err.Error() + ")"
		}
		_ = f.Close()
		return true, "se puede escribir"
	}
	// UN DIRECTORIO SE PRUEBA CREANDO ADENTRO, que es lo que el trabajo real hace. El prefijo con
	// punto y el nombre completo existen para que, si un cierre falla en el peor momento, lo que
	// quede diga de quién es y por qué.
	f, err := os.CreateTemp(ruta, ".musubi-revision-de-blindaje-*")
	if err != nil {
		return false, err.Error()
	}
	nombre := f.Name()
	_ = f.Close()
	_ = os.Remove(nombre)
	return true, "se puede escribir"
}

// informeDeBlindaje arma el texto para una persona. Devuelve además cuántas están bloqueadas para
// que quien llama decida el código de salida.
func informeDeBlindaje(rs []revision) (texto string, fallas int) {
	var b strings.Builder
	if len(rs) == 0 {
		return "Este agente no declara ninguna necesidad de blindaje en esta máquina.\n" +
			"  No es un error: sin podman ni docker instalados, el agente sólo lee /proc.\n", 0
	}
	var bloqueadas, ausentes int
	for _, r := range rs {
		switch r.Estado() {
		case estadoBloqueada:
			bloqueadas++
			fmt.Fprintf(&b, "%s %s — %s (%s)\n", cYellow("✗"), cBold(r.Ruta), r.Trabajo, r.Acceso)
			fmt.Fprintf(&b, "    existe pero el proceso no la alcanza: %s\n", r.Motivo)
			fmt.Fprintf(&b, "    síntoma:  %s\n", r.Sintoma)
			fmt.Fprintf(&b, "    arreglo:  %s\n", r.Directiva)
		case estadoAusente:
			ausentes++
			fmt.Fprintf(&b, "%s %s — %s (%s)\n", cYellow("○"), cBold(r.Ruta), r.Trabajo, r.Acceso)
			fmt.Fprintf(&b, "    NO EXISTE, y no está declarada opcional. Esto NO es el blindaje:\n")
			fmt.Fprintf(&b, "    o el agente no está instalado en esta máquina, o falta crearla.\n")
			fmt.Fprintf(&b, "    arreglo:  mkdir -p %s\n", r.Ruta)
			fmt.Fprintf(&b, "    ojo:      un ReadWritePaths sobre una ruta que no existe hace\n")
			fmt.Fprintf(&b, "              FALLAR el arranque de la unidad entera.\n")
		default:
			estado := r.Motivo
			if !r.Existe {
				estado = "no existe todavía, y está declarada opcional"
			}
			fmt.Fprintf(&b, "%s %s — %s (%s)\n", cGreen("✓"), r.Ruta, r.Trabajo, estado)
		}
	}
	if bloqueadas > 0 {
		fmt.Fprintf(&b, "\n%s %d ruta(s) que EXISTEN y el confinamiento no deja tocar.\n",
			cYellow("→"), bloqueadas)
		b.WriteString("  Las excepciones van en un drop-in, NUNCA editando la unidad base: un\n")
		b.WriteString("  `systemctl edit` sobrevive al próximo despliegue y una unidad editada a mano no.\n")
		b.WriteString("    systemctl edit musubi-agente.service\n")
	}
	if ausentes > 0 {
		fmt.Fprintf(&b, "\n%s %d ruta(s) que faltan. Eso NO se arregla tocando el blindaje.\n",
			cYellow("→"), ausentes)
	}
	return b.String(), bloqueadas + ausentes
}

// dirDeRuntime resuelve el directorio de runtime IGUAL QUE PODMAN, y no leyendo sólo la variable.
//
// MEDIDO CONTRA EL AGENTE REAL (musubi-server, 2026-08-29): el proceso tiene HOME y USER, y NO
// tiene XDG_RUNTIME_DIR. systemd sólo la exporta en unidades de USUARIO; en una unidad de sistema
// con `User=`, no. Podman lo sabe y cae a /run/user/<uid>, que es por qué `podman ps` encuentra
// sus locks igual — y por qué el drop-in de producción tuvo que conceder esas rutas.
//
// Leer sólo la variable dejaba al verificador declarando NADA para /run en la única máquina donde
// importa: en verde, sin mirar las tres rutas que rompieron A42. Exactamente la clase de hueco
// silencioso que este archivo existe para cerrar, y casi la repito adentro del arreglo.
func dirDeRuntime() string {
	if d := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR")); d != "" {
		return d
	}
	// El mismo fallback de podman. Con uid 0 no hay /run/user/0 y devolver "" es lo correcto: un
	// agente corriendo como root no usa el store rootless.
	if uid := os.Getuid(); uid > 0 {
		return filepath.Join("/run", "user", strconv.Itoa(uid))
	}
	return ""
}

// revisarBlindajeDelAgente es lo que corre `musubi agent --revisar-blindaje`.
func revisarBlindajeDelAgente() int {
	// FUERA DE LINUX ESTO NO TIENE NADA QUE DECIR, y decirlo igual sería peor que callarse:
	// `ReadWritePaths` es una directiva de systemd, y en un Windows la herramienta emitiría
	// `ReadWritePaths=C:\Users\gio\...` — un consejo con forma de respuesta que no aplica a
	// nada. Dar una instrucción equivocada con confianza es exactamente el modo de falla que este
	// archivo existe para cerrar; repetirlo acá adentro sería el colmo.
	//
	// Sale con 0: no encontrar nada que revisar NO es una falla.
	if runtime.GOOS != "linux" {
		fmt.Printf("El confinamiento que esta herramienta revisa es el de systemd, y %s no lo tiene.\n",
			runtime.GOOS)
		fmt.Println("  Acá el agente corre sin namespace de montaje propio: lo que lo limita son")
		fmt.Println("  los permisos del usuario con el que arranca, y eso se mira con las")
		fmt.Println("  herramientas del sistema, no con ésta.")
		return 0
	}
	home, _ := os.UserHomeDir()
	ns := necesidadesDelAgente(home, dirDeRuntime(), hayEnPath)
	texto, fallas := informeDeBlindaje(revisarBlindaje(ns))
	fmt.Print(texto)
	if fallas > 0 {
		// SE CORRE ADENTRO DEL CONFINAMIENTO O NO PRUEBA NADA. Un operador que lo corra desde su
		// shell lo va a ver todo en verde: su shell no tiene ProtectHome. Decirlo acá, donde ya
		// hay un problema delante, es tarde; decirlo siempre es ruido. Se dice en la ayuda.
		return 1
	}
	return 0
}

// hayEnPath dice si un ejecutable está instalado. Mismo criterio que usa `enumerarFuente` para
// decidir si un trabajo corre: una sola definición de «esta máquina tiene esto».
func hayEnPath(cli string) bool {
	_, err := exec.LookPath(cli)
	return err == nil
}
