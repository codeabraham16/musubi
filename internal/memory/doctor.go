package memory

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"musubi/internal/logx"
)

// doctor.go implementa diagnóstico y reparación de la base de memoria, pure-Go.
// Un registry de checks evalúa invariantes (integridad SQLite, sincronía del
// índice FTS, digests faltantes, relaciones huérfanas, columnas del esquema) y,
// para los reparables, ofrece reparación con backup previo y modos plan/dry-run/
// apply. Es el equivalente model-free al "doctor" de Engram.

// CheckResult es el resultado de un check individual.
type CheckResult struct {
	Code       string `json:"code"`
	Status     string `json:"status"` // ok | warning | error
	Message    string `json:"message"`
	Repairable bool   `json:"repairable"`
}

// DiagnoseReport agrupa los resultados de todos los checks.
type DiagnoseReport struct {
	Status string        `json:"status"` // ok | issues
	Checks []CheckResult `json:"checks"`
}

// RepairResult describe una corrida de reparación.
type RepairResult struct {
	Code       string `json:"code"`
	Mode       string `json:"mode"` // plan | dry-run | apply
	Affected   int    `json:"affected"`
	Applied    bool   `json:"applied"`
	BackupPath string `json:"backup_path,omitempty"`
	Message    string `json:"message"`
}

// doctorCheck es una entrada del registry. count/apply son nil si el check no es
// reparable (ej. integridad: se reporta, no se auto-repara).
type doctorCheck struct {
	code  string
	run   func(e *DbEngine) CheckResult
	count func(e *DbEngine) (int, error)
	apply func(e *DbEngine) (int, error)
}

// doctorChecks devuelve el registry de checks (orden estable).
func (e *DbEngine) doctorChecks() []doctorCheck {
	return []doctorCheck{
		{code: "db_integrity", run: checkDBIntegrity},
		{code: "schema_migrations", run: checkSchema, count: countMissingColumns, apply: applySchema},
		{code: "fts_consistency", run: checkFTS, count: countFTSInconsistent, apply: applyRebuildFTS},
		{code: "missing_digests", run: checkDigests, count: countMissingDigests, apply: applyBackfillDigests},
		{code: "stale_gists", run: checkStaleGists, count: countStaleGists, apply: applyRegenGists},
		{code: "orphan_relations", run: checkOrphans, count: countOrphans, apply: applyDeleteOrphans},
		{code: "stale_conflicts", run: checkStaleConflicts, count: countStaleConflicts, apply: applyDeleteStaleConflicts},
		{code: "offhost_backup", run: checkOffhostBackup},
		{code: "outbox_stall", run: checkOutboxStall},
	}
}

// offhostMarkerName es el archivo que deploy/musubi-backup.sh toca (con una marca ISO) SÓLO tras
// un envío OFF-HOST exitoso. Vive junto a los snapshots locales (<workspace>/.musubi/backups).
const offhostMarkerName = ".last_offhost"

// offhostErrorMarkerName es el archivo que deploy/musubi-backup.sh escribe cuando el envío
// off-host FALLA (o BACKUP_REMOTE está vacío sin el escape hatch), y BORRA tras un envío exitoso
// (Track 18). Su presencia le permite a `musubi doctor` distinguir "backup configurado pero
// fallando / que NUNCA funcionó" de "instancia local sin backup" — antes ambos daban 'ok'.
const offhostErrorMarkerName = ".last_offhost_error"

// offhostBackupStaleAfter es la antigüedad máxima tolerada del último backup off-host antes de
// que el dead-man's-switch avise. El timer del cerebro corre a diario; 48h = dos corridas
// perdidas, señal clara de que el timer dejó de shipear (no un atraso puntual de una corrida).
const offhostBackupStaleAfter = 48 * time.Hour

// outboxStallAfter es cuánto puede quedar una observación 'shared' pendiente de envío antes de que
// el doctor lo marque. Un nodo que sincroniza drena el outbox en segundos; una pendiente de HORAS
// sólo pasa si el drain NO está corriendo. Es el detector del stall SILENCIOSO que dejó cientos de
// filas 9 días sin que nada avisara: convierte una pila invisible en un warning visible. 6h tolera
// un corte puntual del central/VPN (esas filas llevan last_error) sin gritar por un atraso menor.
const outboxStallAfter = 6 * time.Hour

// autoHealCodes son los checks de BAJO riesgo que el auto-mantenimiento repara sin
// supervisión: tienen apply mecánico con backup. schema_migrations y db_integrity quedan
// FUERA a propósito (se reportan, no se auto-aplican: un cambio de esquema o de integridad
// sin supervisión es demasiado riesgoso).
var autoHealCodes = map[string]bool{
	"fts_consistency":  true,
	"missing_digests":  true,
	"orphan_relations": true,
}

