# Instalador interactivo de Musubi para Windows.
#
# Doble clic: usa install.bat (te pregunta el alcance y llama a este script).
# Linea de comandos:
#   Interactivo (pregunta local/global):
#     irm -useb https://raw.githubusercontent.com/codeabraham16/musubi/main/scripts/install.ps1 | iex
#   No interactivo (global):
#     $env:MUSUBI_SCOPE='global'; irm -useb .../install.ps1 | iex
#
# Variables de entorno reconocidas:
#   MUSUBI_SCOPE   = local | global   (si falta, se pregunta; sin consola -> global)
#   MUSUBI_DIR     = carpeta del proyecto (default: carpeta actual)
#   MUSUBI_NOSETUP = 1                 (no correr 'musubi setup')
#   MUSUBI_BINARY  = ruta a un musubi.exe ya descargado (evita la descarga)

$ErrorActionPreference = 'Stop'
$repo = 'codeabraham16/musubi'

# --- Resolver alcance (local | global) ---
$scope = $env:MUSUBI_SCOPE
if ([string]::IsNullOrWhiteSpace($scope)) {
    if ([Environment]::UserInteractive) {
        Write-Host ""
        Write-Host "Donde queres instalar Musubi?"
        Write-Host "  [L] Solo este repo (local, NO toca la PC ni el PATH)"
        Write-Host "  [G] Global en la PC (PATH del usuario, sin admin)"
        $resp = Read-Host "Eleccion (L/G)"
        switch ($resp.Trim().ToUpper()) {
            'G' { $scope = 'global' }
            default { $scope = 'local' }
        }
    } else {
        $scope = 'global'
    }
}
$scope = $scope.ToLower()
if ($scope -ne 'local' -and $scope -ne 'global') {
    throw "MUSUBI_SCOPE invalido: '$scope' (usa local|global)"
}

# --- Resolver carpeta del proyecto ---
$dir = $env:MUSUBI_DIR
if ([string]::IsNullOrWhiteSpace($dir)) { $dir = (Get-Location).Path }
$dir = (Resolve-Path $dir).Path

# --- Detectar arquitectura / asset ---
$asset = 'Musubi.exe'
if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { $asset = 'Musubi-arm64.exe' }

# --- Obtener el binario (MUSUBI_BINARY o descarga del release) ---
$tmp = Join-Path $env:TEMP ("musubi-dl-{0}.exe" -f $PID)
if (-not [string]::IsNullOrWhiteSpace($env:MUSUBI_BINARY) -and (Test-Path $env:MUSUBI_BINARY)) {
    Copy-Item $env:MUSUBI_BINARY $tmp -Force
    Write-Host "Usando binario provisto: $($env:MUSUBI_BINARY)"
} else {
    $url = "https://github.com/$repo/releases/latest/download/$asset"
    Write-Host "Descargando $asset ..."
    $ok = $false
    try {
        Invoke-WebRequest -Uri $url -OutFile $tmp -UseBasicParsing
        $ok = $true
    } catch {
        if (Get-Command gh -ErrorAction SilentlyContinue) {
            Write-Host "Descarga directa fallo (repo privado?). Probando con gh CLI..."
            gh release download --repo $repo --pattern $asset --output $tmp --clobber
            $ok = $true
        }
    }
    if (-not $ok) {
        throw "No se pudo descargar $asset. Instala gh CLI y autenticate, o descarga el .exe y usa MUSUBI_BINARY."
    }
}

# --- Instalar segun alcance ---
if ($scope -eq 'global') {
    $installDir = Join-Path $env:LOCALAPPDATA 'Programs\musubi'
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    $exe = Join-Path $installDir 'musubi.exe'
    Copy-Item $tmp $exe -Force
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($userPath -notlike "*$installDir*") {
        [Environment]::SetEnvironmentVariable('Path', "$userPath;$installDir", 'User')
        Write-Host "PATH del usuario actualizado (abri una terminal nueva para que tome efecto)."
    }
    Write-Host "Musubi (GLOBAL) instalado en $exe"
} else {
    $binDir = Join-Path $dir '.musubi\bin'
    New-Item -ItemType Directory -Force -Path $binDir | Out-Null
    $exe = Join-Path $binDir 'musubi.exe'
    Copy-Item $tmp $exe -Force
    # Proteger el binario local en git (no se commitea).
    $gi = Join-Path $dir '.gitignore'
    $line = '.musubi/bin/'
    if (-not (Test-Path $gi) -or -not (Select-String -Path $gi -SimpleMatch $line -Quiet)) {
        Add-Content -Path $gi -Value $line
    }
    Write-Host "Musubi (LOCAL) instalado en $exe (no se toco el PATH ni la PC)."
}
Remove-Item $tmp -Force -ErrorAction SilentlyContinue

# --- Setup del proyecto (inyecta .musubi/ + .mcp.json + hook) ---
if ($env:MUSUBI_NOSETUP -ne '1') {
    Write-Host "Preparando el proyecto en $dir ..."
    Push-Location $dir
    try { & $exe setup } finally { Pop-Location }
}

Write-Host ""
if ($scope -eq 'local') {
    Write-Host "Listo (LOCAL): Musubi vive dentro de '$dir\.musubi\' y no dejo nada en tu PC."
    Write-Host "Para desinstalar: borra la carpeta .musubi\ del repo."
} else {
    Write-Host "Listo (GLOBAL): usa 'musubi setup' en cualquier otro repo para sumarlo."
}
Write-Host "Reabri el proyecto en Claude Code y el server 'musubi' cargara solo."
