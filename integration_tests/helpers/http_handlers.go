package helpers

import (
	"io/ioutil"
	"net/http"
	"strings"

	. "github.com/onsi/gomega"
)

type FakeResponse struct {
	Body       string
	StatusCode int
}

type Request struct {
	Body string
	URL  string
}

type FakeHandler struct {
	callCount       int
	responses       map[int]FakeResponse
	defaultResponse FakeResponse

	handlers map[string]*FakeHandler

	requestsReceived []Request
}

func (h *FakeHandler) RespondsOnCall(call, statusCode int, body string) {
	if h.responses == nil {
		h.responses = map[int]FakeResponse{}
	}
	h.responses[call] = FakeResponse{Body: body, StatusCode: statusCode}
}

func (h *FakeHandler) WithQueryParams(params ...string) *FakeHandler {
	if h.handlers == nil {
		h.handlers = map[string]*FakeHandler{}
	}

	newHandler := new(FakeHandler)
	h.handlers[strings.Join(params, "&")] = newHandler
	return newHandler
}

func (h *FakeHandler) RespondsWith(statusCode int, body string) {
	h.defaultResponse = FakeResponse{Body: body, StatusCode: statusCode}
}

func (h *FakeHandler) RequestsReceived() int {
	return len(h.requestsReceived)
}

func (h *FakeHandler) GetRequestForCall(call int) Request {
	return h.requestsReceived[call]
}

func (h *FakeHandler) Handle(w http.ResponseWriter, req *http.Request) {
	if h.requestsReceived == nil {
		h.requestsReceived = []Request{}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	rawBody, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	Expect(err).NotTo(HaveOccurred())

	h.requestsReceived = append(h.requestsReceived, Request{
		Body: string(rawBody),
		URL:  req.URL.String(),
	})

	if handler, found := h.handlers[req.URL.RawQuery]; found {
		handler.Handle(w, req)
		return
	}

	var resp FakeResponse
	resp, found := h.responses[h.callCount]
	if !found {
		resp = h.defaultResponse
	}
	h.callCount++
	w.WriteHeader(resp.StatusCode)
	w.Write([]byte(resp.Body))
}
