package collector

import (
	"context"
	"testing"
	"time"
)

func TestSourcesTableSanity(t *testing.T) {
	// TODO: Want acceptance tests on the Sources table. ie go through the whole table, verify against rules, and test the source's symbols are up.
	// This doesn't work yet since we're not checking error returns from sources completely.
	// This implements the minimum check to make sure we our API calls are getting responses basically.
	// A better way to implement this would be to make sure we receive a few messages or get some minimum amount of bytes transfered.
	checkTopics := func(source Source) {
		for i := range source.Topics {
			t.Log("Checking:", source.Name, source.Topics[i])
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			_, errChan, err := source.JoinFunc(ctx, source, source.Topics[i])

			if err != nil {
				t.Error(err)
			} else {
				select {
				case err := <-errChan:
					t.Error(err)
				case <-ctx.Done():
				}
			}
		}

		t.Log("Success with:", source.Name)
	}

	for _, source := range Sources {
		t.Log("Running some code")
		checkTopics(source)
	}
}

func TestBinanceJoin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Log("Got here no issue")
	t.Log(Sources[2].Topics[0])
	msgChan, errChan, err := defaultJoinCEX(ctx, Sources[2], Sources[2].Topics[0])

	if err != nil {
		t.Error(err)
	} else {
		for i := 0; i < 10; i++ {
			select {
			case msg := <-msgChan:
				t.Log(string(msg))
			case err := <-errChan:
				t.Error(err)
			case <-ctx.Done():
				t.Log("Context canceled")
				return
			}
		}
		cancel()
		t.Log("Stopping...")
		t.Log("This ran")
	}
}

func TestAnkrJoin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Log("Got here no issue")
	t.Log(Sources[4].Topics[0])
	msgChan, errChan, err := ankrJoinRPC(ctx, Sources[4], Sources[4].Topics[0])

	if err != nil {
		t.Error(err)
	} else {
		for i := 0; i < 100; i++ {
			select {
			case <-msgChan:
			case err := <-errChan:
				t.Error(err)
			case <-ctx.Done():
				t.Log("Context canceled")
				return
			}
		}
		cancel()
		t.Log("Stopping...")
		t.Log("This ran")
	}
}

func TestByBit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Log("Running ByBit collector!!!")
	t.Log(Sources[3].Topics[0])
	msgChan, errChan, err := defaultJoinCEX(ctx, Sources[3], Sources[3].Topics[0])

	if err != nil {
		t.Error(err)
	} else {
		for i := 0; i < 100; i++ {
			select {
			case msg := <-msgChan:
				t.Log(string(msg))
			case err := <-errChan:
				t.Error(err)
			case <-ctx.Done():
				t.Log("Context canceled")
				return
			}
		}
		cancel()
	}
}

func TestOKX(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Log("Running OKX collector!!!")
	t.Log(Sources[4].Topics[1])
	msgChan, errChan, err := Sources[4].JoinFunc(ctx, Sources[4], Sources[4].Topics[1])

	if err != nil {
		t.Error(err)
	} else {
		for i := 0; i < 100; i++ {
			select {
			case msg := <-msgChan:
				t.Log(string(msg))
			case err := <-errChan:
				t.Error(err)
			case <-ctx.Done():
				t.Log("Context canceled")
				return
			}
		}
		cancel()
	}
}

func TestOpenSea(t *testing.T) {
	// Note(Tom): Have to disable this test since I don't have Opensea Creds.

	// topic := Sources[3].Topics[0]
	// t.Logf("Using topic: %s", topic)
	// sourceUrl := Sources[3].ApiURL
	// t.Logf("Using source url: %s", sourceUrl)

	// ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	// defer cancel()

	// msgChan, errChan, err := defaultJoinNFTCEX(ctx, Sources[3], topic)
	// if err != nil {
	// 	t.Fatalf("Failed to join NFT CEX: %v", err)
	// }

	// receivedMessages := 0
	// for receivedMessages < 100 {
	// 	select {
	// 	case msg := <-msgChan:
	// 		t.Logf("Received message: %s", string(msg))
	// 		receivedMessages++
	// 	case err := <-errChan:
	// 		t.Fatalf("Error received from defaultJoinNFTCEX: %v", err)
	// 	case <-ctx.Done():
	// 		t.Logf("Context canceled or timed out")
	// 		return
	// 	}
	// }

	// cancel()
	// t.Log("Stopping...")

	// if receivedMessages < 100 {
	// 	t.Errorf("Expected 100 messages, but received %d", receivedMessages)
	// }
}

func TestAnkrPolygonJoin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Log("Got here no issue")
	t.Log(Sources[4].Topics[0])
	msgChan, errChan, err := ankrJoinRPC(ctx, Sources[5], Sources[5].Topics[0])

	if err != nil {
		t.Error(err)
	} else {
		for i := 0; i < 100; i++ {
			select {
			case <-msgChan:
			case err := <-errChan:
				t.Error(err)
			case <-ctx.Done():
				t.Log("Context canceled")
				return
			}
		}
		cancel()
		t.Log("Stopping...")
		t.Log("This ran")
	}
}

// use timeout flag : go test -timeout 600s -run TestSourcesDataSize
func TestSourcesDataSize(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dataWriter, err := NewDataWriter("data_size1.csv")
	if err != nil {
		t.Fatal("Error creating writer.")
	}
	// Time period to collect datd : Seconds (integer values only. !)
	var timeToCollect int = 60
	var windowTimeFrame int = 10
	CalculateDataSize(t, ctx, dataWriter, timeToCollect, windowTimeFrame)
}