// AutoHeal diagnostica y repara automáticamente SOLO los checks de bajo riesgo
// (autoHealCodes) en modo apply (con backup previo). Persiste el reporte final
// (post-repair) en meta (MetaLastHealth) para que el hook de arranque lo surfacee.
// Best-effort: el fallo de una reparación individual no aborta el resto. La usa el
// scheduler de fondo (T5.4) para que el ciclo automático también se auto-cure.
func (e *DbEngine) AutoHeal() (DiagnoseReport, error) {
	rep, err := e.Diagnose()
	if err != nil {
		return rep, err
	}
	for _, c := range rep.Checks {
		if c.Status == "ok" || !autoHealCodes[c.Code] {
			continue
		}
		_, _ = e.Repair(c.Code, "apply") // best-effort
	}
	final, err := e.Diagnose()
	if err != nil {
		return rep, err
	}
	if data, mErr := json.Marshal(final); mErr == nil {
		_ = e.SetMeta(MetaLastHealth, string(data))
	}
	return final, nil
}

// Diagnose corre todos los checks y resume el estado general.
func (e *DbEngine) Diagnose() (DiagnoseReport, error) {
	rep := DiagnoseReport{Status: "ok", Checks: []CheckResult{}}
	for _, c := range e.doctorChecks() {
		r := c.run(e)
		rep.Checks = append(rep.Checks, r)
		if r.Status != "ok" {
			rep.Status = "issues"
		}
	}
	return rep, nil
}

// RunCheck corre un único check por código.
func (e *DbEngine) RunCheck(code string) (CheckResult, error) {
	for _, c := range e.doctorChecks() {
		if c.code == code {
			return c.run(e), nil
		}
	}
	return CheckResult{}, fmt.Errorf("check desconocido: %q", code)
}

// Repair repara un check reparable. mode: "plan"/"dry-run" reportan sin mutar;
// "apply" hace un backup del archivo SQLite y aplica la reparación.
func (e *DbEngine) Repair(code, mode string) (RepairResult, error) {
	var chk *doctorCheck
	for _, c := range e.doctorChecks() {
		if c.code == code {
			cc := c
			chk = &cc
			break
		}
	}
	if chk == nil {
		return RepairResult{}, fmt.Errorf("check desconocido: %q", code)
	}
	if chk.apply == nil || chk.count == nil {
		return RepairResult{}, fmt.Errorf("el check %q no es reparable automáticamente", code)
	}

	affected, err := chk.count(e)
	if err != nil {
		return RepairResult{}, err
	}
	res := RepairResult{Code: code, Mode: mode, Affected: affected}

	switch mode {
	case "plan", "dry-run":
		res.Message = fmt.Sprintf("Repararía %d elemento(s) en %q (modo %s, sin cambios).", affected, code, mode)
		return res, nil
	case "apply":
		backup, err := e.backupDB()
		if err != nil {
			return RepairResult{}, fmt.Errorf("no se pudo crear el backup antes de reparar: %w", err)
		}
		n, err := chk.apply(e)
		if err != nil {
			return RepairResult{}, fmt.Errorf("error al aplicar la reparación: %w", err)
		}
		res.Affected = n
		res.Applied = true
		res.BackupPath = backup
		res.Message = fmt.Sprintf("Reparado %q: %d elemento(s). Backup en %s.", code, n, backup)
		return res, nil
	default:
		return RepairResult{}, fmt.Errorf("modo inválido %q (usá plan|dry-run|apply)", mode)
	}
}

