// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package serviceadapter_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
)

var _ = Describe("CommandRunner", func() {
	var (
		scriptPath string

		stdout         string
		stderr         string
		runErr         error
		actualExitCode *int
	)

	createScript := func(script string) string {
		tempFile, err := ioutil.TempFile("", "cmd")
		Expect(err).NotTo(HaveOccurred())

		_, err = tempFile.WriteString("#! /bin/sh\n")
		Expect(err).NotTo(HaveOccurred())
		_, err = tempFile.WriteString(script)
		Expect(err).NotTo(HaveOccurred())

		err = tempFile.Chmod(0o755)
		Expect(err).NotTo(HaveOccurred())

		err = tempFile.Close()
		Expect(err).NotTo(HaveOccurred())

		return tempFile.Name()
	}

	Describe("Run", func() {
		JustBeforeEach(func() {
			runner := serviceadapter.NewCommandRunner()
			var stdoutBytes, stderrBytes []byte
			stdoutBytes, stderrBytes, actualExitCode, runErr = runner.Run(scriptPath)
			stdout = string(stdoutBytes)
			stderr = string(stderrBytes)
		})

		AfterEach(func() {
			os.Remove(scriptPath)
		})

		Context("when the command runs normally", func() {
			BeforeEach(func() {
				scriptPath = createScript("echo output; echo error >&2")
			})

			It("returns the text written to standard output", func() {
				Expect(stdout).To(Equal("output\n"))
			})

			It("returns the text written to standard error", func() {
				Expect(stderr).To(Equal("error\n"))
			})

			It("returns no error", func() {
				Expect(runErr).NotTo(HaveOccurred())
			})

			It("returns exit 0", func() {
				Expect(*actualExitCode).To(BeZero())
			})
		})

		Context("when the command does not exist", func() {
			BeforeEach(func() {
				scriptPath = "/no/such/script"
			})

			It("returns an error", func() {
				Expect(runErr).To(HaveOccurred())
			})

			It("returns no exit code", func() {
				Expect(actualExitCode).To(BeNil())
			})
		})

		Context("when the command exists but is not executable", func() {
			BeforeEach(func() {
				scriptPath = createScript("this is not a script")
				os.Chmod(scriptPath, 0o644)
			})

			It("returns an error", func() {
				Expect(runErr).To(HaveOccurred())
			})

			It("returns no exit code", func() {
				Expect(actualExitCode).To(BeNil())
			})
		})

		Context("when the command exits with nonzero status", func() {
			BeforeEach(func() {
				scriptPath = createScript("echo output; echo error >&2; exit 23")
			})

			It("returns the text written to standard output", func() {
				Expect(stdout).To(Equal("output\n"))
			})

			It("returns the text written to standard error", func() {
				Expect(stderr).To(Equal("error\n"))
			})

			It("returns no error", func() {
				Expect(runErr).NotTo(HaveOccurred())
			})

			It("returns exit 23", func() {
				Expect(*actualExitCode).To(Equal(23))
			})
		})
	})

	Describe("RunWithInputParams", func() {
		var inputParams interface{}

		BeforeEach(func() {
			inputParams = ""
		})

		JustBeforeEach(func() {
			runner := serviceadapter.NewCommandRunner()
			var stdoutBytes, stderrBytes []byte
			stdoutBytes, stderrBytes, actualExitCode, runErr = runner.RunWithInputParams(inputParams, scriptPath)
			stdout = string(stdoutBytes)
			stderr = string(stderrBytes)
		})

		AfterEach(func() {
			os.Remove(scriptPath)
		})

		Context("when the command runs normally", func() {
			BeforeEach(func() {
				inputParams = randSeq(500000)

				scriptPath = createScript(`while read line; do echo $line; done; echo error >&2`)
			})

			It("succeeds", func() {
				Expect(stdout).To(Equal(fmt.Sprintf("\"%s\"\n", inputParams)))
				Expect(stderr).To(Equal("error\n"))
				Expect(runErr).NotTo(HaveOccurred())
				Expect(*actualExitCode).To(BeZero())
			})
		})

		Context("when the command fails to run", func() {
			BeforeEach(func() {
				scriptPath = "/no/such/script"
			})

			It("returns an error", func() {
				Expect(runErr).To(HaveOccurred())
				Expect(actualExitCode).To(BeNil())
			})
		})

		Context("when the inputParams cannot be serialized", func() {
			BeforeEach(func() {
				inputParams = func() {}
			})

			It("returns an error", func() {
				Expect(runErr).To(HaveOccurred())
				Expect(runErr.Error()).To(ContainSubstring("unsupported type"))
				Expect(actualExitCode).To(BeNil())
			})
		})

		Context("when the command exits with nonzero status", func() {
			BeforeEach(func() {
				scriptPath = createScript("echo output; echo error >&2; exit 23")
			})

			It("returns the text written to standard output", func() {
				Expect(stdout).To(Equal("output\n"))
				Expect(stderr).To(Equal("error\n"))
				Expect(runErr).NotTo(HaveOccurred())
				Expect(*actualExitCode).To(Equal(23))
			})
		})
	})
})

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
