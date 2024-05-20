# File system Stress Tool

This is a simple tool to generate IO on a given file system.

To explain how to use it, let's create two Ubuntu 24.04 VMs using [Multipass](https://multipass.run/), one as an NFS Server and another as an NFS Client.

## Create Server

```bash
cat <<EOF | multipass launch --name nfsserver --cloud-init -
#cloud-config
packages:
- nfs-kernel-server

runcmd:
- systemctl enable --now nfs-kernel-server
- mkdir /data
- chmod 777 /data
- echo "/data *(rw,all_squash,no_subtree_check)" >> /etc/exports
- exportfs -a
EOF
```

## Create Client

```bash
SERVER_IP=$(multipass info nfsserver | grep IPv4 | awk '{print $2}')
cat <<EOF | multipass launch --name nfsclient --cloud-init -
#cloud-config
packages:
- nfs-common

mounts:
- ["$SERVER_IP:/data", "/mnt/data", "nfs", "defaults,nconnect=8,noatime,_netdev", "0", "0"]

runcmd:
- snap install go --classic
- git clone https://github.com/agalue/fsstress.git /tmp/fsstress
- cd /tmp/fsstress
- su -c 'go build -o /home/ubuntu/fsstress -buildvcs=false' ubuntu
- mount /mnt/data
EOF
```

Once the VM is up, the tool's code and the NFS file system will be available. Just in case, ensure that's the case before continuing by running the following:

```bash
multipass exec nfsclient -- mount | grep nfs
```

You should see something like this:

```
192.168.66.64:/data on /mnt/data type nfs4 (rw,noatime,vers=4.2,rsize=131072,wsize=131072,namlen=255,hard,proto=tcp,nconnect=8,timeo=600,retrans=2,sec=sys,clientaddr=192.168.66.65,local_lock=none,addr=192.168.66.64,_netdev)
```

## Start the tool

```bash
multipass exec nfsclient -- ./fsstress -path /mnt/data -workers 4
```

On another shell, you can verify that the data is being copied to the server:

```bash
multipass exec nfsserver -- ls -als /data
```

## Cleanup

```bash
multipass delete --purge nfsclient nfsserver
```