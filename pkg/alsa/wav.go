package alsa

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/pkg/errors"
	"github.com/yobert/alsa"

	"github.com/renan-campos/sound-utils/pkg/logging"
)

func PlayWav(device *alsa.Device, wavFileName string) error {
	var err error

	f, err := os.Open(wavFileName)
	if err != nil {
		return errors.Wrapf(err, "failed to open %q", wavFileName)
	}
	wavDecoder := wav.NewDecoder(f)
	if !wavDecoder.IsValidFile() {
		return fmt.Errorf("%q is not a valid wav file", wavFileName)
	}

	if err = device.Open(); err != nil {
		return err
	}

	dur, err := wavDecoder.Duration()
	if err != nil {
		return errors.Wrapf(err, "failed to determine duration of %q", wavFileName)
	}

	// Cleanup device when done or force cleanup 3 seconds after the duration of the wav file.
	wg := sync.WaitGroup{}
	wg.Add(1)
	defer wg.Wait()
	childCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(dur).Add(3*time.Second))
	defer cancel()
	go func(ctx context.Context) {
		defer device.Close()
		<-ctx.Done()
		fmt.Println("Closing device.")
		wg.Done()
	}(childCtx)

	wavFormat := wavDecoder.Format()
	// Note:
	// When playing a wav file:
	// The number of channels should be what the file specifies.
	channels, err := device.NegotiateChannels(wavFormat.NumChannels, 2)
	if err != nil {
		return err
	}

	// Note:
	// When playing a wav file:
	// The sample rate should be that or higher than what the file specifieds.
	// The sample rate should be greater than or equal to what the file specifies.
	// Only supporting outputs of 44.1 kHz, as these are the only outputs I have!
	rate, err := device.NegotiateRate(44100)
	if err != nil {
		return err
	}

	// Note:
	// When playing a wav file:
	// The format should be what the wav format will be.
	// In the case of wav, the codec library will have int.
	// But the ratio between sample rate and bytes per second
	// of the file I was reading was 1 byte per sample.
	// This means that the data format will be S8_LE (assuming little endian)
	// If this is the case, the data should be set to it or higher,
	// and the buffer data needs to adapt to what it was set to.
	format, err := device.NegotiateFormat(alsa.S32_LE, alsa.S16_LE)
	if err != nil {
		return err
	}

	// A 50ms period is a sensible value to test low-ish latency.
	// We adjust the buffer so it's of minimal size (period * 2) since it appear ALSA won't
	// start playback until the buffer has been filled to a certain degree and the automatic
	// buffer size can be quite large.
	// Some devices only accept even periods while others want powers of 2.
	wantPeriodSize := 2048 // 46ms @ 44100Hz

	periodSize, err := device.NegotiatePeriodSize(wantPeriodSize)
	if err != nil {
		return err
	}

	bufferSize, err := device.NegotiateBufferSize(2 * periodSize * channels)
	if err != nil {
		return err
	}

	if err = device.Prepare(); err != nil {
		return err
	}

	logging.Debugf("Negotiated parameters: %d channels, %d hz, %v, %d period size, %d buffer size\n",
		channels, rate, format, periodSize, bufferSize)

	inbuf := audio.IntBuffer{
		Format: wavFormat,
		Data:   make([]int, int(float64(periodSize)*float64(wavFormat.NumChannels)*float64(wavFormat.SampleRate)/float64(rate))),
	}

	for !wavDecoder.EOF() {
		nSamples, err := wavDecoder.PCMBuffer(&inbuf)
		if err != nil {
			return errors.Wrap(err, "failed to fill buffer with wav data")
		}
		if nSamples == 0 {
			break
		}

		frames := bytes.Buffer{}
		for i, sample := range inbuf.Data {
			var copies int
			switch {
			case wavFormat.NumChannels < channels:
				// Wav file is mono, output is stereo
				// Double the samples written out the the buffer.
				copies = 2
			case wavFormat.NumChannels == channels:
				// Wav file and output have the same number of channels
				copies = 1
			case wavFormat.NumChannels > channels:
				// Wav file is stereo, output is mono
				// In this case... skip every odd sample!
				if i%2 == 0 {
					continue
				}
			}
			if wavFormat.SampleRate == rate/2 {
				// Duplicate this sample as the next sample.
				copies *= 2
			}
			for ; copies > 0; copies-- {
				switch format {
				case alsa.S16_LE:
					// If the wav format is 32_LE, the PCM value must be converted to 16_LE.
					// The simplest way is to rightshift 16 bits.
					// However, could there be a smoother way?
					// Yes! With bit coefficients! I'll do this later.
					var err error
					switch wavDecoder.BitDepth {
					case 32:
						err = binary.Write(&frames, binary.LittleEndian, int16(scale32To16(sample)))
					case 16:
						err = binary.Write(&frames, binary.LittleEndian, int16(sample))
					case 8:
						err = binary.Write(&frames, binary.LittleEndian, int16(scale8To16(sample)))
					default:
						return fmt.Errorf("Can't play this yet")
					}

					if err != nil {
						fmt.Println(err)
					}
				case alsa.S32_LE:
					switch wavDecoder.BitDepth {
					case 32:
						if err := binary.Write(&frames, binary.LittleEndian, int32(sample)); err != nil {
							fmt.Println(err)
						}
					case 16:
						// If the wav format is 16_LE, the PCM value must be converted to int32
						// The simplest way would be to leftshift it 16 bits.
						// However, could the be a smoother way?
						// There sure is pal.
						if err := binary.Write(&frames, binary.LittleEndian, int32(scale16To32(sample))); err != nil {
							fmt.Println(err)
						}
					case 8:
						if err := binary.Write(&frames, binary.LittleEndian, int32(scale8To32(sample))); err != nil {
							fmt.Println(err)
						}
					}

					// TODO What about when the number of channels arent the same?
					// If the wav file is mono but the speakers are stereo, just double the samples.
					// TODO What about when the sample frequency isn't the same?
					// When the sample size of the wav file is half of that of the speaker
					// 22050Hz vs 44100Hz
					// There are less samples than what is played.
					// We could duplicate the samples.
				default:
					return fmt.Errorf("Unhandled sample format: %v", format)
				}
			}
		}

		if err := device.Write(frames.Bytes(), periodSize); err != nil {
			return err
		}
	}
	// Wait for playback to complete.
	fmt.Printf("Playback should be complete now.\n")

	return nil
}

