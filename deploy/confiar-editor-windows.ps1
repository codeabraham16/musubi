<#
  confiar-editor-windows.ps1 - instala en ESTA maquina el certificado del editor de Musubi, para
  que Windows reconozca al agente como firmado por alguien conocido.

  --------------------------------------------------------------------------------------------
  QUE ES ESTO Y POR QUE NO CUESTA PLATA

  Un certificado de firma de codigo comprado sirve para que una maquina AJENA confie en el binario
  sin que nadie le diga nada. Dentro de una flota propia eso no hace falta: las maquinas se enrolan
  a mano y con administrador, y en ese mismo acto se les puede decir en quien confiar. Es lo que
  hace cualquier empresa con su PKI interna.

  El certificado lo genera `deploy/firmar-windows.sh --crear-certificado`, la privada vive fuera de
  linea, y a esta maquina llega SOLO la parte publica (.crt), que no sirve para firmar nada.

  --------------------------------------------------------------------------------------------
  LO QUE ESTE GUION SI HACE Y LO QUE NO

  SI:  que `Get-AuthenticodeSignature musubi.exe` diga Valid, y que una regla de AppLocker/WDAC o
       una excepcion de Defender se puedan escribir POR EDITOR: una vez, y no una por cada
       release. Eso es lo que necesita el autoupdate por anillos, donde el hash cambia siempre y
       el editor no.

  NO:  reputacion de SmartScreen. Se gana con el historial del certificado ante Microsoft, y un
       certificado que Microsoft no conoce arranca -y se queda- en cero.

  NO:  la excepcion de NordVPN de A31. Eso esta MEDIDO y no es un problema de firma: el mismo
       binario sin firmar latio como `musubi.exe` y fallo como `musubi-nuevo.exe`. Es una lista
       blanca por RUTA. Ningun certificado, pago o propio, la mueve.

  --------------------------------------------------------------------------------------------
  POR QUE ESTE GUION DESCONFIA DEL CERTIFICADO QUE LE DAN

  Instalar algo en "Entidades de certificacion raiz de confianza" es serio: un certificado ahi
  puede avalar TODO lo que la maquina verifique. Si ademas fuera una CA, quien tenga su clave
  emite certificados para lo que quiera -el TLS de cualquier sitio incluido- y esta maquina los
  cree. Por eso, antes de importar, se comprueba:

     CA:FALSE          no puede emitir otros certificados: solo se avala a si mismo.
     EKU = codeSigning y NADA MAS. Un certificado raiz sin EKU acotado tambien vale para TLS.

  El que instala no tiene por que confiar en el que genero. Estas dos cosas las pone
  firmar-windows.sh y las vuelve a verificar este guion, del otro lado.

  Uso:
    .\confiar-editor-windows.ps1 -Certificado musubi-editor.crt -Binario "$env:LOCALAPPDATA\Musubi\musubi.exe"
    .\confiar-editor-windows.ps1 -Certificado musubi-editor.crt -Huella <SHA256 sin dos puntos>
    .\confiar-editor-windows.ps1 -Quitar
#>
param(
  [string]$Certificado = "",
  # El .exe que deberia estar firmado por ese certificado. Si se pasa, se comprueba ANTES de
  # importar nada: si no cierra, no se toca ningun almacen.
  [string]$Binario = "",
  # Huella SHA-256 esperada, para comparar contra un canal distinto del que trajo el archivo.
  [string]$Huella = "",
  [switch]$Quitar
)

$ErrorActionPreference = "Stop"

$id = [Security.Principal.WindowsIdentity]::GetCurrent()
if (-not (New-Object Security.Principal.WindowsPrincipal $id).IsInRole(
      [Security.Principal.WindowsBuiltInRole]::Administrator)) {
  throw "Hace falta PowerShell COMO ADMINISTRADOR: se escribe en los almacenes de la maquina."
}

$NOMBRE = "CN=Musubi"

