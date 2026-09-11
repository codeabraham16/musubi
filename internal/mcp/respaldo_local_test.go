package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// EL ESTADO QUE NO SINCRONIZA TIENE QUIEN LO RESPALDE, Y QUIEN LO VIGILE.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LO QUE VIAJA Y LO QUE NO. La memoria de equipo se sincroniza al central y ahí hay snapshot
// diario con copia off-host. El grafo de código, las entidades, los proyectos LOCALES y las
// observaciones con scope `local` NO VIAJAN: viven sólo en el disco de esta máquina. Si se pierde
// el disco, se pierden — no hay una segunda copia en ningún lado.
//
// Y ESTA MÁQUINA NO LA SCRAPEA PROMETHEUS. Todo el plano de monitoreo de este track vigila al
// CEREBRO; una instalación local no exporta a ningún lado. Así que `musubi doctor` es literalmente
// lo único que puede decir que su respaldo dejó de correr — y no tenía ningún chequeo que lo
// mirara: su hermano `offhost_backup` existe desde hace tiempo y éste no.
// ────────────────────────────────────────────────────────────────────────────────────────────

func TestExisteLaUnidadDeRespaldoDelEstadoLocal(t *testing.T) {
	for _, n := range []string{"musubi-respaldo-local.service", "musubi-respaldo-local.timer"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "deploy", "systemd", n))
		if err != nil {
			t.Fatalf("falta %s: el estado local no sincroniza a ningún lado, así que sin esta unidad "+
				"no tiene NINGUNA copia — ni acá ni afuera: %v", n, err)
		}
		texto := string(b)

		// `Persistent=true` ES LA LÍNEA QUE HACE QUE ESTO SIRVA EN UNA ESTACIÓN DE TRABAJO. Sin
		// ella, un disparo que cae con la máquina apagada simplemente NO OCURRE, y en un equipo
		// que se apaga de noche eso significa no tener NUNCA un snapshot, sin que nada lo diga.
		if strings.HasSuffix(n, ".timer") && !strings.Contains(texto, "Persistent=true") {
			t.Errorf("%s no lleva `Persistent=true`: en una máquina que se apaga de noche el disparo "+
				"perdido no se recupera, y no hay snapshot nunca", n)
		}
		// EL MODO LOCAL-ONLY SE DECLARA. Sin `BACKUP_ALLOW_LOCAL_ONLY=1` el guion falla-cerrado a
		// propósito —porque sin destino remoto no es DR— y la unidad quedaría en `failed` cada
		// noche. Con la variable, la decisión queda escrita y el aviso del doctor la entiende.
		if strings.HasSuffix(n, ".service") && !strings.Contains(texto, "BACKUP_ALLOW_LOCAL_ONLY=1") {
			t.Errorf("%s no declara `BACKUP_ALLOW_LOCAL_ONLY=1`: el guion falla-cerrado sin destino "+
				"remoto, así que la unidad quedaría en `failed` todas las noches", n)
		}
		// Y REUSA EL GUION DEL CEREBRO. Un segundo guion sería una copia de «cómo se toma un
		// snapshot», y la copia que se queda vieja es siempre la del camino que se mira menos —
		// que acá sería justamente ésta.
		if strings.HasSuffix(n, ".service") && !strings.Contains(texto, "deploy/musubi-backup.sh") {
			t.Errorf("%s no usa `deploy/musubi-backup.sh`: dos guiones que toman snapshots divergen, "+
				"y el que se queda viejo es el del camino que menos se mira", n)
		}
	}
}

// Y EL DOCTOR LO MIRA. Sin chequeo, la unidad puede dejar de dispararse y no lo dice nadie: un
// timer que NO DISPARA no falla, así que ni `systemctl status` ni un `OnFailure` lo ven.
func TestElDoctorChequeaElSnapshotLocal(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "internal", "memory", "doctor.go"))
	if err != nil {
		t.Fatalf("no pude leer doctor.go: %v", err)
	}
	// Se mira la LÍNEA DE LA TABLA y no el nombre suelto: el nombre aparece también en el
	// comentario que explica el chequeo, y un chequeo escrito pero no registrado en `doctorChecks`
	// no corre nunca — que es exactamente la clase de defecto que este plan persigue.
	if !strings.Contains(string(b), `{code: "snapshot_local", run: checkSnapshotLocal}`) {
		t.Error("`snapshot_local` no está en la tabla `doctorChecks`.\n" +
			"  Un chequeo escrito y no registrado no corre NUNCA, y esta máquina no la scrapea " +
			"Prometheus: el doctor es lo único que puede decir que el respaldo del estado local dejó " +
			"de correr.\n" +
			"  Y un timer que NO DISPARA no falla — ni `systemctl status` ni un `OnFailure` lo ven.")
	}
}
