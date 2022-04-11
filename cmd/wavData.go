package main

import (
	"fmt"
	"os"

	"github.com/go-audio/wav"
	"github.com/pkg/errors"
)

func usage() string {
	return fmt.Sprintf(`%s "wav file name"
Displays data about wav file`, os.Args[0])
}

func stderr(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", a...)
}

func main() {
	if len(os.Args) < 2 {
		stderr("Expected wav filename as command line argument")
		fmt.Println(usage())
		os.Exit(1)
	}
	wavFileName := os.Args[1]
	f, err := os.Open(wavFileName)
	if err != nil {
		stderr(errors.Wrapf(err, "failed to open %q", wavFileName).Error())
		os.Exit(1)
	}
	wavDecoder := wav.NewDecoder(f)
	if !wavDecoder.IsValidFile() {
		stderr("%q is not a valid wav file", wavFileName)
		os.Exit(1)
	}

	fmt.Println("=== Information on", wavFileName, "===")

	// Duration
	dur, err := wavDecoder.Duration()
	if err != nil {
		stderr(errors.Wrapf(err, "failed to determine duration of %q", wavFileName).Error())
		os.Exit(1)
	}

	// Format
	format := wavDecoder.Format()

	// Info
	fmt.Printf(`
%-25s%s
%-25s%d
%-25s%d

== Internal Data:
%-25s%d
%-25s%d
%-25s%d
%-25s%d
%-25s%d
%-25s%d

== Meta Data:
`,
		"Duration:", dur,
		"Number of Channels:", format.NumChannels,
		"Sample rate:", format.SampleRate,
		"NumChans:", wavDecoder.NumChans,
		"BitDepth:", wavDecoder.BitDepth,
		"SampleRate:", wavDecoder.SampleRate,
		"AvgBytesPerSec:", wavDecoder.AvgBytesPerSec,
		"WavAudioFormat:", wavDecoder.WavAudioFormat,
		"PCMSize:", wavDecoder.PCMSize,
	)

	// Metadata
	wavDecoder.ReadMetadata()
	if err := wavDecoder.Err(); err != nil {
		stderr(errors.Wrap(err, "failed to read wav metadata").Error())
		os.Exit(1)
	}
	if wavDecoder.Metadata != nil {
		fmt.Printf("%#v\n", wavDecoder.Metadata)
	}
	fmt.Println("\n\n=== Information on", wavFileName, "===")
}
