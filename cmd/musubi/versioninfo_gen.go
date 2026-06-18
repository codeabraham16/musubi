package main

// Recursos de versión de Windows (VERSIONINFO + icono) embebidos en el .exe.
//
// Los archivos rsrc_windows_*.syso se generan a partir de versioninfo.json e
// icon.ico con goversioninfo. versioninfo.json es la ÚNICA fuente de verdad de la
// versión que muestran las propiedades del .exe en Windows: editá ahí y regenerá,
// no toques los .syso a mano.
//
// Requiere goversioninfo (instalar una vez):
//
//	go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
//
// Para regenerar (desde cmd/musubi/):
//
//	go generate ./...
//
//go:generate goversioninfo -64 -o rsrc_windows_amd64.syso versioninfo.json
//go:generate goversioninfo -arm -64 -o rsrc_windows_arm64.syso versioninfo.json
