param(
    [Parameter(Mandatory = $true)]
    [string]$Prompt,

    [string]$OutputDir = ".\outputs",
    [string[]]$File = @(),
    [string]$Mask = "",
    [string]$Size = "auto",

    [ValidateSet("auto", "low", "medium", "high")]
    [string]$Quality = "auto",

    [ValidateRange(1, 10)]
    [int]$Count = 1,

    [ValidateSet("", "png", "jpeg", "webp")]
    [string]$Format = "",

    [ValidateRange(-1, 100)]
    [int]$Compression = -1,

    [ValidateSet("auto", "transparent", "opaque")]
    [string]$Background = "auto",

    [ValidateSet("", "auto", "low")]
    [string]$Moderation = "",

    [string]$ApiKey = "",
    [string]$BaseUrl = "",
    [switch]$VerboseCli
)

$ErrorActionPreference = "Stop"

$skillRoot = Split-Path -Parent $PSScriptRoot
$exe = Join-Path $skillRoot "bin\kuaima_cli.exe"

if (-not (Test-Path -LiteralPath $exe)) {
    throw "Missing bundled CLI executable: $exe. Rebuild the skill package."
}

$argsList = @(
    "image",
    "-image-model", "gpt-image-2",
    "-save-images", $OutputDir,
    "-image-size", $Size,
    "-image-quality", $Quality,
    "-image-count", $Count.ToString(),
    "-image-background", $Background
)

if ($Format -ne "") {
    $argsList += @("-image-output-format", $Format)
}
if ($Compression -ge 0) {
    $argsList += @("-image-output-compression", $Compression.ToString())
}
if ($Moderation -ne "") {
    $argsList += @("-image-moderation", $Moderation)
}
if ($Mask -ne "") {
    $argsList += @("-image-mask", $Mask)
}
if ($ApiKey -ne "") {
    $argsList += @("-api-key", $ApiKey)
}
if ($BaseUrl -ne "") {
    $argsList += @("-base-url", $BaseUrl)
}
if ($VerboseCli) {
    $argsList += "-v"
}
foreach ($item in $File) {
    if ($item -ne "") {
        $argsList += @("-file", $item)
    }
}

$argsList += $Prompt

& $exe @argsList
$exitCode = $LASTEXITCODE

exit $exitCode