func RecordWav(rec *alsa.Device, duration time.Duration, channels, rate int) (alsa.Buffer, error) {
	var err error

	if err = rec.Open(); err != nil {
		return alsa.Buffer{}, err
	}
	defer rec.Close()

	_, err = rec.NegotiateChannels(channels)
	if err != nil {
		return alsa.Buffer{}, err
	}

	_, err = rec.NegotiateRate(rate)
	if err != nil {
		return alsa.Buffer{}, err
	}

	_, err = rec.NegotiateFormat(alsa.S16_LE, alsa.S32_LE)
	if err != nil {
		return alsa.Buffer{}, err
	}

	bufferSize, err := rec.NegotiateBufferSize(8192, 16384)
	if err != nil {
		return alsa.Buffer{}, err
	}

	if err = rec.Prepare(); err != nil {
		return alsa.Buffer{}, err
	}

	buf := rec.NewBufferDuration(duration)

	fmt.Printf("Negotiated parameters: %v, %d frame buffer, %d bytes/frame\n",
		buf.Format, bufferSize, rec.BytesPerFrame())

	fmt.Printf("Recording for %s (%d frames, %d bytes)...\n", duration, len(buf.Data)/rec.BytesPerFrame(), len(buf.Data))
	err = rec.Read(buf.Data)
	if err != nil {
		return alsa.Buffer{}, err
	}
	fmt.Println("Recording stopped.")
	return buf, nil
}

func SaveWav(recording alsa.Buffer, file string) error {
	of, err := os.Create(file)
	if err != nil {
		return err
	}
	defer of.Close()

	var sampleBytes int
	switch recording.Format.SampleFormat {
	case alsa.S32_LE:
		sampleBytes = 4
	case alsa.S16_LE:
		sampleBytes = 2
	default:
		return fmt.Errorf("Unhandled ALSA format %v", recording.Format.SampleFormat)
	}

	// normal uncompressed WAV format (I think)
	// https://web.archive.org/web/20080113195252/http://www.borg.com/~jglatt/tech/wave.htm
	wavformat := 1

	enc := wav.NewEncoder(of, recording.Format.Rate, sampleBytes*8, recording.Format.Channels, wavformat)

	sampleCount := len(recording.Data) / sampleBytes
	data := make([]int, sampleCount)

	format := &audio.Format{
		NumChannels: recording.Format.Channels,
		SampleRate:  recording.Format.Rate,
	}

	// Convert into the format go-audio/wav wants
	var off int
	switch recording.Format.SampleFormat {
	case alsa.S32_LE:
		inc := binary.Size(uint32(0))
		for i := 0; i < sampleCount; i++ {
			data[i] = int(binary.LittleEndian.Uint32(recording.Data[off:]))
			off += inc
		}
	case alsa.S16_LE:
		inc := binary.Size(uint16(0))
		for i := 0; i < sampleCount; i++ {
			data[i] = int(binary.LittleEndian.Uint16(recording.Data[off:]))
			off += inc
		}
	default:
		return fmt.Errorf("Unhandled ALSA format %v", recording.Format.SampleFormat)
	}

	intBuf := &audio.IntBuffer{Data: data, Format: format, SourceBitDepth: sampleBytes * 8}

	if err := enc.Write(intBuf); err != nil {
		return err
	}

	if err := enc.Close(); err != nil {
		return err
	}
	fmt.Printf("Saved recording to %s\n", file)
	return nil
}
