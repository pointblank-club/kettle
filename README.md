# kettle
Kettle is a minimal wacky container runtime engine written to mimic a containerd like system for performing elementary actions on containers. Currently there is nothing custom about this project. I have made sure not to use any of containerd's out of the box functions to make my life easier (because where's the fun in that XD) although I will be using libcontainer, runc and other building blocks of containerd (I swear I will rewrite libcontainer too). The plan is to understand how this thing works and eventually make a tweak to it (like kata for example) to support a niche usecase (probably something gpu/ai training related but we already have beta9). If you're reading this, you're an awesome person.  

![image](https://github.com/user-attachments/assets/87e78fdf-a3f3-4528-9599-4f8f2bb80b46)
<sup><sub>

## Setup Instuctions:
The project mainly comprises of three components:
- Kettle-Shim
- KCTL
- Kettle (GRPC server)
KCTL talks to Kettle grpc server which in turn creates runc container along with it's shim process
To start this container we contact the shim server.
These three components are available in ./cmd folder. API proto buf can be found in the same folder.

First start the kettle grpc server
```
sudo ./cmd/kettle/kettle
```
Now use Kctl to specify the runc bundle path along with container-id (`test-container` for example)
```
sudo ./kctl create --bundle ~/projects/kettle/cmd/kctl/sample-container --id test-container
```
## Development instructions:
Make sure to have runc installed on your system. The given project has only been tested for linux as of now.
Run Makefile to generate protobuf for grpc and ttrpc servers.
```
make generate-proto
make build-all
```
The contents present in `./hack` folder can be used as a reference to add delete, process-as-argument and other features.
![image](https://raw.githubusercontent.com/rahulk789/kettle/refs/heads/main/assets/kettle.png)


