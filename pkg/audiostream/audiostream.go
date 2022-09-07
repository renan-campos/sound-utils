package audiostream

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/yobert/alsa"
)

type AudioStreamStatus string

const (
	statusRecording AudioStreamStatus = "recording"
	statusStandby   AudioStreamStatus = "standby"
	statusOff       AudioStreamStatus = "off"
	statusError     AudioStreamStatus = "error"
)

// ALSA Device constants
const (
	numChannels  = 1
	sampleRate   = 44100
	sampleFormat = alsa.S32_LE
	bitDepth     = 32
)

type DeviceConfig struct {
	NumChannels int
	FrameRate   int
	FrameFormat alsa.FormatType
	BufferSize  int
}

type AudioStream struct {
	device       *alsa.Device
	deviceConfig DeviceConfig
	fileName     string
	status       AudioStreamStatus
	fmStatus     chan AudioStreamStatus
	dmStatus     chan AudioStreamStatus
	fmDone       chan struct{}
	dmDone       chan struct{}
}

func NewAudioStream() AudioStream {
	return AudioStream{
		device:   nil,
		fileName: "",
		status:   statusOff,
		fmStatus: make(chan AudioStreamStatus, 1),
		dmStatus: make(chan AudioStreamStatus, 1),
		fmDone:   make(chan struct{}, 1),
		dmDone:   make(chan struct{}, 1),
	}
}

func (a *AudioStream) SetDevice(device *alsa.Device, config DeviceConfig) error {
	if a.status != statusOff {
		return fmt.Errorf("AudioStream must be off to change files")
	}
	a.device = device
	return nil
}

func (a *AudioStream) GetDevice(device *alsa.Device) *alsa.Device {
	return a.device
}

func (a *AudioStream) SetFileName(fileName string) error {
	if a.status != statusStandby || a.status != statusOff {
		return fmt.Errorf("AudioStream must be off or on standby to change files")
	}
	a.fileName = fileName
	return nil
}

func (a *AudioStream) GetFileName() string {
	return a.fileName
}

func (a *AudioStream) Record() error {
	// TODO
	return nil
}

