package network

import "crypto/x509"

func AppendCertsFromPEM(certsPEM ...string) (*x509.CertPool, error) {
	systemCertPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	for _, cert := range certsPEM {
		systemCertPool.AppendCertsFromPEM([]byte(cert))
	}
	return systemCertPool, nil
}
