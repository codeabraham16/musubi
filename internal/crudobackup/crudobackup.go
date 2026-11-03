// Package crudobackup empaqueta el CRUDO de Claude Code —los transcripts .jsonl que registran
// literalmente las sesiones del agente— y lo empuja a un destino off-host. Es el Frente 2 de «la
// memoria sigue al proyecto»: memory.db ya tiene su respaldo propio (internal/memory.BackupTo +
// deploy/musubi-backup.sh), pero el crudo no se respaldaba por ningún lado y se perdía con el disco.
//
// El flujo honra el gotcha MEDIDO del respaldo manual (2026-11-02): tar-ear un .jsonl que una sesión
// VIVA está escribiendo nace con el stream TRUNCADO, y el transporte lo copia fiel, así que un
// exit=0 NO prueba nada. El arreglo robusto tiene cinco pasos y ninguno se saltea:
//
//  1. STAGE QUIETO: se copian las carpetas a un directorio temporal que nadie más escribe.
//  2. EMPAQUETAR: se arma el .tgz (archive/tar + compress/gzip, puro-Go) sobre esa copia estable.
//  3. VERIFICAR EN ORIGEN: se reabre el .tgz, se recorre entero y se calcula su sha256. Si el
//     paquete está truncado, esto FALLA acá —antes de enviar nada—, no en el destino.
//  4. ENVIAR: el transporte copia el .tgz off-host.
//  5. CONFIRMAR LA LLEGADA: el transporte devuelve el sha256 que el DESTINO reporta del archivo
//     recién llegado; si no es idéntico al de origen, la transferencia se corrompió y el respaldo
//     FALLA. La igualdad de sha256 es la única prueba de llegada íntegra; el exit=0 no lo es.
//
// Sin destino declarado el respaldo FALLA-CERRADO (salvo AllowLocalOnly): un respaldo que se cree
// hecho y haya quedado sólo en el disco que se quería proteger es peor que ninguno, porque apaga la
// alarma. El dead-man vive en dos marcas (MarcaOffhost / MarcaOffhostError) que el doctor-check
// crudo_offhost vigila.
package crudobackup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"musubi/internal/transcripts"
)

// Nombres de las marcas dead-man que deja el respaldo del crudo, en el directorio de paquetes. El
// consumidor es el doctor-check crudo_offhost (internal/memory), que NO puede importar este paquete
// —crearía un ciclo crudobackup→transcripts→memory—, así que DUPLICA estos literales en
// crudoOffhostMarkerName / crudoOffhostErrorMarkerName. El contrato es el nombre del archivo en
// disco, igual que entre deploy/musubi-backup.sh y offhostMarkerName. Si cambian acá, cambialos allá;
// un test en cada lado clava el literal para que una divergencia rompa una prueba.
const (
	// MarcaOffhost se (re)escribe en cada envío off-host exitoso. El doctor mira su MTIME para la
	// caducidad; el contenido es informativo (sello, remote, sha256).
	MarcaOffhost = ".last_crudo_offhost"
	// MarcaOffhostError se escribe cuando un envío falla y se BORRA en el siguiente éxito. Un error
	// más nuevo que la última marca buena es lo que vuelve warning al check.
	MarcaOffhostError = ".last_crudo_offhost_error"
)

// Alcance dice qué carpetas de proyecto respaldar.
type Alcance int

const (
	// AlcanceProyecto respalda sólo la carpeta del proyecto actual (derivada del workspace).
	AlcanceProyecto Alcance = iota
	// AlcanceTodos respalda todas las carpetas de proyecto de la máquina, menos los experimentos de
	// la temporal del sistema (transcripts.ExclusionesPorDefecto).
	AlcanceTodos
)

func alcanceSlug(a Alcance) string {
	if a == AlcanceTodos {
		return "todos"
	}
	return "proyecto"
}

// Transporte envía un paquete a un destino off-host y confirma que llegó ÍNTEGRO. La confirmación es
// responsabilidad del transporte porque sólo él sabe cómo preguntarle al destino por el sha256 (p.ej.
// `ssh host sha256sum`); un exit=0 del envío NO es prueba de llegada íntegra. Es inyectable para
// probar la orquestación sin red.
type Transporte interface {
	// Enviar copia el archivo local `paquete` al `remote` (user@host:/ruta, un DIRECTORIO del
	// destino) y devuelve el sha256 (hex) que el DESTINO reporta del archivo recién llegado. El
	// caller compara ese sha contra el de origen: si difieren, la transferencia se corrompió.
	Enviar(paquete, remote string) (shaRemoto string, err error)
}

