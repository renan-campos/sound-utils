/*
The highest quality 16-bit audio format.
Developed by Micrsoft.

+---------------------------------------------------------------+
|  Endian  |  Field offset  |  Field names    |  Field Size     |
|          |    (bytes)     |                 |     (bytes)     |
+----------+----------------------------------------------------+
|  big     |      4         |  ChunkID        |        4        |
|  little  |      4         |  ChunkSize      |        4        | RIFF
|  big     |      8         |  Format         |        4        | WAVE (Chunk Descriptor)
+----------+----------------+-----------------+-----------------+
|  big     |      12        |  Subchunk1ID    |        4        | "fmt "
|  little  |      16        |  Subchunk1Size  |        4        |
|  little  |      20        |  AudioFormat    |        2        | "fmt" sub-chunk
|  little  |      22        |  NumChannels    |        2        |
|  little  |      24        |  SampleRate     |        4        |
|  little  |      28        |  ByteRate       |        4        |
|  little  |      32        |  BlockAlign     |        2        |
|  little  |      34        |  BitsPerSample  |        2        |
|  little  |      36        |  ExtraData      |   Subchunk1Size |
|          |                |                 |                 |
+----------+----------------+-----------------+-----------------+
|  big     |      36        |  Subchunk2ID    |        4        |
|  little  |      40        |  Subchunk2Size  |        4        |
|  little  |      44        |  data           |  Subchunk2Size  | "data" sub-chunk
|  little  |                |                 |                 |
|          |                |                 |                 |
+---------------------------------------------------------------+

Note: The strings are big endian. Everything else is little endian.

http://www.tactilemedia.com/info/MCI_Control_Info.html

*/

package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	w := newWav(os.Args[1])
	err := w.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()

	err = w.ReadRiffChunk()
	if err != nil {
		log.Fatal(err)
	}
	w.PrintRiffChunk()

	err = w.ReadFmtChunk()
	if err != nil {
		log.Fatal(err)
	}
	w.PrintFmtChunk()

	err = w.ReadDataChunk()
	if err != nil {
		log.Fatal(err)
	}
	w.PrintDataChunk()
}

func bytesToLittleEndianInt(buffer []byte) int {
	var out int
	for i, b := range buffer {
		intB := int(b)
		intB <<= 8 * i
		out |= intB
	}
	return out
}

type RiffChunk struct {
	ChunkID   []byte // big endian.        "RIFF"
	ChunkSize []byte // little endian.
	Format    []byte // big endian.        "WAVE"
}

type FmtChunk struct {
	Subchunk1ID   []byte // big endian.    "fmt "
	Subchunk1Size []byte // little endian.
	AudioFormat   []byte // little endian.
	NumChannels   []byte // little endian.
	SampleRate    []byte // little endian.
	ByteRate      []byte // little endian.
	BlockAlign    []byte // little endian.
	BitsPerSample []byte // little endian.
	ExtraData     []byte // For when the format chunk is greated than 16
}

type DataChunk struct {
	Subchunk2ID   []byte // big endian.    "data"
	Subchunk2Size []byte // little endian.
	data          []byte // little endian. The size of the data = Subchunk2Size
}

type Wav struct {
	FileName string
	fp       *os.File

	riffChunk RiffChunk
	fmtChunk  FmtChunk
	dataChunk DataChunk
}

func newWav(fileName string) *Wav {
	return &Wav{
		FileName: fileName,
	}
}

func (w *Wav) Open() error {
	fp, err := os.Open(w.FileName)
	if err != nil {
		return fmt.Errorf("Failed to open wav file: %v", err)
	}
	w.fp = fp
	return nil
}

func (w *Wav) Close() error {
	return w.fp.Close()
}

func (w *Wav) ReadRiffChunk() error {
	w.riffChunk = RiffChunk{
		ChunkID:   make([]byte, 4),
		ChunkSize: make([]byte, 4),
		Format:    make([]byte, 4),
	}
	if _, err := w.fp.Read(w.riffChunk.ChunkID); err != nil {
		return fmt.Errorf("Failed to read riff chunk: %v", err)
	}
	if _, err := w.fp.Read(w.riffChunk.ChunkSize); err != nil {
		return fmt.Errorf("Failed to read riff chunk: %v", err)
	}
	if _, err := w.fp.Read(w.riffChunk.Format); err != nil {
		return fmt.Errorf("Failed to read riff chunk: %v", err)
	}
	return nil
}

