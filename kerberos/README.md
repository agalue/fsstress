# Use Kerberos

Kerberos and Samba require proper DNS configuration. We could configure a DNS server, but to simplify the procedure and because we cannot easily start VMs with Multipass on macOS with static IP addresses, the idea would be to create the VMs, update `/etc/hosts` with the assigned IP addresses, and then configure the VMs. For that reason, via cloud-init, we're just going to create setup scripts, install dependencies, and invoke them once the DNS is ready.

Run the following to start the server and client:

```bash
./start.sh
```

Access the client and then run `kinit` for your logged user (`ubuntu`, using password `U3untu;`).

> After getting the Kerberos Ticket, you should be able to access and modify files for `/mnt/smb` and `/mng/nfs`.

New users must belong to the `users` group to make changes, for instance, create the following in both servers:

```bash
sudo useradd -m -g users agalue
```

Then, on the server:

```bash
sudo kadmin.local -q "addprinc -pw @l3jandr0 agalue"
```

Finally, on the client, use `su` to change to that user, and run `kinit`
```bash
sudo su -l agalue -s /bin/bash
```

Use `kinit` to get a Kerberos Ticket using the provided credentials and then you should be able to modify the SMB and NFS volumes.
