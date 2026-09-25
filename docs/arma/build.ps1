<#
    ASOT_Ctab build script.

    Packs every component folder under ..\addons into a .pbo using Bohemia's
    AddonBuilder (installed with Arma 3 Tools on Steam), and assembles a
    loadable @ASOT_Ctab mod folder next to this project.

    AddonBuilder derives each PBO's prefix from the virtual source path
    RELATIVE to the -project root.  Our script_mod.hpp defines PREFIX=ctab
    and MAINPREFIX=z, so COMPILE_FILE macros generate paths like
    \z\ctab\addons\core\XEH_preStart.sqf.  To match, every component
    (except ctab_main) is junctioned under C:\asot_vroot\z\ctab\addons\.
    ctab_main stays under C:\asot_vroot\z\asot_ctab\addons\main so that the
    #include "\z\asot_ctab\addons\main\script_mod.hpp" chain resolves at
    both build-time and runtime.
    ctab_compat_data is built from C:\asot_vroot\cTab which is a REAL
    directory (not a junction) so AddonBuilder can follow the img\ and
    shared\ sub-junctions inside it and pack them into the PBO.

    Usage:
        .\build.ps1                  # unsigned build (local/Eden testing)
        .\build.ps1 -Sign            # also sign with keys\ASOT_Ctab.biprivatekey
        .\build.ps1 -AddonBuilder "C:\path\to\AddonBuilder.exe"
#>
param(
    [string]$AddonBuilder = "",
    [switch]$Sign
)

$ErrorActionPreference = "Stop"

# Abort early if Arma 3 is running - it locks PBO files and the build cannot replace them.
if (Get-Process arma3_x64 -ErrorAction SilentlyContinue) {
    Write-Error "Arma 3 is running. Close it before rebuilding, then restart Arma 3 to pick up the new PBOs."
    exit 1
}

$root = Split-Path -Parent $PSScriptRoot
$sourceAddons = Join-Path $root "addons"
$releaseRoot = Join-Path $root "release\@ASOT_Ctab"
$releaseAddons = Join-Path $releaseRoot "addons"
$keysDir = Join-Path $root "keys"
$vroot = "C:\asot_vroot"
$depCache = "C:\asot_vroot_cba_temp"

if (-not $AddonBuilder) {
    $candidates = @(
        "C:\Program Files (x86)\Steam\steamapps\common\Arma 3 Tools\AddonBuilder\AddonBuilder.exe",
        "C:\Program Files\Steam\steamapps\common\Arma 3 Tools\AddonBuilder\AddonBuilder.exe",
        "E:\SteamLibrary\steamapps\common\Arma 3 Tools\AddonBuilder\AddonBuilder.exe",
        "G:\SteamLibrary\steamapps\common\Arma 3 Tools\AddonBuilder\AddonBuilder.exe"
    )
    $AddonBuilder = $candidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
}
if (-not $AddonBuilder -or -not (Test-Path -LiteralPath $AddonBuilder)) {
    Write-Error "AddonBuilder.exe not found. Install Arma 3 Tools from Steam, or pass -AddonBuilder <path>."
    exit 1
}
$toolsDirectory = Split-Path -Parent (Split-Path -Parent $AddonBuilder)
$cfgConvert = Join-Path $toolsDirectory "CfgConvert\CfgConvert.exe"
$pboConsoleCandidates = @(
    "C:\Program Files\PBO Manager\pboc.exe",
    "C:\Program Files\PBO Manager v.1.4 beta\PBOConsole.exe",
    "C:\Program Files (x86)\PBO Manager v.1.4 beta\PBOConsole.exe"
)
$pboConsole = $pboConsoleCandidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1

# ---- Virtual root: our own components ----
# Maps virtual path (relative to vroot) -> disk folder name under addons\.
# AddonBuilder derives the PBO prefix from the virtual path relative to -project,
# so the virtual path must match what COMPILE_FILE/QPATHTOF macros generate.
$componentMap = [ordered]@{
    # ctab_main stays at z\asot_ctab so #include "\z\asot_ctab\addons\main\..." resolves.
    "z\asot_ctab\addons\main"   = "ctab_main"
    # Base cTab addons under z\ctab\addons\ to match PREFIX=ctab in script_mod.hpp.
    "z\ctab\addons\core"        = "ctab_core"
    "z\ctab\addons\cTab"        = "ctab_cTab"
    "z\ctab\addons\compass"     = "ctab_compass"
    "z\ctab\addons\rangefinder" = "ctab_rangefinder"
    # ASOT extension addons under z\asot_ctab\addons\ to match PREFIX=asot_ctab in their script_component.hpp.
    "z\asot_ctab\addons\core_ext"    = "asot_ctab_core_ext"
    "z\asot_ctab\addons\camera"      = "asot_ctab_camera"
    "z\asot_ctab\addons\intel"       = "asot_ctab_intel"
    "z\asot_ctab\addons\messages"    = "asot_ctab_messages"
    "z\asot_ctab\addons\markers"     = "asot_ctab_markers"
    # NOTE: cTab compat PBO is handled separately below - it needs a real directory
    # at vroot level so AddonBuilder can follow its img\ and shared\ sub-junctions.
}