if ($Quitar) {
  $sacados = 0
  foreach ($almacen in @("Root", "TrustedPublisher")) {
    Get-ChildItem "Cert:\LocalMachine\$almacen" |
      Where-Object { $_.Subject -eq $NOMBRE -or $_.Subject -like "$NOMBRE,*" } |
      ForEach-Object {
        Remove-Item -Path "Cert:\LocalMachine\$almacen\$($_.Thumbprint)" -Force
        Write-Host "  quitado de $almacen : $($_.Thumbprint)"
        $sacados++
      }
  }
  if ($sacados -eq 0) { Write-Host "No habia nada que quitar." }
  else { Write-Host "`nListo. El agente sigue corriendo: sacar la confianza no lo desinstala," -ForegroundColor Yellow
         Write-Host "pero a partir de ahora su firma figura como de editor desconocido." -ForegroundColor Yellow }
  return
}

if (-not $Certificado) { throw "Falta -Certificado <ruta al .crt>. Con -Quitar se desinstala." }
if (-not (Test-Path $Certificado)) { throw "No encuentro el certificado en $Certificado" }

$rutaCert = (Resolve-Path $Certificado).Path
$cert = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2 $rutaCert

# .NET llama "Thumbprint" al SHA-1, y openssl imprime SHA-256. Compararlos directamente no
# coincidiria NUNCA, y eso se lee como un ataque cuando es una unidad distinta. Se calcula el
# SHA-256 a mano sobre los mismos bytes que hashea openssl (el DER del certificado).
$sha256 = ([System.BitConverter]::ToString(
  ([System.Security.Cryptography.SHA256]::Create()).ComputeHash($cert.RawData))
).Replace("-", "")

Write-Host ""
Write-Host "  Editor : $($cert.Subject)"
Write-Host "  Vence  : $($cert.NotAfter.ToString('yyyy-MM-dd'))"
Write-Host "  SHA-256: $sha256"
Write-Host ""

if ($cert.NotAfter -lt (Get-Date)) { throw "El certificado VENCIO el $($cert.NotAfter). No lo instalo." }

# ------------------------------------------------------------------------------------------
# LAS DOS COMPROBACIONES QUE HACEN SEGURO PONERLO EN LA RAIZ. Ver el encabezado.
$bcCruda = $cert.Extensions | Where-Object { $_.Oid.Value -eq "2.5.29.19" }
if (-not $bcCruda) {
  throw ("El certificado no declara basicConstraints. Se rechaza: sin la restriccion escrita no " +
         "hay forma de saber que no es una CA, y una CA en la raiz avala cualquier cosa.")
}

# SE REPARSEA EN VEZ DE CONFIAR EN EL TIPO QUE VINO. La coleccion de extensiones devuelve objetos
# TIPADOS solo para los OID que .NET conoce; si por lo que sea llega uno generico,
# `.CertificateAuthority` es $null y el `if` de abajo no dispara: una CA entraria derecho a la
# raiz. Una valla de seguridad que falla ABIERTA cuando no reconoce la entrada no es una valla.
$bc = New-Object System.Security.Cryptography.X509Certificates.X509BasicConstraintsExtension
$bc.CopyFrom($bcCruda)
if ($bc.CertificateAuthority) {
  throw ("El certificado es una CA (CA:TRUE). NO se instala en la raiz: quien tenga su clave " +
         "emitiria certificados para lo que quiera -el TLS de cualquier sitio incluido- y esta " +
         "maquina se los creeria. Para firmar codigo alcanza uno que solo se avale a si mismo.")
}

$ekuCruda = $cert.Extensions | Where-Object { $_.Oid.Value -eq "2.5.29.37" }
if (-not $ekuCruda) {
  throw "El certificado no acota su uso (sin EKU). En la raiz eso vale tambien para TLS. Se rechaza."
}
$eku = New-Object System.Security.Cryptography.X509Certificates.X509EnhancedKeyUsageExtension
$eku.CopyFrom($ekuCruda)
$usos = @($eku.EnhancedKeyUsages | ForEach-Object { $_.Value })
if ($usos.Count -ne 1 -or $usos[0] -ne "1.3.6.1.5.5.7.3.3") {
  throw ("El certificado sirve para mas que firmar codigo (EKU: " + ($usos -join ", ") + "). Se rechaza.")
}

