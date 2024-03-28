package collector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	openseaSdk "github.com/721tools/stream-api-go/sdk"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	"golang.org/x/net/context"
	"nhooyr.io/websocket" // Docs are hard to find: https://pkg.go.dev/nhooyr.io/websocket; Or, use gorilla websockets?
	// Rate limited, but events don't count after you're subscribed.
)

// TODO: Check which exchanges are wanted / desireable. Also, find which topics we should care about.

// Defines a "source" of data, all supported sources are laid out in the Sources array.
// We opt for this approach over oop for clarity and extensibility.
type Source struct {
	Name     string
	JoinFunc func(ctx context.Context, source Source, topic string) (chan []byte, <-chan error, error)
	ApiURL   string // To-do: Add support for multiple endpoints.
	Topics   []string
	Request  string
}

// The master table with all our sources.
var Sources = [...]Source{
	// Centralised Exchanges:
	// Note that the topics are incomplete as they are undecided.
	// {"binance", defaultJoinCEX, "wss://stream.binance.com:9443/ws", []string{"btc.usdt"}, "{\"method\": \"SUBSCRIBE\", \"params\": [ \"{{topic}}@aggTrade\" ], \"id\": 1}"},
	{"coinbase", defaultJoinCEX, "wss://ws-feed.pro.coinbase.com", []string{"BTC-USD", "ETH-USD", "BTC-ETH"}, "{\"type\": \"subscribe\", \"product_ids\": [ \"{{topic}}\" ], \"channels\": [ \"ticker\" ]}"},
	{"dydx", defaultJoinCEX, "wss://api.dydx.exchange/v3/ws", []string{"MATIC-USD", "LINK-USD", "SOL-USD", "ETH-USD", "BTC-USD"}, "{\"type\": \"subscribe\", \"id\": \"{{topic}}\", \"channel\": \"v3_trades\"}"},
	// // Centralised NFT Exchange:
	// // Opensea Request structure: {topic: \ event: \ payload:{} \ ref: }
	// {"opensea", defaultJoinNFTCEX, "wss://stream.openseabeta.com/socket", []string{"item_listed", "item_cancelled", "item_sold", "item_transferred", "item_received_offer", "item_received_bid"}, "collections:*"},

	// // Decentralised Exchanges
	// // Add Uniswap

	// // Blockchain RPCs:
	{"ethereum-ankr-rpc", ankrJoinRPC, "https://rpc.ankr.com/eth", []string{""}, ""},
	// {"polygon-ankr-rpc", ankrJoinRPC, "https://rpc.ankr.com/polygon", []string{""}, ""},
}

