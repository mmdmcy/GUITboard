$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

if ($args.Count -eq 0) {
    & (Join-Path $scriptDir 'GUITboard.cmd') --current-console
} else {
    & (Join-Path $scriptDir '.portui-runtime\portui.ps1') -ManifestDir (Join-Path $scriptDir 'portui') @args
}

exit $LASTEXITCODE