# Remove stale z\ctab\addons\* junctions for ASOT addons (old location, now moved to z\asot_ctab).
$staleFromCtabKeys = @("camera","core_ext","intel","markers","messages")
foreach ($key in $staleFromCtabKeys) {
    $stale = "$vroot\z\ctab\addons\$key"
    if (Test-Path $stale) {
        [System.IO.Directory]::Delete($stale)
        Write-Output "Removed stale junction: $stale"
    }
}

# Remove stale z\asot_ctab\addons\* junctions for base cTab addons (old location).
$staleFromAsotKeys = @("core","cTab","compass","rangefinder","ctab_compat_data")
foreach ($key in $staleFromAsotKeys) {
    $stale = "$vroot\z\asot_ctab\addons\$key"
    if (Test-Path $stale) {
        [System.IO.Directory]::Delete($stale)
        Write-Output "Removed stale junction: $stale"
    }
}

# Clean up any junctions previously created INSIDE the ctab_compat_data source folder
# (old approach put img/shared junctions there; new approach uses a real vroot directory).
$compatDataSrc = Join-Path $sourceAddons "ctab_compat_data"
foreach ($subDir in @("img", "shared")) {
    $staleInner = Join-Path $compatDataSrc $subDir
    if ((Test-Path $staleInner) -and (Get-Item $staleInner).LinkType -eq "Junction") {
        [System.IO.Directory]::Delete($staleInner)
        Write-Output "Removed stale inner junction from ctab_compat_data source: $staleInner"
    }
}

# Create directory structure and junctions in the virtual root.
foreach ($kv in $componentMap.GetEnumerator()) {
    $link   = Join-Path $vroot $kv.Key
    $target = Join-Path $sourceAddons $kv.Value
    if (-not (Test-Path $target)) { continue }
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $link) | Out-Null
    if (-not (Test-Path $link)) {
        New-Item -ItemType Junction -Path $link -Target $target | Out-Null
    }
}

# ---- cTab compat PBO: real directory with real file copies ----
# AddonBuilder only follows ONE level of junctions. Using sub-junctions inside
# the vroot causes the "copy to temp" step to produce an empty folder.
# Solution: robocopy img\ and shared\ from ctab_cTab into a real directory at
# C:\asot_vroot\cTab so AddonBuilder packs them as \cTab\img\* and \cTab\shared\*.
$ctabVroot = "$vroot\cTab"
$ctabSrc   = Join-Path $sourceAddons "ctab_cTab"

# Remove any old junction at this path (replaced by real directory)
if (Test-Path $ctabVroot) {
    $existing = Get-Item $ctabVroot
    if ($existing.LinkType -eq "Junction") {
        [System.IO.Directory]::Delete($ctabVroot)
        Write-Output "Replaced stale cTab junction with real directory."
    }
}
New-Item -ItemType Directory -Force -Path $ctabVroot | Out-Null

# Also remove any stale sub-junctions left inside the real dir from prior attempts
foreach ($subDir in @("img", "shared", "data")) {
    $subPath = Join-Path $ctabVroot $subDir
    if ((Test-Path $subPath) -and (Get-Item $subPath).LinkType -eq "Junction") {
        [System.IO.Directory]::Delete($subPath)
        Write-Output "Removed stale cTab/$subDir sub-junction."
    }
}

# Mirror img, shared, and data files from ctab_cTab source into the real cTab vroot.
# data\ contains itemDK10.p3d and its textures, referenced as \ctab\data\* in config.
foreach ($subDir in @("img", "shared", "data")) {
    $subSrc = Join-Path $ctabSrc $subDir
    $subDst = Join-Path $ctabVroot $subDir
    if (Test-Path $subSrc) {
        Write-Output "Mirroring ctab_cTab\$subDir -> cTab vroot..."
        robocopy $subSrc $subDst /MIR /NJH /NJS /NP /NDL /NS 2>&1 | Where-Object { $_ -match '\S' } | ForEach-Object { Write-Output "  $_" }
    }
}

# AddonBuilder requires a config.cpp to run. Create a minimal one in the cTab vroot.
# This produces a harmless CfgPatches entry that tells Arma the compat PBO exists.
$ctabConfig = @"
class CfgPatches {
    class ctab_compat_data {
        name = "ASOT cTab - Compat Data";
        units[] = {};
        weapons[] = {};
        requiredAddons[] = {};
    };
};
"@
Set-Content -Path "$ctabVroot\config.cpp" -Value $ctabConfig -Encoding ASCII

