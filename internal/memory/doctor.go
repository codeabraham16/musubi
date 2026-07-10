package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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
		{code: "fts_consistency", run: checkFTS, count: countFTSDrift, apply: applyRebuildFTS},
		{code: "missing_digests", run: checkDigests, count: countMissingDigests, apply: applyBackfillDigests},
		{code: "orphan_relations", run: checkOrphans, count: countOrphans, apply: applyDeleteOrphans},
		{code: "offhost_backup", run: checkOffhostBackup},
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
	q := fmt.Sprintf(`VACUUM INTO '%s'`, strings.ReplaceAll(dest, "'", "''"))
	if _, err := e.db.Exec(q); err != nil {
		return "", fmt.Errorf("VACUUM INTO falló: %w", err)
	}
	return dest, nil
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

func checkFTS(e *DbEngine) CheckResult {
	drift, err := countFTSDrift(e)
	if err != nil {
		return CheckResult{Code: "fts_consistency", Status: "error", Message: err.Error(), Repairable: true}
	}
	if drift != 0 {
		return CheckResult{Code: "fts_consistency", Status: "warning",
			Message: fmt.Sprintf("el índice FTS está desincronizado (diferencia de %d fila(s))", drift), Repairable: true}
	}
	return CheckResult{Code: "fts_consistency", Status: "ok", Message: "índice FTS sincronizado"}
}

func countFTSDrift(e *DbEngine) (int, error) {
	var obs, fts int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&obs); err != nil {
		return 0, fmt.Errorf("error al contar observations: %w", err)
	}
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observations_fts`).Scan(&fts); err != nil {
		return 0, fmt.Errorf("error al contar observations_fts: %w", err)
	}
	d := fts - obs
	if d < 0 {
		d = -d
	}
	return d, nil
}

func applyRebuildFTS(e *DbEngine) (int, error) {
	tx, err := e.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM observations_fts`); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`INSERT INTO observations_fts(id, topic_key, content) SELECT id, topic_key, content FROM observations`); err != nil {
		return 0, err
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
