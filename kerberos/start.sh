#!env bash

multipass launch --name server --cloud-init cloud-init-server.yaml
multipass launch --name client --cloud-init cloud-init-client.yaml

SERVER_IP=$(multipass info server --format json | jq -r '.info.server.ipv4[0]')
CLIENT_IP=$(multipass info client --format json | jq -r '.info.client.ipv4[0]')

cat <<EOF > hosts
127.0.0.1 localhost
$SERVER_IP server.agalue.io server
$CLIENT_IP client.agalue.io client
EOF
multipass transfer hosts server:/tmp/hosts
multipass transfer hosts client:/tmp/hosts

multipass exec server -- sudo /usr/local/bin/setup.sh

multipass transfer server:/tmp/krb5.keytab krb5.keytab
multipass transfer krb5.keytab client:/tmp/krb5.keytab

multipass exec client -- sudo /usr/local/bin/setup.sh

rm krb5.keytab hosts
