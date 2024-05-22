import boto3 
import configparser

regions= [
    # 'ap-northeast-1',
    # 'ap-northeast-2',
    # 'ap-south-1',
    # 'ap-southeast-1',
    # 'ap-southeast-2',
    # 'ca-central-1',
    # 'eu-central-1',
    # 'eu-north-1',
    # 'eu-west-1',
    # 'eu-west-2',
    # 'eu-west-3',
    # 'sa-east-1',
    # 'us-east-1',
    # 'us-east-2',
    # 'us-west-1',
    # 'us-west-2'
]


ec2 = boto3.client('ec2', region_name = 'ap-south-1')
response = ec2.describe_regions()
for region in response['Regions']:
    regions.append(region['RegionName'])


config = configparser.ConfigParser(allow_no_value = True)

for region_name in regions:
    # print(f'region_name: {region_name}')
    ec2= boto3.resource('ec2', region_name=region_name)
    instances= ec2.meta.client.describe_instances()
    for instance in instances['Reservations']:
        for singleInstance in instance['Instances']:
            privateIP= singleInstance['PrivateIpAddress']
            tags= singleInstance['Tags']
            keyName = singleInstance['KeyName']
            name = ''
            functionGroup = ''
            for tag in tags:
                if tag['Key'] == 'Name':
                    name = tag['Value']
                if tag['Key'] == 'function':
                    functionGroup = tag['Value']
            if functionGroup not in config:
                config[functionGroup] = {}
            config[functionGroup][privateIP] = None
            # print(privateIP, name, functionGroup)

with open('aws_inventory.ini', 'w') as configfile:
  config.write(configfile)

import os
import paramiko
from scp import SCPClient

remote_ip = ''
username = ''
password = ''

local_path = os.getcwd() + '/aws_inventory.ini'
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