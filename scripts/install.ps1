# Instalador de Musubi para Windows.
# Uso (una linea):
#   iwr -useb https://raw.githubusercontent.com/codeabraham16/musubi/main/scripts/install.ps1 | iex
#
# Descarga el binario de la ultima release, lo instala en
# %LOCALAPPDATA%\Programs\musubi y lo agrega al PATH del usuario.

$ErrorActionPreference = 'Stop'
$repo = 'codeabraham16/musubi'

# Detectar arquitectura
$arch = 'amd64'
if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { $arch = 'arm64' }
$asset = "musubi-windows-$arch.exe"

$installDir = Join-Path $env:LOCALAPPDATA 'Programs\musubi'
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
$dest = Join-Path $installDir 'musubi.exe'

Write-Host "Descargando $asset ..."
$url = "https://github.com/$repo/releases/latest/download/$asset"
$downloaded = $false
try {
    Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing
    $downloaded = $true
} catch {
    # Repo privado: intentar con gh CLI autenticado.
    if (Get-Command gh -ErrorAction SilentlyContinue) {
        Write-Host "Descarga directa fallo (repo privado?). Probando con gh CLI..."
        gh release download --repo $repo --pattern $asset --output $dest --clobber
        $downloaded = $true
    }
}
if (-not $downloaded) {
    throw "No se pudo descargar $asset. Si el repo es privado, instala gh CLI y autenticate (gh auth login), o haz el repo publico."
}

# Agregar al PATH del usuario si falta
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$installDir", 'User')
    Write-Host "PATH actualizado (abri una terminal nueva para que tome efecto)."
}

Write-Host ""
Write-Host "Musubi instalado en $dest"
Write-Host "Ahora, dentro de cualquier proyecto, corre:  musubi setup"
