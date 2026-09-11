package mcp

import (
	"strings"
	"testing"
)

// LA MARCA DEL SNAPSHOT SE ESCRIBE DESPUÉS DE TODO LO QUE PUEDE DEJARLO INCOMPLETO.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// `musubi_backup_local_age_seconds` sale de `.last_snapshot`, y esa marca se escribía APENAS el
// snapshot salía bien — ANTES de la mitad que decide si un restore devuelve una FLOTA o un
// LADRILLO.
//
// La identidad de cada máquina se resuelve SÓLO por su token: no hay ningún campo en el latido
// que diga quién es. Así que un cerebro restaurado sin `principals.yaml` le contesta 401 a todos
// los agentes, y un 401 los DETIENE SIN REINTENTAR. Se pierde la flota entera y hay que ir
// máquina por máquina a levantarla a mano.
//
// El guion ya sabía eso —hace `die` si no puede copiar el archivo— pero la marca ya estaba
// fresca. Prometheus veía la edad sana y un backup a medias se leía exactamente igual que uno
// completo: el cero que miente, con otra forma.
//
// La copia OFF-HOST no entra en esta cuenta: tiene su propia marca, responde otra pregunta —el
// DR— y en modo local-only se saltea a propósito.
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestLaMarcaDelSnapshotVaDespuesDeCopiarLosPrincipals(t *testing.T) {
	guion := leerDeploy(t, "musubi-backup.sh")

	marca := strings.Index(guion, `> "$BACKUP_LOCAL_DIR/.last_snapshot"`)
	if marca < 0 {
		t.Fatal("el guion de backup ya no escribe `.last_snapshot`: sin esa marca, el único trabajo " +
			"programado del servidor no tiene ninguna señal — ni al fallar ni al dejar de dispararse")
	}
	// LA COPIA ES LA QUE PUEDE FALLAR Y DEJAR EL BACKUP INCOMPLETO, así que la posición se mide
	// contra ella y no contra el bloque entero de principals.
	copia := strings.Index(guion, `cp "$PRINCIPALS_FILE"`)
	if copia < 0 {
		t.Fatal("el guion ya no copia `principals.yaml` con el snapshot: un restore sin ese archivo " +
			"le contesta 401 a toda la flota, y un 401 detiene al agente SIN REINTENTAR")
	}
	if marca < copia {
		t.Error("`.last_snapshot` se escribe ANTES de copiar `principals.yaml`.\n" +
			"  Si esa copia falla, el guion hace `die` con la marca YA FRESCA: Prometheus ve la edad " +
			"sana y un backup que dejó afuera las identidades de la flota se lee exactamente igual " +
			"que uno completo.\n" +
			"  La marca va después de todo lo que puede dejar el backup incompleto.")
	}
}

// EL -1 DEL OFF-HOST SE PUEDE LEER, PORQUE EL MODO SE DECLARA.
//
// `musubi_backup_offhost_age_seconds` vale -1 en dos situaciones OPUESTAS: un local-only
// DECLARADO —una decisión, con su costo escrito— y un off-host que falla todas las noches. El
// mismo número para una decisión y para un incidente, y la única forma de saber cuál era ir a
// preguntarle a `musubi doctor` a mano.
func TestElGuionDeBackupDeclaraSuModoDeDR(t *testing.T) {
	guion := leerDeploy(t, "musubi-backup.sh")

	// LAS DOS RAMAS. Declarar sólo una deja el modo diciendo lo que pasó la última vez que se
	// escribió, que es peor que no decir nada: un servidor que pasó de remoto a local-only
	// seguiría declarándose remoto para siempre.
	for _, modo := range []string{"local-only", "remoto"} {
		if !strings.Contains(guion, "echo "+modo+` > "$BACKUP_LOCAL_DIR/.offhost_modo"`) {
			t.Errorf("el guion no declara el modo %q en `.offhost_modo`.\n"+
				"  Sin las DOS ramas, el modo se queda con lo que pasó la última vez y un servidor "+
				"que cambió de configuración se declara mal para siempre.", modo)
		}
	}
}

// Y LA ALERTA LO USA. Sin esto, la serie existiría y no la leería nadie — que es el defecto que
// este plan ya cerró tres veces.
func TestLaAlertaDeBackupOffhostDistingueLaDecisionDelIncidente(t *testing.T) {
	reglas := strings.Join(strings.Fields(leerDeploy(t, "musubi-alerts-backup-offhost.yml")), " ")

	if !strings.Contains(reglas, "musubi_backup_offhost_configurado == 0") {
		t.Error("`MusubiBackupOffhostStale` no se condiciona al modo declarado: con un local-only " +
			"declarado, el -1 la hace disparar TODOS LOS DÍAS sin que haya nada que arreglar — y una " +
			"alarma que no se apaga arreglando algo enseña a ignorar el canal, que es cómo se " +
			"pierden las demás.")
	}
	// Y LA OTRA DIRECCIÓN: el `< 0` tiene que seguir estando. Sin él, un off-host que NUNCA
	// funcionó deja de avisarse — el DR apagado se vería igual que el DR sano.
	if !strings.Contains(reglas, "musubi_backup_offhost_age_seconds < 0") {
		t.Error("la alerta perdió el `< 0`: un off-host que nunca funcionó deja de avisarse, y el DR " +
			"apagado se ve igual que el DR sano")
	}
}
