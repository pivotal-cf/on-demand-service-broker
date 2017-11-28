package service_test

import (
	"errors"

	"github.com/pivotal-cf/on-demand-service-broker/service"

	"io/ioutil"
	"net/http"
	"strings"

	"fmt"

	"net/url"

	"crypto/x509"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/service/fakes"
)

var _ = Describe("ServiceInstanceLister", func() {
	var client *fakes.FakeHTTPClient

	BeforeEach(func() {
		client = new(fakes.FakeHTTPClient)
	})

	It("lists service instances", func() {
		client.GetReturns(response(http.StatusOK, `[{"service_instance_id": "foo", "plan_id": "plan"}, {"service_instance_id": "bar", "plan_id": "another-plan"}]`), nil)
		serviceInstanceLister := service.NewInstanceLister(client, false)
		instances, err := serviceInstanceLister.Instances()
		Expect(err).NotTo(HaveOccurred())
		Expect(len(instances)).To(Equal(2))
		Expect(instances[0]).To(Equal(service.Instance{GUID: "foo", PlanUniqueID: "plan"}))
	})

	It("returns an error when the request fails", func() {
		client.GetReturns(nil, errors.New("connection error"))
		serviceInstanceLister := service.NewInstanceLister(client, false)
		_, err := serviceInstanceLister.Instances()
		Expect(err).To(MatchError("connection error"))
	})

	It("returns an error when the broker response is unrecognised", func() {
		client.GetReturns(response(http.StatusOK, `{"not": "a list"}`), nil)
		serviceInstanceLister := service.NewInstanceLister(client, false)
		_, err := serviceInstanceLister.Instances()
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when the HTTP status is not OK", func() {
		client.GetReturns(response(http.StatusInternalServerError, ``), nil)
		serviceInstanceLister := service.NewInstanceLister(client, false)
		_, err := serviceInstanceLister.Instances()
		Expect(err).To(MatchError(fmt.Sprintf(
			"HTTP response status: %d %s",
			http.StatusInternalServerError,
			http.StatusText(http.StatusInternalServerError),
		)))
	})

	It("returns a service instance API error when the HTTP status is not OK and service API is configured", func() {
		client.GetReturns(response(http.StatusInternalServerError, ``), nil)
		serviceInstanceLister := service.NewInstanceLister(client, true)
		_, err := serviceInstanceLister.Instances()
		Expect(err).To(MatchError(fmt.Sprintf(
			"error communicating with service_instances_api (%s): HTTP response status: %d %s",
			"http://example.org/some-path",
			http.StatusInternalServerError,
			http.StatusText(http.StatusInternalServerError),
		)))
	})

	It("returns SSL validation error when service instance API request fails due to unknown authority", func() {
		expectedURL := "https://example.org/service-instances"
		expectedError := &url.Error{
			URL: expectedURL,
			Err: x509.UnknownAuthorityError{},
		}
		client.GetReturns(nil, expectedError)
		serviceInstanceLister := service.NewInstanceLister(client, true)
		_, err := serviceInstanceLister.Instances()
		Expect(err).To(MatchError(fmt.Sprintf(
			"SSL validation error for `service_instances_api.url`: %s. Please configure a `service_instances_api.root_ca_cert` and use a valid SSL certificate",
			expectedURL,
		)))
	})

	It("returns the expected error when service instance API request fails due to generic certificate error", func() {
		expectedURL := "https://example.org/service-instances"
		expectedError := &url.Error{
			URL: expectedURL,
			Err: x509.CertificateInvalidError{},
		}
		client.GetReturns(nil, expectedError)
		serviceInstanceLister := service.NewInstanceLister(client, true)
		_, err := serviceInstanceLister.Instances()
		Expect(err).To(MatchError(Equal(fmt.Sprintf(
			"error communicating with service_instances_api (%s): %s",
			expectedURL,
			expectedError.Error(),
		))))
	})

	It("returns the expected error when service instance API request fails due to a url error with no Err", func() {
		expectedURL := "https://example.org/service-instances"
		expectedError := &url.Error{
			URL: expectedURL,
		}
		client.GetReturns(nil, expectedError)
		serviceInstanceLister := service.NewInstanceLister(client, true)
		_, err := serviceInstanceLister.Instances()
		Expect(err).To(Equal(expectedError))
	})
})

func response(statusCode int, body string) *http.Response {
	url, err := url.Parse("http://example.org/some-path")
	Expect(err).NotTo(HaveOccurred())
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Body:       ioutil.NopCloser(strings.NewReader(body)),
		Request: &http.Request{
			URL: url,
		},
	}
}
