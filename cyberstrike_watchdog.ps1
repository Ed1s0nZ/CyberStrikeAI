# CyberStrikeAI watchdog: start if not running, keep restarting on exit.
# ASCII-only (PS 5.1 reads .ps1 as ANSI when no BOM).
# Single-instance guard uses a named mutex (atomic, auto-released on process death),
# so the scheduled task can safely re-spawn this script every few minutes.
$exe = 'd:\CyberStrikeAI\CyberStrikeAI\CyberStrikeAI.exe'
$work = 'd:\CyberStrikeAI\CyberStrikeAI'
$log = 'd:\CyberStrikeAI\CyberStrikeAI\data\watchdog.log'

function Log($msg) {
    $line = '{0} {1}' -f (Get-Date).ToString('yyyy-MM-dd HH:mm:ss'), $msg
    Add-Content -Path $log -Value $line -Encoding UTF8
}

Log 'watchdog started'
# single-instance guard via named mutex
$mutex = New-Object System.Threading.Mutex($false, 'CyberStrikeAI-Watchdog-Mutex')
if (-not $mutex.WaitOne(0)) {
    Log 'another watchdog already running, exiting'
    exit 0
}
while ($true) {
    try {
        $p = Get-Process -Name CyberStrikeAI -ErrorAction SilentlyContinue
        if (-not $p) {
            Log 'CyberStrikeAI not running, starting...'
            Start-Process -FilePath $exe -WorkingDirectory $work -WindowStyle Hidden
            Start-Sleep -Seconds 20
            $p2 = Get-Process -Name CyberStrikeAI -ErrorAction SilentlyContinue
            if ($p2) {
                Log ("started OK, pid=" + $p2.Id)
            } else {
                Log 'start FAILED, will retry'
            }
        }
    } catch {
        Log ('watchdog error: ' + $_.Exception.Message)
    }
    Start-Sleep -Seconds 30
}
