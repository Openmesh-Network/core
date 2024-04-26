package collector

import (
	"context"
	"encoding/binary"
	"encoding/csv"

	jsoniter "github.com/json-iterator/go"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4"

	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openmesh-network/core/internal/config"
	log "github.com/openmesh-network/core/internal/logger"
	"go.mongodb.org/mongo-driver/bson"
)

// using this for now since we are unmarshaling every value
var jsoniterator = jsoniter.ConfigCompatibleWithStandardLibrary

// All sources compress test
var filePathAll string = "compressed_files/test_bson_AllSources.bson"
var fileAll, _ = os.OpenFile(filePathAll, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)

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
				size, busiestWindow, _ := subscribeAndMeasure(ctx, src, tpc, time.NewTimer(time.Duration(timeToCollect)*time.Second), dc, timeFrameWindowSize, srcMetric)
				throughput := size / int64(timeToCollect)
				// log.Infof("Data size  for %s - %s: %d bytes, with TP : %d \n", src.Name, tpc, size, throughput)
				throughputStr := fmt.Sprintf("%.2f", busiestWindow.Throughput)

				// fmt.Println("Compressing  source : ", src.Name)

				// CompressBSONFileZst(filePathSource+".bson", filePathSource+".zst", 4094)

				// CompressBSONFileLZ4(filePathSource+".bson", filePathSource+".lz4", 4094)

				// err1 := DecompressAndReadBSONLZ4(filePathSource + ".lz4")
				// fmt.Println("decomp lz4 err ", err1)

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

	fileAll.Close()

	fmt.Println("Compressing All sources: ")
	CompressBSONFileZst("compressed_files/"+"test_bson_AllSources.bson", "compressed_files/"+"test_compressed_bson_AllSources.zst", 4094)

	CompressBSONFileLZ4("compressed_files/"+"test_bson_AllSources.bson", "compressed_files/"+"test_compressed_bson_AllSources.lz4", 4094)

	fmt.Println("Decompressing All sources: ")
	err := DecompressAndReadBSONZst("compressed_files/test_compressed_bson_AllSources.zst")
	fmt.Println("decomp err ", err)

	err1 := DecompressAndReadBSONLZ4("compressed_files/test_compressed_bson_AllSources.lz4")
	fmt.Println("decomp lz4 err ", err1)

	// Flushing and close writer "after all go routines are done " (imp)
	if err := dataWriter.Close(); err != nil {
		t.Fatal("Failed to flush and close CSV writer:", err)
	}
}

