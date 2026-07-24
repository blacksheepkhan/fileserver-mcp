$script:FlashGateMaximumSourceDateEpoch = [int64]253402300799

function Get-FlashGateSemanticVersion {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string] $Value
    )

    if (
        [string]::IsNullOrEmpty($Value) -or
        $Value -cne $Value.Trim() -or
        $Value.IndexOfAny([char[]]"`r`n`t") -ge 0
    ) {
        throw "Invalid semantic version: $Value"
    }

    $Pattern =
        '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)' +
        '(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?' +
        '(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?\z'
    $Match = [regex]::Match(
        $Value,
        $Pattern,
        [Text.RegularExpressions.RegexOptions]::CultureInvariant
    )
    if (-not $Match.Success) {
        throw "Invalid semantic version: $Value"
    }

    if ($Match.Groups[4].Success) {
        foreach ($Identifier in $Match.Groups[4].Value.Split('.')) {
            if (
                $Identifier -match '^[0-9]+$' -and
                $Identifier.Length -gt 1 -and
                $Identifier[0] -eq '0'
            ) {
                throw "Invalid numeric prerelease identifier: $Value"
            }
        }
    }

    $Components = foreach ($Index in 1..3) {
        $Parsed = [uint16]0
        if (
            -not [uint16]::TryParse(
                $Match.Groups[$Index].Value,
                [Globalization.NumberStyles]::None,
                [Globalization.CultureInfo]::InvariantCulture,
                [ref]$Parsed
            )
        ) {
            throw "Version component exceeds the Windows 16-bit range: $Value"
        }
        $Parsed
    }

    [pscustomobject]@{
        Value       = $Value
        Major       = $Components[0]
        Minor       = $Components[1]
        Patch       = $Components[2]
        FileVersion = '{0}.{1}.{2}.0' -f $Components
    }
}

function ConvertFrom-FlashGateSourceDateEpoch {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string] $Value
    )

    if ($Value -notmatch '^[0-9]+$') {
        throw 'SOURCE_DATE_EPOCH must contain nonnegative decimal digits only.'
    }

    $Epoch = [int64]0
    if (
        -not [int64]::TryParse(
            $Value,
            [Globalization.NumberStyles]::None,
            [Globalization.CultureInfo]::InvariantCulture,
            [ref]$Epoch
        ) -or
        $Epoch -gt $script:FlashGateMaximumSourceDateEpoch
    ) {
        throw 'SOURCE_DATE_EPOCH is outside the supported range.'
    }

    [DateTimeOffset]::FromUnixTimeSeconds($Epoch).UtcDateTime.ToString(
        'yyyy-MM-ddTHH:mm:ssZ',
        [Globalization.CultureInfo]::InvariantCulture
    )
}
