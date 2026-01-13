# Kettle
Kettle is a minimal container runtime engine designed to emulate a containerd-like architecture for performing fundamental container lifecycle operations. At its current stage, the project does not introduce custom runtime behavior; instead, its primary objective is to develop a deep, first-principles understanding of how modern container runtimes are built and operate.

To that end, Kettle intentionally avoids using containerd’s high-level abstractions or convenience APIs. While foundational components such as libcontainer, runc, and other low-level building blocks from the container ecosystem are leveraged where necessary, the long-term goal is to progressively reimplement and replace these dependencies to gain complete control and insight into the runtime stack.

Ultimately, this project aims to serve as a platform for experimentation. Once the core runtime architecture is well understood, Kettle will be extended or modified—potentially in a manner similar to Kata Containers to support specialized, niche use cases, with a particular focus on GPU-accelerated or AI/ML training workloads.


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


