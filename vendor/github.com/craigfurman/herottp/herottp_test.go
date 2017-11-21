package herottp_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/craigfurman/herottp"

	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HeroTTP", func() {
	var (
		client *herottp.Client
		config herottp.Config

		req *http.Request

		resp    *http.Response
		respErr error

		server *httptest.Server
	)

	createRequest := func(path, method string) *http.Request {
		req, err := http.NewRequest(method, fmt.Sprintf("%s/%s", server.URL, path), nil)
		Expect(err).NotTo(HaveOccurred())
		return req
	}

	startServer := func(useTLS bool) *httptest.Server {
		router := mux.NewRouter()

		router.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/text", http.StatusFound)
		}).Methods("POST")

		router.HandleFunc("/text", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Pork and beans"))
		}).Methods("GET")

		if useTLS {
			return httptest.NewTLSServer(router)
		} else {
			return httptest.NewServer(router)
		}
	}

	BeforeEach(func() {
		server = startServer(false)
	})

	JustBeforeEach(func() {
		client = herottp.New(config)
		resp, respErr = client.Do(req)
	})

	AfterEach(func() {
		server.Close()
	})

	Context("default configuration", func() {
		BeforeEach(func() {
			config = herottp.Config{}
			req = createRequest("redirect", "POST")
		})

		It("follows redirects", func() {
			Expect(respErr).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(body).To(Equal([]byte("Pork and beans")))
		})
	})

	Context("when following redirects is disabled", func() {
		BeforeEach(func() {
			config = herottp.Config{NoFollowRedirect: true}
			req = createRequest("redirect", "POST")
		})

		It("returns the redirect response", func() {
			Expect(respErr).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusFound))
		})
	})

	Context("when the connection is HTTPS", func() {
		BeforeEach(func() {
			server = startServer(true)
		})

		Context("default configuration", func() {
			BeforeEach(func() {
				config = herottp.Config{}
				req = createRequest("text", "GET")
			})

			It("returns error", func() {
				Expect(respErr).To(MatchError(ContainSubstring("certificate signed by unknown authority")))
			})
		})

		Context("when certificate checking is disabled", func() {
			BeforeEach(func() {
				config = herottp.Config{
					DisableTLSCertificateVerification: true,
				}
				req = createRequest("text", "GET")
			})

			It("returns the response", func() {
				Expect(respErr).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			})
		})
	})
})
