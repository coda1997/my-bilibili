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
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, func(ctx context.Context) serviceprobe.QRDiagnostic { return serviceprobe.RunQRDiagnostic(ctx, nil) }))
}

func run(args []string, out, diagnostics io.Writer, probe func(context.Context) serviceprobe.QRDiagnostic) int {
	flags := flag.NewFlagSet("qr-diagnostic", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	live := flags.Bool("live", false, "explicitly perform one QR-only format diagnostic")
	if flags.Parse(args) != nil || flags.NArg() != 0 || !*live {
		fmt.Fprintln(diagnostics, "usage: qr-diagnostic --live")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	report := probe(ctx)
	if json.NewEncoder(out).Encode(report) != nil {
		fmt.Fprintln(diagnostics, "could not write sanitized QR diagnostic")
		return 1
	}
	if report.Observation.Status != "pass" {
		return 1
	}
	return 0
}
