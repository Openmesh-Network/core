package collector

import (
	"context"
	"math"
	"time"

	"github.com/ipfs/go-cid"
	log "github.com/openmesh-network/core/internal/logger"
	"github.com/openmesh-network/core/resourcepool"
	"github.com/sourcegraph/conc"
)

type Request struct {
	Source Source
	Topic  int
}

type Summary struct {
	DataHashes []cid.Cid
}

type MessageTimeTuple struct {
	messageData []byte
	messageTime time.Time
}

type CollectorWorker struct {
	// Anchor mode means the worker is recording the last N messages and timestamps for syncing, but not passing it to the resource pool.
	anchorMode bool

	// If we're in anchor mode then we keep these buffers around to track things.
	// XXX: Give the anchor message buffer a fixed length? Should improve performance.
	// Don't want to store all our data .
	anchorDataBuffer    []byte
	anchorMessageBuffer []MessageTimeTuple

	summary  *Summary
	request  Request
	message  chan []byte
	rpStream *resourcepool.Stream

	// Could make these into the same function.
	pause  chan bool
	resume chan bool
}

type CollectorInstance struct {
	rpInstance           *resourcepool.Instance
	ctx                  context.Context
	workers              [WORKER_COUNT]CollectorWorker
	workerWaitGroup      conc.WaitGroup
	summariesNew         [WORKER_COUNT / 2]Summary
	summariesOld         [WORKER_COUNT / 2]Summary
	subscriptionsContext context.Context
	subscriptionsCancel  context.CancelFunc
}

// Odd workers look at current. Even workers look ahead?
// Otherwise I can do a treadmill system.
const WORKER_COUNT = 2

// const BUFFER_SIZE_MAX = 1024
// const BUFFER_MAX = 1024

func NewInstance(rpInstance *resourcepool.Instance) *CollectorInstance {
	return &CollectorInstance{rpInstance: rpInstance}
}

func (ci *CollectorInstance) SubmitRequests(requests []Request, requestsNext []Request, targetAnchorTime time.Time) []Summary {
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
		ci.summariesOld[i].DataHashes = ci.summariesOld[i].DataHashes[:0]
		ci.summariesOld[i].DataHashes = append(ci.summariesOld[i].DataHashes, ci.summariesNew[i].DataHashes...)
	}

	log.Info("Subscribing to requests.")

	// NOTE: I only multithread this bit because its the slowest, all the other sections of this run pretty quickly
	// so there's no reason to multithread them.
	subscribeWaitGroup := conc.NewWaitGroup()
	for i := 0; i < min(len(ci.workers), len(requests)+len(requestsNext)); i++ {
		// Have to declare variable here otherwise go will pass i as value and cause problems.
		var anchorMode bool
		if i%2 == 0 {
			anchorMode = false
		} else {
			anchorMode = true
		}

		nextNodeIsAnchor := false
		if i+1 < len(ci.workers) && !anchorMode {
			if len(ci.workers[i+1].anchorMessageBuffer) > 0 && ci.workers[i+1].message != nil {
				nextNodeIsAnchor = true
			}
		}

		index := i / 2

		subscribeFunc := func() {
			var r Request

			if anchorMode {
				r = requests[index]
			} else {
				r = requestsNext[index]
			}

			log.Info("Subscribing ", requests[index])

			var messageChannel chan []byte
			if anchorMode || !nextNodeIsAnchor {
				var err error
				messageChannel, err = Subscribe(ci.subscriptionsContext, r.Source, r.Source.Topics[r.Topic])

				if err != nil {
					log.Warn("Couldnt connect to source: ", r.Source.Name, "-", r.Source.Topics[r.Topic], ", skipping instead. Reason: ", err)
				}
			} else {
				// Just keep message channel from next node.
				// This way no data is lost while we're paused.
				messageChannel = ci.workers[i+1].message
			}

			// XXX: Handle this case by skipping this request.
			// Worker will do nothing for this period. Maybe optimize this?
			if messageChannel != nil {
				ci.workers[i].message = messageChannel
				// Note(Tom): The anchor mode doesn't change at all currently.
				// Just added this here since it might be changed in the future.
				ci.workers[i].anchorMode = anchorMode

				if !anchorMode {
					// Here's where we do the anchoring.
					ci.workers[i].rpStream.Reset()
					ci.workers[i].summary.DataHashes = ci.workers[i].summary.DataHashes[:0]

					// Find the message in the next nodes that's closest to the time.
					if nextNodeIsAnchor {
						// The next worker is in anchor mode, they have the data.
						closestIndex := 0
						closestDistance := math.MaxInt

						for pairIndex, pair := range ci.workers[i+1].anchorMessageBuffer {
							dist := (targetAnchorTime.Unix() - pair.messageTime.Unix())

							if dist < int64(closestDistance) && dist > 0 {
								closestDistance = int(dist)
								closestIndex = pairIndex
							}
						}

						// Append closest to the buffer for the next chunk.
						for _, m := range ci.workers[i+1].anchorMessageBuffer[closestIndex:] {
							ci.workers[i].rpStream.Append(m.messageData)
						}

						// XXX: Might have to flush here to make sure no copies of the data persist.
						// Not sure though.
						// ci.workers[i].rpStream.Flush()
					}
				}
			}
		}

		// subscribeWaitGroup.Go(subscribeFunc)
		subscribeFunc()
	}

	subscribeWaitGroup.Wait()

	for i := range ci.workers {
		log.Info("Resuming ", i)
		ci.workers[i].resume <- true
	}
	for i := range ci.workers {
		<-ci.workers[i].resume
	}

	maxSummaries := min(len(ci.summariesOld), len(requests))
	return ci.summariesOld[:maxSummaries]
}

