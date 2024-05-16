package resourcepool

import (
	"context"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/klauspost/compress/zstd"
	log "github.com/openmesh-network/core/internal/logger"
)

// Keeps track of Cids in data.
type Stream struct {
	inst                *Instance
	buffer              []byte
	cidHashes           []cid.Cid
	compressedBuffer    []byte
	compressedBufferOld []byte
}

func (inst *Instance) NewStream() *Stream {

	s := &Stream{
		inst:                inst,
		buffer:              make([]byte, 0, DEFAULT_CHUNK_SIZE*4),
		compressedBuffer:    make([]byte, 0, DEFAULT_CHUNK_SIZE*4),
		compressedBufferOld: make([]byte, 0, DEFAULT_CHUNK_SIZE*4),
		cidHashes:           make([]cid.Cid, 0),
	}
	return s
}

// Resets the cids and the buffer to 0.
func (s *Stream) resetBuffers() {
	s.buffer = s.buffer[:0]
	s.compressedBuffer = s.compressedBuffer[:0]
	s.compressedBufferOld = s.compressedBufferOld[:0]
}

func (s *Stream) Reset() {
	s.resetBuffers()
	s.cidHashes = s.cidHashes[:0]
}

func (s *Stream) flush(buffer []byte) {
	// Add buffer to blockstore with padding and reset buffer.

	size := len(buffer)
	for i := 0; i < cap(buffer)-size; i++ {
		buffer = append(buffer, 0)
	}

	{
		// XXX: Maybe move this to other resource pool function.
		var c cid.Cid
		{
			bufferCopy := make([]byte, len(buffer))
			copy(bufferCopy, buffer[:])
			block := blocks.NewBlock(bufferCopy)
			c = block.Cid()

			err := s.inst.Bservice.AddBlock(context.Background(), block)
			if err != nil {
				panic(err)
			}

			log.Debug("Got cid: ", c)
		}

		s.cidHashes = append(s.cidHashes, c)
	}

	s.resetBuffers()
}

func (s *Stream) Flush() {
	// Add buffer to blockstore with padding and reset buffer.

	s.flush(s.compressedBufferOld)
}

var encoder *zstd.Encoder

func init() {
	enc, err := zstd.NewWriter(nil)
	if err != nil {
		panic(err)
	}
	encoder = enc
}

func (s *Stream) Append(message []byte) {
	// XXX: Decide on a proper way to handle this.
	// ("This" meaning getting a single large message that is bigger than the entire buffer)
	// - Indicating message is fragmented somewhere.
	// - Assuming this never happens.
	// - Etc

	// If buffer will become full.
	// if len(*s.compressedBuffer) > DEFAULT_CHUNK_SIZE-len(message) {
	// 	s.Flush()
	// }

	// if len(message) > DEFAULT_CHUNK_SIZE {
	// 	for len(message) > 0 {

	// 		fmt.Println("Message too long, splitting.")
	// 		s.Append(message[:min(len(message), DEFAULT_CHUNK_SIZE)])
	// 		message = message[DEFAULT_CHUNK_SIZE:]
	// 	}
	// }

	s.buffer = append(s.buffer, message...)

	s.compressedBufferOld = s.compressedBufferOld[:0]
	s.compressedBufferOld = append(s.compressedBufferOld, s.compressedBuffer...)

	s.compressedBuffer = s.compressedBuffer[:0]
	// XXX: Improve performance here, options:
	//	- Use faster implementation.
	//	- Use stream API to reduce overhead?
	s.compressedBuffer = encoder.EncodeAll(s.buffer, s.compressedBuffer)

	if len(s.compressedBuffer) > DEFAULT_CHUNK_SIZE {
		s.Flush()

		s.Append(message[:len(message)/2])
		s.Append(message[len(message)/2:])
	}
}

func (s *Stream) GetCids() []cid.Cid {
	return s.cidHashes
}
