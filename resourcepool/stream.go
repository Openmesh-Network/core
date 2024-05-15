package resourcepool

import (
	"context"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	log "github.com/openmesh-network/core/logger"
)

// Keeps track of Cids in data.
type Stream struct {
	inst             *Instance
	buffer           []byte
	cidHashes        []cid.Cid
	// compressedBuffer []byte
}

func (inst *Instance) NewStream() *Stream {
	return &Stream{
		inst:             inst,
		buffer:           make([]byte, 0, DEFAULT_CHUNK_SIZE),
		// compressedBuffer: make([]byte, 0, DEFAULT_CHUNK_SIZE),
		cidHashes:        make([]cid.Cid, 0),
	}
}

// Resets the cids and the buffer to 0.
func (s *Stream) Reset() {
	s.buffer = s.buffer[:0]
	s.cidHashes = s.cidHashes[:0]
}

func (s *Stream) Flush() {
	// Add buffer to blockstore with padding and reset buffer.

	size := len(s.buffer)
	for i := 0; i < cap(s.buffer)-size; i++ {
		s.buffer = append(s.buffer, 0)
	}

	{
		// XXX: Maybe move this to other resource pool function.
		var c cid.Cid
		{
			bufferCopy := make([]byte, len(s.buffer))
			copy(bufferCopy, s.buffer[:])
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

	// Reset buffer.
	s.buffer = s.buffer[:0]
}

func (s *Stream) Append(message []byte) {
	// XXX: Decide on a proper way to handle this.
	// ("This" meaning getting a single large message that is bigger than the entire buffer)
	// - Indicating message is fragmented somewhere.
	// - Assuming this never happens.
	// - Etc
	for len(message) > cap(s.buffer) {
		// Append as unique messages, until we reached a message size that can be used.
		// WARN: Do NOT multithread any of this code. All this logic depends on being run sequentially.
		s.Append(message[:cap(s.buffer)])
		message = message[cap(s.buffer):]
	}

	// If buffer will become full.
	if len(s.buffer) > cap(s.buffer)-len(message) {
		s.Flush()
	}

	s.buffer = append(s.buffer, message...)
}

func (s *Stream) GetCids() []cid.Cid {
	return s.cidHashes
}