// BackupTo crea un snapshot CONSISTENTE de la base en destDir (lo crea si falta), con
// nombre `memory.db.<timestamp>`, usando `VACUUM INTO`. Antes el backup copiaba el archivo
// con io.Copy tras un wal_checkpoint, pero eso podía capturar un estado a medias si había
// escrituras concurrentes (el checkpoint y la copia no son atómicos entre sí). `VACUUM INTO`
// produce una copia transaccionalmente consistente en un solo paso, sin lockear la base para
// el resto de lectores/escritores, y de paso compacta el resultado. Es puro-Go (no requiere
// el CLI sqlite3 en el host). Lo usan el auto-heal del doctor (vía backupDB) y el comando
// `musubi backup` que dispara el timer de backup off-host del cerebro central
// (deploy/musubi-backup.sh, ver docs/Server_Brain_Onboarding.md).
func (e *DbEngine) BackupTo(destDir string) (string, error) {
	if e.path == "" {
		return "", fmt.Errorf("ruta de la base desconocida; no se puede respaldar")
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}
	ts := time.Now().UTC().Format("20060102-150405")
	dest := filepath.Join(destDir, "memory.db."+ts)
	// VACUUM INTO exige que el destino NO exista.
	if _, err := os.Stat(dest); err == nil {
		return "", fmt.Errorf("el destino de backup ya existe: %s", dest)
	}
	// El destino va como literal SQL (VACUUM INTO no admite parámetros enlazados); se
	// escapan las comillas simples duplicándolas. `dest` lo construimos nosotros
	// (directorio + timestamp), no viene de entrada del usuario.
	// #nosec G201 -- VACUUM INTO no admite parámetros enlazados; `dest` lo construimos nosotros
	// (destDir + timestamp), no es entrada del usuario, y las comillas se escapan arriba.
	q := fmt.Sprintf(`VACUUM INTO '%s'`, strings.ReplaceAll(dest, "'", "''"))
	if _, err := e.db.Exec(q); err == nil {
		return dest, nil
	} else {
		// P0.1 — VACUUM INTO LEE Y REESCRIBE TODAS LAS PÁGINAS, así que es exactamente lo que NO se
		// puede hacer sobre una base CORRUPTA: falla, y con él se caía el auto-heal ANTES de reparar
		// nada (el backup previo abortaba la reparación que iba a curar la corrupción).
		//
		// Fallback page-agnóstico: copiar los BYTES crudos. No parsea páginas ⇒ funciona sobre una
		// base corrupta. Es un backup PEOR (puede quedar a medias si hay escrituras concurrentes;
		// para eso existe VACUUM INTO) pero infinitamente mejor que NINGUNO.
		logx.Warn("el backup consistente (VACUUM INTO) falló; cayendo a una copia CRUDA de bytes",
			"error", err, "nota", "backup DE RESCATE: puede quedar inconsistente si hay escrituras concurrentes")
		if cerr := rawCopyDB(e.path, dest); cerr != nil {
			return "", fmt.Errorf("VACUUM INTO falló (%v) y la copia cruda de rescate también: %w", err, cerr)
		}
		return dest, nil
	}
}

// rawCopyDB copia los BYTES del archivo SQLite (y su -wal / -shm, si existen) sin interpretarlos.
// Es el backup de RESCATE: al no parsear páginas, es el único que sobrevive a una base corrupta.
// El -wal se copia porque sin él la copia del .db puede quedar sin los commits más recientes.
func rawCopyDB(src, dest string) error {
	if err := copyFile(src, dest); err != nil {
		return err
	}
	// Sidecars del WAL: best-effort — si no existen, no hay nada que copiar.
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(src + suffix); err != nil {
			continue
		}
		if err := copyFile(src+suffix, dest+suffix); err != nil {
			return fmt.Errorf("no se pudo copiar %s: %w", src+suffix, err)
		}
	}
	return nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// backupDB crea un snapshot en .musubi/backups/ (junto a la base). Es el backup local
// que toma el auto-heal del doctor antes de reparar. Delega en BackupTo.
func (e *DbEngine) backupDB() (string, error) {
	if e.path == "" {
		return "", fmt.Errorf("ruta de la base desconocida; no se puede respaldar")
	}
	return e.BackupTo(filepath.Join(filepath.Dir(e.path), "backups"))
}

// ---- checks individuales ----

