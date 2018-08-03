// Copyright (C) 2015-Present Pivotal Software, Inc. All rights reserved.

// This program and the accompanying materials are made available under
// the terms of the under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package credhub_tests

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials/values"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/permissions"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	odbcredhub "github.com/pivotal-cf/on-demand-service-broker/credhub"
	"github.com/pivotal-cf/on-demand-service-broker/task"
)

var _ = Describe("Credential store", func() {

	var (
		subject       *odbcredhub.Store
		credhubClient *credhub.CredHub
		logBuffer     *gbytes.Buffer
		logger        *log.Logger
	)

	BeforeEach(func() {
		subject = getCredhubStore()
		credhubClient = underlyingCredhubClient()
		logBuffer = gbytes.NewBuffer()
		logger = log.New(logBuffer, "contract-tests", log.LstdFlags)
	})

	Describe("Set (and delete)", func() {
		It("sets and deletes a key-value map credential", func() {
			keyPath := makeKeyPath("new-name")
			err := subject.Set(keyPath, map[string]interface{}{"hi": "there"})
			Expect(err).NotTo(HaveOccurred())

			err = subject.Delete(keyPath)
			Expect(err).NotTo(HaveOccurred())
		})

		It("can store plain string values", func() {
			keyPath := makeKeyPath("stringy-cred")
			err := subject.Set(keyPath, "I JUST LOVE CREDENTIALS.")
			Expect(err).NotTo(HaveOccurred())
		})
		It("can store JSON values", func() {
			keyPath := makeKeyPath("JSON-cred")
			err := subject.Set(keyPath, map[string]interface{}{"jsonKey": "jsonValue"})
			Expect(err).NotTo(HaveOccurred())
		})
		It("produces error when storing other types", func() {
			keyPath := makeKeyPath("esoteric-cred")
			err := subject.Set(keyPath, []interface{}{"asdf"})
			Expect(err).To(MatchError("Unknown credential type"))
		})
	})

	Describe("BulkSet", func() {
		It("sets multiple values", func() {
			path1 := makeKeyPath("secret-1")
			path2 := makeKeyPath("secret-2")
			err := subject.BulkSet([]task.ManifestSecret{
				{Name: "secret-1", Path: path1, Value: map[string]interface{}{"hi": "there"}},
				{Name: "secret-2", Path: path2, Value: "value2"},
			})
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				credhubClient.Delete(path1)
				credhubClient.Delete(path2)
			}()

			cred1, err := credhubClient.GetLatestJSON(path1)
			Expect(err).NotTo(HaveOccurred(), path1)
			cred2, err := credhubClient.GetLatestValue(path2)
			Expect(err).NotTo(HaveOccurred(), path2)

			Expect(cred1.Value).To(Equal(values.JSON{"hi": "there"}))
			Expect(cred2.Value).To(Equal(values.Value("value2")))
		})
	})

	Describe("Add permission", func() {
		It("can add permissions", func() {
			keyPath := makeKeyPath("new-name")
			err := subject.Set(keyPath, map[string]interface{}{"hi": "there"})
			Expect(err).NotTo(HaveOccurred())

			_, err = subject.AddPermissions(keyPath, []permissions.Permission{
				{Actor: "alice", Operations: []string{"read"}},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Build", func() {
		It("can't be constructed with a bad URI", func() {
			_, err := odbcredhub.Build("💩://hi.there#you", credhub.SkipTLSValidation(true))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot contain colon"))
		})
	})

	Describe("BulkGet", func() {
		var (
			jsonSecret     credentials.JSON
			passwordSecret credentials.Password
			certSecret     credentials.Certificate
			valueSecret    credentials.Value
			rsaSecret      credentials.RSA
			sshSecret      credentials.SSH
			userSecret     credentials.User
		)

		BeforeEach(func() {
			var err error
			valueSecret, err = credhubClient.SetValue("value-name", values.Value("value-secret"), "overwrite")
			Expect(err).NotTo(HaveOccurred())
			passwordSecret, err = credhubClient.SetPassword("password-name", "password", "overwrite")
			Expect(err).NotTo(HaveOccurred())
			jsonSecret, err = credhubClient.SetJSON("jsonsecret", values.JSON{"value": "foo"}, "overwrite")
			Expect(err).NotTo(HaveOccurred())
			val := values.Certificate{
				Ca:          "-----BEGIN CERTIFICATE-----\nMIIDSjCCAjKgAwIBAgIUIwnRYqjEnzeMzNYuoctat+bi818wDQYJKoZIhvcNAQEL\nBQAwGTEXMBUGA1UEAxMOdG9tLmRpY2suaGFycnkwHhcNMTgwNzE2MTU0MzQwWhcN\nMTkwNzE2MTU0MzQwWjAZMRcwFQYDVQQDEw50b20uZGljay5oYXJyeTCCASIwDQYJ\nKoZIhvcNAQEBBQADggEPADCCAQoCggEBALzyeXfpTM0ek6FVzTuOjpBYGLk2Kdl3\nAJ2gKx1FDqyeXS2Hn9nEEWAWYAQ4xvZzI1gnYm/2EXmZ1t4fY4fL6XXwjirNtOyF\n+R5UvG6uVdyfQU+FNnqnE2TQ37wNr8oWCfpoVr0T1Z9n7fPnZZg0+DRXv6x/1bzG\nqfl029bxxJMl64psR8Ew8UfrZ7zT+/URE7ex1XznwWM68rfllGaB7myPjXG6Io6I\nn7fptsCFqI7/EwofjNARIqoRwmbdpOOVz53kR0WeppfiafPsKEC0KT4hvJqgdVr7\nt4YDD4JDdCNTX/NL4BOl3pp9iBpCnz2Rk9E3tEd8JUkcjTc86KsQLYUCAwEAAaOB\niTCBhjAdBgNVHQ4EFgQU8RxuIlg9XT6/S+HDOWfUayaOvWUwVAYDVR0jBE0wS4AU\n8RxuIlg9XT6/S+HDOWfUayaOvWWhHaQbMBkxFzAVBgNVBAMTDnRvbS5kaWNrLmhh\ncnJ5ghQjCdFiqMSfN4zM1i6hy1q35uLzXzAPBgNVHRMBAf8EBTADAQH/MA0GCSqG\nSIb3DQEBCwUAA4IBAQCu50sl64yo8n8/JRDEVibFwjmJj8h+ajcFGcFK9/iBq1Do\n4q8wibMH35sP9kDTGPJqu0IPxKUBaxkzZgIFjf7ujmyv5zEVQIqj9TdJiZs1QwkA\nKUaSBsFLSH9pweZhLVOgYab/ywc3xaKiQCuLAFovFKgqhfW5K6z3XpTEwknfP2Sj\n3An9KN9ZTp+x0f85oCuB8MXHyRTBF+js1pAMdfBGD6VnAfxn3QFx72x3x7YgG2zh\nyGNByRONHukFlzraQQ986237DXdhcAedkMA+OIZl+drLbEXDuPJT/dWp255FasZ4\n+pjdblNisoHZhV3W36NWxoQycjES2siEm8xHO43f\n-----END CERTIFICATE-----\n",
				Certificate: "-----BEGIN CERTIFICATE-----\nMIIDSjCCAjKgAwIBAgIUIwnRYqjEnzeMzNYuoctat+bi818wDQYJKoZIhvcNAQEL\nBQAwGTEXMBUGA1UEAxMOdG9tLmRpY2suaGFycnkwHhcNMTgwNzE2MTU0MzQwWhcN\nMTkwNzE2MTU0MzQwWjAZMRcwFQYDVQQDEw50b20uZGljay5oYXJyeTCCASIwDQYJ\nKoZIhvcNAQEBBQADggEPADCCAQoCggEBALzyeXfpTM0ek6FVzTuOjpBYGLk2Kdl3\nAJ2gKx1FDqyeXS2Hn9nEEWAWYAQ4xvZzI1gnYm/2EXmZ1t4fY4fL6XXwjirNtOyF\n+R5UvG6uVdyfQU+FNnqnE2TQ37wNr8oWCfpoVr0T1Z9n7fPnZZg0+DRXv6x/1bzG\nqfl029bxxJMl64psR8Ew8UfrZ7zT+/URE7ex1XznwWM68rfllGaB7myPjXG6Io6I\nn7fptsCFqI7/EwofjNARIqoRwmbdpOOVz53kR0WeppfiafPsKEC0KT4hvJqgdVr7\nt4YDD4JDdCNTX/NL4BOl3pp9iBpCnz2Rk9E3tEd8JUkcjTc86KsQLYUCAwEAAaOB\niTCBhjAdBgNVHQ4EFgQU8RxuIlg9XT6/S+HDOWfUayaOvWUwVAYDVR0jBE0wS4AU\n8RxuIlg9XT6/S+HDOWfUayaOvWWhHaQbMBkxFzAVBgNVBAMTDnRvbS5kaWNrLmhh\ncnJ5ghQjCdFiqMSfN4zM1i6hy1q35uLzXzAPBgNVHRMBAf8EBTADAQH/MA0GCSqG\nSIb3DQEBCwUAA4IBAQCu50sl64yo8n8/JRDEVibFwjmJj8h+ajcFGcFK9/iBq1Do\n4q8wibMH35sP9kDTGPJqu0IPxKUBaxkzZgIFjf7ujmyv5zEVQIqj9TdJiZs1QwkA\nKUaSBsFLSH9pweZhLVOgYab/ywc3xaKiQCuLAFovFKgqhfW5K6z3XpTEwknfP2Sj\n3An9KN9ZTp+x0f85oCuB8MXHyRTBF+js1pAMdfBGD6VnAfxn3QFx72x3x7YgG2zh\nyGNByRONHukFlzraQQ986237DXdhcAedkMA+OIZl+drLbEXDuPJT/dWp255FasZ4\n+pjdblNisoHZhV3W36NWxoQycjES2siEm8xHO43f\n-----END CERTIFICATE-----\n",
				PrivateKey:  "-----BEGIN RSA PRIVATE KEY-----\nMIIEogIBAAKCAQEAvPJ5d+lMzR6ToVXNO46OkFgYuTYp2XcAnaArHUUOrJ5dLYef\n2cQRYBZgBDjG9nMjWCdib/YReZnW3h9jh8vpdfCOKs207IX5HlS8bq5V3J9BT4U2\neqcTZNDfvA2vyhYJ+mhWvRPVn2ft8+dlmDT4NFe/rH/VvMap+XTb1vHEkyXrimxH\nwTDxR+tnvNP79RETt7HVfOfBYzryt+WUZoHubI+Ncboijoift+m2wIWojv8TCh+M\n0BEiqhHCZt2k45XPneRHRZ6ml+Jp8+woQLQpPiG8mqB1Wvu3hgMPgkN0I1Nf80vg\nE6Xemn2IGkKfPZGT0Te0R3wlSRyNNzzoqxAthQIDAQABAoIBAFjfjHb0i6VnnnUi\nkJhU44XNikOD0IdzTBzYO69WziIvkxBZXLznVmzl2V/i/OLrIVLTo5+aFHon/EMa\nbIxxQ2ywK47Clzkxgw3bOY6t/cD6P5QRyqBCegLPpI0luuvJFgRsk2/4JmEGV4yD\n6OuA7sZgB84xiu1yXHzzlHwz2AyF2JL8dXe82DM33DnlERdT93pvoOgd4G65fnlw\nUVj4qMXaLlCRX3kDVyLInNfUHfTBNLAd31K2pRbNfgh6/A+hszO2lOU4jY3C6dGl\nJvcjMl/MP1flwCd8sN5OqWaSw8vvDpKy3V0T/nbvVmkxBmIRWFNUGip0tzB739m0\noMHL1/kCgYEA42d3LzYp7Kq6bDCe4DNfuEN3KfFAgCV56mjXm3IG82G+qkwE5HX5\nlzsVI6CFzgLHIC0y5k36q3PN9YV3bVBzyumBLsGqfmYpc3n0RNsBdCSYFBWx8Skm\nMO6a2MBb+DO7VAFbNj66k8zSgUSxtnNETvVmdQ8DLfvk1Ygs5DORwR8CgYEA1LUC\n8b3y+JadEHX9cTmew8Hm5eEzna8UjQsEHdmsPwDkayNzoqEQc7dyZmAvxgLmPDtt\nT6co/Js2MLgzGwjlK9/Wxl4BhWdAJltIY4T43pCnpTI5gder5lYJXDwIDU/SSp08\nrxSr0KaFfrdXeku1I//wbUpR/J+O2PBzGuLJCNsCgYB+YRQFsu5dzwxH8EV7iFGc\nEDJ7ps4X6bv1oEqi4x4lyJ6z+geGCGKrv3QiFqYGNdkAct4kzBWRj4xY9NHIeLvB\ne0AGAi+Ei7ZhrNcqJSSLrYKvNtdrlVjaPODlsRHrwKRNLWvJm9cJKP2cRdcV9L1z\nvEIysCMuPR2R5lo8gMRyNQKBgHnqIfzi7W9UDEQSDKin6Pq0mZ4qvMXlQrcwmDRv\nvc0Cuuk5kZ6mCGL6w0QwX1Fz+fiN6zJbUh+u6pl0Cj61k3zZOCXMXbzTmC4j5dK8\ntVQDv0LtDY8BSZKkv4qxEcBnftWrV8vV4kCeISem+CmtWO6AVJKfpWxRG7P15VOE\npss/AoGASRnijgkQE8cOuzoUSkYcNaKhRxo3m6OC7j2h6/Y3kLq1R9HgziEfoBpk\nkc1zdGLK02jHXLndbq07PHxNX6UctZllS/UjKNNgPgEjrGpmCy5K3CCxVR74plwo\nbbOUktEp2PuBY28iHugtbFWKqsqEx1O0r2/1tRxkEKUdKumnnYU=\n-----END RSA PRIVATE KEY-----\n",
			}
			certSecret, err = credhubClient.SetCertificate("certsecret", val, "overwrite")
			Expect(err).NotTo(HaveOccurred())
			rsaSecret, err = credhubClient.SetRSA("rsa-name", values.RSA{
				PublicKey:  "-----BEGIN RSA PUBLIC KEY-----\nMIIEogIBAAKCAQEAvPJ5d+lMzR6ToVXNO46OkFgYuTYp2XcAnaArHUUOrJ5dLYef\n2cQRYBZgBDjG9nMjWCdib/YReZnW3h9jh8vpdfCOKs207IX5HlS8bq5V3J9BT4U2\neqcTZNDfvA2vyhYJ+mhWvRPVn2ft8+dlmDT4NFe/rH/VvMap+XTb1vHEkyXrimxH\nwTDxR+tnvNP79RETt7HVfOfBYzryt+WUZoHubI+Ncboijoift+m2wIWojv8TCh+M\n0BEiqhHCZt2k45XPneRHRZ6ml+Jp8+woQLQpPiG8mqB1Wvu3hgMPgkN0I1Nf80vg\nE6Xemn2IGkKfPZGT0Te0R3wlSRyNNzzoqxAthQIDAQABAoIBAFjfjHb0i6VnnnUi\nkJhU44XNikOD0IdzTBzYO69WziIvkxBZXLznVmzl2V/i/OLrIVLTo5+aFHon/EMa\nbIxxQ2ywK47Clzkxgw3bOY6t/cD6P5QRyqBCegLPpI0luuvJFgRsk2/4JmEGV4yD\n6OuA7sZgB84xiu1yXHzzlHwz2AyF2JL8dXe82DM33DnlERdT93pvoOgd4G65fnlw\nUVj4qMXaLlCRX3kDVyLInNfUHfTBNLAd31K2pRbNfgh6/A+hszO2lOU4jY3C6dGl\nJvcjMl/MP1flwCd8sN5OqWaSw8vvDpKy3V0T/nbvVmkxBmIRWFNUGip0tzB739m0\noMHL1/kCgYEA42d3LzYp7Kq6bDCe4DNfuEN3KfFAgCV56mjXm3IG82G+qkwE5HX5\nlzsVI6CFzgLHIC0y5k36q3PN9YV3bVBzyumBLsGqfmYpc3n0RNsBdCSYFBWx8Skm\nMO6a2MBb+DO7VAFbNj66k8zSgUSxtnNETvVmdQ8DLfvk1Ygs5DORwR8CgYEA1LUC\n8b3y+JadEHX9cTmew8Hm5eEzna8UjQsEHdmsPwDkayNzoqEQc7dyZmAvxgLmPDtt\nT6co/Js2MLgzGwjlK9/Wxl4BhWdAJltIY4T43pCnpTI5gder5lYJXDwIDU/SSp08\nrxSr0KaFfrdXeku1I//wbUpR/J+O2PBzGuLJCNsCgYB+YRQFsu5dzwxH8EV7iFGc\nEDJ7ps4X6bv1oEqi4x4lyJ6z+geGCGKrv3QiFqYGNdkAct4kzBWRj4xY9NHIeLvB\ne0AGAi+Ei7ZhrNcqJSSLrYKvNtdrlVjaPODlsRHrwKRNLWvJm9cJKP2cRdcV9L1z\nvEIysCMuPR2R5lo8gMRyNQKBgHnqIfzi7W9UDEQSDKin6Pq0mZ4qvMXlQrcwmDRv\nvc0Cuuk5kZ6mCGL6w0QwX1Fz+fiN6zJbUh+u6pl0Cj61k3zZOCXMXbzTmC4j5dK8\ntVQDv0LtDY8BSZKkv4qxEcBnftWrV8vV4kCeISem+CmtWO6AVJKfpWxRG7P15VOE\npss/AoGASRnijgkQE8cOuzoUSkYcNaKhRxo3m6OC7j2h6/Y3kLq1R9HgziEfoBpk\nkc1zdGLK02jHXLndbq07PHxNX6UctZllS/UjKNNgPgEjrGpmCy5K3CCxVR74plwo\nbbOUktEp2PuBY28iHugtbFWKqsqEx1O0r2/1tRxkEKUdKumnnYU=\n-----END RSA PRIVATE KEY-----\n",
				PrivateKey: "-----BEGIN RSA PRIVATE KEY-----\nMIIEogIBAAKCAQEAvPJ5d+lMzR6ToVXNO46OkFgYuTYp2XcAnaArHUUOrJ5dLYef\n2cQRYBZgBDjG9nMjWCdib/YReZnW3h9jh8vpdfCOKs207IX5HlS8bq5V3J9BT4U2\neqcTZNDfvA2vyhYJ+mhWvRPVn2ft8+dlmDT4NFe/rH/VvMap+XTb1vHEkyXrimxH\nwTDxR+tnvNP79RETt7HVfOfBYzryt+WUZoHubI+Ncboijoift+m2wIWojv8TCh+M\n0BEiqhHCZt2k45XPneRHRZ6ml+Jp8+woQLQpPiG8mqB1Wvu3hgMPgkN0I1Nf80vg\nE6Xemn2IGkKfPZGT0Te0R3wlSRyNNzzoqxAthQIDAQABAoIBAFjfjHb0i6VnnnUi\nkJhU44XNikOD0IdzTBzYO69WziIvkxBZXLznVmzl2V/i/OLrIVLTo5+aFHon/EMa\nbIxxQ2ywK47Clzkxgw3bOY6t/cD6P5QRyqBCegLPpI0luuvJFgRsk2/4JmEGV4yD\n6OuA7sZgB84xiu1yXHzzlHwz2AyF2JL8dXe82DM33DnlERdT93pvoOgd4G65fnlw\nUVj4qMXaLlCRX3kDVyLInNfUHfTBNLAd31K2pRbNfgh6/A+hszO2lOU4jY3C6dGl\nJvcjMl/MP1flwCd8sN5OqWaSw8vvDpKy3V0T/nbvVmkxBmIRWFNUGip0tzB739m0\noMHL1/kCgYEA42d3LzYp7Kq6bDCe4DNfuEN3KfFAgCV56mjXm3IG82G+qkwE5HX5\nlzsVI6CFzgLHIC0y5k36q3PN9YV3bVBzyumBLsGqfmYpc3n0RNsBdCSYFBWx8Skm\nMO6a2MBb+DO7VAFbNj66k8zSgUSxtnNETvVmdQ8DLfvk1Ygs5DORwR8CgYEA1LUC\n8b3y+JadEHX9cTmew8Hm5eEzna8UjQsEHdmsPwDkayNzoqEQc7dyZmAvxgLmPDtt\nT6co/Js2MLgzGwjlK9/Wxl4BhWdAJltIY4T43pCnpTI5gder5lYJXDwIDU/SSp08\nrxSr0KaFfrdXeku1I//wbUpR/J+O2PBzGuLJCNsCgYB+YRQFsu5dzwxH8EV7iFGc\nEDJ7ps4X6bv1oEqi4x4lyJ6z+geGCGKrv3QiFqYGNdkAct4kzBWRj4xY9NHIeLvB\ne0AGAi+Ei7ZhrNcqJSSLrYKvNtdrlVjaPODlsRHrwKRNLWvJm9cJKP2cRdcV9L1z\nvEIysCMuPR2R5lo8gMRyNQKBgHnqIfzi7W9UDEQSDKin6Pq0mZ4qvMXlQrcwmDRv\nvc0Cuuk5kZ6mCGL6w0QwX1Fz+fiN6zJbUh+u6pl0Cj61k3zZOCXMXbzTmC4j5dK8\ntVQDv0LtDY8BSZKkv4qxEcBnftWrV8vV4kCeISem+CmtWO6AVJKfpWxRG7P15VOE\npss/AoGASRnijgkQE8cOuzoUSkYcNaKhRxo3m6OC7j2h6/Y3kLq1R9HgziEfoBpk\nkc1zdGLK02jHXLndbq07PHxNX6UctZllS/UjKNNgPgEjrGpmCy5K3CCxVR74plwo\nbbOUktEp2PuBY28iHugtbFWKqsqEx1O0r2/1tRxkEKUdKumnnYU=\n-----END RSA PRIVATE KEY-----\n",
			}, "overwrite")
			Expect(err).NotTo(HaveOccurred())
			sshSecret, err = credhubClient.SetSSH("ssh-name", values.SSH{
				PublicKey:  "-----BEGIN RSA PUBLIC KEY-----\nMIIEogIBAAKCAQEAvPJ5d+lMzR6ToVXNO46OkFgYuTYp2XcAnaArHUUOrJ5dLYef\n2cQRYBZgBDjG9nMjWCdib/YReZnW3h9jh8vpdfCOKs207IX5HlS8bq5V3J9BT4U2\neqcTZNDfvA2vyhYJ+mhWvRPVn2ft8+dlmDT4NFe/rH/VvMap+XTb1vHEkyXrimxH\nwTDxR+tnvNP79RETt7HVfOfBYzryt+WUZoHubI+Ncboijoift+m2wIWojv8TCh+M\n0BEiqhHCZt2k45XPneRHRZ6ml+Jp8+woQLQpPiG8mqB1Wvu3hgMPgkN0I1Nf80vg\nE6Xemn2IGkKfPZGT0Te0R3wlSRyNNzzoqxAthQIDAQABAoIBAFjfjHb0i6VnnnUi\nkJhU44XNikOD0IdzTBzYO69WziIvkxBZXLznVmzl2V/i/OLrIVLTo5+aFHon/EMa\nbIxxQ2ywK47Clzkxgw3bOY6t/cD6P5QRyqBCegLPpI0luuvJFgRsk2/4JmEGV4yD\n6OuA7sZgB84xiu1yXHzzlHwz2AyF2JL8dXe82DM33DnlERdT93pvoOgd4G65fnlw\nUVj4qMXaLlCRX3kDVyLInNfUHfTBNLAd31K2pRbNfgh6/A+hszO2lOU4jY3C6dGl\nJvcjMl/MP1flwCd8sN5OqWaSw8vvDpKy3V0T/nbvVmkxBmIRWFNUGip0tzB739m0\noMHL1/kCgYEA42d3LzYp7Kq6bDCe4DNfuEN3KfFAgCV56mjXm3IG82G+qkwE5HX5\nlzsVI6CFzgLHIC0y5k36q3PN9YV3bVBzyumBLsGqfmYpc3n0RNsBdCSYFBWx8Skm\nMO6a2MBb+DO7VAFbNj66k8zSgUSxtnNETvVmdQ8DLfvk1Ygs5DORwR8CgYEA1LUC\n8b3y+JadEHX9cTmew8Hm5eEzna8UjQsEHdmsPwDkayNzoqEQc7dyZmAvxgLmPDtt\nT6co/Js2MLgzGwjlK9/Wxl4BhWdAJltIY4T43pCnpTI5gder5lYJXDwIDU/SSp08\nrxSr0KaFfrdXeku1I//wbUpR/J+O2PBzGuLJCNsCgYB+YRQFsu5dzwxH8EV7iFGc\nEDJ7ps4X6bv1oEqi4x4lyJ6z+geGCGKrv3QiFqYGNdkAct4kzBWRj4xY9NHIeLvB\ne0AGAi+Ei7ZhrNcqJSSLrYKvNtdrlVjaPODlsRHrwKRNLWvJm9cJKP2cRdcV9L1z\nvEIysCMuPR2R5lo8gMRyNQKBgHnqIfzi7W9UDEQSDKin6Pq0mZ4qvMXlQrcwmDRv\nvc0Cuuk5kZ6mCGL6w0QwX1Fz+fiN6zJbUh+u6pl0Cj61k3zZOCXMXbzTmC4j5dK8\ntVQDv0LtDY8BSZKkv4qxEcBnftWrV8vV4kCeISem+CmtWO6AVJKfpWxRG7P15VOE\npss/AoGASRnijgkQE8cOuzoUSkYcNaKhRxo3m6OC7j2h6/Y3kLq1R9HgziEfoBpk\nkc1zdGLK02jHXLndbq07PHxNX6UctZllS/UjKNNgPgEjrGpmCy5K3CCxVR74plwo\nbbOUktEp2PuBY28iHugtbFWKqsqEx1O0r2/1tRxkEKUdKumnnYU=\n-----END RSA PRIVATE KEY-----\n",
				PrivateKey: "-----BEGIN RSA PRIVATE KEY-----\nMIIEogIBAAKCAQEAvPJ5d+lMzR6ToVXNO46OkFgYuTYp2XcAnaArHUUOrJ5dLYef\n2cQRYBZgBDjG9nMjWCdib/YReZnW3h9jh8vpdfCOKs207IX5HlS8bq5V3J9BT4U2\neqcTZNDfvA2vyhYJ+mhWvRPVn2ft8+dlmDT4NFe/rH/VvMap+XTb1vHEkyXrimxH\nwTDxR+tnvNP79RETt7HVfOfBYzryt+WUZoHubI+Ncboijoift+m2wIWojv8TCh+M\n0BEiqhHCZt2k45XPneRHRZ6ml+Jp8+woQLQpPiG8mqB1Wvu3hgMPgkN0I1Nf80vg\nE6Xemn2IGkKfPZGT0Te0R3wlSRyNNzzoqxAthQIDAQABAoIBAFjfjHb0i6VnnnUi\nkJhU44XNikOD0IdzTBzYO69WziIvkxBZXLznVmzl2V/i/OLrIVLTo5+aFHon/EMa\nbIxxQ2ywK47Clzkxgw3bOY6t/cD6P5QRyqBCegLPpI0luuvJFgRsk2/4JmEGV4yD\n6OuA7sZgB84xiu1yXHzzlHwz2AyF2JL8dXe82DM33DnlERdT93pvoOgd4G65fnlw\nUVj4qMXaLlCRX3kDVyLInNfUHfTBNLAd31K2pRbNfgh6/A+hszO2lOU4jY3C6dGl\nJvcjMl/MP1flwCd8sN5OqWaSw8vvDpKy3V0T/nbvVmkxBmIRWFNUGip0tzB739m0\noMHL1/kCgYEA42d3LzYp7Kq6bDCe4DNfuEN3KfFAgCV56mjXm3IG82G+qkwE5HX5\nlzsVI6CFzgLHIC0y5k36q3PN9YV3bVBzyumBLsGqfmYpc3n0RNsBdCSYFBWx8Skm\nMO6a2MBb+DO7VAFbNj66k8zSgUSxtnNETvVmdQ8DLfvk1Ygs5DORwR8CgYEA1LUC\n8b3y+JadEHX9cTmew8Hm5eEzna8UjQsEHdmsPwDkayNzoqEQc7dyZmAvxgLmPDtt\nT6co/Js2MLgzGwjlK9/Wxl4BhWdAJltIY4T43pCnpTI5gder5lYJXDwIDU/SSp08\nrxSr0KaFfrdXeku1I//wbUpR/J+O2PBzGuLJCNsCgYB+YRQFsu5dzwxH8EV7iFGc\nEDJ7ps4X6bv1oEqi4x4lyJ6z+geGCGKrv3QiFqYGNdkAct4kzBWRj4xY9NHIeLvB\ne0AGAi+Ei7ZhrNcqJSSLrYKvNtdrlVjaPODlsRHrwKRNLWvJm9cJKP2cRdcV9L1z\nvEIysCMuPR2R5lo8gMRyNQKBgHnqIfzi7W9UDEQSDKin6Pq0mZ4qvMXlQrcwmDRv\nvc0Cuuk5kZ6mCGL6w0QwX1Fz+fiN6zJbUh+u6pl0Cj61k3zZOCXMXbzTmC4j5dK8\ntVQDv0LtDY8BSZKkv4qxEcBnftWrV8vV4kCeISem+CmtWO6AVJKfpWxRG7P15VOE\npss/AoGASRnijgkQE8cOuzoUSkYcNaKhRxo3m6OC7j2h6/Y3kLq1R9HgziEfoBpk\nkc1zdGLK02jHXLndbq07PHxNX6UctZllS/UjKNNgPgEjrGpmCy5K3CCxVR74plwo\nbbOUktEp2PuBY28iHugtbFWKqsqEx1O0r2/1tRxkEKUdKumnnYU=\n-----END RSA PRIVATE KEY-----\n",
			}, "overwrite")
			Expect(err).NotTo(HaveOccurred())
			userSecret, err = credhubClient.SetUser("user-name", values.User{
				Username: "bob",
				Password: "pass",
			}, "overwrite")
			Expect(err).NotTo(HaveOccurred())

		})

		AfterEach(func() {
			Expect(credhubClient.Delete(passwordSecret.Name)).To(Succeed())
			Expect(credhubClient.Delete(jsonSecret.Name)).To(Succeed())
			Expect(credhubClient.Delete(certSecret.Name)).To(Succeed())
			Expect(credhubClient.Delete(sshSecret.Name)).To(Succeed())
			Expect(credhubClient.Delete(rsaSecret.Name)).To(Succeed())
			Expect(credhubClient.Delete(userSecret.Name)).To(Succeed())
			Expect(credhubClient.Delete(valueSecret.Name)).To(Succeed())
		})

		Describe("types returned by credhub cli library", func() {
			It("returns a password type as a string, and not a values.Password", func() {
				secret, err := credhubClient.GetLatestVersion(passwordSecret.Name)
				Expect(err).NotTo(HaveOccurred())
				secretValue := secret.Value

				_, ok := secretValue.(string)
				Expect(ok).To(BeTrue(), "secret is not a string")
				_, ok = secretValue.(values.Password)
				Expect(ok).To(BeFalse(), "secret is actually a values.Password")
			})

			It("returns a json type as a map[string]interface{}, and not a values.JSON", func() {
				secret, err := credhubClient.GetLatestVersion(jsonSecret.Name)
				Expect(err).NotTo(HaveOccurred())
				secretValue := secret.Value

				_, ok := secretValue.(map[string]interface{})
				Expect(ok).To(BeTrue(), "secret is not a map[string]interface{}")
				_, ok = secretValue.(values.JSON)
				Expect(ok).To(BeFalse(), "secret is actually a values.JSON")
			})

			It("returns a certificate type as a map[string]interface{}, and not a values.Certificate", func() {
				secret, err := credhubClient.GetLatestVersion(certSecret.Name)
				Expect(err).NotTo(HaveOccurred())
				secretValue := secret.Value

				secretValueMap, ok := secretValue.(map[string]interface{})
				Expect(ok).To(BeTrue(), "secret is not a map[string]interface{}")
				_, ok = secretValue.(values.Certificate)
				Expect(ok).To(BeFalse(), "secret is actually a values.Certificate")

				_, ok = secretValueMap["private_key"]
				Expect(ok).To(BeTrue(), "secret doesn't have a private_key key")
			})

			It("returns a value type as a string, and not a values.Value", func() {
				secret, err := credhubClient.GetLatestVersion(valueSecret.Name)
				Expect(err).NotTo(HaveOccurred())
				secretValue := secret.Value

				_, ok := secretValue.(string)
				Expect(ok).To(BeTrue(), "secret is not a string")
				_, ok = secretValue.(values.Value)
				Expect(ok).To(BeFalse(), "secret is actually a values.Value")
			})

			It("returns an SSH type as a map[string]interface{}, and not a values.SSH", func() {
				secret, err := credhubClient.GetLatestVersion(sshSecret.Name)
				Expect(err).NotTo(HaveOccurred())
				secretValue := secret.Value

				_, ok := secretValue.(map[string]interface{})
				Expect(ok).To(BeTrue(), "secret is not a map[string]interface{}")
				_, ok = secretValue.(values.SSH)
				Expect(ok).To(BeFalse(), "secret is actually a values.SSH")
			})

			It("returns an RSA type as a map[string]interface{}, and not a values.RSA", func() {
				secret, err := credhubClient.GetLatestVersion(rsaSecret.Name)
				Expect(err).NotTo(HaveOccurred())
				secretValue := secret.Value

				_, ok := secretValue.(map[string]interface{})
				Expect(ok).To(BeTrue(), "secret is not a map[string]interface{}")
				_, ok = secretValue.(values.RSA)
				Expect(ok).To(BeFalse(), "secret is actually a values.RSA")
			})

			It("returns a user type as a map[string]interface{}, and not a values.User", func() {
				secret, err := credhubClient.GetLatestVersion(userSecret.Name)
				Expect(err).NotTo(HaveOccurred())
				secretValue := secret.Value

				secretValueMap, ok := secretValue.(map[string]interface{})
				Expect(ok).To(BeTrue(), "secret is not a map[string]interface{}")
				_, ok = secretValue.(values.User)
				Expect(ok).To(BeFalse(), "secret is actually a values.User")

				_, ok = secretValueMap["username"]
				Expect(ok).To(BeTrue(), "secret does not have a 'username' key")
			})
		})

		It("can fetch secrets from credhub", func() {
			secretWithSubkeyName := strings.Join([]string{certSecret.Name, "private_key"}, ".")

			secretsToFetch := map[string]boshdirector.Variable{
				passwordSecret.Name:  {Path: passwordSecret.Name},
				jsonSecret.Name:      {Path: jsonSecret.Name, ID: jsonSecret.Id},
				certSecret.Name:      {Path: certSecret.Name},
				secretWithSubkeyName: {Path: certSecret.Name},
			}
			jsonSecretValue, err := json.Marshal(jsonSecret.Value)
			certSecretValue, err := json.Marshal(certSecret.Value)
			Expect(err).NotTo(HaveOccurred())
			expectedSecrets := map[string]string{
				passwordSecret.Name:  string(passwordSecret.Value),
				jsonSecret.Name:      string(jsonSecretValue),
				certSecret.Name:      string(certSecretValue),
				secretWithSubkeyName: string(certSecret.Value.PrivateKey),
			}

			actualSecrets, err := subject.BulkGet(secretsToFetch, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSecrets).To(Equal(expectedSecrets))
		})

		It("should use ID when present", func() {
			By("creating two versions of the same secret")
			newPasswordSecret, err := credhubClient.SetPassword(passwordSecret.Name, "newthepass", "overwrite")
			Expect(err).NotTo(HaveOccurred())

			By("fetching the secret by ID when present")
			secretsToFetch := map[string]boshdirector.Variable{
				passwordSecret.Name: {Path: "foo", ID: passwordSecret.Id},
			}
			expectedSecrets := map[string]string{
				passwordSecret.Name: string(passwordSecret.Value),
			}

			actualSecrets, err := subject.BulkGet(secretsToFetch, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSecrets).To(Equal(expectedSecrets))

			By("fetching the secret by Path when Id isn't present")
			secretsToFetch = map[string]boshdirector.Variable{
				passwordSecret.Name: {Path: passwordSecret.Name},
			}
			expectedSecrets = map[string]string{
				passwordSecret.Name: string(newPasswordSecret.Value),
			}
			actualSecrets, err = subject.BulkGet(secretsToFetch, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSecrets).To(Equal(expectedSecrets))
		})

		It("logs when the credential doesn't exist", func() {
			secretsToFetch := map[string]boshdirector.Variable{
				"blah": {Path: "blah"},
			}
			_, err := subject.BulkGet(secretsToFetch, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(logBuffer).To(gbytes.Say("Could not resolve blah"))
		})
	})

	Describe("FindNameLike", func() {
		var (
			randomGuid string
			paths      []string
		)

		BeforeEach(func() {
			randomGuid = uuid.New()[:7]
			paths = []string{
				"/odb/path/" + randomGuid + "/instance/secret",
				"/pizza/" + randomGuid + "/pie",
			}

			for p := range paths {
				_, err := credhubClient.SetValue(paths[p], values.Value("someValue"), "overwrite")
				Expect(err).NotTo(HaveOccurred())
			}
		})

		AfterEach(func() {
			for p := range paths {
				err := credhubClient.Delete(paths[p])
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("can find all secrets containing a portion of a path in their path", func() {
			actualPaths, err := subject.FindNameLike(randomGuid, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(actualPaths)).To(Equal(2))
			Expect(actualPaths).To(ConsistOf(paths))
		})
	})

	Describe("BulkDelete", func() {
		var (
			randomGuid string
			paths      []string
		)

		BeforeEach(func() {
			randomGuid = uuid.New()[:7]
			paths = []string{
				"/odb/path/" + randomGuid + "/instance/secret",
				"/pizza/" + randomGuid + "/pie",
			}

			for p := range paths {
				_, err := credhubClient.SetValue(paths[p], values.Value("someValue"), "overwrite")
				Expect(err).NotTo(HaveOccurred())
			}
		})

		AfterEach(func() {
			for p := range paths {
				credhubClient.Delete(paths[p])
			}
		})

		It("can delete all secrets", func() {
			err := subject.BulkDelete(paths, nil)
			Expect(err).NotTo(HaveOccurred())

			for p := range paths {
				_, err := credhubClient.GetLatestValue(paths[p])
				Expect(err).To(MatchError(ContainSubstring("credential does not exist")))
			}
		})
	})

})
