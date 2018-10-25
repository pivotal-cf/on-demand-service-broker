# use credhub to generate a certificate
isl-credhub generate -n collaboration_test_cert -t certificate --ca=/islington/root_ca -d 3650 -c localhost -o Pivotal -y GB -a localhost

# retrieve the certificate and private key
isl-credhub get -n /collaboration_test_cert -j | jq -r '.value.certificate' > collaboration_tests/on_demand_service_broker/assets/mybroker.crt
isl-credhub get -n /collaboration_test_cert -j | jq -r '.value.private_key' > collaboration_tests/on_demand_service_broker/assets/mybroker.key
