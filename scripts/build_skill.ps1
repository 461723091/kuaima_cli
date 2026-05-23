param(
    [string]$SkillName = "kuaima-gpt-image-2"
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$dist = Join-Path $root "dist"
$skillSource = Join-Path (Join-Path $root "skill") $SkillName
$skillOutput = Join-Path $dist $SkillName
$zipOutput = Join-Path $dist "$SkillName.zip"
$cliOutput = Join-Path $dist "kuaima_cli.exe"
$cliDarwinArm64 = Join-Path $dist "kuaima_cli-darwin-arm64"
$cliDarwinAmd64 = Join-Path $dist "kuaima_cli-darwin-amd64"

function Invoke-GoBuild {
    param(
        [string]$GoOS,
        [string]$GoArch,
        [string]$Output
    )

    $previousGoOS = $env:GOOS
    $previousGoArch = $env:GOARCH
    try {
        if ($GoOS -eq "") {
            Remove-Item Env:GOOS -ErrorAction SilentlyContinue
        }
        else {
            $env:GOOS = $GoOS
        }

        if ($GoArch -eq "") {
            Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
        }
        else {
            $env:GOARCH = $GoArch
        }

        go build -buildvcs=false -o $Output .
    }
    finally {
        if ($null -eq $previousGoOS) {
            Remove-Item Env:GOOS -ErrorAction SilentlyContinue
        }
        else {
            $env:GOOS = $previousGoOS
        }

        if ($null -eq $previousGoArch) {
            Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
        }
        else {
            $env:GOARCH = $previousGoArch
        }
    }
}

if (-not (Test-Path -LiteralPath $skillSource)) {
    throw "Skill source directory not found: $skillSource"
}

New-Item -ItemType Directory -Force -Path $dist | Out-Null

Push-Location $root
try {
    Invoke-GoBuild "" "" $cliOutput
    Invoke-GoBuild "darwin" "arm64" $cliDarwinArm64
    Invoke-GoBuild "darwin" "amd64" $cliDarwinAmd64
}
finally {
    Pop-Location
}

$distFull = [System.IO.Path]::GetFullPath($dist)
$skillOutputFull = [System.IO.Path]::GetFullPath($skillOutput)
if (-not $skillOutputFull.StartsWith($distFull + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing to remove a skill output outside dist: $skillOutputFull"
}

if (Test-Path -LiteralPath $skillOutputFull) {
    Remove-Item -LiteralPath $skillOutputFull -Recurse -Force
}
if (Test-Path -LiteralPath $zipOutput) {
    Remove-Item -LiteralPath $zipOutput -Force
}

Copy-Item -LiteralPath $skillSource -Destination $skillOutputFull -Recurse

$binDir = Join-Path $skillOutputFull "bin"
New-Item -ItemType Directory -Force -Path $binDir | Out-Null
Copy-Item -LiteralPath $cliOutput -Destination (Join-Path $binDir "kuaima_cli.exe") -Force
Copy-Item -LiteralPath $cliDarwinArm64 -Destination (Join-Path $binDir "kuaima_cli-darwin-arm64") -Force
Copy-Item -LiteralPath $cliDarwinAmd64 -Destination (Join-Path $binDir "kuaima_cli-darwin-amd64") -Force

Compress-Archive -LiteralPath $skillOutputFull -DestinationPath $zipOutput -Force

Write-Host "Skill package: $skillOutputFull"
Write-Host "Skill archive: $zipOutput"