// Opciones parametriza un respaldo del crudo. El caller (el comando backup) las arma desde los flags
// y la CrudoConfig; los campos inyectables (Transporte, Ahora) existen para las pruebas.
type Opciones struct {
	Alcance Alcance // proyecto actual (default) o todos
	// Workspace es la raíz del proyecto actual: se mangle con transcripts.CarpetaDeProyecto para
	// hallar su carpeta de transcripts, y de él cuelga el LocalDir por defecto.
	Workspace string
	// DirTranscripts es dónde Claude Code guarda los transcripts. Vacío ⇒ transcripts.DirPorDefecto.
	DirTranscripts string
	// LocalDir es dónde se arma el .tgz, se dejan las marcas y se purgan los viejos. Vacío ⇒
	// <Workspace>/.musubi/backups (el mismo lugar que vigila el doctor).
	LocalDir string
	// Remote es el destino off-host (user@host:/ruta). Vacío + !AllowLocalOnly ⇒ error.
	Remote string
	// AllowLocalOnly permite un Remote vacío: arma y verifica local, no envía (no deja marca
	// offhost, así que el doctor dirá «sin medir», que es honesto).
	AllowLocalOnly bool
	// RetentionDays purga los musubi-crudo-*.tgz locales más viejos. El caller pasa el valor
	// efectivo (CrudoConfig.EffectiveRetentionDays). <=0 ⇒ no purgar.
	RetentionDays int
	// WaitNetworkSeconds se lo pasa al transporte real: cuánto esperar a que el host del destino sea
	// alcanzable antes de rendirse. 0 ⇒ no esperar.
	WaitNetworkSeconds int
	// Transporte envía y confirma. Vacío ⇒ el scp real. Inyectable en pruebas.
	Transporte Transporte
	// Ahora es el reloj (timestamp del nombre y de las marcas). Vacío ⇒ time.Now.
	Ahora func() time.Time
	// Log recibe líneas de progreso. Vacío ⇒ se descartan.
	Log func(string)
}

// Resultado informa qué se hizo: el comando lo imprime y deja trazabilidad.
type Resultado struct {
	Paquete  string   // ruta del .tgz local
	Bytes    int64    // tamaño del paquete
	SHA256   string   // sha256 del paquete, calculado en ORIGEN
	Entradas int      // archivos regulares dentro del tar
	Carpetas []string // carpetas de proyecto incluidas (nombres bajo DirTranscripts)
	Enviado  bool     // true si se envió off-host y la llegada se confirmó por sha
	Remote   string   // destino, si se envió
	Purgados int      // .tgz viejos borrados por retención
}

