package fleet

import (
	"encoding/hex"
	"strings"
	"testing"

	"musubi/internal/redact"
)

// TestNingunaCredencialQueAcunamosAtraviesaElRedactor exige que lo que este paquete EMITE no pueda
// salir en claro por una salida de comando.
//
// ────────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ EXISTE: LA FUGA ESTÁ CONSUMADA, NO ES HIPOTÉTICA
//
// El 2026-09-13 se midieron CUATRO tokens de dispositivo en claro dentro de un transcripto de
// sesión (un solo archivo, del 2026-09-08): dos impresos por un guion como `token: <64hex>` y dos
// en la respuesta JSON de `enroll`. Dos de los cuatro son de máquinas con `exec` y `shell`, una de
// ellas tier A.
//
// LA CAUSA NO ES EL REDACTOR: ES EL ALFABETO DE LA CREDENCIAL. `redact` tiene que dejar pasar el
// hex de 40 y 64 caracteres —son los SHA-1 y SHA-256 de git, y taparlos volvería ilegible toda
// salida que hable de commits—, y `NuevoToken` emitía exactamente 64 hex. La credencial era
// indistinguible de un digest PARA EL REDACTOR, así que el salteo legítimo la dejaba pasar.
// Medido entonces: 9 de 14 formas textuales dejaban salir un token.
//
// EL ARREGLO YA ESTABA ESCRITO EN ESTE REPO, EN LA OTRA MITAD DEL SISTEMA. El token de PERSONA
// (`GenerateToken`, internal/mcp/principals_admin.go) es `msb_` + base64url, y el redactor lo tapa
// por entropía desde siempre. El de DISPOSITIVO era hex. Es el defecto dominante de este árbol
// —la lección aprendida de un lado y no del hermano— y acá se usa como remedio: darle a esta
// credencial la forma que su hermana ya tenía.
//
// LO QUE TAPA ES EL ALFABETO Y NO EL PREFIJO, y hay un subtest que lo fija: `msbd_` pegado a un
// cuerpo hex SIGUE fugando, porque la entropía del hex (~4,0) queda abajo del umbral. Un arreglo
// que agregue sólo el prefijo no sirve, y sin esa mitad alguien lo «simplificaría» así.
//
// ────────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ EL TOKEN SE PIDE AL GENERADOR Y NO SE ESCRIBE A MANO
//
// Un literal acá sería una COPIA: el día que alguien cambie `NuevoToken` esta guarda seguiría
// midiendo la forma vieja y quedaría verde sobre la credencial nueva. Se le pide al generador, que
// es la única fuente, y se afirma la PROPIEDAD —no atraviesa— en vez de la forma.
//
// Sabotaje que la hace fallar: en `NuevoToken`, volver a emitir hex —`hex.EncodeToString(b)` en
// lugar de la forma con prefijo y base64url— y la credencial vuelve a ser indistinguible de un
// digest de git.
//
// EL `_ = base64.RawURLEncoding` DEL SABOTAJE NO ES ADORNO Y SE APRENDIÓ CORRIÉNDOLO. Sin esa
// línea, reemplazar el `return` deja `encoding/base64` importado y sin usar: el paquete no compila
// y el arnés contesta «sin veredicto» —correcto, y se lee como un límite de la herramienta cuando
// el que está mal es el sabotaje—. La regla general: al elegir dónde arranca y termina el `de`, hay
// que mirar qué otras referencias de cada símbolo quedan vivas después del corte.
// arnes: archivo="internal/fleet/device.go"
// arnes: de="\treturn tokenPrefijo + base64.RawURLEncoding.EncodeToString(b), nil"
// arnes: a="\t_ = base64.RawURLEncoding\n\treturn hex.EncodeToString(b), nil"
// arnes: prueba="TestNingunaCredencialQueAcunamosAtraviesaElRedactor"
func TestNingunaCredencialQueAcunamosAtraviesaElRedactor(t *testing.T) {
	// LAS FORMAS EN QUE UNA CREDENCIAL APARECE DE VERDAD EN UNA SALIDA DE COMANDO. No son
	// inventadas: las dos primeras son las dos formas EXACTAS en que los cuatro tokens medidos
	// quedaron en el transcripto.
	formas := []struct{ nombre, plantilla string }{
		{"rótulo de guion, con relleno", "  token   : %T"},
		{"respuesta JSON de enroll", `{"name":"nas","project_id":"casa","tier":"B","token":"%T"}`},
		{"rótulo corto", "token: %T"},
		{"JSON con espacio", `{"token": "%T"}`},
		{"línea pelada de un cat", "%T"},
		{"argumento de CLI", "musubi agent --token %T"},
		{"export de shell", "export MUSUBI_DEVICE_TOKEN=%T"},
		{"dentro de una URL", "https://cerebro/latido?t=%T"},
		{"pegada a texto", "la credencial es %T y vence mañana"},
	}

	atraviesa := func(t *testing.T, secreto string) []string {
		t.Helper()
		var fugas []string
		for _, f := range formas {
			entrada := strings.ReplaceAll(f.plantilla, "%T", secreto)
			salida, _ := redact.Redact(entrada)
			if strings.Contains(salida, secreto) {
				fugas = append(fugas, f.nombre)
			}
		}
		return fugas
	}

	t.Run("el token de dispositivo no sale en claro por NINGUNA forma", func(t *testing.T) {
		tok, err := NuevoToken()
		if err != nil {
			t.Fatalf("NuevoToken falló: %v — no medí nada", err)
		}
		// CONTROL DE QUE SE MIDIÓ ALGO: un secreto vacío o trivial lo taparía cualquier cosa, y
		// entonces el verde no diría nada.
		if len(tok) < 16 {
			t.Fatalf("NuevoToken devolvió algo de %d caracteres: demasiado corto para que esta "+
				"prueba signifique algo", len(tok))
		}
		if fugas := atraviesa(t, tok); len(fugas) > 0 {
			t.Errorf("la credencial que acuña este paquete SALE EN CLARO por %d de %d formas: %s\n"+
				"  Esto ya pasó de verdad: cuatro tokens de dispositivo quedaron en claro en un\n"+
				"  transcripto, dos de máquinas con `exec` y `shell`.\n"+
				"  NO arregles el redactor: tiene que dejar pasar el hex de 40 y 64 (los SHA de git).\n"+
				"  Lo que tiene que cambiar es la FORMA de la credencial — que no comparta alfabeto\n"+
				"  con un digest. El token de persona (`msb_` + base64url) ya lo hace.",
				len(fugas), len(formas), strings.Join(fugas, ", "))
		}
	})

	// EL HERMANO, BUSCADO Y ENCONTRADO ROTO — Y POR QUÉ NO SE ARREGLA ACÁ.
	//
	// Este paquete acuña DOS credenciales. La otra es `NuevaPassPantalla`, y escribí el subtest
	// esperando que estuviera sana: salió ROJA, 6 de 9 formas la dejan pasar. No es un descuido que
	// falte de esta prueba, es una decisión, y vale decir las dos razones.
	//
	// LA PRIMERA es que su arreglo no es el mismo. La pass se DICTA POR TELÉFONO —su alfabeto
	// excluye 0/O, 1/l/I, 5/S, 8/B justamente por eso— así que no puede pasarse a base64url, y sus
	// propias guardas (pantalla_test.go:38 y :46) fijan largo exacto 16 y el alfabeto sin `_`.
	// Cambiarla es cambiar lo que un humano lee en voz alta: es una decisión de producto, no de
	// redacción.
	//
	// LA SEGUNDA es la diferencia de evidencia, y pesa más: la fuga del token está CONSUMADA —
	// cuatro en claro en un transcripto— y la de la pass, por ahora, sólo está demostrada contra
	// estas formas sintéticas.
	//
	// LO QUE SÍ SE HIZO es no dejarla como una nota que nadie relee: su hueco es un caso de una
	// propiedad general del redactor —la entropía de n caracteres no puede pasar de log2(n), así que
	// NADA de 22 caracteres o menos puede alcanzar el umbral de 4,5— y eso quedó fijado como clase
	// en TestElCatchAllDeEntropiaTieneUnPisoYEstaDeclarado (internal/redact). La pass cae ahí, con
	// sus 16, y cualquier credencial corta futura también.

	t.Run("CONTROL: un SHA-256 de git tiene que seguir INTACTO", func(t *testing.T) {
		// LA MITAD QUE IMPIDE QUE ESTO SE «ARREGLE» ROMPIENDO EL REDACTOR. Si alguien cierra el
		// agujero tapando todo el hex, esta rama se pone roja — y con razón: una salida donde los
		// commits salen como [REDACTED] es inservible, y el que la vea va a apagar la redacción
		// entera.
		crudo := make([]byte, 32)
		for i := range crudo {
			crudo[i] = byte(i * 7)
		}
		sha := hex.EncodeToString(crudo) // 64 hex, la forma exacta de un sha256 de git
		entrada := "commit " + sha + " verificado"
		salida, _ := redact.Redact(entrada)
		if !strings.Contains(salida, sha) {
			t.Errorf("el redactor tapó un SHA-256 de git:\n  %s\n"+
				"  El arreglo de la credencial NO puede pasar por taparle el hex al redactor.",
				salida)
		}
	})

	t.Run("la credencial no comparte alfabeto con un digest", func(t *testing.T) {
		// LO QUE TAPA ES EL ALFABETO, NO EL PREFIJO, y sin esta rama el arreglo se podría
		// «simplificar» a pegarle `msbd_` a un cuerpo hex. Medido: eso SIGUE fugando, porque la
		// entropía del hex (~4,0) queda abajo del umbral del catch-all de entropía, y el catch-all
		// de hex tiene que saltear 40 y 64.
		//
		// Se afirma la PROPIEDAD —el cuerpo no es hex— y no el nombre del codificador, para que
		// cambiar base64url por otro alfabeto de alta entropía no ponga roja una guarda sana.
		tok, err := NuevoToken()
		if err != nil {
			t.Fatalf("NuevoToken falló: %v — no medí nada", err)
		}
		cuerpo := strings.TrimPrefix(tok, tokenPrefijo)
		if cuerpo == "" {
			t.Fatalf("el token quedó vacío al sacarle el prefijo %q: no medí nada", tokenPrefijo)
		}
		todoHex := true
		for _, r := range cuerpo {
			if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
				todoHex = false
				break
			}
		}
		if todoHex {
			t.Errorf("el cuerpo de la credencial (%d caracteres) es TODO HEX.\n"+
				"  Un cuerpo hex es indistinguible de un digest para el redactor, que tiene que\n"+
				"  dejar pasar el hex de 40 y 64. Agregarle un prefijo NO alcanza: la entropía del\n"+
				"  hex queda abajo del umbral y el catch-all de entropía tampoco lo ve.\n"+
				"  Lo que cierra el agujero es el ALFABETO — base64url, como el token de persona.",
				len(cuerpo))
		}
	})
}