// checkOffhostBackup es el DEAD-MAN'S-SWITCH del backup OFF-HOST (DR, Track 17). Lee la marca que
// deja deploy/musubi-backup.sh tras cada envío remoto exitoso y AVISA (warning) si envejeció más
// de offhostBackupStaleAfter: el timer dejó de shipear ⇒ la memoria central volvió a quedar sin
// protección off-host (el CRÍTICO del baseline). Es INFORMATIVO y NO reparable: no es un problema
// de integridad de la base y NO afecta readyz (que sólo sondea GetMeta). Marca AUSENTE ⇒ ok a
// propósito: esta instancia es local (no un cerebro) o el backup aún no corrió/está mal
// configurado — ese caso lo cubre, de forma ruidosa, el fallo-cerrado del propio script (visible
// en `systemctl status musubi-backup`). Así el check no genera falsos positivos en las máquinas de
// desarrollo, que no tienen timer de backup.
func checkOffhostBackup(e *DbEngine) CheckResult {
	if e.path == "" {
		return CheckResult{Code: "offhost_backup", Status: "ok", Message: "ruta de la base desconocida; no aplica"}
	}
	dir := filepath.Join(filepath.Dir(e.path), "backups")
	okInfo, okErr := os.Stat(filepath.Join(dir, offhostMarkerName))
	errInfo, errErr := os.Stat(filepath.Join(dir, offhostErrorMarkerName))

	// Estado de FALLO (Track 18): hay marca de error y, o bien NUNCA hubo un envío exitoso, o el
	// último error es más nuevo que el último éxito. Cierra el falso-negativo del baseline: antes,
	// sin marca de éxito, el check daba 'ok' aunque el timer fallara cada noche (BACKUP_REMOTE mal
	// tipeado/vacío) — el cerebro se veía sano con CERO backups off-host.
	if errErr == nil && (okErr != nil || errInfo.ModTime().After(okInfo.ModTime())) {
		since := time.Since(errInfo.ModTime()).Round(time.Hour)
		if okErr != nil {
			return CheckResult{Code: "offhost_backup", Status: "warning",
				Message: fmt.Sprintf("el backup off-host está configurado pero NUNCA tuvo éxito (último intento falló hace %s); revisá `systemctl status musubi-backup` y BACKUP_REMOTE", since)}
		}
		return CheckResult{Code: "offhost_backup", Status: "warning",
			Message: fmt.Sprintf("el backup off-host viene fallando desde el último éxito (último error hace %s); revisá `systemctl status musubi-backup`", since)}
	}

	if okErr != nil {
		return CheckResult{Code: "offhost_backup", Status: "ok",
			Message: "sin registro de backup off-host (instancia local, o backup no configurado — el timer falla-cerrado si BACKUP_REMOTE está vacío)"}
	}
	if age := time.Since(okInfo.ModTime()); age > offhostBackupStaleAfter {
		return CheckResult{Code: "offhost_backup", Status: "warning",
			Message: fmt.Sprintf("el último backup off-host fue hace %s (> %s): el timer podría haber dejado de shipear (dead-man's-switch)",
				age.Round(time.Hour), offhostBackupStaleAfter)}
	}
	return CheckResult{Code: "offhost_backup", Status: "ok",
		Message: fmt.Sprintf("último backup off-host hace %s", time.Since(okInfo.ModTime()).Round(time.Hour))}
}

// checkOutboxStall es el detector del STALL SILENCIOSO del sync saliente (F2). Un nodo sano drena el
// outbox en segundos; que haya observaciones 'shared' pendientes desde hace horas sólo pasa si el
// drain no corre. Distingue por last_error: VACÍO ⇒ nunca se intentó enviar (el drain ni arrancó:
// bloque sync ausente/inválido, o un nodo terminal con filas huérfanas de un binario viejo); CON
// error ⇒ el envío viene fallando (central/VPN caídos). Es INFORMATIVO y no reparable a ciegas: si
// purgar o reintentar depende de saber si el nodo DEBE sincronizar, y eso lo sabe el arranque (que
// tiene la config), no el doctor. Outbox vacío o al día ⇒ ok. Cierra el agujero que dejó 650 filas
// 9 días en silencio: ahora el doctor y el hook de arranque lo muestran en amarillo.
func checkOutboxStall(e *DbEngine) CheckResult {
	h, err := e.OutboxHealth()
	if err != nil {
		return CheckResult{Code: "outbox_stall", Status: "error", Message: "no se pudo leer la salud del outbox: " + err.Error()}
	}
	if h.Pending == 0 || h.OldestPendingAgeSec < int64(outboxStallAfter.Seconds()) {
		return CheckResult{Code: "outbox_stall", Status: "ok", Message: "el outbox del sync saliente está al día"}
	}
	age := (time.Duration(h.OldestPendingAgeSec) * time.Second).Round(time.Hour)
	if strings.TrimSpace(h.LastError) == "" {
		return CheckResult{Code: "outbox_stall", Status: "warning",
			Message: fmt.Sprintf("%d observación(es) 'shared' llevan hasta %s pendientes SIN UN SOLO intento de envío: el drain del sync no está corriendo (bloque sync ausente/inválido, o nodo terminal). Un nodo terminal se auto-purga al reiniciar el binario; si debería sincronizar, revisá el bloque sync de .musubi/config.yaml", h.Pending, age)}
	}
	return CheckResult{Code: "outbox_stall", Status: "warning",
		Message: fmt.Sprintf("%d observación(es) 'shared' pendientes desde hace %s; el envío al central viene fallando: %s", h.Pending, age, h.LastError)}
}

func checkDBIntegrity(e *DbEngine) CheckResult {
	var result string
	if err := e.db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return CheckResult{Code: "db_integrity", Status: "error", Message: "no se pudo correr integrity_check: " + err.Error()}
	}
	if result != "ok" {
		return CheckResult{Code: "db_integrity", Status: "error", Message: "integridad comprometida: " + result}
	}
	return CheckResult{Code: "db_integrity", Status: "ok", Message: "integridad SQLite correcta"}
}