# ---- Virtual root: external dependencies (derapped from installed Workshop copies) ----
$externalDeps = @(
    @{ Path = "x\cba\addons\main"; Pbo = "C:\Program Files (x86)\Steam\steamapps\workshop\content\107410\450814997\addons\cba_main.pbo" },
    @{ Path = "z\ace\addons\main"; Pbo = "C:\Program Files (x86)\Steam\steamapps\workshop\content\107410\463939057\addons\ace_main.pbo" },
    @{ Path = "z\ace\addons\common"; Pbo = "C:\Program Files (x86)\Steam\steamapps\workshop\content\107410\463939057\addons\ace_common.pbo" },
    @{ Path = "z\ace\addons\interaction"; Pbo = "C:\Program Files (x86)\Steam\steamapps\workshop\content\107410\463939057\addons\ace_interaction.pbo" },
    @{ Path = "z\mts\addons\markers"; Pbo = "C:\Program Files (x86)\Steam\steamapps\workshop\content\107410\1508091616\addons\mts_markers.pbo" }
)
if ($pboConsole -and (Test-Path -LiteralPath $cfgConvert)) {
    foreach ($dep in $externalDeps) {
        $link = Join-Path $vroot $dep.Path
        if (Test-Path $link) { continue }
        if (-not (Test-Path -LiteralPath $dep.Pbo)) {
            Write-Warning "Dependency PBO not found, skipping: $($dep.Pbo)"
            continue
        }
        $cacheDir = Join-Path $depCache ([IO.Path]::GetFileNameWithoutExtension($dep.Pbo))
        if (-not (Test-Path "$cacheDir\config.cpp")) {
            New-Item -ItemType Directory -Force -Path $cacheDir | Out-Null
            & $pboConsole unpack $dep.Pbo -o $cacheDir | Out-Null
            Start-Sleep -Milliseconds 500
            if (Test-Path "$cacheDir\config.bin") {
                & $cfgConvert -txt -dst "$cacheDir\config.cpp" "$cacheDir\config.bin" | Out-Null
            }
        }
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $link) | Out-Null
        New-Item -ItemType Junction -Path $link -Target $cacheDir | Out-Null
    }
} else {
    Write-Warning "PBO Manager or CfgConvert not found - external dependency includes (CBA/ACE/mts_markers) won't resolve."
}

New-Item -ItemType Directory -Force -Path $releaseAddons | Out-Null

$privateKey = Join-Path $keysDir "ASOT_Ctab.biprivatekey"
if ($Sign -and -not (Test-Path -LiteralPath $privateKey)) {
    Write-Warning "No signing key found at $privateKey - building unsigned."
    $Sign = $false
}

$failed = @()
foreach ($kv in $componentMap.GetEnumerator()) {
    $componentName = $kv.Value
    $vSource = Join-Path $vroot $kv.Key
    Write-Output "Building $componentName..."
    $buildArgs = @($vSource, $releaseAddons, "-project=$vroot", "-toolsDirectory=$toolsDirectory")
    if ($Sign) {
        $buildArgs += "-sign=$privateKey"
    }
    # All components use -packonly so that script_mod.hpp (and all other HPP/SQF/SQFC
    # files) end up inside the PBO. binarize mode would process script_mod.hpp at
    # build time but NOT pack it, so every other config.cpp's runtime #include of
    # "\z\asot_ctab\addons\main\script_mod.hpp" would fail with "Include file not found".
    $buildArgs += "-packonly"
    $ErrorActionPreference = "Continue"
    & $AddonBuilder @buildArgs
    $ErrorActionPreference = "Stop"
    if ($LASTEXITCODE -ne 0) {
        Write-Warning "AddonBuilder failed on $componentName (exit $LASTEXITCODE)"
        $failed += $componentName
        continue
    }
    # AddonBuilder names the PBO after the leaf folder of the virtual source path.
    $leafName = Split-Path -Leaf $kv.Key
    $vpbo = Join-Path $releaseAddons "$leafName.pbo"
    $properName = Join-Path $releaseAddons "$componentName.pbo"
    if ((Test-Path $vpbo) -and ($vpbo -ne $properName)) {
        try {
            if (Test-Path $properName) { Remove-Item -LiteralPath $properName -Force -ErrorAction Stop }
            Move-Item -LiteralPath $vpbo -Destination $properName -ErrorAction Stop
        } catch {
            Write-Warning "Could not rename $leafName.pbo -> $componentName.pbo: $_"
            $failed += "$componentName (rename)"
        }
    }
}

