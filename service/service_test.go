package service_test

import (
	"errors"

	"github.com/pivotal-cf/on-demand-service-broker/service"

	"io/ioutil"
	"net/http"
	"strings"

	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/on-demand-service-broker/service/fakes"
)

var _ = Describe("Service", func() {
	var client *fakes.FakeHTTPClient

	BeforeEach(func() {
		client = new(fakes.FakeHTTPClient)
	})

	It("lists service instances", func() {
		client.GetReturns(response(http.StatusOK, `[{"service_instance_id": "foo", "plan_id": "plan"}, {"service_instance_id": "bar", "plan_id": "another-plan"}]`), nil)
		serviceInstanceLister := service.NewInstanceLister(client)
		instances, err := serviceInstanceLister.Instances()
		Expect(err).NotTo(HaveOccurred())
		Expect(len(instances)).To(Equal(2))
		Expect(instances[0]).To(Equal(service.Instance{GUID: "foo", PlanGUID: "plan"}))
	})

	It("returns an error when the request fails", func() {
		client.GetReturns(nil, errors.New("connection error"))
		serviceInstanceLister := service.NewInstanceLister(client)
		_, err := serviceInstanceLister.Instances()
		Expect(err).To(MatchError("connection error"))
	})

	It("returns an error when the broker response is unrecognised", func() {
		client.GetReturns(response(http.StatusOK, `{"not": "a list"}`), nil)
		serviceInstanceLister := service.NewInstanceLister(client)
		_, err := serviceInstanceLister.Instances()
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when the HTTP status is not OK", func() {
		client.GetReturns(response(http.StatusInternalServerError, ``), nil)
		serviceInstanceLister := service.NewInstanceLister(client)
		_, err := serviceInstanceLister.Instances()
		Expect(err).To(MatchError(fmt.Sprintf(
			"HTTP response status: %d %s",
			http.StatusInternalServerError,
			http.StatusText(http.StatusInternalServerError),
		)))
	})
})

func response(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Body:       ioutil.NopCloser(strings.NewReader(body)),
	}
}
