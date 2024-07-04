# File system Stress Tool

This is a simple tool to generate IO on a given file system.

To explain how to use it, let's create two Ubuntu 24.04 VMs using [Multipass](https://multipass.run/), one as an NFS Server and another as an NFS Client.

## Create Server

The following creates a server that exposes two directories, one with NFS (`/srv/nfs`) and another with SMB/CIFS (`/srv/samba`).

```bash
cat <<EOF | multipass launch --name server --cloud-init -
#cloud-config
packages:
- nfs-kernel-server
- samba
write_files:
- path: /etc/nfs.conf.d/custom.conf
  content: |
    [nfsd]
    udp=y
    tcp=y
- path: /etc/samba/share.conf
  content: |
    [share]
      comment = Shared Drive
      path = /srv/smb
      read only = no
      browsable = yes
      guest ok = yes
      writeable = yes
      create mask = 0666
      directory mask = 0775
runcmd:
- cat /etc/samba/smb.conf /etc/samba/share.conf > /etc/samba/tmp.conf
- mv /etc/samba/tmp.conf /etc/samba/smb.conf
- mkdir -p /srv/smb /srv/nfs
- chown -R 1000:1000 /srv/*
- chmod -R 777 /srv/*
- echo "/srv/nfs *(rw,all_squash,no_subtree_check)" >> /etc/exports
- systemctl enable --now nfs-kernel-server smbd nmbd
- systemctl restart smbd nmbd
- exportfs -a
EOF
```

## Create Client

The following creates a client and mount the NFS file system at `/mnt/nfs` and the CIFS one at `/mnt/smb`.

```bash
SERVER_IP=$(multipass info server | grep IPv4 | awk '{print $2}')
cat <<EOF | multipass launch --name client --cloud-init -
#cloud-config
write_files:
- path: /etc/environment
  content: |
    SERVER_IP="$SERVER_IP"
  append: true
runcmd:
- snap install go --classic
- git clone https://github.com/agalue/fsstress.git /tmp/fsstress
- cd /tmp/fsstress
- go build -o /usr/local/bin/fsstress -buildvcs=false
- apt install -y linux-modules-extra-\$(uname -r) nfs-common cifs-utils
- modprobe cifs
- mkdir -p /mnt/nfs /mnt/smb
- mount -t nfs -o sec=sys,noatime,hard,nconnect=8 $SERVER_IP:/srv/nfs /mnt/nfs
- mount -t cifs -o uid=1000,gid=1000,noatime,hard,guest //$SERVER_IP/share /mnt/smb
EOF
```

Once the VM is up, the tool's code and the shared file systems will be available. Just in case, run the following:

```bash
multipass exec client -- mount | grep "\/mnt"
```

The output should see something like this:

```
192.168.66.83:/srv/nfs on /mnt/nfs type nfs4 (rw,noatime,vers=4.2,rsize=131072,wsize=131072,namlen=255,hard,proto=tcp,nconnect=8,timeo=600,retrans=2,sec=sys,clientaddr=192.168.66.84,local_lock=none,addr=192.168.66.83)

//192.168.66.83/share on /mnt/smb type cifs (rw,relatime,vers=3.1.1,sec=none,cache=strict,uid=1000,noforceuid,gid=1000,noforcegid,addr=192.168.66.83,file_mode=0755,dir_mode=0755,hard,nounix,serverino,mapposix,rsize=4194304,wsize=4194304,bsize=1048576,retrans=1,echo_interval=60,actimeo=1,closetimeo=1)
```

> Note the number of client retransmissions for NFS over TCP defaults to 2 (with a timeout of 60 seconds) and for SMB to 1. However, for NFS over TCP, the retransmissions are handled at the protocol level, so the `retrans` parameter is ignored.

## Start the tool

The following starts the tool for the NFS folder (you can do the same for the CIFS folder from another shell):

```bash
multipass exec client -- fsstress -path /mnt/nfs -workers 4
```

While the above is running, on another shell, you can verify that the data is being copied to the server:

```bash
multipass exec server -- ls -als /srv/nfs
```

## Cleanup

```bash
multipass delete --purge client server
```
