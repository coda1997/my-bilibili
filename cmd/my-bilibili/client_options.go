package main

import (
	"flag"
	"io"

	"my-bilibili/internal/catalog"
)

type clientOptions struct {
	fixture              bool
	verifyPath, scenario string
}

func parseClientOptions(args []string, e2e bool, scenario string) (clientOptions, error) {
	f := flag.NewFlagSet("my-bilibili", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	var options clientOptions
	f.BoolVar(&options.fixture, "fixture", false, "open the offline developer preview")
	f.StringVar(&options.verifyPath, "verify-readonly", "", "run the fixed read-only verification plan")
	if f.Parse(args) != nil || f.NArg() != 0 || !e2e && scenario != "" || scenario != "" && options.fixture {
		return clientOptions{}, catalog.Invalid
	}
	if validateVerificationOption(options.verifyPath, options.fixture, e2e) != nil {
		return clientOptions{}, catalog.Invalid
	}
	if e2e && scenario == "" {
		options.fixture = true
	}
	options.scenario = scenario
	return options, nil
}