# ---- Post-process: strip config.bin from ctab_main and all ASOT addons ----
# AddonBuilder's binarizer leaves cross-PBO class references (ASCT_*, etc.)
# unresolved in config.bin. Arma then can't find them at startup.
# Stripping config.bin forces Arma to load config.cpp at runtime, after ALL
# mods are merged, so ASCT_* from ctab_main's RscTitles resolves correctly.
# This is the same reason ctab_cTab.pbo (which has no config.bin) works fine.
$stripComponents = @("ctab_main", "asot_ctab_core_ext", "asot_ctab_camera", "asot_ctab_intel", "asot_ctab_messages", "asot_ctab_markers")
if ($pboConsole) {
    Write-Output "Post-processing: stripping config.bin from ASOT PBOs..."
    foreach ($comp in $stripComponents) {
        $compFailed = ($failed -contains $comp) -or ($failed -contains "$comp (rename)")
        if ($compFailed) { Write-Output "  $comp - skipped (build failed)"; continue }
        $compPbo = Join-Path $releaseAddons "$comp.pbo"
        if (-not (Test-Path $compPbo)) { Write-Output "  $comp.pbo not found - skipped"; continue }
        # Temp dir named after $comp so pboc.exe pack produces $comp.pbo automatically.
        $tempDir = Join-Path $env:TEMP $comp
        if (Test-Path $tempDir) { Remove-Item $tempDir -Recurse -Force }
        New-Item -ItemType Directory -Path $tempDir | Out-Null
        & $pboConsole unpack $compPbo -o $tempDir | Out-Null
        Start-Sleep -Milliseconds 500
        $binFile = Join-Path $tempDir "config.bin"
        if (Test-Path $binFile) {
            Remove-Item $binFile -Force
            Remove-Item $compPbo -Force
            & $pboConsole pack $tempDir -o $releaseAddons | Out-Null
            Start-Sleep -Milliseconds 500
            Write-Output "  $comp.pbo - config.bin stripped"
        } else {
            Write-Output "  $comp.pbo - no config.bin present"
        }
        Remove-Item $tempDir -Recurse -Force
    }
} else {
    Write-Warning "PBO Manager not found - cannot strip config.bin from ASOT PBOs. ASCT_* base classes may be undefined at runtime."
}

# ---- Build ctab_compat_data (the bare cTab-prefix PBO) ----
# Built from the real C:\asot_vroot\cTab directory that has img\ and shared\ sub-junctions.
Write-Output "Building ctab_compat_data (cTab prefix PBO)..."
# -packonly required: the data\ folder contains p3d models that crash binarize.exe.
$ctabBuildArgs = @($ctabVroot, $releaseAddons, "-project=$vroot", "-toolsDirectory=$toolsDirectory", "-packonly")
if ($Sign) { $ctabBuildArgs += "-sign=$privateKey" }
$ErrorActionPreference = "Continue"
& $AddonBuilder @ctabBuildArgs
$ErrorActionPreference = "Stop"
if ($LASTEXITCODE -ne 0) {
    Write-Warning "AddonBuilder failed on ctab_compat_data (exit $LASTEXITCODE)"
    $failed += "ctab_compat_data"
} else {
    # AddonBuilder names it after the leaf folder "cTab", rename to ctab_compat_data.pbo.
    $ctabPbo = Join-Path $releaseAddons "cTab.pbo"
    $compatPbo = Join-Path $releaseAddons "ctab_compat_data.pbo"
    if (Test-Path $ctabPbo) {
        try {
            if (Test-Path $compatPbo) { Remove-Item -LiteralPath $compatPbo -Force -ErrorAction Stop }
            Move-Item -LiteralPath $ctabPbo -Destination $compatPbo -ErrorAction Stop
        } catch {
            Write-Warning "Could not rename cTab.pbo -> ctab_compat_data.pbo: $_"
            $failed += "ctab_compat_data (rename)"
        }
    }
}

Copy-Item -LiteralPath (Join-Path $root "mod.cpp") -Destination $releaseRoot -Force
Copy-Item -LiteralPath (Join-Path $root "meta.cpp") -Destination $releaseRoot -Force
if (Test-Path -LiteralPath $keysDir) {
    Copy-Item -LiteralPath $keysDir -Destination $releaseRoot -Recurse -Force
}

Write-Output ""
if ($failed.Count -gt 0) {
    Write-Output "Build complete WITH FAILURES: $($failed -join ', ')"
} else {
    Write-Output "Build complete, all components succeeded: $releaseRoot"
}
if (-not $Sign) {
    Write-Output "NOTE: unsigned build - fine for local/Eden testing, but a verifySignatures dedicated server will reject it. Re-run with -Sign once a real key exists."
}
