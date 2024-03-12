package validator

import (
	"context"
	"fmt"
	"testing"
)

// TODO: Want acceptance tests on the Sources table. ie go through the whole table, verify against rules, and test the source's symbols are up.

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
			fmt.Println(string(<-c))
		}
		cancel()
		t.Log("Stopping...")
		t.Log("This ran")
	}
}

func TestBinanceFull(t *testing.T) {
}
