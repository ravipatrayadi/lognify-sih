# Architecture
![Image Alt Text](/Architecture.jpeg)

### Links to Access the Application (Self Deployed)
> Dashboard : https://gf-dev.helo-k8s.fun    
> Fleet Manager : https://ansible-prod.helo-k8s.fun

<!-- Places it is currently deployed to : AWS, Azure, DigitalOcean, On-Prem Servers -->

### Scripts for Testing/Scaling (Docker Mandatory)
```yaml
Linux/MacOS:
    Launch:
        curl -sSL https://test0101singer.s3.ap-south-1.amazonaws.com/linux_up.sh | bash
    End:
        curl -sSL https://test0101singer.s3.ap-south-1.amazonaws.com/linux_down.sh | bash

```

```yaml
Windows:
    Launch:
        Invoke-Expression (Invoke-WebRequest -Uri " https://test0101singer.s3.ap-south-1.amazonaws.com/win_up.ps1”).Content
    End:
        Invoke-Expression (Invoke-WebRequest -Uri " https://test0101singer.s3.ap-south-1.amazonaws.com/win_down.ps1”).Content
```
