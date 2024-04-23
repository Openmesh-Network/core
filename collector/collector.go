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
	// XXX: This might not be efficient, array of pointers means many cache misses.
	// Not sure if the Go compiler will realize we want these sequentially in memory.
	// But whatever man we're only doing a handfull of these a second.
	DataHashes []cid.Cid
}

type CollectorWorker struct {
	summary *Summary
	request Request
	message chan []byte

	// Could make these into the same function.
	pause  chan bool
	resume chan bool
}

type CollectorInstance struct {
	ctx                       context.Context
	workers                   [WORKER_COUNT]CollectorWorker
	workerWaitGroup           conc.WaitGroup
	requestsByPriorityCurrent []Request
	requestsByPriorityNew     []Request
	summariesNew              [WORKER_COUNT]Summary
	summariesOld              [WORKER_COUNT]Summary
	subscriptionsContext      context.Context
	subscriptionsCancel       context.CancelFunc
}

const WORKER_COUNT = 5

// const BUFFER_SIZE_MAX = 1024
// const BUFFER_MAX = 1024

func New() *CollectorInstance {
	return &CollectorInstance{}
}

func (ci *CollectorInstance) SubmitRequests(requestsSortedByPriority []Request) []Summary {
	// XXX: Wasteful, shouldn't have to remake but whatever.
	ci.requestsByPriorityNew = make([]Request, len(requestsSortedByPriority))

	copy(ci.requestsByPriorityNew, requestsSortedByPriority)

	if ci.subscriptionsCancel != nil {
		ci.subscriptionsCancel()
	}

	ci.subscriptionsContext, ci.subscriptionsCancel = context.WithCancel(ci.ctx)

	log.Info("Pausing workers")
	for i := range ci.workers {
		log.Info("Pausing worker ", i)
		// This tells worker to flush the messages buffer and stop reading the summary pointer.
		ci.workers[i].pause <- true
	}

	for i := range ci.workers {
		// Wait until they've finished.
		<-ci.workers[i].pause
	}

	// Now the old summaries are up to date.
	copy(ci.summariesOld[:], ci.summariesNew[:])
	for i := range ci.summariesNew {
		// XXX: Test if this actually allocates memory or if Go compiler understands I want the same buffer basically.
		ci.summariesOld[i].DataHashes = make([]cid.Cid, len(ci.summariesNew[i].DataHashes))
		copy(ci.summariesOld[i].DataHashes, ci.summariesNew[i].DataHashes)
	}

	log.Info("Subscribing to requests.")

	// NOTE: I only multithread this bit because its the slowest, all the other sections of this run pretty quickly
	// so there's no reason to multithread them.
	subscribeWaitGroup := conc.NewWaitGroup()
	for i := 0; i < min(len(ci.workers), len(requestsSortedByPriority)); i++ {

		// Have to declare variable here otherwise go will pass i as value and cause problems.
		index := i
		subscribeFunc := func() {
			r := requestsSortedByPriority[index]

			// Subscribe to new source.
			// TODO: If a worker is already subscribed to a source don't end the subscription.
			// Significant rewrite, but might improve performance.

			log.Info("Subscribing ", index)
			messageChannel, err := Subscribe(ci.subscriptionsContext, r.Source, r.Source.Topics[r.Topic])
			if err != nil {
				// XXX: Handle this case by skipping this request.
				// Panicking now to highlight this.
				panic(err)
			}

			ci.workers[index].message = messageChannel
		}

		subscribeWaitGroup.Go(subscribeFunc)
	}
	subscribeWaitGroup.Wait()

	for i := range ci.workers {
		log.Info("Resuming ", i)
		ci.workers[i].resume <- true
	}
	for i := range ci.workers {
		<-ci.workers[i].resume
	}

	maxSummaries := min(len(ci.summariesOld), len(requestsSortedByPriority))
	return ci.summariesOld[:maxSummaries]
}

func (cw *CollectorWorker) run(ctx context.Context, buffer []byte) {
	log.Info("Started worker.")

	if buffer == nil {
		panic("Buffer is nil dummy.")
	}
	if len(buffer) < 100 {
		panic("Buffer is too small, is this an error?")
	}

	// XXX: Maybe move this function to RP? Also it will crash if length == 0
	// Also I could move this to another function.
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

		log.Info("Added ", length, "  bytes, now: ", c.String())
	}

	bufferOffset := 0
	printedDebug := false
	paused := false
	log.Info("Running for loop.")
	for {
		select {
		case <-ctx.Done():
			log.Info("Context cancelled.")
			return
		default:
			if paused {
				select {
				case <-cw.resume:
					log.Info("Worker resumed.")
					// Clear the summary cid buffer.
					cw.summary.DataHashes = cw.summary.DataHashes[:0]
					paused = false

					// Tell collector we're done resuming.
					cw.resume <- true
				}
			} else {
				select {
				case <-cw.pause:
					log.Info("Channel stopped.")

					// Flush the buffer!
					if len(buffer) > 0 {
						log.Info("Flushed")
						summaryAppend(cw.summary, buffer, len(buffer))
					}

					// Hopefully go will just call memset here...
					for i := range buffer {
						buffer[i] = 0
					}
					bufferOffset = 0

					log.Info("Worker paused until resume is called.")
					paused = true

					// Tell collector we've finished pausing.
					cw.pause <- true
				case message := <-cw.message:
					// XXX: This looks ugly, whatever.
					if len(message) == 0 {
						if !printedDebug {
							log.Info("Got message with length 0, that means we probs disconnected :(")
						}
						printedDebug = true
						break
					}

					if bufferOffset+len(message) > len(buffer) {
						// TODO: Add to Resource Pool from here?
						summaryAppend(cw.summary, buffer, bufferOffset)
						bufferOffset = 0
					}

					// If the message still doesn't fit, divide it into chunks and add it until it fits.
					for len(message) > len(buffer) {
						// XXX: Should the cids we post be capped at the length of the buffer?
						// Or can they be any size? For now I assume they are capped at the size of the buffer.
						// Do we do padding? Need a spreadsheet to "empirically" test this.
						summaryAppend(cw.summary, message, len(buffer))
						message = message[len(buffer):]
					}

					// Add message to buffer.
					copy(buffer[bufferOffset:], message)
					// log.Info("Done here.")

					bufferOffset += len(message)
				}
			}
		}
	}
}

func (ci *CollectorInstance) Start(ctx context.Context) {
	log.Infof("Started collector instance.")
	ci.ctx = ctx

	for i := range ci.workers {
		buffer := make([]byte, 4096)
		ci.workers[i].pause = make(chan bool)
		ci.workers[i].resume = make(chan bool)
		ci.workers[i].message = make(chan []byte)
		ci.workers[i].summary = &ci.summariesNew[i]

		index := i
		runFunc := func() { ci.workers[index].run(ci.ctx, buffer) }

		log.Infof("Deploying worker for collector.")
		ci.workerWaitGroup.Go(runFunc)
	}
}

func (ci *CollectorInstance) Stop() {
	// This only works if the context was cancelled, otherwise the worker goroutines will block this.
	ci.workerWaitGroup.Wait()
}
