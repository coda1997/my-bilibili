// output-fixtures creates new synthetic paired-event media, never recordings.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"my-bilibili/experiments/playback/outputprobe"
)

func main() {
	destination := flag.String("out", "", "new destination directory (must not exist; parent must exist)")
	ffmpeg := flag.String("ffmpeg", "ffmpeg", "explicit FFmpeg executable")
	ffprobe := flag.String("ffprobe", "ffprobe", "explicit ffprobe executable")
	timeout := flag.Duration("timeout", 180*time.Second, "bounded preparation deadline, at most 600s")
	verify := flag.String("verify", "", "verify existing manifests and media identities without invoking media tools")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	if *verify != "" {
		if *destination != "" {
			fmt.Fprintln(os.Stderr, "choose generation or verification")
			os.Exit(2)
		}
		m, err := outputprobe.VerifyDirectory(*verify)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(m.Summary())
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	m, err := outputprobe.Generate(ctx, outputprobe.GenerateOptions{Destination: *destination, FFmpeg: *ffmpeg, FFprobe: *ffprobe, Timeout: *timeout})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(m.Summary())
}
