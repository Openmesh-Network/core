package validator

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSourcesTableSanity(t *testing.T) {
	// TODO: Want acceptance tests on the Sources table. ie go through the whole table, verify against rules, and test the source's symbols are up.
	// This doesn't work yet since we're not checking error returns from sources completely.
	// This implements the minimum check to make sure we our API calls are getting responses basically.
	// A better way to implement this would be to make sure we receive a few messages or get some minimum amount of bytes transfered.
	var wg sync.WaitGroup
	checkSymbols := func(source Source) {
		for i := range source.Symbols {
			t.Log("Checking:", source.Name, source.Symbols[i])
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			_, err := source.JoinFunc(ctx, source, source.Symbols[i])

			if err != nil {
				t.Error(err)
			}
		}
		wg.Done()
	}

	for _, source := range Sources {
		// Made this parallel at the source level, since we don't want to risk getting rate limited.
		// If this is too slow, we'll have to do a different approach.
		wg.Add(1)
		t.Log("Running some code")
		go checkSymbols(source)
	}

	wg.Wait()
}

func TestBinanceJoin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Log("Got here no issue")
	t.Log(Sources[2].Symbols[0])
	c, err := defaultJoinCEX(ctx, Sources[2], Sources[2].Symbols[0])

	if err != nil {
		t.Error(err)
	} else {
		for i := 0; i < 10; i++ {
			t.Log(string(<-c))
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
	t.Log(Sources[3].Symbols[0])
	c, err := ankrJoinRPC(ctx, Sources[3], Sources[3].Symbols[0])

	if err != nil {
		t.Error(err)
	} else {
		for i := 0; i < 100; i++ {
			// fmt.Println(string(<-c))
			<-c
		}
		cancel()
		t.Log("Stopping...")
		t.Log("This ran")
	}
}

func TestBinanceFull(t *testing.T) {
}
