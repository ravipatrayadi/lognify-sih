

from azure.identity import ClientSecretCredential
from azure.mgmt.compute import ComputeManagementClient
from azure.mgmt.network import NetworkManagementClient
import configparser

credential = ClientSecretCredential(
    tenant_id='',
    client_id='',
    client_secret=''
)
SUBSCRIPTION_ID = ''
compute_client = ComputeManagementClient(
    credential=credential,
    subscription_id=SUBSCRIPTION_ID
)

network_client = NetworkManagementClient(credential,SUBSCRIPTION_ID)
config = configparser.ConfigParser(allow_no_value = True)
import json
def list_virtual_machines():
    for vm in compute_client.virtual_machines.list_all():
        # print(vm.os_profile.linux_configuration.patch_settings)
        # Create a ComputeManagementClient instance

        os_type = vm
        print(os_type.os_profile)
        if vm.tags is None:
            continue
        functionGroup = vm.tags['function']
        if functionGroup not in config:
            config[functionGroup] = {}
        for interface in vm.network_profile.network_interfaces:
            name=" ".join(interface.id.split('/')[-1:])
            sub="".join(interface.id.split('/')[4])

            try:
                thing=network_client.network_interfaces.get(sub, name).ip_configurations
                for x in thing:
                    config[functionGroup][x.private_ip_address] = None
            except:
                print("failed to fetch private ip")


list_virtual_machines()
with open('azure_inventory.ini', 'w') as configfile:
  config.write(configfile)


import os
import paramiko
from scp import SCPClient

remote_ip = ''
username = ''
password = ''

local_path = os.getcwd() + '/azure_inventory.ini'
remote_path = '~/'

# Create an SSH client
ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())

try:
    ssh.connect(remote_ip, username=username, password=password)

    with SCPClient(ssh.get_transport()) as scp:
        scp.put(local_path, remote_path)

finally:
    ssh.close()