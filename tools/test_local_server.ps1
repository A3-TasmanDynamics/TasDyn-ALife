# Deploys the mission + C++ extension to a local Arma 3 Server install,
# launches it headless, and reports whether the mission's config
# (description.ext, CfgFunctions.hpp, CfgRemoteExec.hpp, ...) parsed without
# errors. Cleans up everything it copies afterward -- this never leaves
# files behind in the Arma 3 Server install, which is a Steam-managed
# directory outside this repo, not something to permanently modify.
#
# This does NOT test a real player joining (load/save/spawn) -- that needs
# an interactive client connecting, which this script can't drive. It's a
# config-parses-cleanly smoke test, the thing most likely to silently break
# from a typo in a .hpp/.ext file with no other way to catch it early.
#
# Usage:
#   .\tools\test_local_server.ps1 -ArmaServerPath "G:\SteamLibrary\steamapps\common\Arma 3 Server"
#
# Prerequisites: src/cpp_extension/build.ps1 already run, and a real
# config.ini at the repo root (copied from config.ini.example).

param(
    [Parameter(Mandatory = $true)]
    [string]$ArmaServerPath,

    [int]$WaitSeconds = 30
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path $PSScriptRoot -Parent
$extensionBuild = Join-Path $repoRoot "src\cpp_extension\build"
$missionSrc = Join-Path $repoRoot "src\ALife.Altis"

if (-not (Test-Path (Join-Path $extensionBuild "tasdyn_alife_x64.dll"))) {
    throw "Extension not built -- run src/cpp_extension/build.ps1 first."
}
if (-not (Test-Path (Join-Path $repoRoot "config.ini"))) {
    throw "config.ini not found at repo root -- copy config.ini.example and fill in real values first."
}
if (-not (Test-Path (Join-Path $ArmaServerPath "arma3server_x64.exe"))) {
    throw "arma3server_x64.exe not found under '$ArmaServerPath' -- check -ArmaServerPath."
}

$missionDest = Join-Path $ArmaServerPath "mpmissions\ALife.Altis"
$serverCfgPath = Join-Path $ArmaServerPath "server.cfg"
$profileDir = Join-Path $env:TEMP "alife_test_profile_$(Get-Random)"
$deployedFiles = @()
$proc = $null

try {
    # --- Deploy ---
    if (Test-Path $missionDest) { Remove-Item -Recurse -Force $missionDest }
    Copy-Item -Recurse $missionSrc $missionDest

    $runtimeDeps = @(
        "tasdyn_alife_x64.dll", "libpq.dll", "libssl-3-x64.dll",
        "libcrypto-3-x64.dll", "libintl-9.dll", "libwinpthread-1.dll", "libiconv-2.dll"
    )
    foreach ($dep in $runtimeDeps) {
        $dest = Join-Path $ArmaServerPath $dep
        Copy-Item (Join-Path $extensionBuild $dep) $dest -Force
        $deployedFiles += $dest
    }
    $configDest = Join-Path $ArmaServerPath "config.ini"
    Copy-Item (Join-Path $repoRoot "config.ini") $configDest -Force
    $deployedFiles += $configDest

    # TEMPORARY dev-test config -- verifySignatures/BattlEye disabled to
    # smoke-test an unpacked, unsigned local mission folder without
    # fighting signature checks. Production config MUST enable BattlEye
    # per docs/ANTI_CHEAT.md Layer 0 -- this is not that config.
    @"
hostname = "TasDyn-ALife Dev Test";
password = "";
passwordAdmin = "admin123";
serverCommandPassword = "admin123";
maxPlayers = 4;
persistent = 1;
verifySignatures = 0;
allowedFilePatching = 2;
BattlEye = 0;
kickDuplicate = 1;

class Missions
{
    class ALifeTest
    {
        template = "ALife.Altis";
        difficulty = "Custom";
    };
};
difficulty = "Custom";
"@ | Set-Content -Path $serverCfgPath -Encoding ASCII
    $deployedFiles += $serverCfgPath

    New-Item -ItemType Directory -Force -Path $profileDir | Out-Null

    # --- Run ---
    Write-Host "Launching arma3server_x64.exe..."
    $proc = Start-Process -FilePath (Join-Path $ArmaServerPath "arma3server_x64.exe") `
        -ArgumentList "-config=server.cfg", "-profiles=$profileDir", "-name=alife_test", "-port=2302", "-noSound" `
        -WorkingDirectory $ArmaServerPath -PassThru -WindowStyle Hidden

    Write-Host "Waiting $WaitSeconds seconds for mission init..."
    Start-Sleep -Seconds $WaitSeconds

    $stillRunning = -not $proc.HasExited
    Write-Host "Server still running: $stillRunning"

    # --- Report ---
    $rpt = Get-ChildItem $profileDir -Filter "*.rpt" -Recurse | Select-Object -First 1
    if ($rpt) {
        Write-Host "`n--- RPT: errors/warnings ---"
        Select-String -Path $rpt.FullName -Pattern "error|warning|cannot|fail" |
            ForEach-Object { Write-Host $_.Line }

        Write-Host "`n--- RPT: [ALife] lines ---"
        $alifeLines = Select-String -Path $rpt.FullName -Pattern "\[ALife\]"
        if ($alifeLines) {
            $alifeLines | ForEach-Object { Write-Host $_.Line }
        } else {
            Write-Host "(none -- expected unless a real player connected; this script can't drive a client)"
        }

        Write-Host "`nFull log: $($rpt.FullName)"
    } else {
        Write-Warning "No RPT log found in $profileDir"
    }
} finally {
    # --- Clean up: never leave files in the Arma 3 Server install ---
    if ($proc -and -not $proc.HasExited) {
        Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    }
    if (Test-Path $missionDest) { Remove-Item -Recurse -Force $missionDest }
    foreach ($f in $deployedFiles) {
        if (Test-Path $f) { Remove-Item -Force $f }
    }
    Write-Host "`nCleaned up deployed files from '$ArmaServerPath'."
    Write-Host "RPT profile dir left at $profileDir for inspection (not auto-deleted)."
}
