# Armstrong installer for Windows. In PowerShell:
#
#   irm https://raw.githubusercontent.com/ark4660/armstrong/main/scripts/install.ps1 | iex
#
# Installs the latest launcher to %LOCALAPPDATA%\Programs\Armstrong (no admin
# needed, so the launcher can update itself), adds it to your PATH so
# "armstrong" works in any new terminal, and adds a Start Menu shortcut.
# Run again to reinstall. To remove it:
#
#   $env:ARMSTRONG_UNINSTALL = 1; irm https://raw.githubusercontent.com/ark4660/armstrong/main/scripts/install.ps1 | iex

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue' # the progress bar makes downloads very slow in Windows PowerShell
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$Repo = 'ark4660/armstrong'
$Releases = if ($env:ARMSTRONG_RELEASES_URL) { $env:ARMSTRONG_RELEASES_URL } else { "https://github.com/$Repo/releases/latest/download" }
$Assets = if ($env:ARMSTRONG_ASSETS_URL) { $env:ARMSTRONG_ASSETS_URL } else { "https://raw.githubusercontent.com/$Repo/main/assets" }
$Dir = if ($env:ARMSTRONG_INSTALL_DIR) { $env:ARMSTRONG_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\Armstrong' }
$Exe = Join-Path $Dir 'armstrong.exe'
$Icon = Join-Path $Dir 'armstrong.ico'
$Shortcut = Join-Path ([Environment]::GetFolderPath('Programs')) 'Armstrong.lnk'

function Get-UserPath { [Environment]::GetEnvironmentVariable('Path', 'User') }
function Set-UserPath([string]$Value) { [Environment]::SetEnvironmentVariable('Path', $Value, 'User') }
function Test-OnPath([string]$Path, [string]$Folder) {
    ($Path -split ';' | Where-Object { $_.TrimEnd('\') -ieq $Folder.TrimEnd('\') }).Count -gt 0
}

if ($env:ARMSTRONG_UNINSTALL -eq '1') {
    Remove-Item -Force -ErrorAction SilentlyContinue $Exe, "$Exe.old", "$Exe.new", $Icon, $Shortcut
    $kept = ((Get-UserPath) -split ';' | Where-Object { $_ -and $_.TrimEnd('\') -ine $Dir.TrimEnd('\') }) -join ';'
    Set-UserPath $kept
    Remove-Item -ErrorAction SilentlyContinue $Dir
    Write-Host 'Armstrong is uninstalled.'
    Write-Host "Your instances and settings are still in $env:APPDATA\armstrong (delete that folder to remove them)."
    return
}

$arch = if ($env:PROCESSOR_ARCHITEW6432 -eq 'ARM64' -or $env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
if (-not [Environment]::Is64BitOperatingSystem) { throw 'Armstrong needs 64-bit Windows.' }
$name = "armstrong-windows-$arch.exe"

Write-Host "Downloading Armstrong for windows/$arch..."
$index = Invoke-RestMethod -UseBasicParsing "$Releases/release.json"
$asset = $index.launcher."windows-$arch"
if (-not $asset) { throw "The latest release has no build for windows/$arch." }

New-Item -ItemType Directory -Force -Path $Dir | Out-Null
$tmp = Join-Path $Dir "$name.download"
Invoke-WebRequest -UseBasicParsing -Uri $asset.url -OutFile $tmp
$got = (Get-FileHash -Algorithm SHA256 -Path $tmp).Hash.ToLowerInvariant()
if ($got -ne $asset.sha256) {
    Remove-Item -Force $tmp
    throw "Checksum mismatch (got $got, want $($asset.sha256)); not installing."
}
try {
    Move-Item -Force -Path $tmp -Destination $Exe
} catch {
    Remove-Item -Force -ErrorAction SilentlyContinue $tmp
    throw 'Couldn''t replace armstrong.exe: close Armstrong and run the installer again.'
}
Write-Host "Installed Armstrong $($index.version) to $Exe"

try {
    Invoke-WebRequest -UseBasicParsing -Uri "$Assets/armstrong.ico" -OutFile $Icon
} catch {
    Write-Host '(couldn''t download the icon; the shortcut will use the default one)'
}

# Make "armstrong" a command in new terminals.
$userPath = Get-UserPath
if (-not (Test-OnPath $userPath $Dir)) {
    Set-UserPath ((@($userPath, $Dir) | Where-Object { $_ }) -join ';')
    Write-Host 'Added Armstrong to your PATH'
}
if (-not (Test-OnPath $env:Path $Dir)) { $env:Path = "$env:Path;$Dir" }

# Start Menu shortcut. Windows Terminal renders the TUI best; fall back to the console.
$shell = New-Object -ComObject WScript.Shell
$lnk = $shell.CreateShortcut($Shortcut)
$wt = Get-Command wt.exe -ErrorAction SilentlyContinue
if ($wt) {
    $lnk.TargetPath = $wt.Source
    $lnk.Arguments = "--title Armstrong `"$Exe`""
} else {
    $lnk.TargetPath = $Exe
}
$lnk.WorkingDirectory = $env:USERPROFILE
if (Test-Path $Icon) { $lnk.IconLocation = $Icon }
$lnk.Description = 'Terminal Minecraft launcher and performance client'
$lnk.Save()
Write-Host 'Added Armstrong to the Start Menu'

Write-Host ''
Write-Host 'Done. Open Armstrong from the Start Menu, or open a new terminal and run: armstrong'
