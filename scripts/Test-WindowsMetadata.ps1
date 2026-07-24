[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string] $BinaryPath,

    [Parameter(Mandatory)]
    [string] $ExpectedProductVersion,

    [Parameter(Mandatory)]
    [string] $ExpectedFileVersion,

    [Parameter(Mandatory)]
    [ValidateSet('x64', 'arm64')]
    [string] $ExpectedPublicArch,

    [Parameter(Mandatory)]
    [ValidateSet('amd64', 'arm64')]
    [string] $ExpectedGOARCH,

    [Parameter(Mandatory)]
    [ValidatePattern('^[0-9a-f]{40}$')]
    [string] $ExpectedCommit,

    [Parameter(Mandatory)]
    [string] $ExpectedSourceTime,

    [Parameter(Mandatory)]
    [ValidateSet('true', 'false')]
    [string] $ExpectedModified,

    [Parameter(Mandatory)]
    [ValidateRange(1970, 9999)]
    [int] $ExpectedCopyrightYear
)

$ErrorActionPreference = 'Stop'
$RootPath = Split-Path -Parent $PSScriptRoot
$ExpectedIconPath = Join-Path $RootPath 'assets\branding\flashgate.ico'
$InputValidationScript = Join-Path `
    $RootPath `
    'scripts\Build-InputValidation.ps1'

$Warnings = [System.Collections.Generic.List[string]]::new()
$Errors = [System.Collections.Generic.List[string]]::new()
$ExitCode = 1

$Result = [ordered]@{
    Status                 = 'FAIL'
    BinaryPath             = $BinaryPath
    ExpectedProductVersion = $ExpectedProductVersion
    ExpectedFileVersion    = $ExpectedFileVersion
    ExpectedPublicArch     = $ExpectedPublicArch
    FileDescription        = $null
    FileVersion            = $null
    ProductName            = $null
    ProductVersion         = $null
    CompanyName            = $null
    LegalCopyright         = $null
    OriginalFilename       = $null
    InternalName           = $null
    Comments               = $null
    Machine                = $null
    IconFrameCount         = $null
    IconFrameIdentity      = $null
    WarningCount           = 0
    ErrorCount             = 0
    Warnings               = $null
    Errors                 = $null
}

function Invoke-ProcessRequired {
    param(
        [Parameter(Mandatory)]
        [string] $FilePath,

        [Parameter(Mandatory)]
        [string[]] $Arguments,

        [Parameter(Mandatory)]
        [string] $WorkingDirectory
    )

    $StartInfo = [Diagnostics.ProcessStartInfo]::new()
    $StartInfo.FileName = $FilePath
    $StartInfo.WorkingDirectory = $WorkingDirectory
    $StartInfo.UseShellExecute = $false
    $StartInfo.RedirectStandardOutput = $true
    $StartInfo.RedirectStandardError = $true
    $StartInfo.CreateNoWindow = $true
    foreach ($Argument in $Arguments) {
        $null = $StartInfo.ArgumentList.Add($Argument)
    }

    $Process = [Diagnostics.Process]::new()
    $Process.StartInfo = $StartInfo
    try {
        if (-not $Process.Start()) {
            throw "Unable to start process: $FilePath"
        }
        $Output = $Process.StandardOutput.ReadToEnd()
        $ErrorOutput = $Process.StandardError.ReadToEnd()
        $Process.WaitForExit()
        $ProcessExitCode = $Process.ExitCode
    }
    finally {
        $Process.Dispose()
    }
    if ($ProcessExitCode -ne 0) {
        throw (
            "$FilePath $($Arguments -join ' ') failed with exit code " +
            "${ProcessExitCode}: $ErrorOutput $Output"
        )
    }
    return $Output
}

function Assert-Equal {
    param(
        [Parameter(Mandatory)]
        [string] $Name,

        [AllowNull()]
        [object] $Actual,

        [AllowNull()]
        [object] $Expected
    )

    if ([string]$Actual -cne [string]$Expected) {
        $Errors.Add(
            "$Name mismatch. Expected '$Expected'; found '$Actual'."
        )
    }
}

