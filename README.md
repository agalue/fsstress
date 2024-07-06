# File system Stress Tool

This is a simple tool to generate IO on a given file system.

To explain how to use it, let's create two Ubuntu 24.04 VMs using [Multipass](https://multipass.run/), one as an NFS Server and another as an NFS Client.

## Create Server

The following creates a server that exposes two directories, one with NFS (`/srv/nfs`) and another with SMB/CIFS (`/srv/smb`).

> Run it from the folder on which you checked out this repository.

```bash
multipass launch --name server --cloud-init cloud-init-server.yaml
```

## Create Client

The following creates a client and mounts the NFS file system at `/mnt/nfs` and the CIFS one at `/mnt/smb`.

> Run it from the folder on which you checked out this repository.

```bash
multipass launch --name client --cloud-init cloud-init-client.yaml
GOOS=linux GOARCH=amd64 go build -o fsstress .
multipass transfer fsstress client:/home/ubuntu/fsstress
```

Once the VM is up, the tool's code and the shared file systems will be available. Just in case, run the following:

```bash
multipass exec client -- mount | grep "\/mnt"
```

The output should see something like this:

```
server.local:/srv/nfs on /mnt/nfs type nfs4 (rw,noatime,vers=4.2,rsize=131072,wsize=131072,namlen=255,hard,proto=tcp,nconnect=8,timeo=600,retrans=2,sec=sys,clientaddr=192.168.66.134,local_lock=none,addr=192.168.66.133)

//server.local/share on /mnt/smb type cifs (rw,relatime,vers=3.1.1,sec=none,cache=strict,uid=1000,noforceuid,gid=1000,noforcegid,addr=192.168.66.133,file_mode=0755,dir_mode=0755,hard,nounix,serverino,mapposix,rsize=4194304,wsize=4194304,bsize=1048576,retrans=1,echo_interval=60,actimeo=1,closetimeo=1)
```

> Note the number of client retransmissions for NFS over TCP defaults to 2 (with a timeout of 60 seconds) and for SMB to 1. However, for NFS over TCP, the retransmissions are handled at the protocol level, so the `retrans` parameter is ignored.

## Start the tool

The following starts the tool for the NFS folder (you can do the same for the CIFS folder from another shell):

```bash
multipass exec client -- ./fsstress -path /mnt/nfs -workers 4
```

While the above is running, on another shell, you can verify that the data is being copied to the server:

```bash
multipass exec server -- ls -als /srv/nfs
```

From the client VM, you can use `nfsiostat` or `cifsiostat` to check the protocol-level statistics.

## Cleanup

```bash
multipass delete --purge client server
```
