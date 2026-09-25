# Builds the TasDyn-ALife extension (tasdyn_alife_x64.dll).
#
# Locates the Visual Studio x64 build tools and libpq (via pg_config, which
# ships with any PostgreSQL server install) automatically -- no vcpkg/cmake
# needed. Run from anywhere; paths are resolved relative to this script.

$ErrorActionPreference = "Stop"

$root = $PSScriptRoot
$srcDir = Join-Path $root "src"
$buildDir = Join-Path $root "build"
New-Item -ItemType Directory -Force -Path $buildDir | Out-Null

# --- Locate Visual Studio's x64 C++ build tools ---
$vswhere = "${env:ProgramFiles(x86)}\Microsoft Visual Studio\Installer\vswhere.exe"
if (-not (Test-Path $vswhere)) {
    throw "vswhere.exe not found at '$vswhere' -- is Visual Studio installed?"
}

$vsInstallPath = & $vswhere -latest -products '*' `
    -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 `
    -property installationPath
if (-not $vsInstallPath) {
    throw "No Visual Studio install with the 'C++ x64 build tools' component found."
}

$vcvars = Join-Path $vsInstallPath "VC\Auxiliary\Build\vcvars64.bat"
if (-not (Test-Path $vcvars)) {
    throw "vcvars64.bat not found at '$vcvars'."
}

Write-Host "Using Visual Studio at: $vsInstallPath"

# Import vcvars64's environment (INCLUDE/LIB/PATH for cl.exe) into this
# PowerShell session -- cl.exe doesn't resolve any of that on its own.
$envDump = cmd /c "`"$vcvars`" >nul 2>&1 && set"
foreach ($line in $envDump) {
    if ($line -match '^([^=]+)=(.*)$') {
        Set-Item -Path "Env:$($Matches[1])" -Value $Matches[2]
    }
}

# --- Locate libpq (ships with the PostgreSQL server install) ---
$pgConfigPath = $null
$cmd = Get-Command pg_config.exe -ErrorAction SilentlyContinue
if ($cmd) {
    $pgConfigPath = $cmd.Source
} else {
    $candidates = Get-ChildItem "C:\Program Files\PostgreSQL" -Directory -ErrorAction SilentlyContinue |
        Sort-Object Name -Descending
    foreach ($c in $candidates) {
        $candidate = Join-Path $c.FullName "bin\pg_config.exe"
        if (Test-Path $candidate) {
            $pgConfigPath = $candidate
            break
        }
    }
}
if (-not $pgConfigPath) {
    throw "pg_config.exe not found -- install PostgreSQL, or put pg_config.exe on PATH."
}

$pgInclude = (& $pgConfigPath --includedir).Trim()
$pgLib = (& $pgConfigPath --libdir).Trim()

Write-Host "Using libpq include: $pgInclude"
Write-Host "Using libpq lib:     $pgLib"

# --- Compile ---
$sources = Get-ChildItem (Join-Path $srcDir "*.cpp") | ForEach-Object { $_.FullName }
$outputDll = Join-Path $buildDir "tasdyn_alife_x64.dll"

Write-Host "Compiling:`n  $($sources -join "`n  ")"

Push-Location $buildDir
try {
    $clArgs = @(
        "/nologo", "/EHsc", "/std:c++17", "/W4", "/MD",
        "/I$pgInclude"
    ) + $sources + @(
        "/LD", "/Fe:$outputDll",
        "/link", "/LIBPATH:$pgLib", "libpq.lib"
    )

    & cl.exe @clArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Build failed (cl.exe exit code $LASTEXITCODE)"
    }
} finally {
    Pop-Location
}

# --- Copy runtime dependencies next to the built DLL ---
# libpq.dll's own dependency chain (verified via `dumpbin /dependents`,
# transitively -- libintl-9.dll pulls in libwinpthread-1.dll/libiconv-2.dll
# a level deeper than libpq.dll's own direct imports show). All system
# DLLs (KERNEL32, WS2_32, the api-ms-win-crt-* apiset stubs, ...) are assumed
# present on any Windows 10/11 host and aren't copied.
$pgBin = Join-Path (Split-Path $pgLib -Parent) "bin"
$runtimeDeps = @(
    "libpq.dll", "libssl-3-x64.dll", "libcrypto-3-x64.dll",
    "libintl-9.dll", "libwinpthread-1.dll", "libiconv-2.dll"
)
foreach ($dep in $runtimeDeps) {
    $source = Join-Path $pgBin $dep
    if (Test-Path $source) {
        Copy-Item $source $buildDir -Force
    } else {
        Write-Warning "Runtime dependency '$dep' not found at '$source' -- the built DLL may fail to load."
    }
}

Write-Host "`nBuilt: $outputDll"
Write-Host "Runtime dependencies copied to $buildDir."
