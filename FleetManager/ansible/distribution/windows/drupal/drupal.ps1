# Download and Install XAMPP
$xamppInstallerUrl = 'https://downloads.sourceforge.net/project/xampp/XAMPP%20Windows/7.4.9/xampp-windows-x64-7.4.9-0-VC15-installer.exe'
$xamppInstallerPath = "$env:TEMP\xampp-installer.exe"

Invoke-WebRequest -Uri $xamppInstallerUrl -OutFile $xamppInstallerPath
Start-Process -Wait -FilePath $xamppInstallerPath -ArgumentList '/S'

# Download and Install Composer
$composerInstallerUrl = 'https://getcomposer.org/installer'
$composerInstallerPath = "$env:TEMP\composer-installer.php"

Invoke-WebRequest -Uri $composerInstallerUrl -OutFile $composerInstallerPath
Start-Process -Wait -FilePath 'C:\xampp\php\php.exe' -ArgumentList $composerInstallerPath

# Install Drupal using Composer
$drupalInstallerPath = 'C:\xampp\htdocs\drupal'

Start-Process -Wait -FilePath 'C:\xampp\php\php.exe' -ArgumentList 'composer.phar create-project drupal/recommended-project:^9.0', "--install-dir=$drupalInstallerPath"

# Configure XAMPP and Start Apache and MySQL services
Start-Process -FilePath 'C:\xampp\xampp-control.exe' -ArgumentList 'startapache'

Start-Process -FilePath 'C:\xampp\xampp-control.exe' -ArgumentList 'startmysql'

# Wait for services to start (adjust the sleep duration based on your machine)
Start-Sleep -Seconds 10

# Open Drupal in default web browser
Start-Process "http://localhost/drupal/install.php"
