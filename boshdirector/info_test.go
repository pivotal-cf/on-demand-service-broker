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
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockbosh"
)

var _ = Describe("info", func() {
	Describe("GetInfo", func() {
		It("returns a info object data structure", func() {
			director.VerifyAndMock(
				mockbosh.Info().RespondsOKWith(
					`{
						"name": "garden-bosh",
						"uuid": "b0f9e86f-357f-409c-8f64-a2363d2d9e3b",
						"version": "1.3262.0.0 (00000000)",
						"user": null,
						"cpi": "warden_cpi",
						"user_authentication": {
							"type": "uaa",
							"options": {
								"url": "https://this-is-the-uaa-url.example.com"
							}
						}
					}`,
				),
			)
			info, err := c.GetInfo(logger)
			Expect(err).NotTo(HaveOccurred())
			expectedInfo := boshdirector.Info{
				Version: "1.3262.0.0 (00000000)",
				UserAuthentication: boshdirector.UserAuthentication{
					Options: boshdirector.AuthenticationOptions{
						URL: "https://this-is-the-uaa-url.example.com",
					},
				},
			}
			Expect(info).To(Equal(expectedInfo))
		})

		It("returns an error if the request fails", func() {
			director.VerifyAndMock(
				mockbosh.Info().RespondsInternalServerErrorWith("nothing today, thank you"),
			)
			_, err := c.GetInfo(logger)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("GetDirectorVersion", func() {
		It("returns stemcell version when it has a stemcell version", func() {
			boshInfo := createBoshInfoWithVersion("1.3262.0.0 (00000000)")
			directorVersion, directorVersionErr := boshInfo.GetDirectorVersion()
			Expect(directorVersionErr).NotTo(HaveOccurred())
			Expect(directorVersion).To(Equal(boshdirector.Version{VersionType: "stemcell", MajorVersion: 3262}))
		})

		It("returns a semver version when it has a semi-semver version (bosh director 260.4)", func() {
			boshInfo := createBoshInfoWithVersion("260.4 (00000000)")
			directorVersion, directorVersionErr := boshInfo.GetDirectorVersion()
			Expect(directorVersionErr).NotTo(HaveOccurred())
			Expect(directorVersion).To(Equal(boshdirector.Version{VersionType: "semver", MajorVersion: 260}))
		})

		It("returns a semver version when it has a semver version less than 261", func() {
			boshInfo := createBoshInfoWithVersion("260.5.0 (00000000)")
			directorVersion, directorVersionErr := boshInfo.GetDirectorVersion()
			Expect(directorVersionErr).NotTo(HaveOccurred())
			Expect(directorVersion).To(Equal(boshdirector.Version{VersionType: "semver", MajorVersion: 260}))
		})

		It("returns a semver version when it has a semver version of 261 or greater", func() {
			boshInfo := createBoshInfoWithVersion("261.0.0 (00000000)")
			directorVersion, directorVersionErr := boshInfo.GetDirectorVersion()
			Expect(directorVersionErr).NotTo(HaveOccurred())
			Expect(directorVersion).To(Equal(boshdirector.Version{VersionType: "semver", MajorVersion: 261}))
		})

		It("returns an error if version is all zeros", func() {
			boshInfo := createBoshInfoWithVersion("0000 (00000000)")
			_, directorVersionErr := boshInfo.GetDirectorVersion()
			Expect(directorVersionErr).To(HaveOccurred())
			Expect(directorVersionErr).To(MatchError(`unrecognised BOSH Director version: "0000 (00000000)"`))
		})

		It("returns an error if version is empty", func() {
			boshInfo := createBoshInfoWithVersion("")
			_, directorVersionErr := boshInfo.GetDirectorVersion()
			Expect(directorVersionErr).To(HaveOccurred())
			Expect(directorVersionErr).To(MatchError(`unrecognised BOSH Director version: ""`))
		})

		It("returns an error if the major version is not an integer", func() {
			boshInfo := createBoshInfoWithVersion("drone.ver")
			_, directorVersionErr := boshInfo.GetDirectorVersion()
			Expect(directorVersionErr).To(HaveOccurred())
			Expect(directorVersionErr).To(MatchError(`unrecognised BOSH Director version: "drone.ver"`))
		})
	})
})

func createBoshInfoWithVersion(version string) *boshdirector.Info {
	return &boshdirector.Info{
		Version: version,
	}
}
