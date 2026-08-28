package mcp

// exposicion_config_test.go custodia la declaración de los destinos de exposición.
//
// El archivo es de tres campos y todos sus modos de fallo terminan en el mismo lugar: una máquina
// que deja de medirse con un mensaje que manda a mirar a otro lado.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// LA CREDENCIAL NO ENTRA EN ESTE ARCHIVO, NI SIQUIERA METIDA EN LA URL.
//
// `https://usuario:clave@host/metrics` funciona perfecto y convierte este archivo —que se
// versiona, se copia a un ticket y se pega en un chat— en un almacén de secretos por la puerta de
// atrás. Aceptarlo en silencio sería peor que no tener `auth_env`: daría la sensación de que el
// diseño protege algo mientras la puerta de al lado está abierta. Y un secreto que ya entró a un
// archivo versionado no se puede des-filtrar.
//
// Sabotaje que la hace fallar: sacar la guarda de `u.User`.
func TestLaCredencialNoPuedeEntrarPorLaURL(t *testing.T) {
	_, err := resolverExposicion("base", entradaExposicion{
		URL: "https://alguien:la-clave@metrics.example.com/m",
	}, func(string) string { return "" })
	if err == nil {
		t.Fatal("una URL con usuario y clave adentro se aceptó: el archivo pasó a guardar secretos")
	}
	if !strings.Contains(err.Error(), "auth_env") {
		t.Errorf("el error no dice dónde va la credencial: %v", err)
	}
	// Y la URL sin credencial adentro sí pasa: si esto fallara, la de arriba pasaría por rechazar
	// todo en vez de por rechazar lo que dice.
	d, err := resolverExposicion("base", entradaExposicion{
		URL: "https://metrics.example.com/m", AuthEnv: "TOKEN_DE_PRUEBA", Montaje: "/data",
	}, func(k string) string {
		if k == "TOKEN_DE_PRUEBA" {
			return "Bearer abc"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("una declaración buena se rechazó: %v", err)
	}
	if d.Autorizacion != "Bearer abc" || d.Montaje != "/data" {
		t.Errorf("la declaración se resolvió mal: %+v", d)
	}
}

// UNA VARIABLE DECLARADA Y AUSENTE ES UN ERROR, NO UN ENDPOINT SIN CREDENCIAL.
//
// Seguir sin el header manda un pedido sin autorizar, el otro lado contesta 401, y ese 401 manda
// a alguien a revisar el token —que está perfecto— en vez del entorno del cerebro, que es donde
// está el problema. Es la misma regla que separa «podman no está instalado» de «podman está y
// falló»: dos causas distintas no pueden tener el mismo síntoma.
//
// Sabotaje que la hace fallar: seguir con Autorizacion vacía cuando la variable no está.
func TestUnaVariableDeclaradaYAusenteNoEsUnEndpointSinCredencial(t *testing.T) {
	_, err := resolverExposicion("base", entradaExposicion{
		URL: "https://metrics.example.com/m", AuthEnv: "NO_EXISTE_EN_NINGUN_LADO",
	}, func(string) string { return "" })
	if err == nil {
		t.Fatal("una auth_env ausente se tomó como «endpoint sin credencial»")
	}
	if !strings.Contains(err.Error(), "NO_EXISTE_EN_NINGUN_LADO") {
		t.Errorf("el error no nombra la variable que falta: %v", err)
	}

	// Sin `auth_env` declarada, en cambio, no hay error: un endpoint sin credencial es legítimo
	// en una red cerrada, y ésa es la diferencia entre «no pediste» y «pediste y no está».
	d, err := resolverExposicion("base", entradaExposicion{URL: "http://10.0.0.5:9100/metrics"},
		func(string) string { return "" })
	if err != nil {
		t.Fatalf("un endpoint sin credencial se rechazó: %v", err)
	}
	if d.Autorizacion != "" {
		t.Errorf("se inventó una credencial: %q", d.Autorizacion)
	}
}

// UN ARCHIVO AUSENTE NO ES UN ARCHIVO ROTO, Y UN YAML ROTO NO ES UNA AUSENCIA.
//
// Son los dos extremos del mismo error. Que el archivo no exista es lo normal en cualquier
// despliegue sin máquinas de este tipo. Que exista y no parsee tiene que DOLER: tratar una coma
// de más como «no hay nada declarado» degrada el sondeo a SSH sin decir nada, y el síntoma sería
// «esta máquina dejó de medirse» con el archivo a la vista, correcto salvo por una línea.
//
// Sabotaje que la hace fallar: devolver (vacío, false, nil) también cuando el YAML no parsea.
func TestUnYamlRotoNoSeConfundeConUnArchivoAusente(t *testing.T) {
	s := &McpServer{projectPath: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(s.projectPath, ".musubi"), 0o755); err != nil {
		t.Fatal(err)
	}

	// 1) Ausente: no hay destino y no hay error.
	if _, hay, err := s.destinoDeExposicion("base"); hay || err != nil {
		t.Fatalf("un archivo ausente dio hay=%v err=%v", hay, err)
	}

	// 2) Roto: error, y el error nombra el archivo.
	if err := os.WriteFile(s.rutaExposicion(), []byte("dispositivos:\n  base: {url: \"a\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, hay, err := s.destinoDeExposicion("base")
	if err == nil {
		t.Fatal("un YAML roto se leyó como «no hay nada declarado»: el sondeo caería a SSH en silencio")
	}
	if hay {
		t.Error("un YAML roto devolvió un destino")
	}
	if !strings.Contains(err.Error(), "flota-exposicion.yaml") {
		t.Errorf("el error no dice qué archivo mirar: %v", err)
	}

	// 3) Bueno pero sin esta máquina: no hay destino y no hay error. Ese dispositivo va por SSH.
	if err := os.WriteFile(s.rutaExposicion(),
		[]byte("dispositivos:\n  otra:\n    url: https://x.example.com/m\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, hay, err := s.destinoDeExposicion("base"); hay || err != nil {
		t.Fatalf("una máquina no declarada dio hay=%v err=%v", hay, err)
	}
}

// UN ARCHIVO DE CONFIGURACIÓN ROTO NO PUEDE TUMBAR A LAS MÁQUINAS QUE NO ESTÁN EN ÉL.
//
// Es el borde de la guarda de arriba, y la primera versión lo tenía mal: el error de
// configuración era el RESULTADO del sondeo, así que una coma de más en `flota-exposicion.yaml`
// convertía un archivo que habla de UNA base en «ninguna máquina de la flota se pudo medir»,
// routers por SSH incluidos.
//
// Un YAML que no parsea no permite saber quién estaba declarado adentro. La única salida honesta
// es sondear a todos por su transporte por defecto y llevar el aviso PEGADO a la fila: la máquina
// que sí necesitaba la exposición va a fallar con su propio mensaje, y la explicación está al
// lado, no en un log.
//
// Sabotaje que la hace fallar: volver a `fila["error"] = errCfg.Error(); return fila`.
func TestUnaConfiguracionRotaNoTumbaALasMaquinasQueNoEstanEnElla(t *testing.T) {
	s := &McpServer{projectPath: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(s.projectPath, ".musubi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.rutaExposicion(), []byte("dispositivos:\n  base: {url: \"a\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Un router de Tier B con dirección: se sondea por SSH igual. Va a fallar por no llegar —no
	// hay ssh contra `no-existe.invalid`— y eso está bien; lo que NO puede pasar es que el
	// motivo sea el archivo de configuración de otra máquina.
	fila := s.sondearUno(fleet.Device{
		Name: "router", Tier: fleet.TierProtocolo, Address: "no-existe.invalid", OS: "linux",
	}, time.Now())

	if fila["transporte"] != string(fleet.TransporteSSH) {
		t.Errorf("el router no se sondeó por SSH: transporte=%v", fila["transporte"])
	}
	if aviso, hay := fila["aviso_configuracion"]; !hay {
		t.Error("el aviso de la configuración rota no viaja en la fila: quedaría sólo en un log")
	} else if !strings.Contains(aviso.(string), "flota-exposicion.yaml") {
		t.Errorf("el aviso no dice qué archivo mirar: %v", aviso)
	}
	if err, hay := fila["error"]; hay && strings.Contains(err.(string), "YAML") {
		t.Errorf("el archivo roto se llevó puesto el sondeo del router: %v", err)
	}
}
