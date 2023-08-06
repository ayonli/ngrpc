echo "Creating certs folder ..."
mkdir certs && cd certs

echo "Generating certificates ..."

openssl genrsa -passout pass:1111 -des3 -out ca.key 4096

openssl req -passin pass:1111 -new -x509 -days 365 -key ca.key -out ca.crt -subj  "/C=CL/ST=RM/L=Santiago/O=Test/OU=Test/CN=ca"

openssl genrsa -passout pass:1111 -des3 -out cert.key 4096

openssl req -passin pass:1111 -new -key cert.key -out cert.csr -subj  "/C=CL/ST=RM/L=Santiago/O=Test/OU=Test/CN=localhost"

openssl x509 -passin pass:1111 -req -days 365 -in cert.csr -CA ca.crt -CAkey ca.key -set_serial 01 -out cert.crt

openssl rsa -passin pass:1111 -in cert.key -out cert.key

