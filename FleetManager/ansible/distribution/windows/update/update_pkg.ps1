# Run PowerShell as Administrator for the script to have the necessary permissions

# Update Windows Update Agent
Write-Host "Updating Windows Update Agent..."
Start-Process -FilePath "wusa.exe" -ArgumentList "/KB:3173424" -Wait

# Update PowerShellGet Module (required for updating other packages)
Write-Host "Updating PowerShellGet module..."
Install-Module -Name PowerShellGet -Force -AllowClobber -Scope AllUsers -Confirm:$false
Update-Module -Name PowerShellGet -Force -Confirm:$false

# Update all installed modules
Write-Host "Updating all installed modules..."
Get-Module -ListAvailable | Where-Object { $_.Name -ne "PSReadline" } | ForEach-Object {
    Update-Module -Name $_.Name -Force -Confirm:$false
}

# Update Chocolatey (if installed)
if (Test-Path "$env:SystemDrive\ProgramData\chocolatey") {
    Write-Host "Updating Chocolatey packages..."
    choco update all -y
}

# Update Windows Defender Definitions
Write-Host "Updating Windows Defender definitions..."
Update-MpSignature -UpdateSource "DefaultUpdateServer"

Write-Host "All packages updated successfully."
