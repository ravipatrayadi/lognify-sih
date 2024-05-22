# Download Docker Desktop Installer
$installerUrl = 'https://desktop.docker.com/win/stable/Docker%20Desktop%20Installer.exe'
$installerPath = "$env:TEMP\DockerDesktopInstaller.exe"

Invoke-WebRequest -Uri $installerUrl -OutFile $installerPath

# Install Docker Desktop
Start-Process -Wait -FilePath $installerPath -ArgumentList '/S'

# Clean up the installer
Remove-Item -Path $installerPath
