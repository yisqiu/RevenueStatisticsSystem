$ErrorActionPreference='Stop'
$project=Split-Path $PSScriptRoot -Parent
$stage=Join-Path $project 'artifacts\release-stage'
New-Item -ItemType Directory -Path $stage,(Join-Path $stage 'web'),(Join-Path $stage 'deploy') -Force | Out-Null
Copy-Item -LiteralPath (Join-Path $project 'artifacts\revenuestatistics') -Destination (Join-Path $stage 'revenuestatistics')
Copy-Item -Path (Join-Path $project 'frontend\apps\web-antd\dist\*') -Destination (Join-Path $stage 'web') -Recurse -Force
Copy-Item -Path (Join-Path $project 'deploy\*.conf'),(Join-Path $project 'deploy\*.service') -Destination (Join-Path $stage 'deploy') -Force
Copy-Item -LiteralPath (Join-Path $project 'scripts\register-identity.py') -Destination (Join-Path $stage 'register-identity.py')
$files=Get-ChildItem -LiteralPath $stage -File -Recurse | Where-Object Name -ne 'manifest.json' | ForEach-Object { [pscustomobject]@{path=[IO.Path]::GetRelativePath($stage,$_.FullName).Replace('\','/');sha256=(Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()} }
$files | ConvertTo-Json -Depth 3 | Set-Content -LiteralPath (Join-Path $stage 'manifest.json') -Encoding utf8
$release=Join-Path $project 'artifacts\transfer\revenuestatistics-release.tgz'
tar -czf $release -C $stage .
if($LASTEXITCODE){throw '发布打包失败'}
Get-FileHash -LiteralPath $release -Algorithm SHA256 | Select-Object Hash,Path