try {
    if (-not (Test-Path -LiteralPath $BinaryPath -PathType Leaf)) {
        throw "Binary not found: $BinaryPath"
    }

    $ResolvedBinaryPath = (Resolve-Path -LiteralPath $BinaryPath).Path
    if (-not (Test-Path -LiteralPath $ExpectedIconPath -PathType Leaf)) {
        throw "Canonical icon not found: $ExpectedIconPath"
    }
    if (-not (Test-Path -LiteralPath $InputValidationScript -PathType Leaf)) {
        throw "Input validation helper not found: $InputValidationScript"
    }
    . $InputValidationScript
    $VersionIdentity = Get-FlashGateSemanticVersion `
        -Value $ExpectedProductVersion
    if ($VersionIdentity.FileVersion -cne $ExpectedFileVersion) {
        throw (
            "Expected product/file version mapping is inconsistent: " +
            "$ExpectedProductVersion -> $ExpectedFileVersion"
        )
    }
    $MappedGOARCH = if ($ExpectedPublicArch -eq 'x64') {
        'amd64'
    } else {
        'arm64'
    }
    if ($ExpectedGOARCH -cne $MappedGOARCH) {
        throw (
            "Architecture mapping mismatch. '$ExpectedPublicArch' requires " +
            "'$MappedGOARCH', not '$ExpectedGOARCH'."
        )
    }
    $ParsedSourceTime = [DateTimeOffset]::MinValue
    if (
        -not [DateTimeOffset]::TryParseExact(
            $ExpectedSourceTime,
            'yyyy-MM-ddTHH:mm:ssZ',
            [Globalization.CultureInfo]::InvariantCulture,
            [Globalization.DateTimeStyles]::AssumeUniversal,
            [ref]$ParsedSourceTime
        )
    ) {
        throw "Expected source time is not canonical RFC3339 UTC: $ExpectedSourceTime"
    }

    $Stream = [IO.File]::OpenRead($ResolvedBinaryPath)
    try {
        $Reader = [IO.BinaryReader]::new($Stream)
        try {
            if ($Reader.ReadUInt16() -ne 0x5A4D) {
                throw 'Binary does not contain a valid DOS MZ header.'
            }

            $Stream.Position = 0x3C
            $PEOffset = $Reader.ReadInt32()
            $Stream.Position = $PEOffset

            if ($Reader.ReadUInt32() -ne 0x00004550) {
                throw 'Binary does not contain a valid PE signature.'
            }

            $Machine = $Reader.ReadUInt16()
        }
        finally {
            $Reader.Dispose()
        }
    }
    finally {
        $Stream.Dispose()
    }

    $ExpectedMachine = if ($ExpectedPublicArch -eq 'x64') {
        0x8664
    }
    else {
        0xAA64
    }

    if ($Machine -ne $ExpectedMachine) {
        $Errors.Add(
            ('PE machine mismatch. Expected 0x{0:X4}; found 0x{1:X4}.' -f
                $ExpectedMachine,
                $Machine)
        )
    }

    $Result.Machine = '0x{0:X4}' -f $Machine

    $VersionInfo = [System.Diagnostics.FileVersionInfo]::GetVersionInfo(
        $ResolvedBinaryPath
    )

    $ExpectedCopyright =
        "Copyright © $ExpectedCopyrightYear Thomas Weidner"

    Assert-Equal `
        -Name 'FileDescription' `
        -Actual $VersionInfo.FileDescription `
        -Expected 'FlashGate MCP Server'

    Assert-Equal `
        -Name 'FileVersion' `
        -Actual $VersionInfo.FileVersion `
        -Expected $ExpectedFileVersion

    Assert-Equal `
        -Name 'ProductName' `
        -Actual $VersionInfo.ProductName `
        -Expected 'FlashGate MCP'

    Assert-Equal `
        -Name 'ProductVersion' `
        -Actual $VersionInfo.ProductVersion `
        -Expected $ExpectedProductVersion

    Assert-Equal `
        -Name 'CompanyName' `
        -Actual $VersionInfo.CompanyName `
        -Expected 'Thomas Weidner'

    Assert-Equal `
        -Name 'LegalCopyright' `
        -Actual $VersionInfo.LegalCopyright `
        -Expected $ExpectedCopyright

    Assert-Equal `
        -Name 'OriginalFilename' `
        -Actual $VersionInfo.OriginalFilename `
        -Expected 'flashgate-mcp.exe'

    Assert-Equal `
        -Name 'InternalName' `
        -Actual $VersionInfo.InternalName `
        -Expected 'flashgate-mcp'

    Assert-Equal `
        -Name 'Comments' `
        -Actual $VersionInfo.Comments `
        -Expected 'Native Model Context Protocol server for controlled local system access.'

    $GoBuildInfo = Invoke-ProcessRequired `
        -FilePath 'go.exe' `
        -Arguments @('version', '-m', $ResolvedBinaryPath) `
        -WorkingDirectory $RootPath
    foreach ($ExpectedBuildSetting in @(
        "path`tgithub.com/thomasweidner/flashgate-mcp/cmd/server"
        "build`tGOOS=windows"
        "build`tGOARCH=$ExpectedGOARCH"
        "build`tCGO_ENABLED=0"
        "build`t-trimpath=true"
    )) {
        if (-not $GoBuildInfo.Contains(
            $ExpectedBuildSetting,
            [StringComparison]::Ordinal
        )) {
            $Errors.Add(
                "Go build information is missing: $ExpectedBuildSetting"
            )
        }
    }

    $ManifestOutput = Invoke-ProcessRequired `
        -FilePath 'go.exe' `
        -Arguments @(
            '-C'
            $RootPath
            'run'
            '-mod=vendor'
            './cmd/versionmanifest'
            '--binary'
            $ResolvedBinaryPath
            '--expected-version'
            $ExpectedProductVersion
            '--expected-file-version'
            $ExpectedFileVersion
            '--expected-commit'
            $ExpectedCommit
            '--expected-source-time'
            $ExpectedSourceTime
            '--expected-modified'
            $ExpectedModified
            '--expected-goos'
            'windows'
            '--expected-goarch'
            $ExpectedGOARCH
            '--expected-public-arch'
            $ExpectedPublicArch
        ) `
        -WorkingDirectory $RootPath
    if ($ManifestOutput -notmatch '(?m)^Status: PASS$') {
        $Errors.Add('Static build-manifest verifier did not report PASS.')
    }

    $IconOutput = Invoke-ProcessRequired `
        -FilePath 'go.exe' `
        -Arguments @(
            '-C'
            $RootPath
            'run'
            '-mod=vendor'
            './cmd/iconverify'
            '--binary'
            $ResolvedBinaryPath
            '--icon'
            $ExpectedIconPath
        ) `
        -WorkingDirectory $RootPath
    if ($IconOutput -notmatch '(?m)^Status: PASS$') {
        $Errors.Add('Icon identity verifier did not report PASS.')
    }
    if ($IconOutput -match '(?m)^FrameCount: (?<count>[0-9]+)$') {
        $Result.IconFrameCount = [int]$Matches['count']
    }
    else {
        $Errors.Add('Icon identity verifier did not report a frame count.')
    }
    if (
        $IconOutput -match
        '(?m)^FrameIdentitySHA256: (?<hash>[0-9a-f]{64})$'
    ) {
        $Result.IconFrameIdentity = $Matches['hash']
    }
    else {
        $Errors.Add('Icon identity verifier did not report its frame hash.')
    }

    $Result.FileDescription = $VersionInfo.FileDescription
    $Result.FileVersion = $VersionInfo.FileVersion
    $Result.ProductName = $VersionInfo.ProductName
    $Result.ProductVersion = $VersionInfo.ProductVersion
    $Result.CompanyName = $VersionInfo.CompanyName
    $Result.LegalCopyright = $VersionInfo.LegalCopyright
    $Result.OriginalFilename = $VersionInfo.OriginalFilename
    $Result.InternalName = $VersionInfo.InternalName
    $Result.Comments = $VersionInfo.Comments

    if ($Errors.Count -eq 0) {
        $Result.Status = if ($Warnings.Count -gt 0) {
            'PASS_WITH_WARNINGS'
        }
        else {
            'PASS'
        }
        $ExitCode = 0
    }
}
catch {
    $Errors.Add($_.Exception.Message)
}
finally {
    $Result.WarningCount = $Warnings.Count
    $Result.ErrorCount = $Errors.Count
    $Result.Warnings = if ($Warnings.Count -gt 0) {
        $Warnings -join [Environment]::NewLine
    }
    else {
        $null
    }
    $Result.Errors = if ($Errors.Count -gt 0) {
        $Errors -join [Environment]::NewLine
    }
    else {
        $null
    }

    [pscustomobject]$Result | Format-List
}

exit $ExitCode
