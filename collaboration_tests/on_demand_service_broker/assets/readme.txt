
# generate server private key
openssl genrsa -out mybroker.key 2048

# generate server certificate signing request
openssl req -new -sha256 -key mybroker.key \
-subj "/C=GB/ST=London/O=Pivotal/CN=localhost" \
-out mybroker.csr

# sign CSR to create server certificate
openssl x509 -req -in mybroker.csr -CA ./bosh.ca.crt -CAkey ./bosh.ca.key \
-CAcreateserial -out mybroker.crt -days 500 -sha256
