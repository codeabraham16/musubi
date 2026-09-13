package mcp

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// guardarSalidaYLeerla hace el viaje REAL de una salida de comando: la guarda por el único escritor
// (`GuardarResultado`) y la lee por el único camino por el que vuelve al llamador (`ComandoPorID`).
// Probar el redactor suelto no alcanza: lo que se afirma es que la CADENA no deja pasar.
func guardarSalidaYLeerla(t *testing.T, stdout, stderr, errCanal string) fleet.Comando {
	t.Helper()
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConExec(t, s, "casa", "pc-gio")
	d, _, _ := s.engine.DevicePorToken(tok)
	c, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: "casa", Argv: []string{"cat", "/etc/musubi/agente.env"}, Timeout: time.Second})
	if err != nil {
		t.Fatalf("no pude encolar: %v — no medí nada", err)
	}
	cero := 0
	if err := s.engine.GuardarResultado(d.ID, c.ID, &cero, stdout, stderr, errCanal, time.Now()); err != nil {
		t.Fatalf("GuardarResultado falló: %v — no medí nada", err)
	}
	got, existe, err := s.engine.ComandoPorID(c.ID)
	if err != nil || !existe {
		t.Fatalf("no pude releer el comando (existe=%v, err=%v) — no medí nada", existe, err)
	}
	return got
}

// TestLaSalidaDeFleetExecNoDevuelveUnaCredencialEnClaro exige que la bitácora de ejecución remota
// no guarde ni devuelva un secreto.
//
// ────────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ EXISTE
//
// Hasta el 2026-09-13 la salida de `fleet_exec` no pasaba por el redactor en ningún punto: cero
// menciones en toda la cadena, medido. Se guardaba cruda y se devolvía cruda al agente. Y no era
// teórico: cuatro tokens de dispositivo quedaron en claro en un transcripto de sesión.
//
// POR QUÉ PRUEBA LA CADENA Y NO EL REDACTOR. El redactor ya tiene sus pruebas. Lo que faltaba no
// era un redactor mejor sino que alguien lo LLAMARA en este borde, así que la aserción va sobre lo
// que vuelve de la base después de guardar — el mismo camino que usan la tool, la bitácora y la
// cronología.
//
// POR QUÉ EL TOKEN SE PIDE AL GENERADOR. Un literal acá sería una copia de la forma de hoy; si
// `fleet.NuevoToken` cambia, esta guarda seguiría midiendo la vieja.
//
// Sabotaje que la hace fallar: en `GuardarResultado`, borrar la redacción de stdout → la
// credencial vuelve a guardarse y devolverse en claro.
// arnes: archivo="internal/memory/comandos.go"
// arnes: de="\tstdout, _ = redact.Redact(stdout)\n"
// arnes: a=""
// arnes: prueba="TestLaSalidaDeFleetExecNoDevuelveUnaCredencialEnClaro"
// arnes: colision_ok="TestUnTokenCortadoEnElBordeDelTruncadoNoSaleEnClaro"
func TestLaSalidaDeFleetExecNoDevuelveUnaCredencialEnClaro(t *testing.T) {
	tok, err := fleet.NuevoToken()
	if err != nil {
		t.Fatalf("NuevoToken falló: %v — no medí nada", err)
	}

	t.Run("por ningún canal: stdout, stderr ni el error del canal", func(t *testing.T) {
		// LAS FORMAS DE VERDAD: el `cat` del archivo de entorno del agente, y la respuesta de enroll.
		got := guardarSalidaYLeerla(t,
			"MUSUBI_URL=https://cerebro\nMUSUBI_DEVICE_TOKEN="+tok+"\n",
			`{"name":"nas","tier":"B","token":"`+tok+`"}`,
			"ssh: la credencial "+tok+" fue rechazada")
		for canal, texto := range map[string]string{"stdout": got.Stdout, "stderr": got.Stderr, "error": got.Error} {
			if strings.Contains(texto, tok) {
				t.Errorf("la credencial sale EN CLARO por %s de fleet_exec:\n  %s\n"+
					"  Esto ya pasó de verdad. La salida se redacta en GuardarResultado, que es el único\n"+
					"  escritor de la bitácora: si alguien sacó esa llamada, todo lector la devuelve cruda.",
					canal, texto)
			}
		}
	})

	t.Run("CONTROL: un SHA de git sigue intacto", func(t *testing.T) {
		// La mitad que impide «arreglar» esto tapando todo: una bitácora donde los commits salen como
		// [REDACTED] es inservible, y el que la lea va a apagar la redacción entera.
		crudo := make([]byte, 32)
		for i := range crudo {
			crudo[i] = byte(i * 11)
		}
		sha := hex.EncodeToString(crudo)
		got := guardarSalidaYLeerla(t, "HEAD is now at "+sha+"\n", "", "")
		if !strings.Contains(got.Stdout, sha) {
			t.Errorf("la redacción tapó un SHA-256 de git: %q", got.Stdout)
		}
	})

	t.Run("CONTROL: una salida común vuelve TEXTUAL", func(t *testing.T) {
		// Sin esta rama, una redacción que devuelva vacío —o que trunque todo— pasaría la de arriba.
		comun := "active\n● musubi-agente.service - Musubi agent\n   Loaded: loaded\n"
		got := guardarSalidaYLeerla(t, comun, "", "")
		if got.Stdout != comun {
			t.Errorf("una salida sin secretos no volvió textual:\n  guardé %q\n  volvió %q", comun, got.Stdout)
		}
	})
}

