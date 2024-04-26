package collector

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/buger/jsonparser"
	"github.com/klauspost/compress/zstd"
	log "github.com/openmesh-network/core/internal/logger"
	"github.com/pierrec/lz4"
	"go.mongodb.org/mongo-driver/bson"
)

// If using json parser, use this func to dettermine datatypes etc.
func HandleKeyValueType(key, value []byte, dataType jsonparser.ValueType, offset int) error {
	fmt.Printf("Key: %s, Value: %s, Type: %v\n", string(key), string(value), dataType)
	return nil
}

func SetupZstCompressionFile(destFilePath string) (*os.File, *zstd.Encoder, error) {
	var err error

	var encoder *zstd.Encoder
	var file *os.File

	file, err = os.OpenFile(destFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil, nil, err
	}

	// Create a new encoder writing to file, with cleanup to close the file if error.
	encoder, err = zstd.NewWriter(file)
	if err != nil {
		file.Close()
		return nil, nil, err
	}

	return file, encoder, nil
}

func SetupLz4CompressionFile(destFilePath string) (*os.File, *lz4.Writer, error) {
	file, err := os.OpenFile(destFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil, nil, err
	}

	encoder := lz4.NewWriter(file)

	return file, encoder, nil
}

func CompressBSONFileLZ4(sourceFilePath, destFilePath string, chunkSize int) error {
	sourceFile, err := os.Open(sourceFilePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(destFilePath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	encoder := lz4.NewWriter(destFile)
	defer encoder.Close()

	buf := make([]byte, chunkSize)
	for {
		n, err := sourceFile.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if n > 0 {
			if _, err := encoder.Write(buf[:n]); err != nil {
				return err
			}
		}
	}
	return nil
}

func CompressBSONFileZst(sourceFilePath, destFilePath string, chunkSize int) error {
	sourceFile, err := os.Open(sourceFilePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(destFilePath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	encoder, err := zstd.NewWriter(destFile)
	if err != nil {
		return err
	}
	defer encoder.Close()

	buf := make([]byte, chunkSize)
	for {
		n, err := sourceFile.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if n > 0 {
			if _, err := encoder.Write(buf[:n]); err != nil {
				return err
			}
		}
	}
	return nil
}

func DecompressAndReadBSONLZ4(sourceFilePath string) error {

	compressedFile, err := os.Open(sourceFilePath)
	if err != nil {
		return fmt.Errorf("failed to open compressed file: %v", err)
	}
	defer compressedFile.Close()

	decoder := lz4.NewReader(compressedFile)

	bsonBuffer := make([]byte, 4)
	// msgCount := 0

	for {
		if _, err := io.ReadFull(decoder, bsonBuffer); err != nil {
			if err == io.EOF {
				fmt.Println("Reached end of file")
				break
			}
			return fmt.Errorf("failed to read BSON length: %v", err)
		}

		length := int(binary.BigEndian.Uint32(bsonBuffer))
		if length <= 0 {
			return fmt.Errorf("invalid BSON document length: %d", length)
		}

		bsonData := make([]byte, length)
		if _, err := io.ReadFull(decoder, bsonData); err != nil {
			return fmt.Errorf("failed to read BSON data: %v", err)
		}

		var msg map[string]interface{}
		if err := bson.Unmarshal(bsonData, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal BSON: %v", err)
		}

		// msgCount++
		// fmt.Println("Lz4 Decompressed BSON document count:", msg, msgCount)

	}

	return nil
}

func DecompressAndReadBSONZst(destFilePath string) error {

	compressedFile, err := os.Open(destFilePath)
	if err != nil {
		return fmt.Errorf("failed to open compressed file: %v", err)
	}
	defer compressedFile.Close()

	decoder, err := zstd.NewReader(compressedFile)
	if err != nil {
		return fmt.Errorf("failed to create zstd reader: %v", err)
	}
	defer decoder.Close()

	// length prefix buffer, to read only length and skip to the next doc
	bsonBuffer := make([]byte, 4)
	// msgCount := 0

	for {
		// Reads length of the next BSON document
		if _, err := io.ReadFull(decoder, bsonBuffer); err != nil {
			if err == io.EOF {
				fmt.Println("Reached end of file")
				break
			}
			return fmt.Errorf("failed to read BSON length: %v", err)
		}

		length := int(binary.BigEndian.Uint32(bsonBuffer))
		if length <= 0 {
			return fmt.Errorf("invalid BSON document length: %d", length)
		}

		bsonData := make([]byte, length)
		// Read actual BSON data
		if _, err := io.ReadFull(decoder, bsonData); err != nil {
			return fmt.Errorf("failed to read BSON data: %v", err)
		}

		var msg map[string]interface{}

		// Unmarshal BSON to generic interface
		if err := bson.Unmarshal(bsonData, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal BSON: %v", err)
		}

		_, err := jsoniterator.MarshalIndent(msg, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %v", err)
		}

		// msgCount++
		// fmt.Println("Decompressed JSON count :", msg, msgCount)
	}

	return nil
}

func JsonUnmarshaler(data []byte) (map[string]interface{}, error) {
	obj := make(map[string]interface{})
	err := jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		var val interface{}
		var parseErr error
		switch dataType {
		case jsonparser.String:
			val, parseErr = jsonparser.ParseString(value)
		case jsonparser.Number:
			// [IMP !! ] May need to handle float values
			val, parseErr = jsonparser.ParseInt(value)
		case jsonparser.Boolean:
			val, parseErr = jsonparser.ParseBoolean(value)
		case jsonparser.Null:
			val = nil
		default:
			val = value // For objects or arrays, this will just store the raw JSON.
		}
		if parseErr != nil {
			return parseErr
		}
		obj[string(key)] = val
		return nil
	})

	if err != nil {
		return nil, err
	}
	return obj, nil
}

func compressBSONFileNoChunk(sourceFilePath, destFilePath string) error {

	sourceFile, err := os.Open(sourceFilePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.OpenFile(destFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
		panic(err)
	}
	defer destFile.Close()

	// default compression level
	encoder, err := zstd.NewWriter(destFile)
	if err != nil {
		return err
	}
	defer encoder.Close()

	_, err = io.Copy(encoder, sourceFile)
	return err
}

func compressBSONFileLZ4NoChunk(sourceFilePath, destFilePath string) error {

	sourceFile, err := os.Open(sourceFilePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.OpenFile(destFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
		panic(err)
	}
	defer destFile.Close()

	encoder := lz4.NewWriter(destFile)
	defer encoder.Close()

	_, err = io.Copy(encoder, sourceFile)
	return err
}