func checkSchema(e *DbEngine) CheckResult {
	n, err := countMissingColumns(e)
	if err != nil {
		return CheckResult{Code: "schema_migrations", Status: "error", Message: err.Error(), Repairable: true}
	}
	if n > 0 {
		return CheckResult{Code: "schema_migrations", Status: "error", Message: fmt.Sprintf("faltan %d columna(s) del esquema", n), Repairable: true}
	}
	return CheckResult{Code: "schema_migrations", Status: "ok", Message: "esquema de observations al día"}
}

func expectedObsColumns() []string {
	return []string{"gist", "content_hash", "tokens", "last_accessed", "access_count", "importance", "archived", "superseded_by"}
}

func countMissingColumns(e *DbEngine) (int, error) {
	cols, err := e.observationColumns()
	if err != nil {
		return 0, err
	}
	missing := 0
	for _, c := range expectedObsColumns() {
		if !cols[c] {
			missing++
		}
	}
	return missing, nil
}

func applySchema(e *DbEngine) (int, error) {
	n, _ := countMissingColumns(e)
	if err := e.migrateObservations(); err != nil {
		return 0, err
	}
	return n, nil
}

// ftsIntegrityErr corre el comando NATIVO de FTS5 `integrity-check`, que valida la estructura
// INTERNA del índice invertido Y (con rank=1, external-content) que sus tokens coincidan con el
// contenido de `observations`. Un índice corrupto o desincronizado puede tener el COUNT(*)
// PERFECTO — las filas están, lo que está roto es el b-tree del índice o los tokens vs el contenido.
// nil = índice sano.
func ftsIntegrityErr(e *DbEngine) error {
	// rank=1: para una FTS external-content, el integrity-check verifica NO SÓLO el b-tree interno
	// del índice sino que sus tokens COINCIDAN con el contenido actual de `observations` (leído por
	// rowid). Es lo que atrapa el desync más peligroso de external-content: un rowid renumerado por
	// VACUUM sin rebuild, o un trigger que no corrió. El check básico (sin rank) NO ve ese caso —
	// pasa aunque el índice apunte a contenido viejo (verificado). Devuelve "database disk image is
	// malformed" ante el mismatch, que es reparable con applyRebuildFTS.
	_, err := e.db.Exec(`INSERT INTO observations_fts(observations_fts, rank) VALUES('integrity-check', 1)`)
	return err
}

// checkFTS corre el integrity-check external-content (rank=1), que es a la vez el detector de
// corrupción INTERNA del índice y el de DESINCRONIZACIÓN con el contenido (tokens que no coinciden
// con `observations` — p.ej. rowids renumerados por VACUUM sin rebuild, o un trigger que no corrió).
//
// P0.3 — el motivo original era que el doctor viera la corrupción interna (antes sólo miraba un
// drift de COUNT(*), y era ciego a un índice corrupto con el conteo correcto). Con external-content
// (v17) el drift de COUNT ya NO es un modo de falla posible: `COUNT(*) FROM observations_fts` LEE de
// la tabla de contenido, así que siempre iguala a `observations`. El que reemplaza ambos chequeos es
// el integrity-check rank=1, que valida índice↔contenido de una. El que repara (applyRebuildFTS)
// también VE.
func checkFTS(e *DbEngine) CheckResult {
	if err := ftsIntegrityErr(e); err != nil {
		return CheckResult{Code: "fts_consistency", Status: "error",
			Message:    "el índice FTS está CORRUPTO o desincronizado (integrity-check falló): " + err.Error(),
			Repairable: true}
	}
	return CheckResult{Code: "fts_consistency", Status: "ok", Message: "índice FTS sincronizado e íntegro"}
}

// countFTSInconsistent reporta 1 si el índice FTS está corrupto/desincronizado, 0 si está sano.
// Binario (no hay "cuántas filas de drift" con external-content), pero preserva la forma que el
// registry del doctor espera para el conteo de problemas.
func countFTSInconsistent(e *DbEngine) (int, error) {
	if ftsIntegrityErr(e) != nil {
		return 1, nil
	}
	return 0, nil
}

