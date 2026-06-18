# Contribuir a Musubi

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

## Licencia

Al contribuir aceptás que tu aporte se publique bajo la licencia [MIT](LICENSE) del proyecto.
