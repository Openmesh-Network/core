package collector

import (
	"context"

	"github.com/ipfs/go-cid"
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

func New() {
}

func (collectorInstance *CollectorInstance) SubmitRequests(requestsSortedByPriority []Request) {
	// Stop running previous collector?
	// Fill up a new buffer and let the thread design how to proceed.
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

	for {
		select {
		case <-stopChannel:
			cancel()
			return
		case message := <-messageChannel:
			// Got a message, add it to buffer.

			if bufferOffset+len(message) > len(buffer) {
				// Buffer would be filled past total size, convert to cid and push to summary pointer.

				// TODO: Add to Resource Pool at this stage?
			}
		}
	}
}

func (collectorInstance *CollectorInstance) Start() {
	collectorInstance.stop = make(chan struct{})

	go func() {
		var wg conc.WaitGroup
		stopChannel := make(chan struct{})
		for {
			select {
			case <-collectorInstance.stop:
				return
			case <-collectorInstance.requestNotifyChannel:
				// Wait for a request to be submitted.
				// Once they are submitted, we act on it by launching a collector.
				if collectorInstance.requestsByPriorityCurrent != nil {
					// Stop all subscriptions running currently.
					stopChannel <- struct{}{}
					// Have to wait here to avoid race condition when touching summariesLatest data.
					wg.Wait()

					// Window to fetch the requests.
					{
					}

					// Move current to new and new to current.

				}

				// Go through all the available connections and launch a new subscription goroutine for each of them.
				for i := 0; i < CONNECTIONS_MAX; i++ {
					req := collectorInstance.requestsByPriorityNew[i]
					buffer := make([]byte, 1024)
					wg.Go(func() { runSubscription(req, buffer, &collectorInstance.summariesLatest[i], stopChannel) })
				}

				// default:
				// If stop is called, then we stop.
				// If we get another request, we discard all our previous work and do something else.
				// Should we use two functions? We could use a circular buffer and cap our storage at half maybe?

			}
		}
	}()
}

func (collectorInstance *CollectorInstance) Stop() {
	collectorInstance.stop <- struct{}{}
}