// ftsTableDDL es la ÚNICA definición de la tabla FTS: la usan el esquema (database.go), la
// migración v17 y la reconstrucción del doctor, para que no puedan divergir.
//
// EXTERNAL-CONTENT (v17): la FTS NO guarda su propia copia del contenido — lo LEE de
// `observations` por rowid (`content='observations', content_rowid='rowid'`). Elimina la
// duplicación del texto (el contenido pesaba dos veces en disco). La columna `id` (TEXT) ya no
// se almacena en la FTS: el join a observations es por rowid. OJO: `observations` no tiene
// INTEGER PRIMARY KEY, así que su rowid lo puede RENUMERAR un VACUUM — por eso Compact
// reconstruye la FTS después de vacuumear (ver retention.go).
const ftsTableDDL = `CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts USING fts5(
	topic_key UNINDEXED,
	content,
	content='observations',
	content_rowid='rowid'
);`

// ftsTrigger{AI,AD,AU} mantienen la FTS external-content sincronizada con observations. Son la
// ÚNICA fuente de verdad de los triggers (las comparten el esquema baseline, la migración v17 y
// el repair). Patrón external-content: el INSERT indexa por rowid; el 'delete' necesita los
// valores VIEJOS de las columnas indexadas (old.*) para removerlos del índice invertido.
const ftsTriggerAI = `CREATE TRIGGER IF NOT EXISTS observations_ai AFTER INSERT ON observations BEGIN
	INSERT INTO observations_fts(rowid, topic_key, content) VALUES (new.rowid, new.topic_key, new.content);
END;`

const ftsTriggerAD = `CREATE TRIGGER IF NOT EXISTS observations_ad AFTER DELETE ON observations BEGIN
	INSERT INTO observations_fts(observations_fts, rowid, topic_key, content) VALUES('delete', old.rowid, old.topic_key, old.content);
END;`

const ftsTriggerAU = `CREATE TRIGGER IF NOT EXISTS observations_au AFTER UPDATE ON observations BEGIN
	INSERT INTO observations_fts(observations_fts, rowid, topic_key, content) VALUES('delete', old.rowid, old.topic_key, old.content);
	INSERT INTO observations_fts(rowid, topic_key, content) VALUES (new.rowid, new.topic_key, new.content);
END;`

// applyRebuildFTS reconstruye el índice FTS desde `observations` con DROP + recrear + re-poblar.
//
// P0.2 — Antes hacía `DELETE FROM observations_fts`, y ahí estaba el bug: DELETE **recorre el
// b-tree** del índice para borrar fila por fila ⇒ toca las páginas corruptas ⇒ FALLA justo en el
// único caso que tenía que curar. DROP TABLE, en cambio, **libera las páginas sin leer el
// contenido**, así que sobrevive a la corrupción.
//
// Se DROPea (no DELETE) para sobrevivir a la corrupción, y se re-puebla con el comando `'rebuild'`
// de FTS5 — que RELEE el contenido de `observations` por rowid. Ese comando sólo aplica a tablas
// *contentless* o *external-content*; desde v17 `observations_fts` ES external-content, así que ya
// es válido (sobre la FTS regular anterior daba error, y por eso antes se re-poblaba con INSERT
// ... SELECT). Lee de la tabla base, no del índice corrupto que acabamos de dropear.
//
// Los TRIGGERS sobreviven: están definidos sobre `observations` (no sobre la tabla FTS) y apuntan a
// `observations_fts` por NOMBRE, así que al recrearla vuelven a funcionar. Hay un test que lo
// verifica (no se asume).
func applyRebuildFTS(e *DbEngine) (int, error) {
	tx, err := e.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DROP TABLE IF EXISTS observations_fts`); err != nil {
		return 0, fmt.Errorf("no se pudo dropear el índice FTS: %w", err)
	}
	if _, err := tx.Exec(ftsTableDDL); err != nil {
		return 0, fmt.Errorf("no se pudo recrear el índice FTS: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO observations_fts(observations_fts) VALUES('rebuild')`); err != nil {
		return 0, fmt.Errorf("no se pudo re-poblar el índice FTS: %w", err)
	}
	var obs int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&obs); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return obs, nil
}

func checkDigests(e *DbEngine) CheckResult {
	n, err := countMissingDigests(e)
	if err != nil {
		return CheckResult{Code: "missing_digests", Status: "error", Message: err.Error(), Repairable: true}
	}
	if n > 0 {
		return CheckResult{Code: "missing_digests", Status: "warning",
			Message: fmt.Sprintf("%d observación(es) sin gist/content_hash", n), Repairable: true}
	}
	return CheckResult{Code: "missing_digests", Status: "ok", Message: "todas las observaciones tienen digests"}
}

func countMissingDigests(e *DbEngine) (int, error) {
	var n int
	err := e.db.QueryRow(
		`SELECT COUNT(*) FROM observations WHERE gist IS NULL OR gist='' OR content_hash IS NULL OR content_hash=''`,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("error al contar digests faltantes: %w", err)
	}
	return n, nil
}

