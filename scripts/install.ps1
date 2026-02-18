# Crux installer for Windows (PowerShell)
# Usage:
#   irm https://raw.githubusercontent.com/roygabriel/crux/main/scripts/install.ps1 | iex
#
# Environment variables:
#   CRUX_VERSION - Version to install (default: latest)

$ErrorActionPreference = "Stop"

$Repo = "roygabriel/crux"
$Binary = "crux"
$InstallDir = "$env:LOCALAPPDATA\crux"

function Get-Architecture {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default { throw "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
    }
}

function Get-LatestVersion {
    try {
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
        return $release.tag_name
    }
    catch {
        throw "Failed to fetch latest version. Set `$env:CRUX_VERSION explicitly."
    }
}

function Main {
    $arch = Get-Architecture
    $os = "windows"

    if ($env:CRUX_VERSION) {
        $version = $env:CRUX_VERSION
    }
    else {
        Write-Host "  > Fetching latest version..." -ForegroundColor Blue
        $version = Get-LatestVersion
    }

    Write-Host "  > Installing $Binary $version ($os/$arch)" -ForegroundColor Blue

    $zipName = "${Binary}_$($version.TrimStart('v'))_${os}_${arch}.zip"
    $url = "https://github.com/$Repo/releases/download/$version/$zipName"
    $checksumUrl = "https://github.com/$Repo/releases/download/$version/checksums.txt"

    $tmpDir = New-Item -ItemType Directory -Path (Join-Path $env:TEMP "crux-install-$(Get-Random)")

    try {
        $zipPath = Join-Path $tmpDir $zipName

        Write-Host "  > Downloading $url..." -ForegroundColor Blue
        Invoke-WebRequest -Uri $url -OutFile $zipPath

        # Verify checksum if available.
        try {
            $checksumPath = Join-Path $tmpDir "checksums.txt"
            Invoke-WebRequest -Uri $checksumUrl -OutFile $checksumPath
            $checksums = Get-Content $checksumPath
            $expectedLine = $checksums | Where-Object { $_ -match $zipName }
            if ($expectedLine) {
                $expected = ($expectedLine -split '\s+')[0]
                $actual = (Get-FileHash -Algorithm SHA256 $zipPath).Hash.ToLower()
                if ($actual -ne $expected) {
                    throw "Checksum mismatch! Expected $expected, got $actual."
                }
                Write-Host "  > Checksum verified." -ForegroundColor Blue
            }
        }
        catch {
            Write-Host "  > Skipping checksum verification." -ForegroundColor Yellow
        }

        Write-Host "  > Extracting..." -ForegroundColor Blue
        Expand-Archive -Path $zipPath -DestinationPath $tmpDir -Force

        if (-not (Test-Path $InstallDir)) {
            New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        }

        $binaryPath = Join-Path $InstallDir "$Binary.exe"
        Copy-Item -Path (Join-Path $tmpDir "$Binary.exe") -Destination $binaryPath -Force

        # Add to user PATH if not already present.
        $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if ($userPath -notlike "*$InstallDir*") {
            [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
            Write-Host "  > Added $InstallDir to user PATH." -ForegroundColor Blue
            Write-Host "  > Restart your terminal for PATH changes to take effect." -ForegroundColor Yellow
        }

        Write-Host ""
        Write-Host "  > Installed $Binary $version to $binaryPath" -ForegroundColor Green
        Write-Host "  > Run '$Binary --version' to verify." -ForegroundColor Blue
        Write-Host "  > Run '$Binary init' to initialize a new project." -ForegroundColor Blue
    }
    finally {
        Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Main
