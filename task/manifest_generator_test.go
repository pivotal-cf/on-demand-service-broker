// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package task_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	. "github.com/pivotal-cf/on-demand-service-broker/task"
	"github.com/pivotal-cf/on-demand-service-broker/task/fakes"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

var _ = Describe("Manifest Generator", func() {
	var (
		mg               ManifestGenerator
		serviceStemcells []serviceadapter.Stemcell
		serviceReleases  serviceadapter.ServiceReleases
		serviceAdapter   *fakes.FakeServiceAdapterClient
		serviceCatalog   config.ServiceOffering

		existingPlan  config.Plan
		secondPlan    config.Plan
		oldSecretsMap map[string]string
		oldConfigsMap map[string]string

		generatedManifestSecrets serviceadapter.ODBManagedSecrets
		generatedManifestConfigs serviceadapter.BOSHConfigs
	)

	BeforeEach(func() {
		planServiceInstanceLimit := 3
		globalServiceInstanceLimit := 5

		existingPlan = config.Plan{
			ID:   existingPlanID,
			Name: existingPlanName,
			Update: &serviceadapter.Update{
				Canaries:        1,
				CanaryWatchTime: "100-200",
				UpdateWatchTime: "100-200",
				MaxInFlight:     5,
			},
			Quotas: config.Quotas{
				ServiceInstanceLimit: &planServiceInstanceLimit,
			},
			Properties: serviceadapter.Properties{
				"super": "no",
			},
			InstanceGroups: []serviceadapter.InstanceGroup{
				{
					Name:               existingPlanInstanceGroupName,
					VMType:             "vm-type",
					PersistentDiskType: "disk-type",
					Instances:          42,
					Networks:           []string{"networks"},
					AZs:                []string{"my-az1", "my-az2"},
				},
				{
					Name:      "instance-group-name-the-second",
					VMType:    "vm-type",
					Instances: 55,
					Networks:  []string{"networks2"},
				},
			},
		}

		secondPlan = config.Plan{
			ID: secondPlanID,
			Properties: serviceadapter.Properties{
				"super":             "yes",
				"a_global_property": "overrides_global_value",
			},
			InstanceGroups: []serviceadapter.InstanceGroup{
				{
					Name:               existingPlanInstanceGroupName,
					VMType:             "vm-type1",
					PersistentDiskType: "disk-type1",
					Instances:          44,
					Networks:           []string{"networks1"},
					AZs:                []string{"my-az4", "my-az5"},
				},
			},
		}

		serviceCatalog = config.ServiceOffering{
			ID:               serviceOfferingID,
			Name:             "a-cool-redis-service",
			GlobalProperties: serviceadapter.Properties{"a_global_property": "global_value", "some_other_global_property": "other_global_value"},
			GlobalQuotas: config.Quotas{
				ServiceInstanceLimit: &globalServiceInstanceLimit,
			},
			Plans: []config.Plan{
				existingPlan,
				secondPlan,
			},
		}

		serviceReleases = serviceadapter.ServiceReleases{{
			Name:    "name",
			Version: "vers",
			Jobs:    []string{"a", "b"},
		}}

		serviceStemcells = []serviceadapter.Stemcell{{
			OS:      "ubuntu-trusty",
			Version: "1234",
		}}

		serviceAdapter = new(fakes.FakeServiceAdapterClient)

		generatedManifestSecrets = serviceadapter.ODBManagedSecrets{
			"foo":    "bar",
			"secret": "value",
		}

		generatedManifestConfigs = serviceadapter.BOSHConfigs{
			"some-generated-config-type": "some-generated-config-content",
		}

		mg = NewManifestGenerator(
			serviceAdapter,
			serviceCatalog,
			serviceStemcells,
			serviceReleases,
		)

		oldSecretsMap = map[string]string{
			"one": "0n3",
			"two": `t\/\/0`,
		}

		oldConfigsMap = map[string]string{
			"some-config-type": "some-config-content",
		}
	})

	Describe("GenerateManifest", func() {
		var (
			generateManifestOutput serviceadapter.MarshalledGenerateManifest
			manifest               []byte

			err error

			planGUID       string
			previousPlanID *string
			requestParams  map[string]interface{}
			oldManifest    []byte

			expectedUAAClient map[string]string
		)

		BeforeEach(func() {
			planGUID = existingPlanID
			previousPlanID = nil

			requestParams = map[string]interface{}{"foo": "bar"}

			oldManifest = []byte("oldmanifest")

			expectedUAAClient = map[string]string{
				"avocado": "guacamole",
				"potato":  "french fries",
				"carrot":  "cake",
			}
		})

		JustBeforeEach(func() {
			generateManifestOutput, err = mg.GenerateManifest(GenerateManifestProperties{
				DeploymentName:  deploymentName,
				PlanID:          planGUID,
				RequestParams:   requestParams,
				OldManifest:     oldManifest,
				PreviousPlanID:  previousPlanID,
				SecretsMap:      oldSecretsMap,
				PreviousConfigs: oldConfigsMap,
				UAAClient:       expectedUAAClient,
			}, logger)
			manifest = []byte(generateManifestOutput.Manifest)
		})

		Context("when called with correct arguments", func() {
			generatedManifest := []byte("some manifest")
			BeforeEach(func() {
				serviceAdapter.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{Manifest: string(generatedManifest), ODBManagedSecrets: generatedManifestSecrets, Configs: generatedManifestConfigs}, nil)
			})

			It("calls service adapter once", func() {
				Expect(serviceAdapter.GenerateManifestCallCount()).To(Equal(1))
			})

			It("returns result of adapter", func() {
				Expect(manifest).To(Equal(generatedManifest))
				Expect(generateManifestOutput.ODBManagedSecrets).To(Equal(generatedManifestSecrets))
				Expect(generateManifestOutput.Configs).To(Equal(generatedManifestConfigs))
			})

			It("does not return an error", func() {
				Expect(err).To(Not(HaveOccurred()))
			})

			It("logs call to service adapter", func() {
				expectedLog := fmt.Sprintf("service adapter will generate manifest for deployment %s\n", deploymentName)
				Expect(logBuffer.String()).To(ContainSubstring(expectedLog))
			})

			It("calls the service adapter with the service deployment", func() {
				passedServiceDeployment, _, _, _, _, _, _, _, _ := serviceAdapter.GenerateManifestArgsForCall(0)
				expectedServiceDeployment := serviceadapter.ServiceDeployment{
					DeploymentName: deploymentName,
					Releases:       serviceReleases,
					Stemcells:      serviceStemcells,
				}
				Expect(passedServiceDeployment).To(Equal(expectedServiceDeployment))
			})

			It("calls the service adapter with the plan", func() {
				_, passedPlan, _, _, _, _, _, _, _ := serviceAdapter.GenerateManifestArgsForCall(0)
				Expect(passedPlan.InstanceGroups).To(Equal(existingPlan.InstanceGroups))
			})

			It("calls the service adapter with the request params", func() {
				_, _, passedRequestParams, _, _, _, _, _, _ := serviceAdapter.GenerateManifestArgsForCall(0)
				Expect(passedRequestParams).To(Equal(requestParams))
			})

			It("calls the service adapter with the old manifest", func() {
				_, _, _, passedOldManifest, _, _, _, _, _ := serviceAdapter.GenerateManifestArgsForCall(0)
				Expect(passedOldManifest).To(Equal(oldManifest))
			})

			It("calls the service adapter with the secrets map", func() {
				_, _, _, _, _, passedSecretsMap, _, _, _ := serviceAdapter.GenerateManifestArgsForCall(0)
				Expect(passedSecretsMap).To(Equal(oldSecretsMap))
			})

			It("calls the service adapter with the configs map", func() {
				_, _, _, _, _, _, passedConfigsMap, _, _ := serviceAdapter.GenerateManifestArgsForCall(0)
				Expect(passedConfigsMap).To(Equal(oldConfigsMap))
			})

			It("calls the service adapter with the provided uaa client", func() {
				_, _, _, _, _, _, _, passedUAAClient, _ := serviceAdapter.GenerateManifestArgsForCall(0)
				Expect(passedUAAClient).To(Equal(expectedUAAClient))
			})

			It("merges global and plan properties", func() {
				_, actualPlan, _, _, _, _, _, _, _ := serviceAdapter.GenerateManifestArgsForCall(0)
				expectedProperties := serviceadapter.Properties{
					"a_global_property":          "global_value",
					"some_other_global_property": "other_global_value",
					"super":                      "no",
				}
				Expect(actualPlan.Properties).To(Equal(expectedProperties))
			})

			Context("when previous plan ID is provided", func() {
				BeforeEach(func() {
					anotherPlan := secondPlanID
					previousPlanID = &anotherPlan
				})

				It("calls the service adapter with the previous plan", func() {
					_, _, _, _, passedPreviousPlan, _, _, _, _ := serviceAdapter.GenerateManifestArgsForCall(0)
					Expect(passedPreviousPlan.InstanceGroups).To(Equal(secondPlan.InstanceGroups))
				})

				It("merges global and previous plan properties, overriding global with plan props", func() {
					_, _, _, _, previousPlan, _, _, _, _ := serviceAdapter.GenerateManifestArgsForCall(0)
					expectedProperties := serviceadapter.Properties{
						"a_global_property":          "overrides_global_value",
						"some_other_global_property": "other_global_value",
						"super":                      "yes",
					}

					Expect(previousPlan.Properties).To(Equal(expectedProperties))
				})
			})

			Context("when previous plan ID is not provided", func() {
				BeforeEach(func() {
					previousPlanID = nil
				})

				It("calls the service adapter with the nil previous plan", func() {
					_, _, _, _, passedPreviousPlan, _, _, _, _ := serviceAdapter.GenerateManifestArgsForCall(0)
					Expect(passedPreviousPlan).To(BeNil())
				})
			})
		})

		Context("when the plan cannot be found", func() {
			BeforeEach(func() {
				planGUID = "invalid-id"
			})

			It("fails without generating a manifest", func() {
				Expect(serviceAdapter.GenerateManifestCallCount()).To(Equal(0))

				Expect(err).To(Equal(broker.PlanNotFoundError{PlanGUID: planGUID}))
				Expect(logBuffer.String()).To(ContainSubstring(planGUID))
			})
		})

		Context("when the previous plan cannot be found", func() {
			BeforeEach(func() {
				invalidID := "invalid-previous-id"
				previousPlanID = &invalidID
			})

			It("fails without generating a manifest", func() {
				Expect(serviceAdapter.GenerateManifestCallCount()).To(Equal(0))
				Expect(err).To(Equal(broker.PlanNotFoundError{PlanGUID: *previousPlanID}))
				Expect(logBuffer.String()).To(ContainSubstring(*previousPlanID))
			})
		})

		Context("when the adapter returns an error", func() {
			BeforeEach(func() {
				serviceAdapter.GenerateManifestReturns(serviceadapter.MarshalledGenerateManifest{}, errors.New("oops"))
			})

			It("is returned", func() {
				Expect(err).To(MatchError("oops"))
			})
		})
	})
})