func applyBackfillDigests(e *DbEngine) (int, error) {
	n, _ := countMissingDigests(e)
	if err := e.backfillDigests(); err != nil {
		return 0, err
	}
	return n, nil
}

// checkStaleGists busca gists que NO DEJAN DECIDIR: los que un extractor viejo dejó cortados en la
// primera oración, abandonando el presupuesto que les sobraba ("SDD tasks — brain-dashboard
// BACKEND.", 8 tokens de un techo de 24).
//
// El gist existe para UNA cosa: que el agente decida SI VALE LA PENA EXPANDIR la memoria. Uno que no
// deja decidir cuesta tokens y obliga a expandir igual — se paga DOS VECES por lo que debía anticipar.
//
// La reparación es EXPLÍCITA (--fix) a propósito: reescribir 461 gists EN SILENCIO al arrancar el
// binario sería un cambio invisible en la superficie que el agente lee. Y es segura por naturaleza:
// el gist es DERIVADO de content, así que regenerarlo es idempotente y no puede perder nada.
func checkStaleGists(e *DbEngine) CheckResult {
	n, err := countStaleGists(e)
	if err != nil {
		return CheckResult{Code: "stale_gists", Status: "error", Message: err.Error(), Repairable: true}
	}
	if n > 0 {
		return CheckResult{Code: "stale_gists", Status: "warning", Repairable: true,
			Message: fmt.Sprintf("%d gist(s) desaprovechan su presupuesto (se pueden recalcular sin perder nada)", n)}
	}
	return CheckResult{Code: "stale_gists", Status: "ok", Message: "los gists aprovechan su presupuesto"}
}

// staleGist decide si el gist guardado difiere del que produce el extractor ACTUAL. Es la única
// definición de "viejo" que no depende de recordar qué versión lo escribió.
func staleGist(gist, content string) bool {
	return gist != "" && gist != Gist(content, defaultGistMaxTokens)
}

