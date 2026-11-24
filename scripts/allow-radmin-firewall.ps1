# Run in an elevated PowerShell on the computer hosting the Radmin chat service.
$ErrorActionPreference = 'Stop'
$adapter = Get-NetAdapter | Where-Object { $_.InterfaceDescription -like '*Radmin*' -and $_.Status -eq 'Up' } | Select-Object -First 1
if (-not $adapter) { throw 'No active Radmin VPN adapter was found.' }
$address = Get-NetIPAddress -InterfaceIndex $adapter.ifIndex -AddressFamily IPv4 | Select-Object -First 1
if (-not $address) { throw 'Radmin VPN has no IPv4 address.' }
$name = 'LinChat-Radmin-TCP-8787'
if (Get-NetFirewallRule -Name $name -ErrorAction SilentlyContinue) {
    Remove-NetFirewallRule -Name $name
}
New-NetFirewallRule -Name $name -DisplayName 'LinChat Radmin chat server (TCP 8787)' -Direction Inbound -Action Allow -Protocol TCP -LocalPort 8787 -LocalAddress $address.IPAddress -InterfaceAlias $adapter.Name -Profile Any | Select-Object DisplayName,Enabled,Direction,Action
Write-Host "LinChat is allowed on Radmin interface $($adapter.Name), $($address.IPAddress):8787"