// handles the subscription and updates size the data as its received.
func subscribeAndMeasure(ctx context.Context, source Source, symbol string, timer *time.Timer, dc *DataCollector, timeFrameWindowSize int, sourceMetrics *SourceMetric) (int64, *DataWindow, string) {
	msgChan, err := Subscribe(ctx, source, symbol)
	if err != nil {
		log.Infof("Error subscribing to source %s with symbol %s: %v", source.Name, symbol, err)
		return 0, &DataWindow{}, " "
	}

	var dataSize int64
	var messageCount int64
	timerLocal := timer

	// InitialThroughput := float64(len(msgChan)) / float64(1)
	globalWindow := &DataWindow{DataSize: int64(len(msgChan)), MessageCount: 1, Throughput: -1}
	windowChange := make(chan string, 1)
	oldWindowKey := ""

	dirPath := fmt.Sprintf("compressed_files/%s", sourceMetrics.sourceName)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		log.Fatalf("Failed to create directory: %v", err)
		panic(err)
	}

	filePathSource := fmt.Sprintf("%s/test_%s_%s", dirPath, sourceMetrics.sourceName, symbol)

	// Individual source level compression.
	bsonFile, err := os.OpenFile(filePathSource+".bson", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
		panic(err)
	}

	zstFile, zstEncoder, err := SetupZstCompressionFile(filePathSource + ".zst")
	if err != nil {
		fmt.Print("error setup compres")
	}

	lz4File, lz4Encoder, err := SetupLz4CompressionFile(filePathSource + ".lz4")
	if err != nil {
		fmt.Print("error setup compres")
	}

	currentBufferSize := 0

	var bufferSize = 8192

	if source.Name == "dydx" {
		bufferSize = 16384
	}

	buffer := make([]byte, bufferSize)

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
					// log.Infoln("Window changed for ", window.SourceName, window.Symbol, " . Startime : ", window.StartTime, " Window Throughput :", throughput, " Current max throughput : ", globalWindow.Throughput, " Message count in window : ", window.MessageCount)
				}
			case <-ctx.Done():
				close(windowChange)
				return
			}
		}
	}()

	// msgCount := 0

	for {
		select {
		case msg := <-msgChan:

			if int64(len(msg)) == 0 || msg == nil {
				continue
			}

			var bsonData []byte

			// TODO : store Rpc messages
			if !strings.Contains(source.Name, "rpc") {

				var jsonData interface{}

				// fmt.Println("msgcounnt origin , ", msgCount)
				// msgCount++

				// using jsoniter as of now since we are unmarshaling every key value.
				if err := jsoniterator.Unmarshal(msg, &jsonData); err != nil {
					log.Fatalf("Failed to unmarshal JSON: %v", err)
					continue
				}

				// fmt.Println("Msg", string(msg))
				// fmt.Println("Json", jsonData)

				// jsonparser library
				// jsonData, err := JsonUnmarshaler(msg)
				// if err != nil {
				// 	panic(err)
				// }

				bsonData, err = bson.Marshal(jsonData)
				if err != nil {
					log.Fatalf("Failed to marshal to BSON: %v", err)
					continue
				}

				bsonLength := len(bsonData)
				totalLength := 4 + len(bsonData)

				if currentBufferSize+totalLength > bufferSize {

					// Buffer needs to be written and cleared because it won't fit
					if currentBufferSize > 0 {
						// pad, Compress and write the current buffer content to the file
						fmt.Println("buf size old :", len(buffer[:currentBufferSize]))
						paddingLength := bufferSize - currentBufferSize

						if paddingLength > 0 {

							for i := 0; i < paddingLength; i++ {
								buffer[currentBufferSize+i] = 0
							}
						}

						fmt.Println("total size exceeded ],", currentBufferSize, currentBufferSize+totalLength, bufferSize, paddingLength)

						_, err = bsonFile.Write(buffer[:currentBufferSize])
						if err != nil {
							log.Fatalf(source.Name, " Failed to write to bson file: %v", err)
						}

						_, err = fileAll.Write(buffer[:currentBufferSize])
						if err != nil {
							log.Fatalf(source.Name, " Failed to write to All src file: %v", err)
						}

						_, err := zstEncoder.Write(buffer[:currentBufferSize])
						if err != nil {
							fmt.Println("Error writing")
						}
						fmt.Println("buf size :", len(buffer))

						_, err1 := lz4Encoder.Write(buffer[:currentBufferSize])
						if err1 != nil {
							fmt.Println("Error writing")
						}

						if err := zstEncoder.Flush(); err != nil {
							fmt.Println("Error flushing")
						}

						if err := lz4Encoder.Flush(); err != nil {
							fmt.Println("Error flushing")
						}
						fmt.Println("buf size new :", len(buffer))
						currentBufferSize = 0
					}

				}

				// fmt.Println("BSON data : ", source.Name, bsonData, " length : ", bsonLength)

				// Write length of the BSON data to buffer (big-endian format)
				// for larger data --> use :
				// binary.BigEndian.PutUint64(buffer[0:4], uint64(bsonLength))
				binary.BigEndian.PutUint32(buffer[currentBufferSize:], uint32(bsonLength))
				currentBufferSize += 4

				// Copies BSON data to the buffer right after the length
				copy(buffer[currentBufferSize:], bsonData)
				currentBufferSize += bsonLength

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
			// fmt.Println("Message received from ", source, symbol, " : ", string(msg))

		case <-timerLocal.C:
			// log.Infoln("Received entire data for  : ", source.Name, symbol, ".  Now stopping metric collection, message ct : ", messageCount)

			closeCompression(zstEncoder, zstFile)
			closeCompressionLz4(lz4Encoder, lz4File)
			fmt.Println("Closed file & encoder ", filePathSource)
			time.Sleep(3 * time.Second)

			// fmt.Println("Zst Decompressing source: ", sourceMetrics.sourceName)
			err := DecompressAndReadBSONZst(filePathSource + ".zst")
			fmt.Println("Zst decomp err ", sourceMetrics.sourceName, err)

			fmt.Println("Lz4 Decompressing source: ", sourceMetrics.sourceName)
			err1 := DecompressAndReadBSONLZ4(filePathSource + ".lz4")
			fmt.Println("lz4 decomp err ", sourceMetrics.sourceName, err1)

			return dataSize, globalWindow, filePathSource
		case <-ctx.Done():
			return dataSize, globalWindow, filePathSource
		}
	}
}

// Function to cleanly close the compression and file
func closeCompression(encoder *zstd.Encoder, file *os.File) error {
	if encoder != nil {
		if err := encoder.Close(); err != nil {
			file.Close()
			return err
		}
	}

	if file != nil {
		return file.Close()
	}

	return nil
}

func closeCompressionLz4(encoder *lz4.Writer, file *os.File) error {
	if encoder != nil {
		if err := encoder.Close(); err != nil {
			file.Close()
			return err
		}
	}

	if file != nil {
		return file.Close()
	}

	return nil
}