func countStaleGists(e *DbEngine) (int, error) {
	rows, err := e.db.Query(`SELECT gist, content FROM observations WHERE gist IS NOT NULL AND gist != ''`)
	if err != nil {
		return 0, fmt.Errorf("error al consultar gists: %w", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var g, c string
		if err := rows.Scan(&g, &c); err != nil {
			return 0, err
		}
		if staleGist(g, c) {
			n++
		}
	}
	return n, rows.Err()
}

// applyRegenGists recalcula los gists desviados. NO toca content, ni content_hash, ni los
// embeddings, ni las relaciones: el gist es lo ÚNICO que se recalcula.
func applyRegenGists(e *DbEngine) (int, error) {
	rows, err := e.db.Query(`SELECT id, gist, content FROM observations WHERE gist IS NOT NULL AND gist != ''`)
	if err != nil {
		return 0, fmt.Errorf("error al consultar gists: %w", err)
	}
	type fix struct{ id, gist string }
	var pendientes []fix
	for rows.Next() {
		var id, g, c string
		if err := rows.Scan(&id, &g, &c); err != nil {
			rows.Close()
			return 0, err
		}
		if staleGist(g, c) {
			pendientes = append(pendientes, fix{id: id, gist: Gist(c, defaultGistMaxTokens)})
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, p := range pendientes {
		if _, err := e.db.Exec(`UPDATE observations SET gist=? WHERE id=?`, p.gist, p.id); err != nil {
			return 0, fmt.Errorf("error al recalcular el gist de %q: %w", p.id, err)
		}
	}
	return len(pendientes), nil
}

// checkStaleConflicts detecta relaciones de conflicto `pending` que las GUARDAS ACTUALES nunca
// habrían creado — residuo histórico de antes de que existieran (complementaryPair / DetectOnly).
// Dos clases, ambas ruido puro que erosiona la credibilidad de la cola de musubi_judge:
//   - TARGET HISTÓRICO: el destino es un commit o un artefacto SDD (libro mayor: se cita, no se
//     tacha). complementaryPair hoy saltea el par; las viejas quedaron encoladas para siempre.
//   - RECÍPROCO DUPLICADO: existen A→B y B→A (la contradicción es simétrica); sobra una dirección.
// Ninguna clase toca observaciones ni relaciones ya RESUELTAS: sólo poda pendings que no aportan un
// veredicto posible. Reversible en el sentido de que se regenerarían si el par fuera real (no lo es).
func checkStaleConflicts(e *DbEngine) CheckResult {
	n, err := countStaleConflicts(e)
	if err != nil {
		return CheckResult{Code: "stale_conflicts", Status: "error", Message: err.Error(), Repairable: true}
	}
	if n > 0 {
		return CheckResult{Code: "stale_conflicts", Status: "warning", Repairable: true,
			Message: fmt.Sprintf("%d relación(es) de conflicto 'pending' son ruido que las guardas actuales ya no crean (target histórico o recíproco duplicado); se pueden podar sin perder ningún veredicto real", n)}
	}
	return CheckResult{Code: "stale_conflicts", Status: "ok", Message: "la cola de conflictos no tiene ruido estructural"}
}

// staleConflictIDs devuelve los ids de las relaciones 'pending' a podar: (1) las de target histórico
// (reusa historicalRecord, la MISMA regla que complementaryPair, para no divergir) y (2) el lado
// no-canónico (source_id > target_id) de cada par recíproco pending. Determinista, sin borrar nada.
func staleConflictIDs(e *DbEngine) ([]string, error) {
	dead := map[string]bool{}

	// (1) Target histórico: JOIN a observations por target_id y filtro con historicalRecord en Go
	// (misma función que la detección) — no una aproximación en SQL que pueda divergir de la guarda.
	rows, err := e.db.Query(`
		SELECT r.id, COALESCE(o.topic_key,'')
		FROM observation_relations r JOIN observations o ON o.id = r.target_id
		WHERE r.status = ?`, RelStatusPending)
	if err != nil {
		return nil, fmt.Errorf("error al listar conflictos con target histórico: %w", err)
	}
	for rows.Next() {
		var id, topicKey string
		if err := rows.Scan(&id, &topicKey); err != nil {
			rows.Close()
			return nil, err
		}
		if historicalRecord(topicKey) {
			dead[id] = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// (2) Recíproco duplicado: si existen A→B y B→A ambas pending, se poda el lado no-canónico
	// (source_id > target_id) y se conserva source_id < target_id. La contradicción es simétrica.
	rows2, err := e.db.Query(`
		SELECT a.id FROM observation_relations a
		JOIN observation_relations b ON b.source_id = a.target_id AND b.target_id = a.source_id
		WHERE a.status = ? AND b.status = ? AND a.source_id > a.target_id`, RelStatusPending, RelStatusPending)
	if err != nil {
		return nil, fmt.Errorf("error al listar conflictos recíprocos: %w", err)
	}
	for rows2.Next() {
		var id string
		if err := rows2.Scan(&id); err != nil {
			rows2.Close()
			return nil, err
		}
		dead[id] = true
	}
	if err := rows2.Err(); err != nil {
		rows2.Close()
		return nil, err
	}
	rows2.Close()

	ids := make([]string, 0, len(dead))
	for id := range dead {
		ids = append(ids, id)
	}
	return ids, nil
}

func countStaleConflicts(e *DbEngine) (int, error) {
	ids, err := staleConflictIDs(e)
	return len(ids), err
}

func applyDeleteStaleConflicts(e *DbEngine) (int, error) {
	ids, err := staleConflictIDs(e)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, id := range ids {
		res, derr := e.db.Exec(`DELETE FROM observation_relations WHERE id = ?`, id)
		if derr != nil {
			return deleted, fmt.Errorf("error al podar la relación %q: %w", id, derr)
		}
		n, _ := res.RowsAffected()
		deleted += int(n)
	}
	return deleted, nil
}

func checkOrphans(e *DbEngine) CheckResult {
	n, err := countOrphans(e)
	if err != nil {
		return CheckResult{Code: "orphan_relations", Status: "error", Message: err.Error(), Repairable: true}
	}
	if n > 0 {
		return CheckResult{Code: "orphan_relations", Status: "warning",
			Message: fmt.Sprintf("%d relación(es) apuntan a observaciones inexistentes", n), Repairable: true}
	}
	return CheckResult{Code: "orphan_relations", Status: "ok", Message: "no hay relaciones huérfanas"}
}

const orphanWhere = `WHERE source_id NOT IN (SELECT id FROM observations)
                        OR target_id NOT IN (SELECT id FROM observations)`

func countOrphans(e *DbEngine) (int, error) {
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observation_relations ` + orphanWhere).Scan(&n); err != nil {
		return 0, fmt.Errorf("error al contar relaciones huérfanas: %w", err)
	}
	return n, nil
}

func applyDeleteOrphans(e *DbEngine) (int, error) {
	res, err := e.db.Exec(`DELETE FROM observation_relations ` + orphanWhere)
	if err != nil {
		return 0, fmt.Errorf("error al borrar relaciones huérfanas: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error al contar filas borradas: %w", err)
	}
	return int(n), nil
}
