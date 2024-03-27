package collector

import (
	"context"
	"time"

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
	requestNotifyChannel      chan bool
	stop                      chan bool
}

const CONNECTIONS_MAX = 10 

// const BUFFER_SIZE_MAX = 1024
// const BUFFER_MAX = 1024

func New() *CollectorInstance {
	return &CollectorInstance{
		requestNotifyChannel: make(chan bool),
		stop:                 make(chan bool),
	}
}

func (collectorInstance *CollectorInstance) SubmitRequests(requestsSortedByPriority []Request) {
	collectorInstance.requestsByPriorityNew = make([]Request, len(requestsSortedByPriority))
	collectorInstance.summariesLatest = make([]Summary, len(requestsSortedByPriority))

	copy(collectorInstance.requestsByPriorityNew, requestsSortedByPriority)

	collectorInstance.requestNotifyChannel <- true
}

// Return latest summaries, wait this is a race condition :facepalm:.
func (collectorInstance *CollectorInstance) FetchSummaries() []Summary {
	// We could tell the collector to run the .
	// TODO: Add a mutex here.

	return collectorInstance.summariesLatest
}

func runSubscription(ctx context.Context, req Request, buffer []byte, summary *Summary, stop chan struct{}) {
	log.Info("Called runSubscription.")
	// XXX: Using context.Background() not ideal according to a senior, might lead to weird bugs.
	childrenCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	if buffer == nil {
		panic("Buffer is nil dummy.")
	}
	if len(buffer) < 100 {
		panic("Buffer is too small, is this an error?")
	}

	log.Info("About to subscribe to ", req.Source.Name)
	messageChannel, err := Subscribe(childrenCtx, req.Source, req.Source.Topics[req.Topic])
	log.Info("Successfuly subscribed to ", req.Source.Name, ", ", req.Source.Topics[req.Topic])

	// TODO: If there's an error connecting to a source tell caller to avoid wasting collector slot on empty data.
	if err != nil {
		// panic(err)
		<-stop
		return
	}

	bufferOffset := 0
	summary.Request = req

	// XXX: Maybe implement this function in RP? Also it will crash if length == 0
	summaryAppend := func(summary *Summary, buffer []byte, length int) {
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
		log.Info("Added ", length, "  bytes, now: "+c.String())
	}

	var prevMessage []byte
	printedDebug := false
	for {
		select {
		case <-ctx.Done():
			log.Info("Context cancelled.")
			return
		case <-stop:
			log.Info("Channel stopped.")
			// TODO: Flush buffer?
			if len(buffer) > 0 {
				summaryAppend(summary, buffer, len(buffer))
			}

			log.Info("Gone.")
			return
		case message := <-messageChannel:
			if len(message) == 0 {
				if !printedDebug {
					log.Info("Got message with length 0, that means we probs disconnected :(")
					log.Info("Last message was: ", string(prevMessage))
				}
				printedDebug = true
				break
			} else {
				log.Info("Got message: ", len(message))
				prevMessage = message
				if len(message) > 100 {
					log.Info(string(message[:100]))
				}
			}

			// Got a message, add it to buffer.

			// log.Info("Went through this part here.")
			// We do a for loop here because the message itself might be bigger than the total size of the buffer.
			if bufferOffset+len(message) > len(buffer) {
				// Buffer would be filled past total size, convert to cid and push to summary pointer.
				// This might introduce a problem. Lets say we store 1kb buffer:
				//	If a source puts out less than 1kb per blocktime, then it won't get registered at all!
				//	Might have to flush the buffer on stopChannel event, to preserve that data.

				// TODO: Add to Resource Pool at this stage?
				summaryAppend(summary, buffer, bufferOffset)
				bufferOffset = 0
			}

			// log.Info("Went through this part.")
			// If the message still doesn't fit, divide it into chunks and add it until it fits.
			for len(message) > len(buffer) {
				// XXX: Should the cids we post be capped at the length of the buffer?
				// Or can they be any size? For now I assume they are capped at the size of the buffer.
				summaryAppend(summary, message, len(buffer))
				message = message[len(buffer):]
			}

			// Add message to buffer.
			copy(buffer[bufferOffset:], message)
			// log.Info("Done here.")

			bufferOffset += len(message)
		}
	}
}

func (collectorInstance *CollectorInstance) Start(ctx context.Context) {
	log.Infof("Started collector instance.")

	go func() {
		// XXX: Might have to remake the waitgroup on every iteration.
		var wg conc.WaitGroup

		subscriptionsCtx, subscriptionsCancel := context.WithCancel(ctx)
		defer subscriptionsCancel()

		subscriptionStopChannels := make([]chan struct{}, CONNECTIONS_MAX)
		for i := range subscriptionStopChannels {
			subscriptionStopChannels[i] = make(chan struct{})
		}
		subscriptionCount := 0

		for {
			log.Infof("Polling..")
			select {
			case <-collectorInstance.requestNotifyChannel:
				log.Info("Got new notification...")
				// Wait for a request to be submitted.
				// Once they are submitted, we act on it by launching a collector.
				if collectorInstance.requestsByPriorityNew != nil {
					// Stop all subscriptions running currently.

					log.Info("Stopping ", subscriptionCount, " subscriptions...")

					// We have to wait for as long as we have subscriptions.
					for i := 0; i < subscriptionCount; i++ {
						log.Info("Sending message to channel")
						subscriptionStopChannels[i] <- struct{}{}
						log.Info("Canceled subscription")
					}
					subscriptionCount = 0
					log.Info("Waiting now.")
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
				iterations := min(CONNECTIONS_MAX,
					len(collectorInstance.requestsByPriorityNew),
					len(collectorInstance.summariesLatest))

				for i := 0; i < iterations; i++ {
					log.Info(len(collectorInstance.requestsByPriorityNew), len(collectorInstance.summariesLatest), len(subscriptionStopChannels), i)
					req := collectorInstance.requestsByPriorityNew[i]
					buffer := make([]byte, 1024*4)

					stopChannel := subscriptionStopChannels[i]
					summaryPtr := &collectorInstance.summariesLatest[i]

					runSubscriptionFunc := func() {
						runSubscription(subscriptionsCtx, req, buffer, summaryPtr, stopChannel)
					}

					wg.Go(runSubscriptionFunc)
					// go runSubscriptionFunc()
					subscriptionCount++
				}

				// Move new to current.
				copy(collectorInstance.requestsByPriorityCurrent, collectorInstance.requestsByPriorityNew)
				collectorInstance.requestsByPriorityNew = nil

			case <-ctx.Done():
				wg.Wait()
				return
			}
		}
	}()
}
