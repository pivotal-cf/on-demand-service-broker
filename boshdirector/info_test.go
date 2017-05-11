// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package boshdirector_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/mockbosh"
)

var _ = Describe("info", func() {
	Describe("GetDirectorVersion", func() {
		var (
			directorVersionErr error
			directorVersion    boshdirector.BoshDirectorVersion
		)

		JustBeforeEach(func() {
			directorVersion, directorVersionErr = c.GetDirectorVersion(logger)
		})

		Context("when the info request succeeds", func() {
			Context("and has a stemcell version", func() {
				BeforeEach(func() {
					director.VerifyAndMock(
						mockbosh.Info().RespondsOKWith(
							`{
		            "name": "garden-bosh",
		            "uuid": "b0f9e86f-357f-409c-8f64-a2363d2d9e3b",
		            "version": "1.3262.0.0 (00000000)",
		            "user": null,
		            "cpi": "warden_cpi",
		            "user_authentication": {
		              "type": "basic",
		              "options": {}
		            },
		            "features": {
		              "dns": {
		                "status": false,
		                "extras": {
		                  "domain_name": "bosh"
		                }
		              },
		              "compiled_package_cache": {
		                "status": false,
		                "extras": {
		                  "provider": null
		                }
		              },
		              "snapshots": {
		                "status": false
		              }
		            }
		          }`,
						),
					)
				})

				It("does not return an error", func() {
					Expect(directorVersionErr).NotTo(HaveOccurred())
				})

				It("supports ODB", func() {
					Expect(directorVersion.SupportsODB()).To(BeTrue())
				})

				It("does not support lifecycle errands", func() {
					Expect(directorVersion.SupportsLifecycleErrands()).To(BeFalse())
				})
			})

			Context("and has a semi-semver version (bosh director 260.4)", func() {
				BeforeEach(func() {
					director.VerifyAndMock(
						mockbosh.Info().RespondsOKWith(
							`{
								"name": "garden-bosh",
								"uuid": "b0f9e86f-357f-409c-8f64-a2363d2d9e3b",
								"version": "260.4 (00000000)",
								"user": null,
								"cpi": "warden_cpi",
								"user_authentication": {
									"type": "basic",
									"options": {}
								},
								"features": {
									"dns": {
										"status": false,
										"extras": {
											"domain_name": "bosh"
										}
									},
									"compiled_package_cache": {
										"status": false,
										"extras": {
											"provider": null
										}
									},
									"snapshots": {
										"status": false
									}
								}
							}`,
						),
					)
				})

				It("does not return an error", func() {
					Expect(directorVersionErr).NotTo(HaveOccurred())
				})

				It("returns a version that supports ODB", func() {
					Expect(directorVersion.SupportsODB()).To(BeTrue())
				})

				It("does not support lifecycle errands", func() {
					Expect(directorVersion.SupportsLifecycleErrands()).To(BeFalse())
				})
			})

			Context("and has a semver version less than 261", func() {
				BeforeEach(func() {
					director.VerifyAndMock(
						mockbosh.Info().RespondsOKWith(
							`{
		            "name": "garden-bosh",
		            "uuid": "b0f9e86f-357f-409c-8f64-a2363d2d9e3b",
		            "version": "260.5.0 (00000000)",
		            "user": null,
		            "cpi": "warden_cpi",
		            "user_authentication": {
		              "type": "basic",
		              "options": {}
		            },
		            "features": {
		              "dns": {
		                "status": false,
		                "extras": {
		                  "domain_name": "bosh"
		                }
		              },
		              "compiled_package_cache": {
		                "status": false,
		                "extras": {
		                  "provider": null
		                }
		              },
		              "snapshots": {
		                "status": false
		              }
		            }
		          }`,
						),
					)
				})

				It("does not return an error", func() {
					Expect(directorVersionErr).NotTo(HaveOccurred())
				})

				It("returns a version that supports ODB", func() {
					Expect(directorVersion.SupportsODB()).To(BeTrue())
				})

				It("does not support lifecycle errands", func() {
					Expect(directorVersion.SupportsLifecycleErrands()).To(BeFalse())
				})
			})

			Context("and has a semver version of 261 or greater", func() {
				BeforeEach(func() {
					director.VerifyAndMock(
						mockbosh.Info().RespondsOKWith(
							`{
		            "name": "garden-bosh",
		            "uuid": "b0f9e86f-357f-409c-8f64-a2363d2d9e3b",
		            "version": "261.0.0 (00000000)",
		            "user": null,
		            "cpi": "warden_cpi",
		            "user_authentication": {
		              "type": "basic",
		              "options": {}
		            },
		            "features": {
		              "dns": {
		                "status": false,
		                "extras": {
		                  "domain_name": "bosh"
		                }
		              },
		              "compiled_package_cache": {
		                "status": false,
		                "extras": {
		                  "provider": null
		                }
		              },
		              "snapshots": {
		                "status": false
		              }
		            }
		          }`,
						),
					)
				})

				It("does not return an error", func() {
					Expect(directorVersionErr).NotTo(HaveOccurred())
				})

				It("returns a version that supports ODB", func() {
					Expect(directorVersion.SupportsODB()).To(BeTrue())
				})

				It("does not support lifecycle errands", func() {
					Expect(directorVersion.SupportsLifecycleErrands()).To(BeTrue())
				})
			})

			Context("and the version is all zeros (e.g. bosh director 260.3)", func() {
				BeforeEach(func() {
					director.VerifyAndMock(
						mockbosh.Info().RespondsOKWith(
							`{
								"name": "garden-bosh",
								"uuid": "b0f9e86f-357f-409c-8f64-a2363d2d9e3b",
								"version": "0000 (00000000)",
								"user": null,
								"cpi": "warden_cpi",
								"user_authentication": {
									"type": "basic",
									"options": {}
								},
								"features": {
									"dns": {
										"status": false,
										"extras": {
											"domain_name": "bosh"
										}
									},
									"compiled_package_cache": {
										"status": false,
										"extras": {
											"provider": null
										}
									},
									"snapshots": {
										"status": false
									}
								}
							}`,
						),
					)
				})

				It("returns an error", func() {
					Expect(directorVersionErr).To(MatchError(`unrecognised BOSH Director version: "0000 (00000000)"`))
				})
			})

			Context("and the version is empty", func() {
				BeforeEach(func() {
					director.VerifyAndMock(
						mockbosh.Info().RespondsOKWith(
							`{
								"name": "garden-bosh",
								"uuid": "b0f9e86f-357f-409c-8f64-a2363d2d9e3b",
								"version": "",
								"user": null,
								"cpi": "warden_cpi",
								"user_authentication": {
									"type": "basic",
									"options": {}
								},
								"features": {
									"dns": {
										"status": false,
										"extras": {
											"domain_name": "bosh"
										}
									},
									"compiled_package_cache": {
										"status": false,
										"extras": {
											"provider": null
										}
									},
									"snapshots": {
										"status": false
									}
								}
							}`,
						),
					)
				})

				It("returns an error", func() {
					Expect(directorVersionErr).To(MatchError(`unrecognised BOSH Director version: ""`))
				})
			})

			Context("and the major version is not a integer", func() {
				BeforeEach(func() {
					director.VerifyAndMock(
						mockbosh.Info().RespondsOKWith(
							`{
								"name": "garden-bosh",
								"uuid": "b0f9e86f-357f-409c-8f64-a2363d2d9e3b",
								"version": "drone.ver",
								"user": null,
								"cpi": "warden_cpi",
								"user_authentication": {
									"type": "basic",
									"options": {}
								},
								"features": {
									"dns": {
										"status": false,
										"extras": {
											"domain_name": "bosh"
										}
									},
									"compiled_package_cache": {
										"status": false,
										"extras": {
											"provider": null
										}
									},
									"snapshots": {
										"status": false
									}
								}
							}`,
						),
					)
				})

				It("returns an error", func() {
					Expect(directorVersionErr).To(MatchError(`unrecognised BOSH Director version: "drone.ver"`))
				})
			})
		})

		Context("when the info request fails", func() {
			BeforeEach(func() {
				director.VerifyAndMock(
					mockbosh.Info().RespondsInternalServerErrorWith("nothing today, thank you"),
				)
			})

			It("returns an error", func() {
				Expect(directorVersionErr).To(MatchError("expected status 200, was 500. Response Body: nothing today, thank you"))
			})
		})
	})
})
