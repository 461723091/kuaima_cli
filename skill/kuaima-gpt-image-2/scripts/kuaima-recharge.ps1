param(
    [bool]$PrintOnly = $false,
    [double]$Amount = 0,
    [int]$PlanId = 0,
    [string]$PaymentMethod = "custom1_wxpay",
    [bool]$QrImage = $true,
    [string]$QrFile = "",
    [string]$ApiKey = "",
    [string]$BaseUrl = "",
    [string]$Username = "",
    [string]$Password = "",
    [switch]$VerboseCli
)

$ErrorActionPreference = "Stop"

$skillRoot = Split-Path -Parent $PSScriptRoot
$exe = Join-Path $skillRoot "bin\kuaima_cli.exe"

if (-not (Test-Path -LiteralPath $exe)) {
    throw "Missing bundled CLI executable: $exe. Rebuild the skill package."
}

$argsList = @("recharge")

if ($PrintOnly) {
    $argsList += "-print-url"
}
if ($Amount -gt 0) {
    $argsList += @("-amount", $Amount.ToString([System.Globalization.CultureInfo]::InvariantCulture))
}
if ($PlanId -gt 0) {
    $argsList += @("-plan-id", $PlanId.ToString())
}
if ($PaymentMethod -ne "") {
    $argsList += @("-payment-method", $PaymentMethod)
}
if (-not $QrImage) {
    $argsList += @("-qr-image", "false")
}
if ($QrFile -ne "") {
    $argsList += @("-qr-file", $QrFile)
}
if ($ApiKey -ne "") {
    $argsList += @("-api-key", $ApiKey)
}
if ($BaseUrl -ne "") {
    $argsList += @("-base-url", $BaseUrl)
}
if ($Username -ne "") {
    $argsList += @("-username", $Username)
}
if ($Password -ne "") {
    $argsList += @("-password", $Password)
}
if ($VerboseCli) {
    $argsList += "-v"
}

& $exe @argsList
exit $LASTEXITCODE
