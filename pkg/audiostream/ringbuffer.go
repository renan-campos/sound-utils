package audiostream

import "sync"

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