func (cw *CollectorWorker) run(ctx context.Context) {
	log.Info("Started worker.")

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
					paused = false

					// Tell collector we're done resuming.
					cw.resume <- true

					if cw.anchorMode {
						// Reset the buffers.
						cw.anchorMessageBuffer = cw.anchorMessageBuffer[:0]
						cw.anchorDataBuffer = cw.anchorDataBuffer[:0]
					}
				}
			} else {
				select {
				case <-cw.pause:
					log.Info("Channel stopped.")

					if cw.anchorMode {
						// Nothing extra to do, just wait...
					} else {
						cw.rpStream.Flush()
						cw.summary.DataHashes = make([]cid.Cid, len(cw.rpStream.GetCids()))

						copy(cw.summary.DataHashes[:], cw.rpStream.GetCids())
					}

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

					if cw.anchorMode {
						start := len(cw.anchorDataBuffer)
						cw.anchorDataBuffer = append(cw.anchorDataBuffer, message...)
						cw.anchorMessageBuffer = append(cw.anchorMessageBuffer, MessageTimeTuple{
							// XXX: Message data is a slice into the buffer.
							// Note this means the buffer should be copied later on.
							messageData: cw.anchorDataBuffer[start : start+len(message)],
							messageTime: time.Now(),
						})
					} else {
						cw.rpStream.Append(message)
					}
				}
			}
		}
	}
}

func (ci *CollectorInstance) Start(ctx context.Context) {
	log.Infof("Started collector instance.")
	ci.ctx = ctx

	for i := range ci.workers {
		ci.workers[i].pause = make(chan bool)
		ci.workers[i].resume = make(chan bool)
		ci.workers[i].message = make(chan []byte)

		if i%2 == 0 {
			ci.workers[i].anchorMode = false
			ci.workers[i].summary = &ci.summariesNew[i]
			ci.workers[i].rpStream = ci.rpInstance.NewStream()
		} else {
			ci.workers[i].anchorMode = true
			ci.workers[i].anchorDataBuffer = make([]byte, 0, resourcepool.DEFAULT_CHUNK_SIZE)
			ci.workers[i].anchorMessageBuffer = make([]MessageTimeTuple, 0, resourcepool.DEFAULT_CHUNK_SIZE)
		}

		index := i
		runFunc := func() { ci.workers[index].run(ci.ctx) }

		log.Infof("Deploying worker for collector.")
		ci.workerWaitGroup.Go(runFunc)
	}
}

func (ci *CollectorInstance) Stop() {
	// This only works if the context was cancelled, otherwise the worker goroutines will block this.
	ci.workerWaitGroup.Wait()
}
