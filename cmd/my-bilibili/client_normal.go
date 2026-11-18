//go:build !e2e

package main

import (
	"net/http"

	"my-bilibili/internal/account"
	accountapi "my-bilibili/internal/adapters/bilibili/account"
	"my-bilibili/internal/adapters/bilibili/transport"
	"my-bilibili/internal/adapters/credentials"
	"my-bilibili/internal/adapters/localhistory"
	"my-bilibili/internal/adapters/mpv"
	"my-bilibili/internal/catalog"
	"my-bilibili/internal/history"
	"my-bilibili/internal/playback"
	"my-bilibili/internal/previewbundle"
)

func clientBuildMode() (bool, string, error) { return false, "", nil }

func composeClient(_ string, options clientOptions) (clientBackend, error) {
	if options.fixture || options.scenario != "" {
		return clientBackend{}, catalog.Invalid
	}
	var verifier *verificationRun
	var err error
	directory := ""
	if options.verifyPath != "" {
		verifier, err = prepareVerification(options.verifyPath, "", "")
		if err != nil {
			return clientBackend{}, err
		}
		directory = verifier.HistoryDirectory()
	}
	local, err := localhistory.New(directory)
	if err != nil {
		if verifier != nil {
			verifier.Stop()
		}
		return clientBackend{}, err
	}
	var accountStore account.Store = credentials.New()
	var historyStore history.Store = local
	var api http.RoundTripper = clientHTTPTransport()
	var cover http.RoundTripper = clientHTTPTransport()
	var opener playback.Opener = mpv.Opener{Library: previewbundle.Library, Policy: playback.HTTPSPolicy{}}
	if verifier != nil {
		accountStore = verifier.AccountStore(accountStore)
		historyStore, err = verifier.HistoryStore(historyStore)
		if err != nil {
			verifier.Stop()
			return clientBackend{}, err
		}
		api, cover = verifier.APITransport(api), verifier.CoverTransport(cover)
		opener = verifier.Opener(opener)
	}
	metrics := &clientMetrics{}
	api = countedTransport{delegate: api, activity: &metrics.api}
	cover = countedTransport{delegate: cover, activity: &metrics.cover}
	gateway := accountapi.New(api)
	service := transport.New(api)
	return clientBackend{config: clientConfig{gateway: gateway, store: accountStore, closeGateway: gateway.Close, service: service,
		coverTransport: cover, historyStore: historyStore, opener: opener, policy: playback.HTTPSPolicy{}, metrics: metrics}, verify: verifier}, nil
}
