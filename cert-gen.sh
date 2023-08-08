echo "Creating certs folder ..."
mkdir certs && cd certs

echo "Generating certificates ..."

openssl req -x509 -newkey rsa:4096 -nodes -days 365 -keyout certs/ca.key -out certs/ca.pem -subj "/C=CN/ST=GD/L=Shenzhen/O=HyURL/OU=AYON/CN=localhost/emailAddress=the@ayon.li"

openssl req -newkey rsa:4096 -nodes -keyout certs/cert.key -out certs/cert.pem -subj "/C=CN/ST=GD/L=Shenzhen/O=HyURL/OU=AYON/CN=localhost/emailAddress=the@ayon.li"

openssl x509 -req -in certs/cert.pem -CA certs/ca.pem -CAkey certs/ca.key -CAcreateserial -out certs/cert.pem -extfile cert-ext.conf
