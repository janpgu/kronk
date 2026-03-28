# kronk installer for Windows
# Usage: irm https://raw.githubusercontent.com/janpgu/kronk/main/install.ps1 | iex

param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:USERPROFILE\bin"
)

$ErrorActionPreference = "Stop"

$repo     = "janpgu/kronk"
$binary   = "kronk.exe"
$vbsName  = "kronk-tick.vbs"
$taskName = "kronk"

function Write-Step($msg) {
    Write-Host "  --> $msg" -ForegroundColor Cyan
}

function Write-Success($msg) {
    Write-Host "  OK  $msg" -ForegroundColor Green
}

function Write-Fail($msg) {
    Write-Host "  ERR $msg" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "kronk installer" -ForegroundColor White
Write-Host "---------------" -ForegroundColor DarkGray
Write-Host ""

# --- 1. Resolve version ---
Write-Step "Resolving version..."
if ($Version -eq "latest") {
    try {
        $release = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
        $Version = $release.tag_name
    } catch {
        Write-Fail "Could not fetch latest release from GitHub. Check your internet connection."
    }
}
Write-Success "Version: $Version"

# --- 2. Download binary ---
$downloadUrl = "https://github.com/$repo/releases/download/$Version/kronk-windows-amd64.exe"
$tmpPath     = Join-Path $env:TEMP "kronk-download.exe"

Write-Step "Downloading $downloadUrl..."
try {
    Invoke-WebRequest -Uri $downloadUrl -OutFile $tmpPath -UseBasicParsing
} catch {
    Write-Fail "Download failed: $_"
}
Write-Success "Downloaded to $tmpPath"

# --- 3. Install binary ---
Write-Step "Installing to $InstallDir..."
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}
$binaryPath = Join-Path $InstallDir $binary
Copy-Item -Path $tmpPath -Destination $binaryPath -Force
Remove-Item $tmpPath
Write-Success "Binary installed: $binaryPath"

# --- 4. Add to PATH if needed ---
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$InstallDir*") {
    Write-Step "Adding $InstallDir to PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Success "Added to PATH (restart your terminal for this to take effect globally)"
} else {
    Write-Success "$InstallDir already in PATH"
}

# --- 5. Write VBScript wrapper (runs kronk silently, no console window) ---
$vbsPath = Join-Path $InstallDir $vbsName
Write-Step "Writing silent launcher: $vbsPath..."
$vbsContent = "CreateObject(`"WScript.Shell`").Run `"$binaryPath tick`", 0, False"
Set-Content -Path $vbsPath -Value $vbsContent
Write-Success "Launcher written"

# --- 6. Register Task Scheduler entry ---
Write-Step "Registering Task Scheduler task '$taskName'..."

# Remove existing task if present.
schtasks /delete /tn $taskName /f 2>$null | Out-Null

# Create new task: run every minute, start on AC and battery, any user session.
$xml = @"
<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>kronk job scheduler tick</Description>
  </RegistrationInfo>
  <Triggers>
    <TimeTrigger>
      <Repetition>
        <Interval>PT1M</Interval>
        <StopAtDurationEnd>false</StopAtDurationEnd>
      </Repetition>
      <StartBoundary>2000-01-01T00:00:00</StartBoundary>
      <Enabled>true</Enabled>
    </TimeTrigger>
  </Triggers>
  <Settings>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <ExecutionTimeLimit>PT5M</ExecutionTimeLimit>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <Enabled>true</Enabled>
  </Settings>
  <Actions>
    <Exec>
      <Command>wscript.exe</Command>
      <Arguments>//B "$vbsPath"</Arguments>
    </Exec>
  </Actions>
</Task>
"@

$xmlPath = Join-Path $env:TEMP "kronk-task.xml"
$xml | Out-File -FilePath $xmlPath -Encoding Unicode
schtasks /create /tn $taskName /xml $xmlPath /f | Out-Null
Remove-Item $xmlPath
Write-Success "Task '$taskName' registered (runs every minute, including on battery)"

# --- Done ---
Write-Host ""
Write-Host "kronk $Version installed." -ForegroundColor Green
Write-Host ""
Write-Host "Quick start:"
Write-Host "  kronk add `"echo hello`" --name hello --schedule `"every night`""
Write-Host "  kronk status"
Write-Host "  kronk history"
Write-Host ""
Write-Host "Run 'kronk doctor' to verify your setup."
Write-Host ""