if ($Huella) {
  $esperada = $Huella.Replace(":", "").Replace(" ", "").ToUpper()
  if ($esperada -ne $sha256) { throw "La huella NO coincide.`n  esperada: $esperada`n  el archivo: $sha256" }
  Write-Host "  huella verificada contra la que pasaste" -ForegroundColor Green
} else {
  Write-Host "  AVISO: sin -Huella. Si este .crt llego por el mismo canal que el .exe, comprobar" -ForegroundColor Yellow
  Write-Host "  uno contra el otro no prueba nada: quien pudo cambiar uno pudo cambiar los dos." -ForegroundColor Yellow
  Write-Host "  La huella tiene que venir por OTRO lado (la imprime firmar-windows.sh al crearlo)." -ForegroundColor Yellow
}

# ------------------------------------------------------------------------------------------
# SI DIERON UN BINARIO: SE COMPRUEBA LA IDENTIDAD ANTES DE IMPORTAR, NO DESPUES.
#
# Al reves seria un lazo: importar primero y despues preguntar "estas firmado?" acepta cualquier
# .exe firmado por CUALQUIER editor que la maquina ya tenga por bueno. Comparar la huella del
# firmante contra este certificado no depende de que confiemos en el todavia.
if ($Binario) {
  if (-not (Test-Path $Binario)) { throw "No encuentro el binario en $Binario" }
  $firma = Get-AuthenticodeSignature -FilePath $Binario
  if (-not $firma.SignerCertificate) {
    throw "$Binario no esta firmado. Firmalo primero con deploy/firmar-windows.sh."
  }
  if ($firma.SignerCertificate.Thumbprint -ne $cert.Thumbprint) {
    throw ("$Binario esta firmado por OTRO certificado.`n" +
           "  firmante: $($firma.SignerCertificate.Subject)`n" +
           "  este    : $($cert.Subject)`nNo importo nada.")
  }
  Write-Host "  el binario cierra contra este certificado" -ForegroundColor Green
}

# ------------------------------------------------------------------------------------------
# ROOT hace que la cadena valide (el certificado se firma a si mismo: la cadena es el, solo).
# TRUSTEDPUBLISHER es lo que consulta el filtro de editor -AppLocker, WDAC, la advertencia de
# "editor desconocido"-. Hacen falta los dos y no es redundancia: uno responde "la firma es
# valida", el otro "y ademas este editor esta autorizado aca".
foreach ($almacen in @("Root", "TrustedPublisher")) {
  $ya = Get-ChildItem "Cert:\LocalMachine\$almacen" | Where-Object { $_.Thumbprint -eq $cert.Thumbprint }
  if ($ya) { Write-Host "  ya estaba en $almacen" }
  else {
    Import-Certificate -FilePath $rutaCert -CertStoreLocation "Cert:\LocalMachine\$almacen" | Out-Null
    Write-Host "  instalado en $almacen" -ForegroundColor Green
  }
}

if ($Binario) {
  $firma = Get-AuthenticodeSignature -FilePath $Binario
  Write-Host ""
  if ($firma.Status -eq "Valid") {
    Write-Host "Get-AuthenticodeSignature: Valid - Windows ya reconoce al editor." -ForegroundColor Green
  } else {
    Write-Host "Get-AuthenticodeSignature sigue diciendo: $($firma.Status)" -ForegroundColor Red
    Write-Host "$($firma.StatusMessage)" -ForegroundColor Red
    Write-Host "Se importo el certificado igual; revisalo antes de dar esto por hecho." -ForegroundColor Red
  }
}

Write-Host ""
Write-Host "Para deshacerlo:  .\confiar-editor-windows.ps1 -Quitar"
