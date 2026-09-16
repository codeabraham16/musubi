package mcp

import (
	"strings"
	"testing"
)

// UNA SERIE QUE NO LEE NINGUNA REGLA ES UN DATO QUE NO MIRA NADIE.
//
// `musubi_fleet_device_token_rotable` existió así y quedó registrado como defecto: el exportador
// la emitía, el panel no la dibujaba y ninguna alerta la consultaba. Emitir un número no es
// vigilar nada, y la serie nueva de A118 nace con el mismo riesgo — es la mitad silenciosa de una
// pregunta que hasta hoy contestaba sólo su hermana.
//
// SE PIDE `== 1` Y NO CUALQUIER LECTURA. La serie está AUSENTE cuando no se puede comparar (un
// Tier B, o un cerebro sin versión propia), así que un `!= 0` se leería como «todo lo que no es
// cero», que sobre una serie ausente no significa nada.
//
// Y SE EXIGE UN PLAZO LARGO, que es lo que la vuelve utilizable: el binario del cerebro se
// redespliega varias veces por día, así que un 1 recién aparecido es lo NORMAL. Leída al instante,
// esta alerta sería ruido puro y alguien la apagaría — que es peor que no tenerla.
//
// Sabotaje: sacar la alerta AgenteConBuildViejo del archivo de flota, o bajarle el `for`.
//
// arnes: archivo="deploy/musubi-alerts-flota.yml"
// arnes: de="        for: 7d\n        labels: { severity: info }"
// arnes: a="        for: 5m\n        labels: { severity: info }"
func TestLaSerieDelBuildTieneQuienLaLea(t *testing.T) {
	reglas := strings.Join(strings.Fields(leerDeploy(t, "musubi-alerts-flota.yml")), " ")

	if !strings.Contains(reglas, "musubi_fleet_device_agent_build_stale == 1") {
		t.Error("ninguna alerta lee `musubi_fleet_device_agent_build_stale == 1`.\n" +
			"La serie contesta la mitad de A118 que su hermana no puede ver —una máquina que se quedó\n" +
			"commits atrás dentro del mismo release— y sin lector es un número que no mira nadie,\n" +
			"exactamente como pasó con `token_rotable`.")
	}
	if strings.Contains(reglas, "musubi_fleet_device_agent_build_stale != 0") ||
		strings.Contains(reglas, "musubi_fleet_device_agent_build_stale > 0") {
		t.Error("la alerta lee la serie del build con `!= 0` o `> 0`: esa forma se lee como «todo lo " +
			"que no es cero», y esta serie está AUSENTE cuando no se puede comparar.")
	}
	// El plazo. Sin esto, la guarda la satisface una alerta que dispara en cada despliegue.
	i := strings.Index(reglas, "alert: AgenteConBuildViejo")
	if i < 0 {
		t.Fatal("no existe la alerta AgenteConBuildViejo")
	}
	tramo := reglas[i:]
	if j := strings.Index(tramo, "for:"); j < 0 || !strings.HasPrefix(strings.TrimSpace(tramo[j+4:]), "7d") {
		t.Error("AgenteConBuildViejo no espera 7d: con un plazo corto dispara después de cada " +
			"redespliegue del cerebro, que es el caso NORMAL, y una alerta así se apaga sola de ruidosa")
	}
}
