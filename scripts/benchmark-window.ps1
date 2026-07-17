Set-StrictMode -Version Latest

function Get-EuropeViennaTimeZone {
    [CmdletBinding()]
    param()

    foreach ($TimeZoneId in @('W. Europe Standard Time', 'Europe/Vienna')) {
        try {
            return [TimeZoneInfo]::FindSystemTimeZoneById($TimeZoneId)
        }
        catch [TimeZoneNotFoundException] {
            continue
        }
        catch [InvalidTimeZoneException] {
            continue
        }
    }
    throw 'The Europe/Vienna time zone is not available on this host.'
}

function Get-EuropeViennaMeasurementWindowStatus {
    [CmdletBinding()]
    param(
        [DateTimeOffset]$UtcInstant = [DateTimeOffset]::UtcNow
    )

    $TimeZone = Get-EuropeViennaTimeZone
    $LocalTime = [TimeZoneInfo]::ConvertTime($UtcInstant.ToUniversalTime(), $TimeZone)
    $IsBlocked = $LocalTime.Hour -ge 19 -or $LocalTime.Hour -lt 4
    $NextAllowedDate = if ($LocalTime.Hour -ge 19) { $LocalTime.Date.AddDays(1) } else { $LocalTime.Date }

    [pscustomobject]@{
        IsBlocked         = $IsBlocked
        LocalTime         = $LocalTime
        TimeZoneId        = $TimeZone.Id
        NextAllowed       = $NextAllowedDate.AddHours(4)
        NextPreferredStart = $NextAllowedDate.AddHours(4).AddMinutes(15)
    }
}

function Get-BlockedBaselineMessage {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]$WindowStatus
    )

    return ('Performance baseline recording is blocked at {0} Europe/Vienna. Scheduled host-load window: 19:00 inclusive to 04:00 exclusive. Next allowed window starts at {1} Europe/Vienna; preferred measurement window: 04:15-18:45 Europe/Vienna. No baseline was written or replaced.' -f $WindowStatus.LocalTime.ToString('yyyy-MM-dd HH:mm:ss zzz'), $WindowStatus.NextAllowed.ToString('yyyy-MM-dd HH:mm'))
}

function Assert-BaselineMeasurementWindow {
    [CmdletBinding()]
    param(
        [DateTimeOffset]$UtcInstant = [DateTimeOffset]::UtcNow
    )

    $WindowStatus = Get-EuropeViennaMeasurementWindowStatus -UtcInstant $UtcInstant
    if ($WindowStatus.IsBlocked) {
        throw (Get-BlockedBaselineMessage -WindowStatus $WindowStatus)
    }
    return $WindowStatus
}

function Publish-BenchmarkBaselineCandidate {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$CandidatePath,
        [Parameter(Mandatory)][string]$DestinationPath,
        [DateTimeOffset]$UtcInstant = [DateTimeOffset]::UtcNow
    )

    Assert-BaselineMeasurementWindow -UtcInstant $UtcInstant | Out-Null
    Move-Item -LiteralPath $CandidatePath -Destination $DestinationPath -Force
}

function Get-ContaminatedPerformanceWarning {
    [CmdletBinding()]
    param()

    return 'Performance values are contaminated by the scheduled host-load window and are not valid baseline evidence.'
}