func (a *AudioStream) Standby() error {
	switch a.status {
	case statusStandby:
		a.dmStatus <- statusStandby
		a.fmStatus <- statusStandby
		// TODO probably want to flush the framebuffer...
		return nil
	case statusOff:
		// TODO make the errors make sense
		if err := a.device.Open(); err != nil {
			return err
		}

		_, err := a.device.NegotiateChannels(a.deviceConfig.NumChannels)
		if err != nil {
			return err
		}

		_, err = a.device.NegotiateRate(a.deviceConfig.FrameRate)
		if err != nil {
			return err
		}

		_, err = a.device.NegotiateFormat(a.deviceConfig.FrameFormat)
		if err != nil {
			return err
		}

		_, err = a.device.NegotiateBufferSize(a.deviceConfig.BufferSize)
		if err != nil {
			return err
		}

		if err = a.device.Prepare(); err != nil {
			return err
		}

		// The frame buffer will hold 2 seconds
		// For 44.1kHz at 2 bytes per sample, that's 176400 bytes
		// The ring buffer will hold 40 seconds
		// For 44.1kHz at 2 bytes that's 3528000 bytes
		// The write size will be 8 seconds
		// For 44.1kHz at 2 bytes that's 705600 bytes
		// 40 seconds is 20 times the frame buffer. 5 seconds is 1/5 of the ring buffer
		frameBuffer := a.device.NewBufferDuration(2 * time.Second)
		frameBufferSize := len(frameBuffer.Data)
		ringBufferSpec := RingBufferSpec{
			DataSize:  frameBufferSize * 20,
			WriteSize: frameBufferSize,
			ReadSize:  frameBufferSize * 4,
		}
		ringBuffer := NewRingBuffer(ringBufferSpec)

		// Start the dataMover. It needs a pointer to the device frame buffer, and the intermidiate ring buffer.
		// It also takes a done channel
		go func() {
			var recording, die bool
			for {
				select {
				case status := <-a.dmStatus:
					switch status {
					case statusRecording:
						recording = true
					case statusStandby:
						recording = false
					case statusOff:
						recording = false
						die = true
					}
				default:
					if recording {
						a.device.Read(frameBuffer.Data)
						ringBuffer.Write(frameBuffer.Data)
					}
					if die {
						a.dmDone <- struct{}{}
						return
					}
				}
			}
		}()

		// Start the fileMover. It needs the pointer to the intermediate ring buffer, a record channel, and a standby channel
		// It also takes a done channel
		go func() {
			var recording, die bool
			fp, err := os.Create(a.fileName)
			if err != nil {
				// In the future, crashes can be prevented by having an error channel.
				// Then the user just needs to turn the audio stream off, correct the issue and move on.
				// For now, I'll just exit ungracefully.
				fmt.Printf("Failed to create file %s: %v", a.fileName, err)
				os.Exit(1)
			}
			defer fp.Close()
			// normal uncompressed WAV format (I think)
			// https://web.archive.org/web/20080113195252/http://www.borg.com/~jglatt/tech/wave.htm
			wavFormat := 1

			enc := wav.NewEncoder(fp, a.deviceConfig.FrameRate, bitDepth, a.deviceConfig.NumChannels, wavFormat)

			for {
				select {
				case status := <-a.fmStatus:
					switch status {
					case statusRecording:
						recording = true
					case statusStandby:
						recording = false
					case statusOff:
						recording = false
						die = true
					}
				default:
					if recording {
						data, read := ringBuffer.ReadNoBlock()
						if read {

							format := &audio.Format{
								NumChannels: a.deviceConfig.NumChannels,
								SampleRate:  a.deviceConfig.FrameRate,
							}

							// Convert into the format go-audio/wav wants
							var off int
							sampleCount := len(data) / (bitDepth / 8)
							wavData := make([]int, sampleCount)

							inc := binary.Size(uint32(0))
							for i := 0; i < sampleCount; i++ {
								wavData[i] = int(binary.LittleEndian.Uint32(data[off:]))
								off += inc
							}

							intBuf := &audio.IntBuffer{Data: wavData, Format: format, SourceBitDepth: bitDepth}

							err := enc.Write(intBuf)
							if err != nil {
								fmt.Printf("Failed to write to file: %v", a.fileName, err)
								os.Exit(1)
							}
						}
					}
					if die {
						enc.Close()
						a.fmDone <- struct{}{}
						return
					}
				}
			}
		}()

		a.status = statusStandby
		return nil
	case statusRecording:
		a.dmStatus <- statusStandby
		a.fmStatus <- statusStandby
		a.status = statusStandby
		return nil
	}
	return fmt.Errorf("Unknown stream status")
}

func (a *AudioStream) Off() error {
	switch a.status {
	case statusStandby:
		a.dmStatus <- statusOff
		a.fmStatus <- statusOff
		a.device.Close()
		a.status = statusOff
		return nil
	case statusRecording:
		a.dmStatus <- statusOff
		a.fmStatus <- statusOff
		<-a.fmDone
		<-a.dmDone
		a.device.Close()
		a.status = statusOff
		return nil
	case statusOff:
		return nil
	}
	return fmt.Errorf("Unknown stream status")
}

