package validator

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/websocket"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// TODO: Check which exchanges are wanted / desireable. Also, find which symbols we should care about.

// Defines a "source" of data, all supported sources are laid out in the Sources array.
// We opt for this approach over oop for clarity and extensibility.
type Source struct {
	Name     string
	JoinFunc func(ctx context.Context, source Source, symbol string) (chan []byte, error)
	ApiURL   string
	Symbols  []string
	// Request field will have {{symbol}} replaced with the actual symbol in the call to wsCEXJoin.
	Request string
}

// The master table with all our sources.
var Sources = [...]Source{
	// Exchanges:
	// Note that the symbols are incomplete as they are undecided.
	{"binance", defaultJoinCEX, "wss://stream.binance.com:9443/ws", []string{"usdt.usdc", "btc.eth", "eth.usdt"}, "{\"method\": \"SUBSCRIBE\", \"params\": [ \"{{symbol}}@aggTrade\" ], \"id\": 1}"},
	{"coinbase", defaultJoinCEX, "wss://ws-feed.pro.coinbase.com", []string{"BTC-USD", "ETH-USD", "BTC-ETH"}, "{\"type\": \"subscribe\", \"product_ids\": [ \"{{symbol}}\" ], \"channels\": [ \"ticker\" ]}"},
	{"dydx", defaultJoinCEX, "wss://api.dydx.exchange/v3/ws", []string{"MATIC-USD", "LINK-USD", "SOL-USD", "ETH-USD", "BTC-USD"}, "{\"type\": \"subscribe\", \"id\": \"{{symbol}}\", \"channel\": \"v3_trades\"}"},

	// Blockchain RPCs:
	{"ethereum-ankr-rpc", ankrJoinRPC, "https://rpc.ankr.com/eth", []string{""}, ""},
}

// Subscribe will connect to the chosen source and create a channel to get data out of it.
func Subscribe(ctx context.Context, source Source, symbol string, bufferSizeMax int) (chan []byte, error) {
	// TODO: Not sure if it's better to use a shared buffer here instead of a channel.
	// That would let us do custom compression behaviour at the exchange level.
	// If we move to a buffer, using a ring/circular buffer sounds like a good idea.

	msgChannel, err := source.JoinFunc(ctx, source, symbol)
	if err != nil {
		return nil, err
	}

	// We do this instead of just returning the msgChannel because we want control over the compresion in the future.
	outChannel := make(chan []byte)
	go func() {
		for {
			select {
			case msg := <-msgChannel:
				// TODO: Add optional callback to re-encode the data for better size efficiency here.

				outChannel <- msg
				break
			case <-ctx.Done():
				break
			}
		}
	}()

	return outChannel, nil
}

// Default function for CEXs since the majority of them use this functionality.
// Note that we assume that the symbol is in the source's format.
func defaultJoinCEX(ctx context.Context, source Source, symbol string) (chan []byte, error) {
	ws, err := websocket.Dial(source.ApiURL, "", source.ApiURL)
	if err != nil {
		panic(err)
	}

	// HACK: This is the simplest tool for the job right now. Importing a whole templating library is 100% overkill.
	request := strings.Replace(source.Request, "{{symbol}}", symbol, 1)

	ws.Write([]byte(request))

	msgChannel := make(chan []byte)
	go func() {
		// XXX: Move this goshforsaken allocation at some point (Maybe move to global collector struct?).
		// Also note that this means messages higher than 2048 bytes in length will be sent in 2 chunks over the channel.
		buf := make([]byte, 2048)
		for {
			n, err := ws.Read(buf)
			if err != nil {
				// Connection was severed, so quit.
				return
			} else {
				// Append message to message channel.
				msgChannel <- buf[:n]
			}
		}
	}()

	go func() {
		<-ctx.Done()
		// Closing the websocket here should end the other goroutine.
		ws.Close()
	}()

	return msgChannel, nil
}

func ankrJoinRPC(ctx context.Context, source Source, symbol string) (chan []byte, error) {
	// Note that ankr has a 30requests / second guarantee. We can't spam their endpoint more than that.
	// Plus they have a hard limit on the request body size.

	fmt.Println("Dialing...")
	ethereum_client, err := ethclient.Dial(source.ApiURL)
	if err != nil {
		return nil, err
	}
	fmt.Println("Dialed!")

	msgChannel := make(chan []byte)
	go func() {
		buffer := bytes.NewBuffer(make([]byte, 1024000))
		headerPrevious := common.Hash{}

		for {
			fmt.Println("Waiting for block...")
			block, err := ethereum_client.BlockByNumber(ctx, nil)
			fmt.Println("Got block!")

			if err != nil {
				// HACK: Lazy error handling, actually fix this.
				panic(err)
			} else {
				headerProspective := block.Header().Hash()
				if headerPrevious == headerProspective {
					// Same block as last time we checked, ignore.
				} else {
					// Serialize the block in RLP format.
					fmt.Println("Serializing block...")
					buffer.Reset()
					headerPrevious = block.Header().Hash()
					err := block.EncodeRLP(buffer)
					fmt.Println("Block serialized!")

					if err != nil {
						// HACK: Lazy error handling, actually fix this.
						panic(err)
					} else {
						fmt.Println("Sending over channel,", buffer.Len())

						msgChannel <- buffer.Bytes()

						fmt.Println("Sent.")
					}
				}
				time.Sleep(1 * time.Second)
			}
		}
	}()

	return msgChannel, nil
}
