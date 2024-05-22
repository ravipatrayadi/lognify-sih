# Download and Install Apache HTTP Server (httpd)
$apacheInstallerUrl = 'https://www.apachelounge.com/download/VC15/binaries/httpd-2.4.51-win64-VS16.zip'
$apacheInstallerPath = "$env:TEMP\apache-installer.zip"
$apacheInstallPath = 'C:\Apache24'

Invoke-WebRequest -Uri $apacheInstallerUrl -OutFile $apacheInstallerPath
Expand-Archive -Path $apacheInstallerPath -DestinationPath $apacheInstallPath

# Download and Install Apache2
$apache2InstallerUrl = 'https://www.apachelounge.com/download/VC15/binaries/httpd-2.2.34-win64-VS16.zip'
$apache2InstallerPath = "$env:TEMP\apache2-installer.zip"
$apache2InstallPath = 'C:\Apache22'

Invoke-WebRequest -Uri $apache2InstallerUrl -OutFile $apache2InstallerPath
Expand-Archive -Path $apache2InstallerPath -DestinationPath $apache2InstallPath

# Start Apache and Apache2 services
Start-Service -Name 'wampapache'
Start-Service -Name 'wampapache2'

# Wait for services to start (adjust the sleep duration based on your machine)
Start-Sleep -Seconds 10

# Open Apache default web page in default web browser
Start-Process "http://localhost"
