package validator

import (
	"context"
	"testing"
)

func TestBinanceJoin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Log("Got here no issue")
	c, err := wsCEXJoin(ctx, Sources[1], "usdc.usdt")

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

func TestBinanceFull(t *testing.T) {
	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	// buffer, err := Subscribe(ctx, Sources[0], "usdc.usdt", 1024)
	// if err != nil {
	// 	panic(err)
	// } else {
	// 	buffer.Read()
	// }
}