/*
Audiostreams manipulate two objects:
1. ALSA devices capable of recording
2. File for saving WAV data

Audiostreams maintain two goroutines:
1. Device Datamover:
		When recording,
		it gets data from ALSA device and places in an intermediate buffer

2. File datamover:
		When recording,
		it gets data from intermediate buffer and writes to file

Audiostreams have three states:
1. Off
2. Standby
3. Recording

The state the audiostream is in affects the actions that can be performed on it, and the goroutines that are running.
Off -> No goroutines running. Device and File can be changed.
Standby -> Device and File datamovers are running, but not doing anything.

I want the intermediate buffer to look like this:

+-------------------+-------------------+
|----|----|----|----|----|----|----|----|

Where |----| represents a number of frames that is grabbed from the data device.
More succintly, |----| represents the framerate.

+---...---+ represents the data write rate: how many sets of frames can be efficiantly written at once.
The data write rate may be more that one set of frames, or it could be one to one.

Buffer parameters:
- Frame Rate
- Write Rate
- Buffer size

Frames are determined by rate, number of channels, and pcm format (16-bit or 32-bit).
The alsa device itself has a frame buffer.
So we get the frames from the frame buffer and put it in the intermediate buffer.
From the intermediate buffer we write to a file.

Frame chunk size -> Frame rate
File data chunk size -> Write rate

The intermediate buffer must be sized to avoid overflows.
This means the write rate is faster than the frame rate
Suppose the write rate is 16 Mb, and the frame rate is 16 Kb
Having the buffer be 32 Mb would mean the write datamover can write the first 16 Mb
while the frame datamover is filling up the next 1000 frames

Maybe it would be clearer to think of the buffer in terms of time:
The frame rate will be 25ms worth of frame

*/
// Suppose we have a ring buffer where we write in chunks of 2 bytes, we read in chunks of 4 bytes, and the ring buffer size is 16 bytes.
// That means every 2 writes, we can start a read operation.
type RingBuffer struct {
	data      []byte
	writeIdx  int
	readIdx   int
	writeSize int
	readSize  int
	rSem      chan struct{}
	wSem      chan struct{}
	rLock     sync.Mutex
}

type RingBufferSpec struct {
	DataSize  int
	WriteSize int
	ReadSize  int
}

func NewRingBuffer(spec RingBufferSpec) RingBuffer {
	data := make([]byte, spec.DataSize)
	return RingBuffer{
		data:      data,
		writeIdx:  0,
		readIdx:   0,
		writeSize: spec.WriteSize,
		readSize:  spec.ReadSize,
		rSem:      make(chan struct{}, spec.DataSize/spec.ReadSize),
		wSem:      make(chan struct{}, spec.DataSize/spec.WriteSize),
	}
}

func (rb *RingBuffer) Write(buff []byte) {

	rb.wSem <- struct{}{}

	if len(buff) > rb.writeSize {
		buff = buff[:rb.writeSize]
	}
	for _, b := range buff {
		rb.data[rb.writeIdx] = b
		rb.writeIdx++
	}
	for i := 0; i < rb.writeSize-len(buff); i++ {
		rb.data[rb.writeIdx] = 0
		rb.writeIdx++
	}
	if rb.writeIdx%rb.readSize == 0 {
		rb.rSem <- struct{}{}
	}
	if rb.writeIdx == len(rb.data) {
		rb.writeIdx = 0
	}
	// In this ring buffer, we don't want writes to be blocked.
	// That means that if the write pointer has reached the read pointer
	// its time to move the read pointer up a read chunk.
	rb.rLock.Lock()
	defer rb.rLock.Unlock()
	if rb.writeIdx == rb.readIdx {
		rb.readIdx += rb.readSize
		<-rb.rSem
	}
}

func (rb *RingBuffer) ReadNoBlock() ([]byte, bool) {
	buff := make([]byte, rb.readSize)

	select {
	case <-rb.rSem:
		rb.rLock.Lock()
		defer rb.rLock.Unlock()
	default:
		return buff, false
	}

	for i, _ := range buff {
		buff[i] = rb.data[rb.readIdx]
		rb.readIdx++
		if rb.readIdx%rb.writeSize == 0 {
			<-rb.wSem
		}
	}

	if rb.readIdx == len(rb.data) {
		rb.readIdx = 0
	}

	return buff, true
}
