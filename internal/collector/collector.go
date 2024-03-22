package collector

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	log "github.com/openmesh-network/core/internal/logger"
	"github.com/sourcegraph/conc"
)

type Request struct {
	Source Source
	Topic  int
}

type Summary struct {
	Request Request
	// XXX: This might not be efficient, array of pointers means many cache misses.
	// Not sure if the Go compiler will realize we want these sequentially in memory.
	DataHashes []cid.Cid
}

type CollectorInstance struct {
	// Lower in the queue is higher priority
	requestsByPriorityCurrent []Request
	requestsByPriorityNew     []Request
	summariesLatest           []Summary
	requestNotifyChannel      chan struct{}
	stop                      chan struct{}
}

const CONNECTIONS_MAX = 1

// const BUFFER_SIZE_MAX = 1024
// const BUFFER_MAX = 1024

func New() CollectorInstance {
	return CollectorInstance{}
}

func (collectorInstance *CollectorInstance) SubmitRequests(requestsSortedByPriority []Request) {
	collectorInstance.requestsByPriorityNew = make([]Request, len(requestsSortedByPriority))

	copy(collectorInstance.requestsByPriorityNew, requestsSortedByPriority)
	collectorInstance.requestNotifyChannel <- struct{}{}
}

// Return latest summaries, wait this is a race condition :facepalm:.
func (collectorInstance *CollectorInstance) FetchSummaries() []Summary {
	// We could tell the collector to run the .
	// TODO: Add a mutex here.

	return collectorInstance.summariesLatest
}

func runSubscription(req Request, buffer []byte, summary *Summary, stopChannel chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if buffer == nil {
		panic("Buffer is nil dummy.")
	}
	if len(buffer) < 100 {
		panic("Buffer is too small, is this an error?")
	}

	messageChannel, err := Subscribe(ctx, req.Source, req.Source.Topics[req.Topic])

	// TODO: If there's an error connecting to a source tell caller to avoid wasting collector slot on empty data.
	if err != nil {
		log.Error(err)
		return
	}

	bufferOffset := 0
	summary.Request = req

	// XXX: Maybe implement this function in RP?
	bufferSummaryAppend := func(summary *Summary, buffer []byte, length int) {
		// TODO: Consider adding:
		//	- Timestamp.
		//	- Fragmentation flag (Whether there is a half message or not).
		//	- Message count.

		cidBuilder := cid.V1Builder{
			Codec:    uint64(multicodec.DagPb),
			MhType:   uint64(multicodec.Sha2_256),
			MhLength: -1,
		}

		c, err := cidBuilder.Sum(buffer[0:length])
		if err != nil {
			// If this fails to parse a buffer the input is invalid.
			panic(err)
		}

		summary.DataHashes = append(summary.DataHashes, c)
	}

	for {
		select {
		case <-stopChannel:
			// TODO: Flush buffer?

			cancel()
			return
		case message := <-messageChannel:
			// Got a message, add it to buffer.

			// We do a for loop here because the message itself might be bigger than the total size of the buffer.
			if bufferOffset+len(message) > len(buffer) {
				// Buffer would be filled past total size, convert to cid and push to summary pointer.
				// This might introduce a problem. Lets say we store 1kb buffer:
				//	If a source puts out less than 1kb per blocktime, then it won't get registered at all!
				//	Might have to flush the buffer on stopChannel event, to preserve that data.

				// TODO: Add to Resource Pool at this stage?
				bufferSummaryAppend(summary, buffer, bufferOffset)
				bufferOffset = 0
			}

			// If the message still doesn't fit, divide it into chunks and add it until it fits.
			for len(message) > len(buffer) {
				// XXX: Should the cids we post be capped at the length of the buffer?
				// Or can they be any size? For now I assume they are capped at the size of the buffer.
				bufferSummaryAppend(summary, message, len(buffer))
				message = message[len(buffer):]
			}

			// Add message to buffer.
			copy(buffer[bufferOffset:], message)

			bufferOffset += len(message)
		}
	}
}

func (collectorInstance *CollectorInstance) Start() {
	collectorInstance.stop = make(chan struct{})

	go func() {
		// XXX: Might have to remake the waitgroup on every iteration.
		var wg conc.WaitGroup
		stopChannel := make(chan struct{})
		for {
			select {
			case <-collectorInstance.stop:
				return
			case <-collectorInstance.requestNotifyChannel:
				// Wait for a request to be submitted.
				// Once they are submitted, we act on it by launching a collector.
				if collectorInstance.requestsByPriorityNew != nil {
					// Stop all subscriptions running currently.

					log.Info("Stopping subscriptions...")
					stopChannel <- struct{}{}
					// Have to wait here to avoid race condition when touching summariesLatest data.
					wg.Wait()

					log.Info("Stopped subscriptions!")

					// Window to fetch the requests.
					{
						// Do I send them as transactions?
						// Do I wait for someone to request them?
					}
				}

				// Go through all the available connections and launch a new subscription goroutine for each of them.
				log.Info("Adding sources...")
				for i := 0; i < CONNECTIONS_MAX; i++ {
					req := collectorInstance.requestsByPriorityNew[i]
					buffer := make([]byte, 1024*4)
					wg.Go(func() { runSubscription(req, buffer, &collectorInstance.summariesLatest[i], stopChannel) })
				}

				// Move new to current.
				copy(collectorInstance.requestsByPriorityCurrent, collectorInstance.requestsByPriorityNew)
				collectorInstance.requestsByPriorityNew = nil
			}
		}
	}()
}

func (collectorInstance *CollectorInstance) Stop() {
	collectorInstance.stop <- struct{}{}
}
