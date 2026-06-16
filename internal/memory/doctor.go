package memory

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	}
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

// backupDB copia el archivo SQLite a .musubi/backups/ con timestamp.
func (e *DbEngine) backupDB() (string, error) {
	if e.path == "" {
		return "", fmt.Errorf("ruta de la base desconocida; no se puede respaldar")
	}
	dir := filepath.Join(filepath.Dir(e.path), "backups")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	ts := time.Now().UTC().Format("20060102-150405")
	dest := filepath.Join(dir, "memory.db."+ts)
	in, err := os.Open(e.path)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return "", err
	}
	return dest, out.Close()
}

// ---- checks individuales ----

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
	n, _ := res.RowsAffected()
	return int(n), nil
}
