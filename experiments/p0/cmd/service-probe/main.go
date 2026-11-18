package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"my-bilibili/experiments/p0/serviceprobe"
	"os"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, func(ctx context.Context) serviceprobe.Report { return serviceprobe.Run(ctx, nil) }))
}

func run(args []string, out, diagnostics io.Writer, probe func(context.Context) serviceprobe.Report) int {
	flags := flag.NewFlagSet("service-probe", flag.ContinueOnError)
	flags.SetOutput(io.Discard) // Flag errors may contain arbitrary user input.
	live := flags.Bool("live", false, "explicitly perform the bounded anonymous probe")
	if flags.Parse(args) != nil || flags.NArg() != 0 || !*live {
		fmt.Fprintln(diagnostics, "usage: service-probe --live")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	report := probe(ctx)
	if json.NewEncoder(out).Encode(report) != nil {
		fmt.Fprintln(diagnostics, "could not write sanitized probe report")
		return 1
	}
	if !report.Passed() {
		return 1
	}
	return 0
}