// Subscribe will connect to the chosen source and create a channel which will return every message from it.
func Subscribe(ctx context.Context, source Source, topic string) (chan []byte, error) {
	// TODO: Not sure if it's better to use a shared buffer here instead of a channel.
	// That would let us do custom compression behaviour at the exchange level.
	// If we move to a buffer, using a ring/circular buffer sounds like a good idea.

	msgChannel, errChannel, err := source.JoinFunc(ctx, source, topic)
	if err != nil {
		return nil, err
	}

	outChannel := make(chan []byte)
	outErrChannel := make(chan error, 1)

	go func() {
		defer close(outChannel)
		defer close(outErrChannel)
		for {
			select {
			case msg := <-msgChannel:
				select {
				case outChannel <- msg:
				case <-ctx.Done():
					return
				}
			case err := <-errChannel:
				select {
				case outErrChannel <- err:
					return
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return outChannel, nil
}

func defaultJoinCEX(ctx context.Context, source Source, topic string) (chan []byte, <-chan error, error) {
	ws, _, err := websocket.Dial(ctx, source.ApiURL, &websocket.DialOptions{
		Subprotocols: []string{"phoenix"},
	})
	if err != nil {
		// fmt.Println(resp)
		return nil, nil, err
	}

	request := strings.Replace(source.Request, "{{topic}}", topic, 1)
	ws.Write(ctx, websocket.MessageBinary, []byte(request))

	msgChannel := make(chan []byte)
	errChannel := make(chan error, 1)

	go func() {
		defer close(msgChannel)
		defer close(errChannel)
		for {
			_, n, err := ws.Read(ctx)
			// fmt.Println("Received message of type: %s", ntype)
			if err != nil {
				// fmt.Println("Twas an error message :(")
				errChannel <- err
				return
			} else {
				// fmt.Println("Writting message...")
				msgChannel <- n
				// fmt.Println("Wrote message!")
			}
		}
	}()

	go func() {
		<-ctx.Done()
		ws.CloseNow()
	}()
	return msgChannel, errChannel, nil
}

func ankrJoinRPC(ctx context.Context, source Source, topic string) (chan []byte, <-chan error, error) {
	ethereum_client, err := ethclient.Dial(source.ApiURL)
	if err != nil {
		return nil, nil, err
	}

	msgChannel := make(chan []byte)
	errChannel := make(chan error, 1)

	go func() {
		buffer := bytes.NewBuffer(make([]byte, 1024000))
		headerPrevious := common.Hash{}
		defer ethereum_client.Close()

		// 1 block per second + request delay is roughly alright since new blocks take ~11 seconds.
		// Ankr gives us 20 requests per second with their RPC, so we're also not exhausting that.
		timeTicker := time.Tick(time.Second)

		for {
			select {
			case <-ctx.Done():
				// Quit gracefully, out context was handled above.
			case <-timeTicker:
				// XXX: This might add 2 seconds to shutdown. It's unfortunate, but it guarantees error checks below
				// actually error on the state of the request, not the parent's context.
				ctxToPreventHanging, cancel := context.WithTimeout(context.Background(), time.Second*2)
				defer cancel()
				// fmt.Println("Waiting for block...")
				block, err := ethereum_client.BlockByNumber(ctxToPreventHanging, nil)
				// bnumber := block.Number()
				// fmt.Println(fmt.Sprintf("Got block %s!", bnumber))

				if err != nil {
					errChannel <- err
					return
				}

				headerProspective := block.Header().Hash()
				if headerPrevious != headerProspective {
					buffer.Reset()
					headerPrevious = block.Header().Hash()
					err := block.EncodeRLP(buffer)
					if err != nil {
						errChannel <- err
						return
					}
					msgChannel <- buffer.Bytes()
				}
			}
		}
	}()

	return msgChannel, errChannel, nil
}

func defaultJoinNFTCEX(ctx context.Context, source Source, topic string) (chan []byte, <-chan error, error) {
	// Get users api key.
	apiKey := getVarFromEnv("OPENSEA_API_KEY") // Refactor for any NFT CEX later.

	fmt.Println("Found OpenSea API Key in environment")

	ns := openseaSdk.NewNotifyService(openseaSdk.MAIN_NET, apiKey)
	msgChannel := make(chan []byte, 1000)
	errChannel := make(chan error, 1)

	var subscribeErr error

	unsubscribe, subscribeErr := ns.Subscribe("*", topic, func(msg *openseaSdk.Message) error {
		bmsg, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		select {
		case msgChannel <- bmsg:
		case <-ctx.Done():
			fmt.Println("Context is done, exiting goroutine")
			return ctx.Err()
		}
		return nil
	})

	if subscribeErr != nil {
		fmt.Println("Error subscribing:", subscribeErr)
		return nil, nil, subscribeErr
	}

	go func() {
		defer close(msgChannel)
		defer close(errChannel)
		defer unsubscribe() // Unsubscribe when the goroutine exits

		ns.Start()

		<-ctx.Done()
		fmt.Println("Context is done, exiting Go routine")
	}()

	return msgChannel, errChannel, nil
}

func getVarFromEnv(envKey string) string {
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found")
	}

	envVar := os.Getenv(envKey)
	if envVar == "" {
		err := "Unable to find " + envKey
		fmt.Println("ERROR:", err)
		panic(err)
	}
	fmt.Println("Found OpenSea API Key in environment")
	return envVar
}
