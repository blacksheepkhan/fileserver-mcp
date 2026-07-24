[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$RootPath = Split-Path -Parent $PSScriptRoot
$ValidationScript = Join-Path $PSScriptRoot 'Build-InputValidation.ps1'
$FixturePath = Join-Path `
    $RootPath `
    'internal\version\testdata\build-input-validation-fixtures.json'
$Errors = [System.Collections.Generic.List[string]]::new()
$ExitCode = 1

try {
    . $ValidationScript
    $Fixtures = Get-Content -LiteralPath $FixturePath -Raw |
        ConvertFrom-Json -Depth 10 -DateKind String
    if (
        $Fixtures.schema -cne
        'flashgate-build-input-validation-fixtures/v1'
    ) {
        throw "Unexpected fixture schema: $($Fixtures.schema)"
    }

    foreach ($Fixture in $Fixtures.semanticVersions.valid) {
        try {
            $Result = Get-FlashGateSemanticVersion -Value $Fixture.value
            if ($Result.FileVersion -cne $Fixture.fileVersion) {
                throw (
                    "Expected file version '$($Fixture.fileVersion)'; found " +
                    "'$($Result.FileVersion)'."
                )
            }
        }
        catch {
            $Errors.Add(
                "Valid SemVer '$($Fixture.value)' failed: $($_.Exception.Message)"
            )
        }
    }
    foreach ($Value in $Fixtures.semanticVersions.invalid) {
        try {
            $null = Get-FlashGateSemanticVersion -Value ([string]$Value)
            $Errors.Add("Invalid SemVer '$Value' unexpectedly passed.")
        }
        catch {
        }
    }

    foreach ($Fixture in $Fixtures.sourceDateEpoch.valid) {
        try {
            $Actual = ConvertFrom-FlashGateSourceDateEpoch `
                -Value $Fixture.value
            if ($Actual -cne $Fixture.sourceTime) {
                throw (
                    "Expected source time '$($Fixture.sourceTime)'; found " +
                    "'$Actual'."
                )
            }
        }
        catch {
            $Errors.Add(
                "Valid epoch '$($Fixture.value)' failed: $($_.Exception.Message)"
            )
        }
    }
    foreach ($Value in $Fixtures.sourceDateEpoch.invalid) {
        try {
            $null = ConvertFrom-FlashGateSourceDateEpoch -Value ([string]$Value)
            $Errors.Add("Invalid epoch '$Value' unexpectedly passed.")
        }
        catch {
        }
    }

    if ($Errors.Count -eq 0) {
        $ExitCode = 0
    }
}
catch {
    $Errors.Add($_.Exception.Message)
}
finally {
    [pscustomobject]@{
        Status       = if ($Errors.Count -eq 0) { 'PASS' } else { 'FAIL' }
        FixturePath  = $FixturePath
        WarningCount = 0
        ErrorCount   = $Errors.Count
        Warnings     = $null
        Errors       = if ($Errors.Count -gt 0) {
            $Errors -join [Environment]::NewLine
        } else {
            $null
        }
    } | Format-List
}

exit $ExitCode
