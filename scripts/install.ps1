[CmdletBinding()]
param(
  [string]$Owner = "3leaps",
  [string]$Repo = "goneat"
)

$ErrorActionPreference = "Stop"
$Project = "goneat"

function Get-OsArch {
  $arch = $env:PROCESSOR_ARCHITECTURE
  switch ($arch) {
    "AMD64" { return "amd64" }
    "ARM64" { return "arm64" }
    default { throw "Unsupported architecture: $arch" }
  }
}

$arch = Get-OsArch
$tag = (Invoke-RestMethod -Uri "https://api.github.com/repos/$Owner/$Repo/releases/latest").tag_name
if (-not $tag) { throw "Failed to resolve latest release tag" }
$version = $tag.TrimStart('v')
$asset = "${Project}_${version}_windows_${arch}.zip"
$base = "https://github.com/$Owner/$Repo/releases/download/$tag"
$tmp = New-Item -ItemType Directory -Path ([System.IO.Path]::GetTempPath()) -Name ("goneat_" + [System.Guid]::NewGuid())

Invoke-WebRequest -Uri "$base/$asset" -OutFile "$tmp/$asset"
Invoke-WebRequest -Uri "$base/SHA256SUMS" -OutFile "$tmp/SHA256SUMS"

function Get-FileHashHex($path) {
  (Get-FileHash -Path $path -Algorithm SHA256).Hash.ToLower()
}

$expected = (Select-String -Path "$tmp/SHA256SUMS" -Pattern " $asset$" | ForEach-Object { ($_ -split ' ')[0] })
$actual = Get-FileHashHex "$tmp/$asset"
if ($expected -ne $actual) { throw "Checksum mismatch" }

Add-Type -AssemblyName System.IO.Compression.FileSystem
[System.IO.Compression.ZipFile]::ExtractToDirectory("$tmp/$asset", "$tmp")

$dest = Join-Path $env:LOCALAPPDATA "Programs\$Project"
New-Item -ItemType Directory -Force -Path $dest | Out-Null
Copy-Item "$tmp\$Project.exe" "$dest\$Project.exe" -Force

$path = [Environment]::GetEnvironmentVariable("PATH", "User")
if (-not $path.ToLower().Contains($dest.ToLower())) {
  [Environment]::SetEnvironmentVariable("PATH", "$path;$dest", "User")
  Write-Host "Added $dest to PATH. Restart your terminal to use $Project."
}

Write-Host "Installed $Project to $dest"
