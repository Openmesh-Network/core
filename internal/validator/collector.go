package validator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	//"net/url"
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

// TODO: Check which exchanges are wanted / desireable. Also, find which symbols we should care about.

// Defines a "source" of data, all supported sources are laid out in the Sources array.
// We opt for this approach over oop for clarity and extensibility.
type Source struct {
	Name     string
	JoinFunc func(ctx context.Context, source Source, symbol string) (chan []byte, error)
	ApiURL   string   // To-do: Add support multiple approved endpoints to spread load where possible.
	Symbols  []string // Rename to topics or datapoints.
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
	// CEX NFT marketplace:
	{"opensea", defaultJoinNFTCEX, "wss://stream.openseabeta.com/socket", []string{"item_listed", "item_cancelled", "item_sold", "item_transferred", "item_received_offer", "item_received_bid"}, "collections:*"},

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
	ws, resp, err := websocket.Dial(ctx, source.ApiURL, &websocket.DialOptions{
		Subprotocols: []string{"phoenix"},
	})
	if err != nil {
		fmt.Println(resp)
		panic(err)
	}

	// HACK: This is the simplest tool for the job right now. Importing a whole templating library is 100% overkill.
	request := strings.Replace(source.Request, "{{symbol}}", symbol, 1)

	ws.Write(ctx, websocket.MessageBinary, []byte(request))

	msgChannel := make(chan []byte)
	go func() {
		// XXX: Move this goshforsaken allocation at some point (Maybe move to global collector struct?).
		// Also note that this means messages higher than 2048 bytes in length will be sent in 2 chunks over the channel.
		//buf := make([]byte, 2048)
		for {
			//n, err := ws.Read(buf)
			ntype, n, err := ws.Read(ctx)
			fmt.Printf("Recieved message of type: %s", ntype)
			if err != nil {
				// Connection was severed, so quit.
				return
			} else {
				// Append message to message channel.
				msgChannel <- n
			}
		}
	}()

	go func() {
		<-ctx.Done()
		// Closing the websocket here should end the other goroutine.
		ws.CloseNow() // Close(Status Code, reason)
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
				fmt.Println("Waiting for block...")
				block, err := ethereum_client.BlockByNumber(ctxToPreventHanging, nil)
				fmt.Println("Got block!")

				if err != nil {
					// HACK: Lazy error handling, find better strategy later.
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
							// HACK:: Lazy error handling, find better strategy later.
							panic(err)
						} else {
							fmt.Println("Sending over channel,", buffer.Len())

							msgChannel <- buffer.Bytes()

							fmt.Println("Sent.")
						}
					}
				}
			}
		}
	}()

	return msgChannel, nil
}

func defaultJoinNFTCEX(ctx context.Context, source Source, topic string) (chan []byte, error) {
	// Move this later depending on how we choose to configure API keys.
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found")
	}
	// Find user's API key.
	apiKey := os.Getenv("OPENSEA_API_KEY")
	if apiKey == "" {
		err := "Unable to find API key"
		fmt.Println("ERROR:", err)
		return nil, errors.New(err) // Return error instead of panicking
		// Should handle this error by returning this information the user.
		// Note: Ensure that the node will not try to stake an empty channel to the blockchain.
	}
	fmt.Println("Found OpenSea API Key in environment")
	//socketUrl := fmt.Sprintf("%s?token=%s", source.ApiURL, apiKey) // Not required because URL is hardcoded (fix this)

	// Initialise SDK, message channel and ticker
	ns := openseaSdk.NewNotifyService(openseaSdk.MAIN_NET, apiKey)

	msgChannel := make(chan []byte)
	timeTicker := time.NewTicker(time.Second * 30)

	// Add messages to channel within context
	go func() {
		defer ns.Stop() // Ensure ns.Stop() is called when the goroutine exits
		for {
			select {
			case <-ctx.Done():
				return // Exit goroutine when context is done
			case <-timeTicker.C:
				// Get message stream from subscription.
				unsubscribe, err := ns.Subscribe("*", topic, func(msg *openseaSdk.Message) error {
					bmsg, err := json.Marshal(msg)
					if err != nil {
						return err // Return error instead of panicking
					}
					msgChannel <- bmsg
					return nil
				})
				defer unsubscribe()
				if err != nil {
					fmt.Println("Error subscribing:", err)
					return // Exit goroutine if subscription fails
				}
				ns.Start()
			}
		}
	}()
	return msgChannel, nil
}

/*
// Subscribe to data from a centralised NFT exchange (Default: OpenSea)
func defaultJoinNFTCEX(ctx context.Context, source Source, topic string) (chan []byte, error) {
	// Move this later depending on how we choose to configure API keys.
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found")
	}
	// Find user's API key.
	apiKey := os.Getenv("OPENSEA_API_KEY")
	if apiKey == "" {
		err := "Unable to find API key"
		fmt.Println("ERROR:", err)
		panic(err)
		// Should handle this error by returning this information the user.
		// Note: Ensure that the node will not try to stake an empty channel to the blockchain.
	}
	fmt.Println("Found OpenSea API Key in environment")
	//socketUrl := fmt.Sprintf("%s?token=%s", source.ApiURL, apiKey) // Not required because URL is hardcoded (fix this)

	// Initialise SDK, message channel and ticker
	ns := openseaSdk.NewNotifyService(openseaSdk.MAIN_NET, apiKey)

	msgChannel := make(chan []byte)
	timeTicker := time.NewTicker(time.Second * 30)

	// Add messages to channel within context
	go func() {
		defer ns.Stop()
		select {
		case <-ctx.Done():
			// Quit gracefully, out context was handled above.
		case <-timeTicker.C:
			// Get message stream from subscription.
			ns.Subscribe("*", topic, func(msg *openseaSdk.Message) error {
				bmsg, err := json.Marshal(msg)
				if err != nil {
					panic(err)
				}
				msgChannel <- bmsg
				return nil
			})

			ns.Start()
		}

	}()
	return msgChannel, nil
} */
