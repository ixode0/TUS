<#
.SYNOPSIS
  TUS installer for Windows (Telegram Username Sniper).
.EXAMPLE
  irm https://raw.githubusercontent.com/ixode0/TUS/main/install.ps1 | iex
  & $env:TEMP\tus-install.ps1 -ApiId 123 -ApiHash abc -Phone +1234567890 -Usernames user1,user2
#>
param(
  [string]$ApiId = $env:API_ID,
  [string]$ApiHash = $env:API_HASH,
  [string]$Phone = $env:PHONE,
  [string]$Usernames = $env:USERNAMES,
  [string]$ClaimTo = "channel",
  [int]$SleepMs = 100,
  [string]$Version = "latest",
  [string]$InstallDir = (Join-Path $env:LOCALAPPDATA "TUS"),
  [switch]$Uninstall, [switch]$Help
)
$Repo = "ixode0/TUS"
if ($Help) {
  Write-Host "TUS installer. Params: -ApiId -ApiHash -Phone -Usernames (comma-separated) -ClaimTo channel|user -SleepMs -Version -InstallDir -Uninstall"
  exit 0
}
$ErrorActionPreference = "Stop"

$arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { throw "32-bit Windows is not supported" }
$asset = "sniper-windows-$arch.exe"
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

if ($Uninstall) {
  Remove-Item (Join-Path $InstallDir "sniper.exe") -Force -ErrorAction SilentlyContinue
  Write-Host "[tus-install] binaries removed (config kept at $InstallDir — delete manually if needed)"
  exit 0
}

if ($Version -eq "latest") {
  Write-Host "[tus-install] resolving latest release..."
  $Version = (Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest").tag_name
  if (-not $Version) { throw "cannot resolve latest version (pass -Version vX.Y.Z)" }
}
Write-Host "[tus-install] installing TUS $Version (windows/$arch)"
$base = "https://github.com/$Repo/releases/download/$Version"
$tmp = Join-Path $env:TEMP ("tus-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
try {
  Invoke-WebRequest "$base/$asset" -OutFile (Join-Path $tmp $asset)
  Invoke-WebRequest "$base/SHA256SUMS.txt" -OutFile (Join-Path $tmp "SHA256SUMS.txt")
  $want = ((Get-Content (Join-Path $tmp "SHA256SUMS.txt") | Where-Object { $_ -match " $asset`$" }) -split ' ')[0]
  $got = (Get-FileHash (Join-Path $tmp $asset) -Algorithm SHA256).Hash.ToLower()
  if ($got -ne $want.ToLower()) { throw "checksum mismatch for $asset" }
  Copy-Item (Join-Path $tmp $asset) (Join-Path $InstallDir "sniper.exe") -Force
} finally { Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue }

# PATH (user scope)
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$InstallDir*") {
  [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
  Write-Host "[tus-install] added $InstallDir to user PATH (reopen terminal)"
}

# config
$config = Join-Path $InstallDir "config.json"
if (-not (Test-Path $config)) {
  if (-not $ApiId -or -not $ApiHash -or -not $Phone -or -not $Usernames) {
    Write-Host "[tus-install] writing template config — edit it before running: $config"
    @'
{
  "telegram": {"phone_number": "", "api_id": 0, "api_hash": ""},
  "claim_to": "channel",
  "sleep_between_check": 100,
  "usernames": ["your_username"]
}
'@ | Out-File $config -Encoding utf8
  } else {
    if (-not $Phone.StartsWith("+")) { throw "phone must start with + (e.g. +1234567890)" }
    $list = ($Usernames -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ } | ForEach-Object { '"' + $_ + '"' }) -join ','
    "{`n  `"telegram`": {`"phone_number`": `"$Phone`", `"api_id`": $ApiId, `"api_hash`": `"$ApiHash`"},`n  `"claim_to`": `"$ClaimTo`",`n  `"sleep_between_check`": $SleepMs,`n  `"usernames`": [$list]`n}" | Out-File $config -Encoding utf8
    Write-Host "[tus-install] config written to $config"
  }
}
Write-Host "[tus-install] done. Run: sniper.exe   (first launch asks for Telegram login code)"
