package collector

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/openmesh-network/core/internal/config"
	log "github.com/openmesh-network/core/internal/logger"
)

type DataWindow struct {
	SourceName   string
	Symbol       string
	StartTime    string
	DataSize     int64
	MessageCount int
	Throughput   float64
}

type DataCollector struct {
	Source        Source
	TimeWindow    time.Duration
	DataWindows   map[string]*DataWindow
	BusiestWindow *DataWindow
}

type SourceMetric struct {
	sourceName                     string
	sourceTotalThroughputPerWindow map[string]float64
	busiestTimeWindowStartTime     string
	busiestThroughput              float64
	mu                             sync.Mutex
}

func NewSourceMetric(sourceName string) *SourceMetric {
	return &SourceMetric{
		sourceName:                     sourceName,
		sourceTotalThroughputPerWindow: make(map[string]float64),
		busiestTimeWindowStartTime:     "",
		busiestThroughput:              -1,
	}
}

func NewDataCollector(source Source, windowTimeSize time.Duration) *DataCollector {
	return &DataCollector{
		Source:        source,
		TimeWindow:    windowTimeSize,
		DataWindows:   make(map[string]*DataWindow),
		BusiestWindow: &DataWindow{Throughput: -1},
	}
}

type DataWriter struct {
	file   *os.File
	writer *csv.Writer
	mu     sync.Mutex
}

func NewDataWriter(filename string) (*DataWriter, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	writer := csv.NewWriter(file)
	return &DataWriter{
		file:   file,
		writer: writer,
	}, nil
}

func (dw *DataWriter) Write(data []string) error {
	dw.mu.Lock()
	defer dw.mu.Unlock()
	if err := dw.writer.Write(data); err != nil {
		return err
	}
	return nil
}

func (dw *DataWriter) Close() error {
	defer dw.file.Close()
	if err := dw.writer.Error(); err != nil {
		return err
	}
	dw.writer.Flush()
	return nil
}

// Handler to subscribe to each source and symbol & measure data size over a set period from current time.
func CalculateDataSize(t *testing.T, ctx context.Context, dataWriter *DataWriter, timeToCollect int, timeFrameWindowSize int) {

	config.Path = "../../core"
	config.Name = "config"
	config.ParseConfig(config.Path, true)
	log.InitLogger()

	// Busiest times of a given src for a given symbol --> To detect high trading volume for a given symbol.
	// Fetch data in parts of a whole. 10 mins --> 1 min windows which contain size of
	// msgs received during that period and also number of msgs. Can calculate the load w this. --> Throughput = size / window time frame.
	// for that time frame.
	startTime := time.Now()
	endTime := startTime.Add(time.Duration(timeToCollect) * time.Second)
	timeFrame := time.Duration(timeFrameWindowSize) * time.Second
	timeFormat := "15:04:05, 02 Jan 2006"
	busyHeader := fmt.Sprintf("Busiest Time (Data collected for %d seconds Btw %s till %s)", timeToCollect, startTime.Format(timeFormat), endTime.Format(timeFormat))

	//headerCSv
	if err := dataWriter.writer.Write([]string{"Source", "Symbol", "Data Size (bytes)", "Throughput", busyHeader, "Throughput in the interval", "messages count received during interval"}); err != nil {
		t.Fatalf("Failed to write header to CSV: %s", err)
	}

	var wg sync.WaitGroup
	sourceMetricsList := make([]*SourceMetric, 0)

	for _, source := range Sources {
		sourceMetric := NewSourceMetric(source.Name)
		sourceMetricsList = append(sourceMetricsList, sourceMetric)
		for _, topic := range source.Topics {
			dc := NewDataCollector(source, timeFrame)
			wg.Add(1)
			go func(src Source, tpc string, srcMetric *SourceMetric) {
				defer wg.Done()
				// subscribe and wait till the time period for each src/symbol.
				size, busiestWindow := subscribeAndMeasure(ctx, src, tpc, time.NewTimer(time.Duration(timeToCollect)*time.Second), dc, timeFrameWindowSize, srcMetric)
				throughput := size / int64(timeToCollect)
				log.Infof("Data size  for %s - %s: %d bytes, with TP : %d \n", src.Name, tpc, size, throughput)
				throughputStr := fmt.Sprintf("%.2f", busiestWindow.Throughput)

				record := []string{src.Name, tpc, fmt.Sprintf("%d", size), fmt.Sprintf("%d", throughput), busiestWindow.StartTime, throughputStr, fmt.Sprintf("%d", busiestWindow.MessageCount)}

				dataWriter.mu.Lock()
				defer dataWriter.mu.Unlock()
				if err := dataWriter.writer.Write(record); err != nil {
					t.Errorf("Failed to write data to CSV for %s - %s: %s", src.Name, tpc, err)
				}
			}(source, topic, sourceMetric)
		}
	}
	wg.Wait()

	// Get the busiest source at a given time period
	for _, sourceMetric := range sourceMetricsList {
		log.Infof("Source: %s, Max Throughput: %.2f during %s \n",
			sourceMetric.sourceName,
			sourceMetric.busiestThroughput,
			sourceMetric.busiestTimeWindowStartTime)
	}

	// Flushing and close writer "after all go routines are done " (imp)
	if err := dataWriter.Close(); err != nil {
		t.Fatal("Failed to flush and close CSV writer:", err)
	}
}

