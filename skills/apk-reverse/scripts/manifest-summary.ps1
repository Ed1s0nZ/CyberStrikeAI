#requires -Version 5

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$ManifestPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
[Console]::InputEncoding = [System.Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = [System.Text.UTF8Encoding]::new($false)

if (-not (Test-Path -LiteralPath $ManifestPath)) {
    throw "Manifest not found: $ManifestPath"
}

$xml = [xml](Get-Content -LiteralPath $ManifestPath -Raw -Encoding UTF8)
$androidNs = 'http://schemas.android.com/apk/res/android'
$nsMgr = [System.Xml.XmlNamespaceManager]::new($xml.NameTable)
$nsMgr.AddNamespace('android', $androidNs)

function Get-AndroidAttr {
    param(
        [Parameter(Mandatory = $true)]$Node,
        [Parameter(Mandatory = $true)][string]$Name
    )

    return [string]$Node.GetAttribute($Name, $androidNs)
}

$manifest = $xml.manifest
$packageName = [string]$manifest.package

"package=$packageName"

$permissions = @($manifest.SelectNodes('uses-permission'))
"permission_count=$($permissions.Count)"
foreach ($perm in $permissions) {
    $name = Get-AndroidAttr -Node $perm -Name 'name'
    if ($name) {
        "permission=$name"
    }
}

$componentQueries = @(
    @{ Label = 'activity'; Path = 'application/activity' },
    @{ Label = 'service'; Path = 'application/service' },
    @{ Label = 'receiver'; Path = 'application/receiver' },
    @{ Label = 'provider'; Path = 'application/provider' }
)

foreach ($query in $componentQueries) {
    $nodes = @($manifest.SelectNodes($query.Path))
    ($query.Label + '_count=' + $nodes.Count)
    foreach ($node in $nodes) {
        $name = Get-AndroidAttr -Node $node -Name 'name'
        $exported = Get-AndroidAttr -Node $node -Name 'exported'
        $enabled = Get-AndroidAttr -Node $node -Name 'enabled'
        ($query.Label + '=' + $name + "`t" + $exported + "`t" + $enabled)
    }
}

$mainActivities = @()
$activities = @($manifest.SelectNodes('application/activity'))
foreach ($activity in $activities) {
    $filters = @($activity.SelectNodes('intent-filter'))
    foreach ($filter in $filters) {
        $hasMain = @($filter.SelectNodes('action')).Where({ (Get-AndroidAttr -Node $_ -Name 'name') -eq 'android.intent.action.MAIN' }).Count -gt 0
        $hasLauncher = @($filter.SelectNodes('category')).Where({ (Get-AndroidAttr -Node $_ -Name 'name') -eq 'android.intent.category.LAUNCHER' }).Count -gt 0
        if ($hasMain -and $hasLauncher) {
            $mainActivities += (Get-AndroidAttr -Node $activity -Name 'name')
        }
    }
}

foreach ($entry in ($mainActivities | Select-Object -Unique)) {
    "main_activity=$entry"
}
