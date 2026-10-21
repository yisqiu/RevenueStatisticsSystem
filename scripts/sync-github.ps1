param([string]$Message='更新营收统计系统')
$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
Set-Location $projectRoot
$token=(Get-Content -LiteralPath 'D:\Keys\githubtoken' -Raw).Trim()
try {
  $headers=@{Authorization="Bearer $token";Accept='application/vnd.github+json';'X-GitHub-Api-Version'='2022-11-28'}
  $repo=Invoke-RestMethod -Uri 'https://api.github.com/repos/yisqiu/RevenueStatisticsSystem' -Headers $headers -Proxy 'http://127.0.0.1:7897'
  $auth=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes("x-access-token:$token"))
  $env:GIT_CONFIG_COUNT='3'
  $env:GIT_CONFIG_KEY_0='http.extraHeader'; $env:GIT_CONFIG_VALUE_0="Authorization: Basic $auth"
  $env:GIT_CONFIG_KEY_1='http.proxy'; $env:GIT_CONFIG_VALUE_1='http://127.0.0.1:7897'
  $env:GIT_CONFIG_KEY_2='safe.directory'; $env:GIT_CONFIG_VALUE_2=$projectRoot.Replace('\','/')
  if(!(Test-Path .git)){git init -b main; if($LASTEXITCODE){throw '初始化仓库失败'}}
  $remotes=git remote
  if($remotes -notcontains 'origin'){git remote add origin 'https://github.com/yisqiu/RevenueStatisticsSystem.git'}
  git -c core.autocrlf=false add -- backend deploy docs frontend scripts agent_context agent_memory .gitignore LICENSE README.md AGENTS.md
  if($LASTEXITCODE){throw 'git add 失败'}
  git diff --cached --quiet
  if($LASTEXITCODE -eq 1){git commit --quiet -m $Message; if($LASTEXITCODE){throw '提交失败'}}
  git -c http.version=HTTP/1.1 -c http.postBuffer=524288000 push -u origin HEAD:main
  if($LASTEXITCODE){throw '推送失败，请保留本地提交核对远端历史'}
  $local=git rev-parse HEAD; $remote=git ls-remote origin refs/heads/main
  if(!$remote.StartsWith($local)){throw '远端提交核对失败'}
  Write-Output "GitHub SHA verified: $local"
} finally {
  'GIT_CONFIG_COUNT','GIT_CONFIG_KEY_0','GIT_CONFIG_VALUE_0','GIT_CONFIG_KEY_1','GIT_CONFIG_VALUE_1','GIT_CONFIG_KEY_2','GIT_CONFIG_VALUE_2' | ForEach-Object {Remove-Item "Env:$_" -ErrorAction SilentlyContinue}
  $token=$null; $auth=$null; $headers=$null
}


