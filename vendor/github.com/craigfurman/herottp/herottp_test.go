package herottp_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/craigfurman/herottp"

	"time"

	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HeroTTP", func() {
	var (
		client *herottp.Client
		config herottp.Config

		req         *http.Request
		reqAttempts int

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
		reqAttempts = 0
		router := mux.NewRouter()

		router.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/text", http.StatusFound)
		}).Methods("POST")

		router.HandleFunc("/text", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Pork and beans"))
		}).Methods("GET")

		router.HandleFunc("/retries", func(w http.ResponseWriter, r *http.Request) {
			reqAttempts += 1

			if reqAttempts == 4 {
				w.Write([]byte("Mashed potatoes and gravy"))
			} else {
				time.Sleep(time.Second)
			}
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

	Context("retries", func() {
		BeforeEach(func() {
			req = createRequest("retries", "GET")
		})

		Context("when max retries are not set", func() {
			BeforeEach(func() {
				config = herottp.Config{Timeout: time.Millisecond}
			})

			It("it tries the request once", func() {
				Expect(reqAttempts).To(Equal(1))
				Expect(respErr).To(MatchError(ContainSubstring("Client.Timeout exceeded while awaiting headers")))
			})
		})

		Context("when max retries result in an error", func() {
			BeforeEach(func() {
				config = herottp.Config{MaxRetries: 2, Timeout: time.Millisecond}
			})

			It("makes multiple attempts and returns the error", func() {
				Expect(reqAttempts).To(Equal(3))
				Expect(respErr).To(MatchError(ContainSubstring("Client.Timeout exceeded while awaiting headers")))
			})
		})

		Context("when max retries result in success", func() {
			BeforeEach(func() {
				config = herottp.Config{MaxRetries: 5, Timeout: time.Millisecond}
			})

			It("makes enough attempts and returns the response", func() {
				Expect(reqAttempts).To(Equal(4))
				Expect(respErr).NotTo(HaveOccurred())
				defer resp.Body.Close()
				Expect(ioutil.ReadAll(resp.Body)).To(Equal([]byte("Mashed potatoes and gravy")))
			})
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