// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package serviceadapter_test

import (
	"errors"
	"io"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	yaml "gopkg.in/yaml.v2"

	"encoding/json"

	"strings"

	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter/fakes"
	bosh "github.com/pivotal-cf/on-demand-services-sdk/bosh"
	sdk "github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("external service adapter", func() {
	const externalBinPath = "/thing"
	var validManifestContent = sdk.MarshalledGenerateManifest{
		Manifest: `name: "a-service-deployment"`,
	}

	var (
		a                 *serviceadapter.Client
		cmdRunner         *fakes.FakeCommandRunner
		logs              *gbytes.Buffer
		logger            *log.Logger
		serviceDeployment sdk.ServiceDeployment
		plan              sdk.Plan
		previousPlan      *sdk.Plan
		params            map[string]interface{}
		previousManifest  []byte

		inputParams sdk.InputParams

		manifest    []byte
		generateErr error
	)

	BeforeEach(func() {
		logs = gbytes.NewBuffer()
		logger = log.New(io.MultiWriter(GinkgoWriter, logs), "[unit-tests] ", log.LstdFlags)
		cmdRunner = new(fakes.FakeCommandRunner)
		a = &serviceadapter.Client{
			CommandRunner:   cmdRunner,
			ExternalBinPath: externalBinPath,
		}
		cmdRunner.RunReturns([]byte(validManifestContent.Manifest), []byte(""), intPtr(serviceadapter.SuccessExitCode), nil)
		cmdRunner.RunWithInputParamsReturns([]byte(toJson(validManifestContent)), []byte(""), intPtr(serviceadapter.SuccessExitCode), nil)

		serviceDeployment = sdk.ServiceDeployment{
			DeploymentName: "a-service-deployment",
			Releases: sdk.ServiceReleases{
				{Name: "a-bosh-release"},
			},
			Stemcell: sdk.Stemcell{
				OS:      "BeOS",
				Version: "2",
			},
		}

		plan = sdk.Plan{
			Properties: sdk.Properties{
				"foo": "bar",
				"baz": map[interface{}]interface{}{
					"qux": "quux",
				},
			},
		}
		params = map[string]interface{}{
			"key": "value",
			"anotherkey": map[string]interface{}{
				"innerkey": "innervalue",
			},
		}
		previousManifest = []byte("a-manifest")
		previousPlan = &sdk.Plan{
			Properties: sdk.Properties{
				"previous": "props",
				"baz": map[interface{}]interface{}{
					"qux": "quux",
				},
			},
		}

		inputParams = sdk.InputParams{
			GenerateManifest: sdk.GenerateManifestParams{
				ServiceDeployment: toJson(serviceDeployment),
				Plan:              planToJson(plan),
				PreviousPlan:      planToJson(*previousPlan),
				PreviousManifest:  string(previousManifest),
				RequestParameters: toJson(params),
			},
		}

	})

	JustBeforeEach(func() {
		manifest, generateErr = a.GenerateManifest(serviceDeployment, plan, params, previousManifest, previousPlan, logger)
	})

	It("invokes external manifest generator with serialised parameters when 'UsingStdin' not set", func() {
		serialisedServiceDeployment, err := json.Marshal(serviceDeployment)
		Expect(err).NotTo(HaveOccurred())

		plan.Properties = serviceadapter.SanitiseForJSON(plan.Properties)
		serialisedPlan, err := json.Marshal(plan)
		Expect(err).NotTo(HaveOccurred())

		serialisedParams, err := json.Marshal(params)
		Expect(err).NotTo(HaveOccurred())

		serialisedPreviousPlan, err := json.Marshal(previousPlan)
		Expect(err).NotTo(HaveOccurred())

		Expect(cmdRunner.RunCallCount()).To(Equal(1))
		argsPassed := cmdRunner.RunArgsForCall(0)
		Expect(argsPassed).To(ConsistOf(externalBinPath, "generate-manifest",
			string(serialisedServiceDeployment), string(serialisedPlan),
			string(serialisedParams), string(previousManifest), string(serialisedPreviousPlan)))
	})

	Context("when the external service adapter succeeds", func() {
		Context("when the generated manifest is valid", func() {
			It("returns no error", func() {
				Expect(generateErr).ToNot(HaveOccurred())
			})

			It("returns the deserialised stdout from the external process as a bosh manifest", func() {
				Expect(manifest).To(Equal([]byte(validManifestContent.Manifest)))
			})
		})

		Context("when the generated manifest is invalid", func() {
			Context("with an incorrect deployment name", func() {
				BeforeEach(func() {
					invalidManifestContent := "name: not-the-deployment-name-given-to-the-adapter"
					cmdRunner.RunReturns([]byte(invalidManifestContent), []byte(""), intPtr(serviceadapter.SuccessExitCode), nil)
				})

				It("returns an error", func() {
					Expect(generateErr).To(MatchError(ContainSubstring("external service adapter generated manifest with an incorrect deployment name at /thing. expected name: 'a-service-deployment', returned name: 'not-the-deployment-name-given-to-the-adapter'")))
				})
			})

			Context("with an invalid release version", func() {
				BeforeEach(func() {
					invalidManifestContent := `---
name: a-service-deployment
releases:
- version: 42.latest`
					cmdRunner.RunReturns([]byte(invalidManifestContent), []byte(""), intPtr(serviceadapter.SuccessExitCode), nil)
				})

				It("returns an error", func() {
					Expect(generateErr).To(MatchError(ContainSubstring("external service adapter generated manifest with an incorrect version at /thing. expected exact version but returned version: '42.latest'")))
				})
			})

			Context("with an invalid stemcell version", func() {
				BeforeEach(func() {
					invalidManifestContent := `---
name: a-service-deployment
stemcells:
- version: 42.latest`
					cmdRunner.RunReturns([]byte(invalidManifestContent), []byte(""), intPtr(serviceadapter.SuccessExitCode), nil)
				})

				It("returns an error", func() {
					Expect(generateErr).To(MatchError(ContainSubstring("external service adapter generated manifest with an incorrect version at /thing. expected exact version but returned version: '42.latest'")))
				})
			})

			Context("that cannot be unmarshalled", func() {
				BeforeEach(func() {
					cmdRunner.RunReturns([]byte("unparseable"), []byte(""), intPtr(serviceadapter.SuccessExitCode), nil)
				})

				It("returns an error", func() {
					Expect(generateErr).To(MatchError("external service adapter generated manifest that is not valid YAML at /thing. stderr: ''"))
				})
			})
		})
	})

	Context("when the external service adapter exits with status 10", func() {
		BeforeEach(func() {
			cmdRunner.RunReturns([]byte("I'm stdout"), []byte("I'm stderr"), intPtr(sdk.NotImplementedExitCode), nil)
		})

		It("returns an error", func() {
			Expect(generateErr).To(BeAssignableToTypeOf(serviceadapter.NotImplementedError{}))
			Expect(generateErr.Error()).NotTo(ContainSubstring("stdout"))
			Expect(generateErr.Error()).NotTo(ContainSubstring("stderr"))
		})

		It("logs a message to the operator", func() {
			Expect(logs).To(gbytes.Say("external service adapter exited with 10 at /thing: stdout: 'I'm stdout', stderr: 'I'm stderr'\n"))
		})
	})

	Context("when the external service adapter fails", func() {
		Context("when there is a operator error message and a user error message", func() {
			BeforeEach(func() {
				cmdRunner.RunReturns([]byte("I'm stdout"), []byte("I'm stderr"), intPtr(sdk.ErrorExitCode), nil)
			})

			It("returns an UnknownFailureError", func() {
				commandError, ok := generateErr.(serviceadapter.UnknownFailureError)
				Expect(ok).To(BeTrue(), "error should be an SDK GenericError")
				Expect(commandError.Error()).To(Equal("I'm stdout"))
			})

			It("logs a message to the operator", func() {
				Expect(logs).To(gbytes.Say("external service adapter exited with 1 at /thing: stdout: 'I'm stdout', stderr: 'I'm stderr'\n"))
			})
		})
	})

	Context("when the external service adapter fails, without an exit code", func() {
		var err = errors.New("oops")
		BeforeEach(func() {
			cmdRunner.RunReturns(nil, nil, nil, err)
		})

		It("returns an error", func() {
			Expect(generateErr).To(MatchError("an error occurred running external service adapter at /thing: 'oops'. stdout: '', stderr: ''"))
		})
	})

	Context("previous plan is nil", func() {
		BeforeEach(func() {
			previousPlan = nil
		})

		It("returns no error", func() {
			Expect(generateErr).ToNot(HaveOccurred())
		})

		It("it writes 'null' to the argument list", func() {
			argsPassed := cmdRunner.RunArgsForCall(0)
			Expect(argsPassed[6]).To(Equal("null"))
		})
	})

	When("UsingStdin is set to true", func() {
		BeforeEach(func() {
			a.UsingStdin = true
		})

		It("invokes external manifest generator with serialised parameters in the stdin", func() {
			Expect(cmdRunner.RunCallCount()).To(Equal(0))
			Expect(cmdRunner.RunWithInputParamsCallCount()).To(Equal(1))
			actualInputParams, argsPassed := cmdRunner.RunWithInputParamsArgsForCall(0)
			Expect(argsPassed).To(ConsistOf(
				externalBinPath,
				"generate-manifest",
			))
			Expect(actualInputParams).To(Equal(inputParams))
		})

		Context("when the external service adapter succeeds", func() {
			When("the outputted data is not valid json", func() {
				BeforeEach(func() {
					cmdRunner.RunWithInputParamsReturns([]byte("banana"), []byte(""), intPtr(serviceadapter.SuccessExitCode), nil)
				})

				It("returns an error", func() {
					Expect(generateErr).To(MatchError(ContainSubstring("invalid character 'b'")))
				})
			})

			Context("when the generated manifest is invalid", func() {
				Context("with an incorrect deployment name", func() {
					BeforeEach(func() {
						invalidManifestContent := `{"manifest":"name: not-the-deployment-name-given-to-the-adapter"}`
						cmdRunner.RunWithInputParamsReturns([]byte(invalidManifestContent), []byte(""), intPtr(serviceadapter.SuccessExitCode), nil)
					})

					It("returns an error", func() {
						Expect(generateErr).To(MatchError(ContainSubstring("external service adapter generated manifest with an incorrect deployment name at /thing. expected name: 'a-service-deployment', returned name: 'not-the-deployment-name-given-to-the-adapter'")))
					})
				})

				Context("with an invalid release version", func() {
					BeforeEach(func() {
						jsonDoc := sdk.MarshalledGenerateManifest{
							Manifest: toYaml(bosh.BoshManifest{
								Name: "a-service-deployment",
								Releases: []bosh.Release{
									{Version: "42.latest"},
								},
							}),
						}
						cmdRunner.RunWithInputParamsReturns([]byte(toJson(jsonDoc)), []byte(""), intPtr(serviceadapter.SuccessExitCode), nil)
					})

					It("returns an error", func() {
						Expect(generateErr).To(MatchError(ContainSubstring("external service adapter generated manifest with an incorrect version at /thing. expected exact version but returned version: '42.latest'")))
					})
				})

				Context("with an invalid stemcell version", func() {
					BeforeEach(func() {
						jsonDoc := sdk.MarshalledGenerateManifest{
							Manifest: toYaml(bosh.BoshManifest{
								Name: "a-service-deployment",
								Stemcells: []bosh.Stemcell{
									{Version: "42.latest"},
								},
							}),
						}
						cmdRunner.RunWithInputParamsReturns([]byte(toJson(jsonDoc)), []byte(""), intPtr(serviceadapter.SuccessExitCode), nil)
					})

					It("returns an error", func() {
						Expect(generateErr).To(MatchError(ContainSubstring("external service adapter generated manifest with an incorrect version at /thing. expected exact version but returned version: '42.latest'")))
					})
				})

				Context("that cannot be unmarshalled", func() {
					BeforeEach(func() {
						jsonDoc := sdk.MarshalledGenerateManifest{
							Manifest: "unparseable",
						}
						cmdRunner.RunWithInputParamsReturns([]byte(toJson(jsonDoc)), []byte(""), intPtr(serviceadapter.SuccessExitCode), nil)
					})

					It("returns an error", func() {
						Expect(generateErr).To(MatchError("external service adapter generated manifest that is not valid YAML at /thing. stderr: ''"))
					})
				})

			})
		})

		Context("when the external service adapter exits with status 10", func() {
			BeforeEach(func() {
				cmdRunner.RunWithInputParamsReturns([]byte("I'm stdout"), []byte("I'm stderr"), intPtr(sdk.NotImplementedExitCode), nil)
			})

			It("returns an error", func() {
				Expect(generateErr).To(BeAssignableToTypeOf(serviceadapter.NotImplementedError{}))
				Expect(generateErr.Error()).NotTo(ContainSubstring("stdout"))
				Expect(generateErr.Error()).NotTo(ContainSubstring("stderr"))
			})

			It("logs a message to the operator", func() {
				Expect(logs).To(gbytes.Say("external service adapter exited with 10 at /thing: stdout: 'I'm stdout', stderr: 'I'm stderr'\n"))
			})
		})

		Context("when the external service adapter fails", func() {
			Context("when there is a operator error message and a user error message", func() {
				BeforeEach(func() {
					cmdRunner.RunWithInputParamsReturns([]byte("I'm stdout"), []byte("I'm stderr"), intPtr(sdk.ErrorExitCode), nil)
				})

				It("returns an UnknownFailureError", func() {
					commandError, ok := generateErr.(serviceadapter.UnknownFailureError)
					Expect(ok).To(BeTrue(), "error should be an SDK GenericError")
					Expect(commandError.Error()).To(Equal("I'm stdout"))
				})

				It("logs a message to the operator", func() {
					Expect(logs).To(gbytes.Say("external service adapter exited with 1 at /thing: stdout: 'I'm stdout', stderr: 'I'm stderr'\n"))
				})
			})
		})

		Context("when the external service adapter fails, without an exit code", func() {
			var err = errors.New("oops")
			BeforeEach(func() {
				cmdRunner.RunWithInputParamsReturns(nil, nil, nil, err)
			})

			It("returns an error", func() {
				Expect(generateErr).To(MatchError("an error occurred running external service adapter at /thing: 'oops'. stdout: '', stderr: ''"))
			})
		})

		Context("previous plan is nil", func() {
			BeforeEach(func() {
				previousPlan = nil
			})

			It("it writes 'null' to the argument list", func() {
				By("not erroring")
				Expect(generateErr).ToNot(HaveOccurred())

				actualInputParams, _ := cmdRunner.RunWithInputParamsArgsForCall(0)
				Expect(actualInputParams.(sdk.InputParams).GenerateManifest.PreviousPlan).To(Equal("null"))
			})
		})
	})
})

func planToJson(plan sdk.Plan) string {
	plan.Properties = serviceadapter.SanitiseForJSON(plan.Properties)
	return toJson(plan)
}

func toJson(i interface{}) string {
	b := gbytes.NewBuffer()
	json.NewEncoder(b).Encode(i)
	return strings.TrimRight(string(b.Contents()), "\n")
}

func toYaml(i interface{}) string {
	out, err := yaml.Marshal(i)
	Expect(err).NotTo(HaveOccurred())
	return string(out)
}