func (w *Wav) PrintRiffChunk() {
	fmt.Printf(`-- RIFF CHUNK --
%20s %s
%20s %d
%20s %s
`,
		"ChunkID:", w.riffChunk.ChunkID,
		"ChunkSize:", bytesToLittleEndianInt(w.riffChunk.ChunkSize),
		"Format:", w.riffChunk.Format,
	)
}

func (w *Wav) ReadFmtChunk() error {
	w.fmtChunk = FmtChunk{
		Subchunk1ID:   make([]byte, 4),
		Subchunk1Size: make([]byte, 4),
		AudioFormat:   make([]byte, 2),
		NumChannels:   make([]byte, 2),
		SampleRate:    make([]byte, 4),
		ByteRate:      make([]byte, 4),
		BlockAlign:    make([]byte, 2),
		BitsPerSample: make([]byte, 2),
	}
	if _, err := w.fp.Read(w.fmtChunk.Subchunk1ID); err != nil {
		return fmt.Errorf("Failed to read fmt chunk: %v", err)
	}
	if _, err := w.fp.Read(w.fmtChunk.Subchunk1Size); err != nil {
		return fmt.Errorf("Failed to read fmt chunk: %v", err)
	}
	if _, err := w.fp.Read(w.fmtChunk.AudioFormat); err != nil {
		return fmt.Errorf("Failed to read fmt chunk: %v", err)
	}
	if _, err := w.fp.Read(w.fmtChunk.NumChannels); err != nil {
		return fmt.Errorf("Failed to read fmt chunk: %v", err)
	}
	if _, err := w.fp.Read(w.fmtChunk.SampleRate); err != nil {
		return fmt.Errorf("Failed to read fmt chunk: %v", err)
	}
	if _, err := w.fp.Read(w.fmtChunk.ByteRate); err != nil {
		return fmt.Errorf("Failed to read fmt chunk: %v", err)
	}
	if _, err := w.fp.Read(w.fmtChunk.BlockAlign); err != nil {
		return fmt.Errorf("Failed to read fmt chunk: %v", err)
	}
	if _, err := w.fp.Read(w.fmtChunk.BitsPerSample); err != nil {
		return fmt.Errorf("Failed to read fmt chunk: %v", err)
	}

	minFmtChunkSize := 16
	subchunk1Size := bytesToLittleEndianInt(w.fmtChunk.Subchunk1Size)
	if subchunk1Size > minFmtChunkSize {
		// The chunk size does not include the id or size fields, or any padding
		w.fmtChunk.ExtraData = make([]byte, subchunk1Size-16)
		if _, err := w.fp.Read(w.fmtChunk.ExtraData); err != nil {
			return fmt.Errorf("Failed to read fmt chunk: %v", err)
		}
	}

	return nil
}

func (w *Wav) PrintFmtChunk() {
	fmt.Printf(`-- FMT CHUNK --
%20s %s
%20s %d
%20s %d
%20s %d
%20s %d
%20s %d
%20s %d
%20s %d
`,
		"Subchunk1ID:", w.fmtChunk.Subchunk1ID,
		"Subchunk1Size:", bytesToLittleEndianInt(w.fmtChunk.Subchunk1Size),
		"AudioFormat:", bytesToLittleEndianInt(w.fmtChunk.AudioFormat),
		"NumChannels:", bytesToLittleEndianInt(w.fmtChunk.NumChannels),
		"SampleRate:", bytesToLittleEndianInt(w.fmtChunk.SampleRate),
		"ByteRate:", bytesToLittleEndianInt(w.fmtChunk.ByteRate),
		"BlockAlign:", bytesToLittleEndianInt(w.fmtChunk.BlockAlign),
		"BitsPerSample:", bytesToLittleEndianInt(w.fmtChunk.BitsPerSample),
	)
}

func (w *Wav) ReadDataChunk() error {
	w.dataChunk = DataChunk{
		Subchunk2ID:   make([]byte, 4),
		Subchunk2Size: make([]byte, 4),
	}
	if _, err := w.fp.Read(w.dataChunk.Subchunk2ID); err != nil {
		return fmt.Errorf("Failed to read data chunk: %v", err)
	}
	if _, err := w.fp.Read(w.dataChunk.Subchunk2Size); err != nil {
		return fmt.Errorf("Failed to read data chunk: %v", err)
	}
	return nil
}

func (w *Wav) PrintDataChunk() {
	fmt.Printf(`-- DATA CHUNK --
%20s %s
%20s %d
`,
		"Subchunk2ID:", w.dataChunk.Subchunk2ID,
		"Subchunk2Size:", bytesToLittleEndianInt(w.dataChunk.Subchunk2Size),
	)
}
