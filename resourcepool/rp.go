package resourcepool

import (
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
)

// What I want.
const DEFAULT_CHUNK_SIZE = 4096

// Keeps track of Cids in data.
type Stream struct {
	buffer      []byte
	offset      int
	chunkHashes []cid.Cid
}

func NewStream() *Stream {
	return &Stream{
		make([]byte, 0, DEFAULT_CHUNK_SIZE),
		0,
		make([]cid.Cid, 0),
	}
}

func (s *Stream) Flush() {
	// Add buffer to blockstore with padding and reset buffer.

	for i := 0; i < cap(s.buffer)-len(s.buffer); i++ {
		s.buffer = append(s.buffer, 0)
	}

	{ // TODO: Replace this with blockstore.
		cidBuilder := cid.V1Builder{
			Codec:    uint64(multicodec.DagPb),
			MhType:   uint64(multicodec.Sha2_256),
			MhLength: -1,
		}

		c, err := cidBuilder.Sum(s.buffer[:])
		if err != nil {
			// If this fails to parse a buffer the input is invalid.
			panic(err)
		}
		s.chunkHashes = append(s.chunkHashes, c)
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
	if len(message) > cap(s.buffer) {
		// Append as unique messages, until we reached a message size that can be used.
		// WARN: Do NOT multithread any of this code. All this logic depends on being run sequentially.
		for len(message) > cap(s.buffer) {
			s.Append(message)
			message = message[cap(s.buffer):]
		}
	}

	// If buffer will become full.
	if len(s.buffer) > cap(s.buffer)-len(message) {
		s.Flush()
	}

	// Add message to the buffer.
	s.buffer = append(s.buffer, message...)
}

// Add a stream to ipfs.
