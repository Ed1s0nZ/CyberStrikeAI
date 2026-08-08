# Register CyberStrikeAI watchdog scheduled task (AtLogOn, current user)
$ErrorActionPreference = 'Stop'
$task = 'CyberStrikeAI-Watchdog'
$arg = '-NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File d:\CyberStrikeAI\CyberStrikeAI\cyberstrike_watchdog.ps1'
$action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument $arg
# Repetition: re-spawn the watchdog every 5 minutes (single-instance mutex guard
# in cyberstrike_watchdog.ps1 makes redundant instances exit immediately), so a
# watchdog that dies with a terminal session is self-healed without re-login.
# PS 5.1 limitation: -AtLogOn does not accept -RepetitionInterval; copy the
# repetition pattern from a -Once trigger instead. Omit -RepetitionDuration so
# the repetition is indefinite (PT0S would fail the task XML schema).
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
$trigger.Repetition = (New-ScheduledTaskTrigger -Once -At (Get-Date) `
    -RepetitionInterval (New-TimeSpan -Minutes 5)).Repetition
$settings = New-ScheduledTaskSettingsSet -StartWhenAvailable -ExecutionTimeLimit ([TimeSpan]::Zero)
try {
    Register-ScheduledTask -TaskName $task -Action $action -Trigger $trigger -Settings $settings `
        -Description 'Auto start and keep-alive CyberStrikeAI' -Force | Out-Null
    $t = Get-ScheduledTask -TaskName $task
    Write-Output ("registered: name=" + $t.TaskName + " state=" + $t.State)
} catch {
    Write-Output ("FAILED: " + $_.Exception.Message)
}
