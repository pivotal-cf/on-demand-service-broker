# HeroTTP

Thin wrapper for `net/http`. [Docs on godoc.org](https://godoc.org/github.com/craigfurman/herottp).

## Features
1. Disable following of redirects.
1. Disable validation of certificates over HTTPS. **This is very dangerous and should
   only ever be done for testing!** Even then, it should only be a last resort.

## Usage
Example:
```
client := herottp.New(herottp.Config{
    NoFollowRedirect: true,
    DisableTLSCertificateVerification: false,
})
resp, err := client.Do(req)
```
You can take advantage of the zero-value of a `bool` in Go being `false` and pass
a default config (`herottp.Config{}`) if you don't want either feature.

The `Do` method on `*herottp.Client` has the [same signature as the `Do` method of `*Client`
in `net/http`](https://golang.org/pkg/net/http/#Client.Do).

## Planned features
1. Support the `PostForm` method from `net/http`
1. Retries with exponential backoff.
