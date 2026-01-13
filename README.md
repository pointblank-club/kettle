# kettle
Kettle is a minimal wacky container runtime engine written to mimic a containerd like system for performing elementary actions on containers. Currently there is nothing custom about this project. I have made sure not to use any of containerd's out of the box functions to make my life easier (because where's the fun in that XD) although I will be using libcontainer, runc and other building blocks of containerd (I swear I will rewrite libcontainer too). The plan is to understand how this thing works and eventually make a tweak to it (like kata for example) to support a niche usecase (probably something gpu/ai training related but we already have beta9). If you're reading this, you're an awesome person.  

![image](https://github.com/user-attachments/assets/87e78fdf-a3f3-4528-9599-4f8f2bb80b46)
<sup><sub>

## Setup Instuctions:
The project mainly comprises of three components:
- Kettle-Shim
- KCTL
- Kettle (GRPC server) \
\
KCTL talks to Kettle grpc server which in turn creates runc rootfs container along with it's shim process. To start this container we contact the shim server.
These three components are available in ./cmd folder. API proto buf can be found in the same folder.

First start the kettle grpc server.
```bash
sudo ./cmd/kettle/kettle
```
Now use Kctl to specify the runc bundle path along with container-id (`sample-container` for example).
```bash
sudo ./kctl create --bundle ~/projects/kettle/cmd/kctl/sample-container --id sample-container
```
You should see something like this under kettle server after executing the create command with kctl.
```bash
[user@nixos kettle]$ sudo ./kettle
[sudo] password for user:
starting server
gRPC server started on /run/kettle/kettle.sock
function create called on grpc
function create called on grpc
Default runc spec created at: /home/user/kettle/cmd/kctl/sample-container/config.json
ERRO[0000] runc create failed: cannot allocate tty if runc will detach without setting console socket
function create called on gRPC
```
And something like this under kctl:
```bash
[user@nixos kctl]$ sudo ./kctl create --bundle ~/kettle/cmd/kctl/sample-container --id sample-container
create called
Raw socket connects but might be stale
gRPC connected successfully
<nil>
```
You have now created a runc container through grpc! This eventually needs to be automated through lifecycle management.

## Development instructions:
Make sure to have runc installed on your system. The given project has only been tested for linux as of now.
Run Makefile to generate protobuf for grpc and ttrpc servers.
```
make generate-proto
make build-all
```
The contents present in `./hack` folder can be used as a reference to add delete, process-as-argument and other features.
![image](https://raw.githubusercontent.com/rahulk789/kettle/refs/heads/main/assets/kettle.png)


