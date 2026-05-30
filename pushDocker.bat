@echo off
setlocal

for /f %%i in ('powershell -NoProfile -Command "$ts=[DateTimeOffset]::Now.ToUnixTimeSeconds().ToString(); $date=Get-Date -Format yyyyMMdd; $sha1=[System.BitConverter]::ToString(([System.Security.Cryptography.SHA1]::Create()).ComputeHash([System.Text.Encoding]::UTF8.GetBytes($ts))).Replace('-','').ToLower().Substring(0,12); Write-Output ($date + '-' + $sha1)"') do set "TAG=%%i"

if "%TAG%"=="" (
  echo Failed to generate docker tag.
  exit /b 1
)

set "LOCAL_IMAGE=sam/new-nav:%TAG%"
set "REMOTE_IMAGE=dockerhub.samliao.idv.tw/sam/new-nav:%TAG%"
set "REMOTE_LATEST=dockerhub.samliao.idv.tw/sam/new-nav:latest"

echo Using tag: %TAG%

docker build -t %LOCAL_IMAGE% .
if errorlevel 1 exit /b 1

docker tag %LOCAL_IMAGE% %REMOTE_IMAGE%
if errorlevel 1 exit /b 1

docker tag %REMOTE_IMAGE% %REMOTE_LATEST%
if errorlevel 1 exit /b 1

docker login dockerhub.samliao.idv.tw
if errorlevel 1 exit /b 1

docker push %REMOTE_IMAGE%
if errorlevel 1 exit /b 1

docker push %REMOTE_LATEST%
if errorlevel 1 exit /b 1

echo Done.
echo Version tag: %REMOTE_IMAGE%
echo Latest tag:  %REMOTE_LATEST%
