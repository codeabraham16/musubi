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
// formasDeUnaCredencial son las formas EN QUE UNA CREDENCIAL APARECE DE VERDAD EN UNA SALIDA DE
// COMANDO. No son inventadas: las dos primeras son las dos formas EXACTAS en que los cuatro tokens
// medidos quedaron en el transcripto.
//
// Viven a nivel de paquete —y no adentro de una prueba— desde que hay DOS guardas que las usan: ésta
// y TestLaCredencialPropiaLaTapaLaReglaYNoLaSuerte. Copiarlas en la segunda habría sido exactamente
// el defecto que este repo cataloga: un derivado escrito a mano es una copia, y la copia es la
// próxima en envejecer cuando aparezca la décima forma.
var formasDeUnaCredencial = []struct{ nombre, plantilla string }{
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

// atraviesaElRedactor devuelve los nombres de las formas por las que `secreto` sale EN CLARO.
func atraviesaElRedactor(t *testing.T, secreto string) []string {
	t.Helper()
	var fugas []string
	for _, f := range formasDeUnaCredencial {
		entrada := strings.ReplaceAll(f.plantilla, "%T", secreto)
		salida, _ := redact.Redact(entrada)
		if strings.Contains(salida, secreto) {
			fugas = append(fugas, f.nombre)
		}
	}
	return fugas
}

func TestNingunaCredencialQueAcunamosAtraviesaElRedactor(t *testing.T) {
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
		if fugas := atraviesaElRedactor(t, tok); len(fugas) > 0 {
			t.Errorf("la credencial que acuña este paquete SALE EN CLARO por %d de %d formas: %s\n"+
				"  Esto ya pasó de verdad: cuatro tokens de dispositivo quedaron en claro en un\n"+
				"  transcripto, dos de máquinas con `exec` y `shell`.\n"+
				"  NO arregles el redactor: tiene que dejar pasar el hex de 40 y 64 (los SHA de git).\n"+
				"  Lo que tiene que cambiar es la FORMA de la credencial — que no comparta alfabeto\n"+
				"  con un digest. El token de persona (`msb_` + base64url) ya lo hace.",
				len(fugas), len(formasDeUnaCredencial), strings.Join(fugas, ", "))
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

// TestLaCredencialPropiaLaTapaLaReglaYNoLaSuerte exige que el redactor tape las credenciales de
// Musubi POR SU FORMA, y no porque el sorteo del generador haya salido con entropía alta.
//
// ────────────────────────────────────────────────────────────────────────────────────────────────
// LA GUARDA DE ARRIBA PASABA EL 99,978 % DE LAS VECES, Y ESO NO ES PASAR
//
// Esa prueba le pide el token al generador —que es lo correcto— y exige que no atraviese. Pero hasta
// hoy la tabla de reglas del redactor NO tenía ninguna para las credenciales propias: estaban AWS,
// GitHub, GitLab, Stripe, Anthropic, OpenAI, Google, Slack, Telegram, SendGrid, Twilio y npm, y las
// de casa no. Lo único que las tapaba era el catch-all de entropía, que exige 4.5 bits/char.
//
// Y no siempre llega. Medido sobre 200.000 tokens con la forma real (32 bytes al azar → base64url
// sin relleno): 0,022 % de los de dispositivo y 0,045 % de los de persona quedan por DEBAJO del
// umbral. O sea ~1 de cada 4.500 credenciales emitidas, invisible para el redactor.
//
// No es teórico: el CI lo cazó con un token de entropía 4,4647 y puso roja la guarda de fleet_exec.
// Una guarda que depende de un sorteo es intermitente por construcción, y su verde no distingue
// «está cubierto» de «esta vez tuve suerte».
//
// ────────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ EL CUERPO ES DE ENTROPÍA CERO, Y POR QUÉ HAY UN CONTROL
//
// El token de esta prueba se arma con el prefijo REAL —`tokenPrefijo`, no un literal— y un cuerpo
// repetido, cuya entropía de Shannon es 0. Así el catch-all no puede verlo ni por casualidad: si
// queda tapado, lo tapó la REGLA.
//
// Y la segunda mitad es la que impide que esto se lea al revés. El mismo cuerpo con un prefijo
// AJENO tiene que ATRAVESAR el redactor. Si ese control se pone verde, significa que algo más
// empezó a tapar este fixture —una regla nueva, un umbral bajado— y entonces la primera mitad ya no
// prueba lo que dice: estaría midiendo el catch-all otra vez, con otro nombre.
//
// Sabotaje que la hace fallar: desactivar la regla `musubi-token` cambiándole el prefijo por uno que
// no existe. No toca la sintaxis —la tabla sigue compilando— y deja a las credenciales propias
// exactamente como estaban antes de este arreglo: a merced del sorteo.
// arnes: archivo="internal/redact/redact.go"
// arnes: de="msbd?_"
// arnes: a="zzzz?_"
// arnes: prueba="TestLaCredencialPropiaLaTapaLaReglaYNoLaSuerte"
func TestLaCredencialPropiaLaTapaLaReglaYNoLaSuerte(t *testing.T) {
	// EL 24 ESTÁ ESCRITO A MANO, Y VALE DECIR POR QUÉ. El mínimo que la regla exige vive adentro de
	// un regex (`{20,}`), así que derivarlo desde acá pediría parsear ese patrón — más maquinaria que
	// la que custodia. Se elige con margen sobre 20 y se declara: si alguien sube ese mínimo por
	// encima de 24, esta prueba se pone ROJA en vez de quedarse midiendo otra cosa en silencio, que
	// es el desenlace que importa evitar.
	cuerpo := strings.Repeat("a", 24)

	t.Run("con el prefijo PROPIO no sale por ninguna forma", func(t *testing.T) {
		tok := tokenPrefijo + cuerpo
		if fugas := atraviesaElRedactor(t, tok); len(fugas) > 0 {
			t.Errorf("una credencial con la forma de Musubi y entropía BAJA sale en claro por %d de %d "+
				"formas: %s\n"+
				"  El catch-all de entropía no puede verla (entropía del cuerpo = 0), así que lo único\n"+
				"  que puede taparla es una REGLA POR FORMA en internal/redact. Sin ella, ~1 de cada\n"+
				"  4.500 credenciales reales se escribe en claro en la bitácora.",
				len(fugas), len(formasDeUnaCredencial), strings.Join(fugas, ", "))
		}
	})

	t.Run("CONTROL: el mismo cuerpo con un prefijo AJENO sí atraviesa", func(t *testing.T) {
		// Sin esta mitad, la de arriba podría estar en verde porque el catch-all bajó su umbral o
		// porque otra regla empezó a tapar cualquier cosa — y entonces no probaría la regla propia.
		ajeno := "zqxw_" + cuerpo
		fugas := atraviesaElRedactor(t, ajeno)
		if len(fugas) == 0 {
			t.Errorf("el fixture de control (%d caracteres, entropía 0, prefijo ajeno) NO atraviesa el "+
				"redactor por ninguna de las %d formas.\n"+
				"  Algo más lo está tapando, así que la mitad de arriba ya no prueba que la regla\n"+
				"  `musubi-token` sea la que trabaja. Revisá qué regla nueva lo cubre antes de\n"+
				"  confiar en esta guarda.", len(ajeno), len(formasDeUnaCredencial))
		}
	})
}
