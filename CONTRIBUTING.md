# Contribuir a Musubi

<a href="CONTRIBUTING.en.md">English</a> · <strong>Español</strong>

¡Gracias por tu interés en mejorar Musubi! Esta guía resume cómo proponer cambios.

## Antes de empezar

- Para **bugs** o **ideas**, abrí primero un [issue](https://github.com/codeabraham16/musubi/issues)
  usando la plantilla correspondiente. Discutir el enfoque antes de escribir código ahorra trabajo.
- Para cambios chicos y obvios (typos, docs), podés ir directo al PR.

## Entorno de desarrollo

Necesitás **Go 1.26+**. No hace falta nada más: la base de datos es SQLite embebido
(puro Go, sin CGo) y los embeddings son opcionales.

```bash
git clone https://github.com/codeabraham16/musubi.git
cd musubi
go build ./cmd/musubi   # compila el binario
```

## Antes de abrir un PR

Corré localmente lo mismo que corre el CI, y que todo pase:

```bash
go vet ./...
go build ./...
go test -race ./...
```

Si tocás lógica nueva, **agregá tests**. El proyecto apunta a mantener o subir la
cobertura (`go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out`).

## Convenciones

- **Idioma:** comentarios, mensajes de commit y de CLI en **español**; identificadores
  de código y nombres de tools MCP en **inglés** (ej. `musubi_save_observation`).
- **Commits:** título corto en imperativo describiendo el *qué*. Si cierra un issue,
  referencialo (`#NN`).
- **Estilo Go:** `gofmt` (idiomático), errores envueltos con `%w`, sin `panic` en código
  de producción.
- **Local-first:** ningún cambio debe requerir un servicio externo obligatorio. Las
  dependencias de red (embeddings, etc.) son siempre opcionales y con fallback.

## Versionado y changelog

El proyecto sigue [Versionado Semántico](https://semver.org/lang/es/). Si tu cambio es
visible para el usuario, agregá una entrada en la sección `[Unreleased]` de
[CHANGELOG.md](CHANGELOG.md).

## Publicar un release

La versión tiene **una sola fuente de verdad**: el archivo [`VERSION`](VERSION) en la raíz.
De ahí se deriva todo — el tag (verificado en el release), `versioninfo.json` (verificado
por test) y `musubi version` (inyectado desde el tag). No hay que sincronizarlas a mano.

1. Bumpeá **`VERSION`** a `X.Y.Z` y actualizá **`cmd/musubi/versioninfo.json`** para que
   coincida (campos numéricos de `FixedFileInfo` y las cadenas `FileVersion`/`ProductVersion`
   de `StringFileInfo`, que llevan un cuarto componente: `X.Y.Z.0`). El test
   `TestVersioninfoMatchesVERSION` falla si divergen. **No edites los `.syso` a mano**:
   `release.yml` los regenera desde `versioninfo.json` con `goversioninfo` pineado.
2. Pasá el contenido de `[Unreleased]` a una sección `[X.Y.Z]` con fecha en `CHANGELOG.md`
   y actualizá los links de comparación al final del archivo. **Al pasarlo, escribí a mano el
   resumen de la versión**, arriba de `### Added`: `### Destacado` con tres o cuatro viñetas en
   español, una oración llana cada una, y `### Highlights` con las mismas ideas en inglés. Es lo
   que ve quien llega a la portada del repo y a la página del release. Sin ese resumen salen los
   titulares del CHANGELOG tal como están escritos, que son notas de ingeniería con jerga.
3. Regenerá «Novedades» en los dos README: `go run ./deploy/cmd/notas-release -readme`. Lee
   `CHANGELOG.md` —por eso va **después** del paso 2— y reescribe lo que hay entre
   `<!-- novedades:inicio -->` y `<!-- novedades:fin -->` en `README.md` y `README.en.md`: las tres
   versiones más nuevas, con las viñetas de su `### Destacado` (en el README inglés, las de
   `### Highlights`) y el enlace a su sección; si una versión no lo trae, hasta cuatro titulares
   sacados del texto. **Ese bloque no se edita a mano**: `TestReadmeNovedadesAlDia` falla si no
   coincide con lo que sale del CHANGELOG.

   Arriba de ese bloque vive «Ya en `main`, todavía sin publicar», que sí se escribe a mano y
   lleva `base=X.Y.Z` en su marcador de inicio. Al publicar una versión nueva queda vieja y
   `TestSinPublicarDeLosReadmeReales` lo canta: cambiá `base=` a la versión nueva y rehacé los
   titulares con lo que quedó fuera, o borrá el bloque entero.
4. Commiteá, mergeá a `main` y creá el tag: `git tag -a vX.Y.Z -m "..." && git push origin vX.Y.Z`.
   El workflow [`release.yml`](.github/workflows/release.yml) **aborta si el tag no coincide
   con `VERSION`**, regenera el recurso de Windows y compila los binarios cross-platform
   (Windows/Linux/macOS, amd64+arm64) con checksums SHA-256, y publica el release. Las notas
   del release en GitHub salen solas: el job `create-release` corre
   `go run ./deploy/cmd/notas-release -version "$TAG"` y usa lo que imprime como cuerpo (el
   `### Destacado` de la sección `[X.Y.Z]`, después los titulares de sus grupos y un enlace al
   CHANGELOG). Si ese paso falla, el release se publica igual, con el cuerpo vacío.
5. **Firmá el release A MANO, en tu máquina, con las claves montadas el rato que dura.** El
   `sha256sums.txt` que produce el CI dice que el archivo llegó entero, no que sea nuestro: lo
   publica el mismo que publica el binario. Y como el cerebro y el agente son el MISMO binario, un
   release ajeno no entrega una máquina: entrega la flota. **Las claves privadas no viven en el CI**
   — si estuvieran ahí, quien comprometa el CI firma lo que quiera y la firma no compra nada.

   **El orden importa y no es reversible:**

   ```bash
   # 1) Authenticode sobre el .exe — CAMBIA LOS BYTES del archivo.
   ./deploy/firmar-windows.sh dist/musubi-windows-amd64.exe ~/claves/musubi-editor.crt ~/claves/musubi-editor.key
   # 2) Recién ahora el manifiesto ed25519, que hashea los bytes YA firmados.
   ./deploy/firmar-release.sh X.Y.Z ~/claves/release.key dist/
   ```

   Al revés, el sha256 del manifiesto deja de corresponder al archivo publicado y `musubi update`
   falla con «hash mismatch», que no menciona ni firma ni orden. `firmar-windows.sh` aborta si ve
   un `manifest.json` ya armado al lado, pero la regla se entiende mejor acá que en el error.

   Subí `manifest.json` y `manifest.json.sig` junto con los binarios: sin el manifiesto firmado,
   `musubi update` **se niega a instalar**. Y en cada máquina Windows, una sola vez,
   `deploy/confiar-editor-windows.ps1` instala el certificado del editor — con eso una excepción de
   Defender o una regla de AppLocker se escriben por EDITOR y sobreviven a los releases siguientes,
   en vez de reautorizar hash por hash.

## Licencia

Al contribuir aceptás que tu aporte se publique bajo la licencia [MIT](LICENSE) del proyecto.