// ArmarYShipear ejecuta el flujo completo. Devuelve el Resultado (parcial si falla después de armar)
// y, ante cualquier fallo, deja la marca de error para el dead-man.
func ArmarYShipear(op Opciones) (Resultado, error) {
	ahora := op.Ahora
	if ahora == nil {
		ahora = time.Now
	}
	logf := op.Log
	if logf == nil {
		logf = func(string) {}
	}
	remote := strings.TrimSpace(op.Remote)

	localDir := strings.TrimSpace(op.LocalDir)
	if localDir == "" {
		if strings.TrimSpace(op.Workspace) == "" {
			return Resultado{}, fmt.Errorf("crudobackup: falta Workspace o LocalDir")
		}
		localDir = filepath.Join(op.Workspace, ".musubi", "backups")
	}

	// Falla-cerrado ANTES de hacer trabajo: sin destino y sin permiso explícito de quedarse local,
	// es un error, no un éxito silencioso. Se marca para que el dead-man lo vea.
	if remote == "" && !op.AllowLocalOnly {
		err := fmt.Errorf("respaldo del crudo sin destino: declará crudo.remote (user@host:/ruta) en .musubi/config.yaml, o pasá --local-only para armar sólo en disco")
		marcarError(localDir, ahora(), err.Error())
		return Resultado{}, err
	}

	// 1. Localizar las carpetas a respaldar.
	dirTx := strings.TrimSpace(op.DirTranscripts)
	if dirTx == "" {
		d, err := transcripts.DirPorDefecto()
		if err != nil {
			return Resultado{}, fmt.Errorf("no sé dónde están los transcripts de Claude Code: %w", err)
		}
		dirTx = d
	}
	carpetas, err := localizarCarpetas(op.Alcance, dirTx, op.Workspace)
	if err != nil {
		return Resultado{}, err
	}
	if len(carpetas) == 0 {
		return Resultado{}, fmt.Errorf("no encontré carpetas de transcripts para respaldar en %s", dirTx)
	}
	logf(fmt.Sprintf("respaldando %d carpeta(s) de %s", len(carpetas), dirTx))

	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return Resultado{}, fmt.Errorf("no pude crear %s: %w", localDir, err)
	}

	// 2. Stage quieto + empaquetar.
	ts := ahora().UTC().Format("20060102-1504")
	paquete := filepath.Join(localDir, fmt.Sprintf("musubi-crudo-%s-%s.tgz", alcanceSlug(op.Alcance), ts))
	entradas, err := armarPaquete(paquete, dirTx, carpetas, logf)
	if err != nil {
		marcarError(localDir, ahora(), fmt.Sprintf("armado: %v", err))
		return Resultado{}, fmt.Errorf("armando el paquete: %w", err)
	}

	// 3. Verificar EN ORIGEN: reabrir, recorrer entero y sacar el sha. Un truncamiento muere acá.
	sha, nVerif, err := verificarPaquete(paquete)
	if err != nil {
		marcarError(localDir, ahora(), fmt.Sprintf("verificación en origen: %v", err))
		return Resultado{}, fmt.Errorf("el paquete recién armado no se puede reabrir (posible truncamiento): %w", err)
	}
	if nVerif != entradas {
		marcarError(localDir, ahora(), fmt.Sprintf("conteo: armé %d entradas, el paquete tiene %d", entradas, nVerif))
		return Resultado{}, fmt.Errorf("el paquete tiene %d entradas pero se empaquetaron %d: está incompleto", nVerif, entradas)
	}
	st, err := os.Stat(paquete)
	if err != nil {
		return Resultado{}, fmt.Errorf("no pude medir el paquete: %w", err)
	}
	res := Resultado{Paquete: paquete, Bytes: st.Size(), SHA256: sha, Entradas: nVerif, Carpetas: carpetas}
	logf(fmt.Sprintf("paquete verificado en origen: %d entradas, %d bytes, sha256 %s", nVerif, st.Size(), sha))

	// 4+5. Enviar y confirmar la llegada por sha remoto.
	if remote != "" {
		tr := op.Transporte
		if tr == nil {
			tr = transporteSCP{waitSeconds: op.WaitNetworkSeconds}
		}
		shaRemoto, err := tr.Enviar(paquete, remote)
		if err != nil {
			marcarError(localDir, ahora(), fmt.Sprintf("envío a %s: %v", remote, err))
			return res, fmt.Errorf("el envío off-host falló (el paquete local quedó en %s): %w", paquete, err)
		}
		if !strings.EqualFold(strings.TrimSpace(shaRemoto), sha) {
			marcarError(localDir, ahora(), fmt.Sprintf("el destino reportó sha %s, origen %s", shaRemoto, sha))
			return res, fmt.Errorf("el paquete llegó corrupto a %s: sha256 remoto %q ≠ origen %q", remote, shaRemoto, sha)
		}
		res.Enviado = true
		res.Remote = remote
		marcarOffhost(localDir, ahora(), remote, sha)
		logf(fmt.Sprintf("enviado a %s y confirmado por sha256", remote))
	}

	// 6. Retención: sólo tras un ciclo exitoso, para no borrar viejos cuando algo falló.
	if purgados, perr := purgarViejos(localDir, op.RetentionDays, ahora()); perr == nil {
		res.Purgados = purgados
		if purgados > 0 {
			logf(fmt.Sprintf("retención: %d paquete(s) viejo(s) purgado(s)", purgados))
		}
	}
	return res, nil
}

// marcarOffhost (re)escribe la marca buena y BORRA la de error: el dead-man vuelve a verde.
func marcarOffhost(dir string, ahora time.Time, remote, sha string) {
	contenido := fmt.Sprintf("%s\nremote=%s\nsha256=%s\n", ahora.UTC().Format(time.RFC3339), remote, sha)
	_ = os.WriteFile(filepath.Join(dir, MarcaOffhost), []byte(contenido), 0o644)
	_ = os.Remove(filepath.Join(dir, MarcaOffhostError))
}

// marcarError deja la marca de error (sello + motivo). No borra la marca buena: el check compara los
// mtimes de ambas y un error más nuevo que el último éxito es lo que lo pone en warning.
func marcarError(dir string, ahora time.Time, motivo string) {
	if strings.TrimSpace(dir) == "" {
		return
	}
	_ = os.MkdirAll(dir, 0o755)
	contenido := fmt.Sprintf("%s\n%s\n", ahora.UTC().Format(time.RFC3339), motivo)
	_ = os.WriteFile(filepath.Join(dir, MarcaOffhostError), []byte(contenido), 0o644)
}
