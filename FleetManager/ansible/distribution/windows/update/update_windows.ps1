# Trust the PSGallery repository
Set-PSRepository -Name PSGallery -InstallationPolicy Trusted

# Check if the PSWindowsUpdate module is installed
if (!(Get-Module -Name PSWindowsUpdate)) {
    # If the module is not installed, install it using the Install-Module cmdlet
    Install-Module PSWindowsUpdate
}

# Add the Microsoft Update service to the list of available update sources
Add-WUServiceManager -MicrosoftUpdate

# Install all available updates
Install-WindowsUpdate -MicrosoftUpdate -AcceptAll -AutoReboot | Out-File "C:\($env.computername-Get-Date -f yyyy-MM-dd)-MSUpdates.log" -Force