// handles the subscription and updates size the data as its received.
func subscribeAndMeasure(ctx context.Context, source Source, symbol string, timer *time.Timer, dc *DataCollector, timeFrameWindowSize int, sourceMetrics *SourceMetric) (int64, *DataWindow) {
	msgChan, err := Subscribe(ctx, source, symbol)
	if err != nil {
		log.Infof("Error subscribing to source %s with symbol %s: %v", source.Name, symbol, err)
		return 0, &DataWindow{}
	}

	var dataSize int64
	var messageCount int64
	timerLocal := timer

	// InitialThroughput := float64(len(msgChan)) / float64(1)
	globalWindow := &DataWindow{DataSize: int64(len(msgChan)), MessageCount: 1, Throughput: -1}
	windowChange := make(chan string, 1)
	oldWindowKey := ""

	// Manage window changes
	go func() {
		for {
			select {
			case windowKey := <-windowChange:
				window := dc.DataWindows[windowKey]
				if window != nil {
					throughput := float64(window.DataSize) / float64(timeFrameWindowSize)
					window.Throughput = throughput

					if throughput > globalWindow.Throughput {
						globalWindow = window
					}
					log.Infoln("Window changed for ", window.SourceName, window.Symbol, " . Startime : ", window.StartTime, " Window Throughput :", throughput, " Current max throughput : ", globalWindow.Throughput, " Message count in window : ", window.MessageCount)
				}
			case <-ctx.Done():
				close(windowChange)
				return
			}
		}
	}()

	for {
		select {
		case msg := <-msgChan:

			if int64(len(msg)) == 0 || msg == nil {
				continue
			}

			currentTime := time.Now()

			// this rounds down the current time to the timeWindow
			// if say window is 10 min, 21:16:57 is rounded to 21:10:00
			// such that we can have a map of all elements between 21:10:00 till 21:20:00
			// Format preserves quality by conv to string.
			windowKey := currentTime.Truncate(dc.TimeWindow).Format(time.RFC3339)

			window, exists := dc.DataWindows[windowKey]
			if !exists {
				if oldWindowKey == "" {
					windowChange <- windowKey
				} else {
					windowChange <- oldWindowKey
				}
				window = &DataWindow{
					SourceName:   source.Name,
					Symbol:       symbol,
					StartTime:    windowKey,
					DataSize:     0,
					MessageCount: 0,
				}
				dc.DataWindows[windowKey] = window
			}
			oldWindowKey = windowKey
			dc.DataWindows[windowKey].DataSize += int64(len(msg))
			dc.DataWindows[windowKey].MessageCount += 1

			dataSize += int64(len(msg))
			messageCount += 1

			// Aggregate source throughput per window
			// --> Every roiutine (source + symbol) has global structure sourceMetrics
			// they lock and change the throughputs as and when windows change
			// --> This was, cross routine access is localized to that source's topics
			// --> Easier to extract data as well later on.
			sourceMetrics.mu.Lock()
			currentThroughput := float64(window.DataSize) / float64(timeFrameWindowSize)
			totalThroughput := sourceMetrics.sourceTotalThroughputPerWindow[windowKey]
			sourceMetrics.sourceTotalThroughputPerWindow[windowKey] = totalThroughput + currentThroughput
			if totalThroughput+currentThroughput > sourceMetrics.busiestThroughput {
				sourceMetrics.busiestThroughput = totalThroughput + currentThroughput
				sourceMetrics.busiestTimeWindowStartTime = windowKey
			}
			sourceMetrics.mu.Unlock()

			// Debug
			// if strings.Contains(source.Name, "coinbase") {
			// 	fmt.Println("CB public ethusd : ", string(msg), " Len : ", int64(len(msg)), messageCount)
			// }

		case <-timerLocal.C:
			log.Infoln("Received entire data for  : ", source.Name, symbol, ".  Now stopping metric collection, message ct : ", messageCount)

			return dataSize, globalWindow
		case <-ctx.Done():
			return dataSize, globalWindow
		}
	}
}
