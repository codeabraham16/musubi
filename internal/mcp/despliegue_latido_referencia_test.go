package mcp

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// LO QUE CUSTODIA: que la referencia contra la que se comparó VIAJE en el latido.
//
// `comparar-y-latir.sh` empujaba «resultado=0» sin decir contra qué árbol se había comparado, así
// que ese 0 valía lo mismo parado en `main` que parado en una rama de hace un mes. Ahora el
// verificador escribe la referencia en un archivo y el latido la lee con `source` — un solo
// productor y ningún parser que se pueda desfasar.
//
// Y ES UN CRUCE, NO DOS PRUEBAS SUELTAS. El defecto que este repo encuentra una y otra vez es el
// productor y el parser probados cada uno contra su propia idea del formato: las dos verdes, el
// cable cortado en el medio. Acá se corre el latido DE VERDAD, con un `ssh` falso que captura el
// sobre, y se mira el JSON que habría salido.

// armarBancoDelLatido deja un repo de prueba con el guion real del latido, un verificador de
// mentira que escribe lo que se le pida, y un `ssh` falso que en vez de empujar guarda el sobre.
// Devuelve la raíz y la ruta donde va a quedar el JSON capturado.
func armarBancoDelLatido(t *testing.T, cuerpoDelVerificador string) (string, string) {
	t.Helper()
	latido, err := os.ReadFile("../../deploy/comparar-y-latir.sh")
	if err != nil {
		t.Fatalf("no se pudo leer el guion real del latido: %v", err)
	}
	raiz := t.TempDir()
	if err := os.MkdirAll(filepath.Join(raiz, "deploy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(raiz, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raiz, "deploy", "comparar-y-latir.sh"), latido, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raiz, "deploy", "verificar-despliegue.sh"),
		[]byte(cuerpoDelVerificador), 0o755); err != nil {
		t.Fatal(err)
	}
	captura := filepath.Join(raiz, "sobre.json")
	// El `ssh` falso distingue las dos invocaciones del guion: el sondeo de red (`true`) y el
	// empuje (`curl`). Al empuje le guarda stdin, que es el sobre, y contesta 200 como Prometheus.
	ssh := "#!/usr/bin/env bash\nfor a in \"$@\"; do case \"$a\" in *curl*) cat > " +
		captura + "; echo 200; exit 0;; esac; done\nexit 0\n"
	if err := os.WriteFile(filepath.Join(raiz, "bin", "ssh"), []byte(ssh), 0o755); err != nil {
		t.Fatal(err)
	}
	return raiz, captura
}

func correrLatido(t *testing.T, raiz string) {
	t.Helper()
	cmd := exec.Command("bash", filepath.Join(raiz, "deploy", "comparar-y-latir.sh"))
	cmd.Dir = raiz
	cmd.Env = append(os.Environ(),
		"PATH="+filepath.Join(raiz, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"MUSUBI_SSH=banco",
		"PROM_URL=http://127.0.0.1:1",
	)
	salida, err := cmd.CombinedOutput()
	// El código de salida es el del verificador de mentira, así que no se afirma sobre él: lo que
	// se mira es el sobre. Sólo se reporta si además no se capturó nada, para no diagnosticar a
	// ciegas cuando el banco se rompe.
	_ = err
	t.Logf("salida del latido:\n%s", salida)
}

func sobreCapturado(t *testing.T, captura string) map[string]any {
	t.Helper()
	crudo, err := os.ReadFile(captura)
	if err != nil {
		t.Fatalf("el latido no empujó ningún sobre: %v", err)
	}
	var sobre map[string]any
	if err := json.Unmarshal(crudo, &sobre); err != nil {
		t.Fatalf("el sobre OTLP no es JSON válido — Prometheus lo contestaría con 400 y el empuje "+
			"moriría en silencio con la configuración perfecta: %v\nSobre:\n%s", err, crudo)
	}
	return sobre
}

// nombresDeMetricas saca los nombres del sobre sin buscarlos por substring: un `strings.Contains`
// sobre el JSON crudo se satisfaría con el nombre escrito adentro de una `description`.
func nombresDeMetricas(t *testing.T, sobre map[string]any) map[string]float64 {
	t.Helper()
	out := map[string]float64{}
	rms, _ := sobre["resourceMetrics"].([]any)
	for _, rm := range rms {
		m, _ := rm.(map[string]any)
		sms, _ := m["scopeMetrics"].([]any)
		for _, sm := range sms {
			s, _ := sm.(map[string]any)
			ms, _ := s["metrics"].([]any)
			for _, mm := range ms {
				met, _ := mm.(map[string]any)
				nombre, _ := met["name"].(string)
				valor := 0.0
				if g, ok := met["gauge"].(map[string]any); ok {
					if dps, ok := g["dataPoints"].([]any); ok && len(dps) > 0 {
						if dp, ok := dps[0].(map[string]any); ok {
							if v, ok := dp["asDouble"].(float64); ok {
								valor = v
							}
						}
					}
				}
				out[nombre] = valor
			}
		}
	}
	return out
}

const serieReferencia = "musubi_verificacion_referencia_confiable"

func TestElLatidoDiceContraQueReferenciaSeComparo(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("hace falta bash")
	}

	// Un verificador que declara la referencia. `$MUSUBI_REF_SALIDA` es el contrato entre los dos
	// guiones y se escribe acá tal cual lo escribe el de verdad.
	verificadorQueDeclara := func(confiable string) string {
		return "#!/usr/bin/env bash\n" +
			"if [ -n \"${MUSUBI_REF_SALIDA:-}\" ]; then\n" +
			"  printf 'REF_CONFIABLE=" + confiable + "\\nREF_RAMA=main\\nREF_HEAD=abc1234\\n" +
			"REF_ATRAS=0\\nREF_ADELANTE=0\\nREF_SUCIO=0\\n' > \"$MUSUBI_REF_SALIDA\"\n" +
			"fi\nexit 0\n"
	}

	t.Run("comparó contra origin/main limpio: la serie sale en 1", func(t *testing.T) {
		raiz, captura := armarBancoDelLatido(t, verificadorQueDeclara("1"))
		correrLatido(t, raiz)
		metricas := nombresDeMetricas(t, sobreCapturado(t, captura))
		v, ok := metricas[serieReferencia]
		if !ok {
			t.Fatalf("el latido no llevó %s, así que su «coincide» sigue sin decir contra qué.\nLlevó: %v", serieReferencia, metricas)
		}
		if v != 1 {
			t.Errorf("se comparó contra la referencia buena y la serie dice %v", v)
		}
	})

	t.Run("comparó contra otro árbol: la serie sale en 0", func(t *testing.T) {
		raiz, captura := armarBancoDelLatido(t, verificadorQueDeclara("0"))
		correrLatido(t, raiz)
		metricas := nombresDeMetricas(t, sobreCapturado(t, captura))
		v, ok := metricas[serieReferencia]
		if !ok {
			t.Fatalf("el latido no llevó %s.\nLlevó: %v", serieReferencia, metricas)
		}
		if v != 0 {
			t.Errorf("se comparó contra un árbol que no era la referencia y la serie dice %v", v)
		}
	})

	// EL CASO QUE SEPARA «MEDÍ Y ESTÁ MAL» DE «NO SÉ», y es el que más importa: un verificador
	// viejo no escribe el archivo. Emitir 0 ahí sería indistinguible de un 0 medido, y una alerta
	// sobre esa serie se dispararía por una diferencia de versiones en vez de por un problema
	// real. La regla del export de este repo es que lo desconocido NO SE EMITE.
	t.Run("un verificador que no declara la referencia NO produce un cero inventado", func(t *testing.T) {
		raiz, captura := armarBancoDelLatido(t, "#!/usr/bin/env bash\nexit 0\n")
		correrLatido(t, raiz)
		metricas := nombresDeMetricas(t, sobreCapturado(t, captura))
		if v, ok := metricas[serieReferencia]; ok {
			t.Errorf("el verificador no dijo nada de la referencia y el latido igual mandó %s=%v: "+
				"«no sé» y «medí y está mal» pasaron a ser el mismo número", serieReferencia, v)
		}
		// Y el resto del latido tiene que seguir saliendo: degradar no es callarse.
		if _, ok := metricas["musubi_verificacion_despliegue_resultado"]; !ok {
			t.Errorf("sin la referencia el latido dejó de mandar su resultado: la degradación se comió el latido entero.\nLlevó: %v", metricas)
		}
	})
}