// TestUnTokenCortadoEnElBordeDelTruncadoNoSaleEnClaro fija que se redacte ANTES de truncar.
//
// `TruncarSalida` corta por bytes en `SalidaMaxBytes`. Si se trunca primero, un token que cae justo
// en el borde queda partido y lo que sobrevive —diez caracteres acá— es demasiado corto para el
// catch-all de entropía, que no ve NADA de 22 caracteres o menos (internal/redact). Esos caracteres
// del secreto salen en claro. Redactando primero, el token entero se reemplaza y lo que se corta es
// la marca.
//
// EL RELLENO SE ELIGIÓ DOS VECES, Y LAS DOS RAZONES SE MIDIERON:
//
//   - VA SEPARADO DEL TOKEN POR UN SALTO DE LÍNEA: el catch-all de entropía toma tiras de
//     [A-Za-z0-9+/_-], así que un relleno pegado al token formaría UNA sola tira de entropía casi
//     cero y el token se colaría por eso, no por el orden.
//   - ES DE «z» Y NO DE «A», y la primera versión lo aprendió cayendo en su propio control: «A» es un
//     dígito hex, el catch-all de hex (`[0-9a-fA-F]{32,}`) NO mira entropía, y 64 KiB de «A» se
//     redactaron enteros como `hex-secret`. La salida guardada quedó en 46 bytes, nunca llegó al
//     borde, y la prueba no midió nada. «z» no es hex y tiene entropía cero: no lo toca nadie.
//
// Sabotaje que la hace fallar: truncar stdout ANTES de redactarlo.
// arnes: archivo="internal/memory/comandos.go"
// arnes: de="\tstdout, _ = redact.Redact(stdout)\n\tstderr, _ = redact.Redact(stderr)\n\terrCanal, _ = redact.Redact(errCanal)\n\tso, _ := fleet.TruncarSalida(stdout)\n"
// arnes: a="\tstdout, _ = fleet.TruncarSalida(stdout)\n\tstdout, _ = redact.Redact(stdout)\n\tstderr, _ = redact.Redact(stderr)\n\terrCanal, _ = redact.Redact(errCanal)\n\tso := stdout\n"
// arnes: prueba="TestUnTokenCortadoEnElBordeDelTruncadoNoSaleEnClaro"
//
// SIN `colision_ok` A PROPÓSITO, y la primera versión lo tenía: la herramienta la denunció como
// respuesta RANCIA. Este sabotaje reordena pero CONSERVA la línea `stdout, _ = redact.Redact(stdout)`,
// así que no rompe el `de` de TestLaSalidaDeFleetExecNoDevuelveUnaCredencialEnClaro: en esta
// dirección no hay colisión que contestar. La colisión existe sólo en la otra —aquel sabotaje sí
// borra esa línea, que es parte de este `de`— y la contesta aquella directiva.
func TestUnTokenCortadoEnElBordeDelTruncadoNoSaleEnClaro(t *testing.T) {
	tok, err := fleet.NuevoToken()
	if err != nil {
		t.Fatalf("NuevoToken falló: %v — no medí nada", err)
	}
	const sobreviven = 10
	relleno := strings.Repeat("z", fleet.SalidaMaxBytes-sobreviven-1) + "\n"
	if len(relleno)+sobreviven != fleet.SalidaMaxBytes {
		t.Fatalf("el relleno quedó mal medido: el token no empieza a %d bytes del borde — no medí nada", sobreviven)
	}
	got := guardarSalidaYLeerla(t, relleno+tok+"\n", "", "")

	// CONTROL DE QUE HUBO TRUNCADO: sin marca, el borde nunca se tocó y esta prueba no mide el orden.
	if !strings.Contains(got.Stdout, "truncada") {
		t.Fatalf("la salida no se truncó (%d bytes): el token no cayó en el borde y no medí nada", len(got.Stdout))
	}
	if trozo := tok[:sobreviven]; strings.Contains(got.Stdout, trozo) {
		t.Errorf("sobrevivieron %d caracteres del token en el borde del truncado (%q).\n"+
			"  Se truncó ANTES de redactar: el trozo es demasiado corto para el catch-all de entropía,\n"+
			"  que no ve nada de 22 caracteres o menos. Redactá primero y truncá después.",
			sobreviven, trozo)
	}
}
