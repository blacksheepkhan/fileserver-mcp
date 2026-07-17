[CmdletBinding()]
param(
    [switch]$Quick,
    [switch]$RecordBaseline,
    [string]$OutputPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true
. (Join-Path $PSScriptRoot 'benchmark-window.ps1')

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$BuildDirectory = Join-Path $RepoRoot 'build'
$ServerBinary = Join-Path $BuildDirectory 'flashgate-mcp.exe'
$BenchmarkBinary = Join-Path $BuildDirectory 'flashgate-benchmark.exe'
$BudgetPath = Join-Path $RepoRoot 'benchmarks\budgets.json'
$Status = 'FAIL'
$WarningCount = 0
$FailureCount = 0
$NextAction = 'Inspect the reported failure.'
$ReportPath = $null
$FailureMessage = $null
$RunOutputPath = $null
$CandidateCreated = $false
$PerformanceContaminated = $false
$MeasurementWarning = $null

try {
    if ([string]::IsNullOrWhiteSpace($OutputPath)) {
        if ($RecordBaseline) {
            $OutputPath = Join-Path $RepoRoot 'benchmarks\baseline.windows-amd64.json'
        }
        else {
            $OutputPath = Join-Path $BuildDirectory 'benchmark-current.windows-amd64.json'
        }
    }
    elseif (-not [IO.Path]::IsPathRooted($OutputPath)) {
        $OutputPath = Join-Path $RepoRoot $OutputPath
    }

    if ($RecordBaseline) {
        Assert-BaselineMeasurementWindow | Out-Null
    }
    else {
        $PerformanceContaminated = (Get-EuropeViennaMeasurementWindowStatus).IsBlocked
    }

    $InitialStatus = @(& git status --porcelain --untracked-files=all)
    $WorkingTreeDirty = $InitialStatus.Count -gt 0
    if ($RecordBaseline -and $WorkingTreeDirty) {
        throw 'Refusing to record a versioned baseline from a dirty working tree.'
    }

    New-Item -ItemType Directory -Path $BuildDirectory -Force | Out-Null
    & go build -o $ServerBinary ./cmd/server 2>&1 | Out-Null
    & go build -o $BenchmarkBinary ./cmd/benchmark 2>&1 | Out-Null
    $Commit = (& git rev-parse HEAD).Trim()
    $RunOutputPath = if ($RecordBaseline) {
        Join-Path $BuildDirectory ('.benchmark-baseline-candidate-' + [guid]::NewGuid().ToString('N') + '.json')
    }
    else {
        $OutputPath
    }
    $CandidateCreated = $RecordBaseline

    $Arguments = @(
        '-binary', $ServerBinary,
        '-output', $RunOutputPath,
        '-commit', $Commit,
        '-budgets', $BudgetPath
    )
    if ($WorkingTreeDirty) {
        $Arguments += '-working-tree-dirty'
    }
    if ($Quick) {
        $Arguments += '-quick'
    }

    $BenchmarkOutput = @(& $BenchmarkBinary @Arguments 2>&1)
    $Result = Get-Content -LiteralPath $RunOutputPath -Raw | ConvertFrom-Json
    $WarningCount = @($Result.warnings).Count
    $FailureCount = [int]$Result.budget_evaluation.hard_failures
    if ($FailureCount -gt 0) {
        throw "Benchmark reported $FailureCount hard budget failure(s)."
    }

    if ($RecordBaseline) {
        $FinalStatus = @(& git status --porcelain --untracked-files=all)
        if ($FinalStatus.Count -gt 0) {
            throw 'Refusing final baseline recording because the working tree became dirty.'
        }
        Publish-BenchmarkBaselineCandidate -CandidatePath $RunOutputPath -DestinationPath $OutputPath
        $CandidateCreated = $false
    }
    elseif ((Get-EuropeViennaMeasurementWindowStatus).IsBlocked) {
        $PerformanceContaminated = $true
    }

    if ($PerformanceContaminated) {
        $MeasurementWarning = Get-ContaminatedPerformanceWarning
        $WarningCount++
    }

    $Status = 'PASS'
    $NextAction = if ($RecordBaseline) {
        'Review the versioned baseline diff before commit.'
    }
    else {
        'Compare the current result with the versioned baseline.'
    }
}
catch {
    $FailureCount = [Math]::Max(1, $FailureCount)
    $FailureMessage = $_.Exception.Message
}
finally {
    if ($CandidateCreated -and -not [string]::IsNullOrWhiteSpace($RunOutputPath) -and (Test-Path -LiteralPath $RunOutputPath -PathType Leaf)) {
        Remove-Item -LiteralPath $RunOutputPath -Force
    }
    $ReportPath = if ([string]::IsNullOrWhiteSpace($OutputPath)) { '[not-created]' } else { $OutputPath }
    [pscustomobject]@{
        Status         = $Status
        ReportPath     = $ReportPath
        WarningCount   = $WarningCount
        FailureCount   = $FailureCount
        NextAction     = $NextAction
        MeasurementWarning = $MeasurementWarning
        FailureMessage = $FailureMessage
    } | Format-List
}

if ($FailureCount -gt 0) {
    exit 1
}
