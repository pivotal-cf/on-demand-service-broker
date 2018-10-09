package credhub_test

import (
	"errors"
	"fmt"
	"io"
	"log"

	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
	"code.cloudfoundry.org/credhub-cli/credhub/credentials/values"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/credhub"
	"github.com/pivotal-cf/on-demand-service-broker/credhub/fakes"
)

var _ = Describe("CredStore", func() {
	var (
		fakeCredhubClient *fakes.FakeCredhubClient
		store             *credhub.Store
	)

	BeforeEach(func() {
		fakeCredhubClient = new(fakes.FakeCredhubClient)
		store = credhub.New(fakeCredhubClient)
	})

	Describe("Bulk Get", func() {
		var (
			logBuffer *gbytes.Buffer
			logger    *log.Logger
		)

		BeforeEach(func() {
			logBuffer = gbytes.NewBuffer()
			logger = log.New(io.Writer(logBuffer), "my-app", log.LstdFlags)
		})

		var (
			exampleCertificate = map[string]interface{}{
				"ca":          "-----BEGIN CERTIFICATE-----\nMIIDSjCCAjKgAwIBAgIUIwnRYqjEnzeMzNYuoctat+bi818wDQYJKoZIhvcNAQEL\nBQAwGTEXMBUGA1UEAxMOdG9tLmRpY2suaGFycnkwHhcNMTgwNzE2MTU0MzQwWhcN\nMTkwNzE2MTU0MzQwWjAZMRcwFQYDVQQDEw50b20uZGljay5oYXJyeTCCASIwDQYJ\nKoZIhvcNAQEBBQADggEPADCCAQoCggEBALzyeXfpTM0ek6FVzTuOjpBYGLk2Kdl3\nAJ2gKx1FDqyeXS2Hn9nEEWAWYAQ4xvZzI1gnYm/2EXmZ1t4fY4fL6XXwjirNtOyF\n+R5UvG6uVdyfQU+FNnqnE2TQ37wNr8oWCfpoVr0T1Z9n7fPnZZg0+DRXv6x/1bzG\nqfl029bxxJMl64psR8Ew8UfrZ7zT+/URE7ex1XznwWM68rfllGaB7myPjXG6Io6I\nn7fptsCFqI7/EwofjNARIqoRwmbdpOOVz53kR0WeppfiafPsKEC0KT4hvJqgdVr7\nt4YDD4JDdCNTX/NL4BOl3pp9iBpCnz2Rk9E3tEd8JUkcjTc86KsQLYUCAwEAAaOB\niTCBhjAdBgNVHQ4EFgQU8RxuIlg9XT6/S+HDOWfUayaOvWUwVAYDVR0jBE0wS4AU\n8RxuIlg9XT6/S+HDOWfUayaOvWWhHaQbMBkxFzAVBgNVBAMTDnRvbS5kaWNrLmhh\ncnJ5ghQjCdFiqMSfN4zM1i6hy1q35uLzXzAPBgNVHRMBAf8EBTADAQH/MA0GCSqG\nSIb3DQEBCwUAA4IBAQCu50sl64yo8n8/JRDEVibFwjmJj8h+ajcFGcFK9/iBq1Do\n4q8wibMH35sP9kDTGPJqu0IPxKUBaxkzZgIFjf7ujmyv5zEVQIqj9TdJiZs1QwkA\nKUaSBsFLSH9pweZhLVOgYab/ywc3xaKiQCuLAFovFKgqhfW5K6z3XpTEwknfP2Sj\n3An9KN9ZTp+x0f85oCuB8MXHyRTBF+js1pAMdfBGD6VnAfxn3QFx72x3x7YgG2zh\nyGNByRONHukFlzraQQ986237DXdhcAedkMA+OIZl+drLbEXDuPJT/dWp255FasZ4\n+pjdblNisoHZhV3W36NWxoQycjES2siEm8xHO43f\n-----END CERTIFICATE-----\n",
				"ca_name":     "Henry",
				"certificate": "-----BEGIN CERTIFICATE-----\nMIIDSjCCAjKgAwIBAgIUIwnRYqjEnzeMzNYuoctat+bi818wDQYJKoZIhvcNAQEL\nBQAwGTEXMBUGA1UEAxMOdG9tLmRpY2suaGFycnkwHhcNMTgwNzE2MTU0MzQwWhcN\nMTkwNzE2MTU0MzQwWjAZMRcwFQYDVQQDEw50b20uZGljay5oYXJyeTCCASIwDQYJ\nKoZIhvcNAQEBBQADggEPADCCAQoCggEBALzyeXfpTM0ek6FVzTuOjpBYGLk2Kdl3\nAJ2gKx1FDqyeXS2Hn9nEEWAWYAQ4xvZzI1gnYm/2EXmZ1t4fY4fL6XXwjirNtOyF\n+R5UvG6uVdyfQU+FNnqnE2TQ37wNr8oWCfpoVr0T1Z9n7fPnZZg0+DRXv6x/1bzG\nqfl029bxxJMl64psR8Ew8UfrZ7zT+/URE7ex1XznwWM68rfllGaB7myPjXG6Io6I\nn7fptsCFqI7/EwofjNARIqoRwmbdpOOVz53kR0WeppfiafPsKEC0KT4hvJqgdVr7\nt4YDD4JDdCNTX/NL4BOl3pp9iBpCnz2Rk9E3tEd8JUkcjTc86KsQLYUCAwEAAaOB\niTCBhjAdBgNVHQ4EFgQU8RxuIlg9XT6/S+HDOWfUayaOvWUwVAYDVR0jBE0wS4AU\n8RxuIlg9XT6/S+HDOWfUayaOvWWhHaQbMBkxFzAVBgNVBAMTDnRvbS5kaWNrLmhh\ncnJ5ghQjCdFiqMSfN4zM1i6hy1q35uLzXzAPBgNVHRMBAf8EBTADAQH/MA0GCSqG\nSIb3DQEBCwUAA4IBAQCu50sl64yo8n8/JRDEVibFwjmJj8h+ajcFGcFK9/iBq1Do\n4q8wibMH35sP9kDTGPJqu0IPxKUBaxkzZgIFjf7ujmyv5zEVQIqj9TdJiZs1QwkA\nKUaSBsFLSH9pweZhLVOgYab/ywc3xaKiQCuLAFovFKgqhfW5K6z3XpTEwknfP2Sj\n3An9KN9ZTp+x0f85oCuB8MXHyRTBF+js1pAMdfBGD6VnAfxn3QFx72x3x7YgG2zh\nyGNByRONHukFlzraQQ986237DXdhcAedkMA+OIZl+drLbEXDuPJT/dWp255FasZ4\n+pjdblNisoHZhV3W36NWxoQycjES2siEm8xHO43f\n-----END CERTIFICATE-----\n",
				"private_key": "-----BEGIN RSA PRIVATE KEY-----\nMIIEogIBAAKCAQEAvPJ5d+lMzR6ToVXNO46OkFgYuTYp2XcAnaArHUUOrJ5dLYef\n2cQRYBZgBDjG9nMjWCdib/YReZnW3h9jh8vpdfCOKs207IX5HlS8bq5V3J9BT4U2\neqcTZNDfvA2vyhYJ+mhWvRPVn2ft8+dlmDT4NFe/rH/VvMap+XTb1vHEkyXrimxH\nwTDxR+tnvNP79RETt7HVfOfBYzryt+WUZoHubI+Ncboijoift+m2wIWojv8TCh+M\n0BEiqhHCZt2k45XPneRHRZ6ml+Jp8+woQLQpPiG8mqB1Wvu3hgMPgkN0I1Nf80vg\nE6Xemn2IGkKfPZGT0Te0R3wlSRyNNzzoqxAthQIDAQABAoIBAFjfjHb0i6VnnnUi\nkJhU44XNikOD0IdzTBzYO69WziIvkxBZXLznVmzl2V/i/OLrIVLTo5+aFHon/EMa\nbIxxQ2ywK47Clzkxgw3bOY6t/cD6P5QRyqBCegLPpI0luuvJFgRsk2/4JmEGV4yD\n6OuA7sZgB84xiu1yXHzzlHwz2AyF2JL8dXe82DM33DnlERdT93pvoOgd4G65fnlw\nUVj4qMXaLlCRX3kDVyLInNfUHfTBNLAd31K2pRbNfgh6/A+hszO2lOU4jY3C6dGl\nJvcjMl/MP1flwCd8sN5OqWaSw8vvDpKy3V0T/nbvVmkxBmIRWFNUGip0tzB739m0\noMHL1/kCgYEA42d3LzYp7Kq6bDCe4DNfuEN3KfFAgCV56mjXm3IG82G+qkwE5HX5\nlzsVI6CFzgLHIC0y5k36q3PN9YV3bVBzyumBLsGqfmYpc3n0RNsBdCSYFBWx8Skm\nMO6a2MBb+DO7VAFbNj66k8zSgUSxtnNETvVmdQ8DLfvk1Ygs5DORwR8CgYEA1LUC\n8b3y+JadEHX9cTmew8Hm5eEzna8UjQsEHdmsPwDkayNzoqEQc7dyZmAvxgLmPDtt\nT6co/Js2MLgzGwjlK9/Wxl4BhWdAJltIY4T43pCnpTI5gder5lYJXDwIDU/SSp08\nrxSr0KaFfrdXeku1I//wbUpR/J+O2PBzGuLJCNsCgYB+YRQFsu5dzwxH8EV7iFGc\nEDJ7ps4X6bv1oEqi4x4lyJ6z+geGCGKrv3QiFqYGNdkAct4kzBWRj4xY9NHIeLvB\ne0AGAi+Ei7ZhrNcqJSSLrYKvNtdrlVjaPODlsRHrwKRNLWvJm9cJKP2cRdcV9L1z\nvEIysCMuPR2R5lo8gMRyNQKBgHnqIfzi7W9UDEQSDKin6Pq0mZ4qvMXlQrcwmDRv\nvc0Cuuk5kZ6mCGL6w0QwX1Fz+fiN6zJbUh+u6pl0Cj61k3zZOCXMXbzTmC4j5dK8\ntVQDv0LtDY8BSZKkv4qxEcBnftWrV8vV4kCeISem+CmtWO6AVJKfpWxRG7P15VOE\npss/AoGASRnijgkQE8cOuzoUSkYcNaKhRxo3m6OC7j2h6/Y3kLq1R9HgziEfoBpk\nkc1zdGLK02jHXLndbq07PHxNX6UctZllS/UjKNNgPgEjrGpmCy5K3CCxVR74plwo\nbbOUktEp2PuBY28iHugtbFWKqsqEx1O0r2/1tRxkEKUdKumnnYU=\n-----END RSA PRIVATE KEY-----\n",
			}
		)

		DescribeTable("reading all the different credential types with optional subkeys", func(subkey, resolvedSecret string, credhubSecretValue interface{}) {
			ref := "someName"
			if subkey != "" {
				ref += "." + subkey
			}
			ref = "((" + ref + "))"
			expectedSecrets := map[string]string{
				ref: resolvedSecret,
			}
			secretsToFetch := map[string]boshdirector.Variable{
				ref: {
					Path: "/path/to/someName",
				},
			}
			fakeCredhubClient.GetLatestVersionReturns(credentials.Credential{
				Value: credhubSecretValue,
			}, nil)

			secrets, err := store.BulkGet(secretsToFetch, logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(secrets).To(Equal(expectedSecrets))
		},

			Entry("scalar secret", "", "my-secret", "my-secret"),
			Entry("struct secret", "", toJson(exampleCertificate), exampleCertificate),

			Entry("struct with subkey", "private_key", exampleCertificate["private_key"], exampleCertificate),
		)

		DescribeTable("reading all the different credential types when an ID is known", func(resolvedSecret string, credhubSecretValue interface{}) {
			ref := "((someName))"
			expectedSecrets := map[string]string{
				ref: resolvedSecret,
			}
			secretsToFetch := map[string]boshdirector.Variable{
				ref: {
					ID:   "1311",
					Path: "/path/to/someName",
				},
			}
			fakeCredhubClient.GetByIdReturns(credentials.Credential{
				Value: credhubSecretValue,
			}, nil)

			secrets, err := store.BulkGet(secretsToFetch, logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(secrets).To(Equal(expectedSecrets))
		},

			Entry("scalar secret", "my-secret", "my-secret"),
			Entry("struct secret", toJson(exampleCertificate), exampleCertificate),
		)

		DescribeTable("sub keys errors when sub key is not defined on particular credhub value object", func(credType string, credhubSecretValue interface{}) {
			ref := "((someName.badsubkey))"
			secretsToFetch := map[string]boshdirector.Variable{
				ref: {
					Path: "/path/to/someName",
				},
			}
			fakeCredhubClient.GetLatestVersionReturns(credentials.Credential{
				Value: credhubSecretValue,
			}, nil)

			result, err := store.BulkGet(secretsToFetch, logger)
			Expect(err).NotTo(HaveOccurred())

			_, ok := result[ref]
			Expect(ok).To(BeFalse())

			if credType == "map" {
				Expect(logBuffer).To(gbytes.Say("credential does not contain key 'badsubkey'"))
			} else {
				Expect(logBuffer).To(gbytes.Say("string type credential cannot have key 'badsubkey'"))
			}
		},

			Entry("certificate bad subkey", "map", exampleCertificate),
			Entry("scalar any subkey", "Value", "arnold"),
		)

		It("returns multiple values in secrets mapped if asked for more than one secret", func() {
			secretsToFetch := map[string]boshdirector.Variable{
				"((one))": {Path: "/foo"},
				"((two))": {Path: "/bar"},
			}
			fakeCredhubClient.GetLatestVersionStub = func(name string) (credentials.Credential, error) {
				if name == "/foo" {
					return credentials.Credential{Value: "foo-val"}, nil
				}
				if name == "/bar" {
					return credentials.Credential{Value: "bar-val"}, nil
				}
				return credentials.Credential{}, nil
			}
			result, err := store.BulkGet(secretsToFetch, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
			Expect(result["((one))"]).To(Equal("foo-val"))
			Expect(result["((two))"]).To(Equal("bar-val"))
		})

		It("errors when retrieving a subkey for an unknown credential type", func() {
			ref := "((someName.subkey))"
			secretsToFetch := map[string]boshdirector.Variable{
				ref: {
					Path: "/path/to/someName",
				},
			}
			fakeCredhubClient.GetLatestVersionReturns(credentials.Credential{
				Value: 5,
			}, nil)

			result, err := store.BulkGet(secretsToFetch, logger)
			Expect(err).NotTo(HaveOccurred())

			_, ok := result[ref]
			Expect(ok).To(BeFalse())

			Expect(logBuffer).To(gbytes.Say(fmt.Sprintf("unknown credential type")))
		})

		It("logs problem and doesn't include secret when no ID and path does not exist in credhub", func() {
			fakeCredhubClient.GetLatestVersionReturns(credentials.Credential{}, errors.New("oops"))
			secretsToFetch := map[string]boshdirector.Variable{
				"((somePath))": {Path: "/path/to/somePath"},
			}
			secrets, err := store.BulkGet(secretsToFetch, logger)
			Expect(err).NotTo(HaveOccurred())
			_, ok := secrets["((somePath))"]
			Expect(ok).To(BeFalse(), "somePath should not be returned in secrets")
			Expect(logBuffer).To(gbytes.Say(`Could not resolve \(\(somePath\)\): oops`))
		})

		It("logs problem and doesn't include secret when ID is known but does not exist in credhub", func() {
			fakeCredhubClient.GetByIdReturns(credentials.Credential{}, errors.New("oops"))
			secretsToFetch := map[string]boshdirector.Variable{
				"((somePath))": {Path: "/path/to/somePath", ID: "31313"},
			}
			secrets, err := store.BulkGet(secretsToFetch, logger)
			Expect(err).NotTo(HaveOccurred())
			_, ok := secrets["((somePath))"]
			Expect(ok).To(BeFalse(), "somePath should not be returned in secrets")
			Expect(logBuffer).To(gbytes.Say(`Could not resolve \(\(somePath\)\): oops`))
		})
	})

	Describe("Set", func() {
		It("can set a json secret", func() {
			secret := map[string]interface{}{}
			secret["foo"] = "bar"
			err := store.Set("/path/to/secret", secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCredhubClient.SetJSONCallCount()).To(Equal(1))
			path, val := fakeCredhubClient.SetJSONArgsForCall(0)
			Expect(path).To(Equal("/path/to/secret"))
			Expect(val).To(Equal(values.JSON(secret)))
		})

		It("can set a string secret", func() {
			err := store.Set("/path/to/secret", "caravan")
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCredhubClient.SetValueCallCount()).To(Equal(1))
			path, val := fakeCredhubClient.SetValueArgsForCall(0)
			Expect(path).To(Equal("/path/to/secret"))
			Expect(val).To(Equal(values.Value("caravan")))
		})

		It("errors if not a JSON or string secret", func() {
			err := store.Set("/path/to/secret", make(chan int))
			Expect(err).To(MatchError("Unknown credential type"))
		})
	})

	Describe("Delete", func() {
		It("can delete a credhub secret at path p", func() {
			p := "/some/path"
			store.Delete(p)
			Expect(fakeCredhubClient.DeleteCallCount()).To(Equal(1))
			Expect(fakeCredhubClient.DeleteArgsForCall(0)).To(Equal(p))
		})

		It("returns an error if the underlying call fails", func() {
			fakeCredhubClient.DeleteReturns(errors.New("you what?"))
			err := store.Delete("something")
			Expect(err).To(MatchError("you what?"))
		})
	})

	Describe("Add Permission", func() {
		It("can add permissions to a path", func() {
			p := "/some/path"
			expectedActor := "jim"
			expectedOps := []string{"read", "corrupt"}

			_, err := store.AddPermission(p, expectedActor, expectedOps)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCredhubClient.AddPermissionCallCount()).To(Equal(1))
			actualName, actualActor, actualOps := fakeCredhubClient.AddPermissionArgsForCall(0)
			Expect(actualName).To(Equal(p))
			Expect(actualActor).To(Equal(expectedActor))
			Expect(actualOps).To(Equal(expectedOps))
		})

		It("returns an error if the underlying call fails", func() {
			p := "/some/path"
			expectedActor := "jim"
			expectedOps := []string{"read", "corrupt"}
			fakeCredhubClient.AddPermissionReturns(nil, errors.New("you're joking, right?"))
			_, err := store.AddPermission(p, expectedActor, expectedOps)
			Expect(err).To(MatchError("you're joking, right?"))
		})
	})

	Describe("BulkSet", func() {
		It("does not set anything when called with an empty secrets map", func() {
			secretsToSet := []broker.ManifestSecret{}

			err := store.BulkSet(secretsToSet)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCredhubClient.SetJSONCallCount()).To(Equal(0), "SetJSON was called")
			Expect(fakeCredhubClient.SetValueCallCount()).To(Equal(0), "SetValue was called")
		})

		It("stores all secrets", func() {
			secretsToSet := []broker.ManifestSecret{
				{Name: "foo", Path: "/foo/foo", Value: "123"},
				{Name: "bar", Path: "/foo/bar", Value: map[string]interface{}{"key": "value"}},
			}

			err := store.BulkSet(secretsToSet)
			Expect(err).NotTo(HaveOccurred())

			By("calling SetJSON for JSON values")
			Expect(fakeCredhubClient.SetJSONCallCount()).To(Equal(1), "SetJSON wasn't called")
			jsonPath, jsonValue := fakeCredhubClient.SetJSONArgsForCall(0)
			Expect(jsonPath).To(Equal("/foo/bar"))
			Expect(jsonValue).To(Equal(values.JSON(map[string]interface{}{"key": "value"})))

			By("calling SetValue for string values")
			Expect(fakeCredhubClient.SetValueCallCount()).To(Equal(1), "SetValue wasn't called")
			strPath, strValue := fakeCredhubClient.SetValueArgsForCall(0)
			Expect(strPath).To(Equal("/foo/foo"))
			Expect(strValue).To(Equal(values.Value("123")))
		})

		It("errors when one of the credentials is of an unsupported type", func() {
			secretsToSet := []broker.ManifestSecret{
				{Name: "bar", Path: "/foo/bar", Value: map[string]interface{}{"key": "value"}},
				{Name: "foo", Path: "/foo/foo", Value: make(chan bool)},
			}

			err := store.BulkSet(secretsToSet)
			Expect(err).To(MatchError("Unknown credential type"))
		})

		It("errors when fail to store json secrets", func() {
			secretsToSet := []broker.ManifestSecret{
				{Name: "bar", Path: "/foo/bar", Value: map[string]interface{}{"key": "value"}},
			}

			fakeCredhubClient.SetJSONReturns(credentials.JSON{}, errors.New("can't do it right now"))
			err := store.BulkSet(secretsToSet)
			Expect(err).To(MatchError("can't do it right now"))
		})

		It("errors when fail to store string secrets", func() {
			secretsToSet := []broker.ManifestSecret{
				{Name: "bar", Path: "/foo/bar", Value: "value"},
			}

			fakeCredhubClient.SetValueReturns(credentials.Value{}, errors.New("too busy, sorry"))
			err := store.BulkSet(secretsToSet)
			Expect(err).To(MatchError("too busy, sorry"))
		})
	})

	Describe("FindNameLike", func() {
		It("can find all secrets containing a portion of a path in their path", func() {
			fakeCredhubClient.FindByPartialNameReturns(credentials.FindResults{
				Credentials: []credentials.Base{
					{Name: "/tofu/path"},
					{Name: "/not-real-cheese/tofu/other/path"},
				},
			}, nil)

			actualPaths, err := store.FindNameLike("tofu", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualPaths).To(ConsistOf([]string{
				"/tofu/path",
				"/not-real-cheese/tofu/other/path",
			}))
		})

		It("returns an error when there is an error with the credhub client", func() {
			fakeCredhubClient.FindByPartialNameReturns(credentials.FindResults{}, errors.New("couldn't do it"))

			_, err := store.FindNameLike("tofu", nil)
			Expect(err).To(MatchError("couldn't do it"))
		})
	})

	Describe("BulkDelete", func() {
		var (
			logBuffer *gbytes.Buffer
			logger    *log.Logger
		)

		BeforeEach(func() {
			logBuffer = gbytes.NewBuffer()
			logger = log.New(io.Writer(logBuffer), "my-app", log.LstdFlags)
		})

		It("deletes all the secrets for the path provided", func() {
			secretsToDelete := []string{"/some/path/secret", "/some/path/another_secret"}
			fakeCredhubClient.DeleteReturns(nil)

			err := store.BulkDelete(secretsToDelete, logger)

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCredhubClient.DeleteCallCount()).To(Equal(2))
			Expect(fakeCredhubClient.DeleteArgsForCall(0)).To(Equal("/some/path/secret"))
			Expect(fakeCredhubClient.DeleteArgsForCall(1)).To(Equal("/some/path/another_secret"))
		})

		It("logs an error if a call to delete fails", func() {
			secretsToDelete := []string{"/some/path/secret", "/some/path/another_secret"}
			fakeCredhubClient.DeleteReturns(errors.New("too difficult to delete"))

			err := store.BulkDelete(secretsToDelete, logger)

			Expect(err).To(MatchError("too difficult to delete"))
			Expect(logBuffer).To(gbytes.Say("could not delete secret '/some/path/secret': too difficult to delete"))
		})
	})
})
