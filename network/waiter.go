package network

import (
	"net"
	"net/url"
	"strings"
	"time"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/hostlookupper.go . HostLookUpper
type HostLookUpper func(host string) (addrs []string, err error)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/sleeper.go . Sleeper
type Sleeper func(duration time.Duration)

type HostWaiter struct {
	HostLookUpper
	Sleeper
}

func NewHostWaiter() HostWaiter {
	return HostWaiter{HostLookUpper: net.LookupHost, Sleeper: time.Sleep}
}
func (h HostWaiter) Wait(url string, pause, retries int) error {
	host, err := getHostName(url)

	if err != nil {
		return err
	}
	currentPauseDuration := time.Duration(pause) * time.Millisecond

	_, err = h.HostLookUpper(host)

	for i := 0; err != nil &&
		strings.Contains(err.Error(), "no such host") &&
		i < retries; i++ {

		h.Sleeper(currentPauseDuration)
		currentPauseDuration *= 2
		_, err = h.HostLookUpper(host)
	}

	return err
}

func getHostName(path string) (string, error) {
	u, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	return u.Hostname(), nil
}
