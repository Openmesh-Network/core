package validator

import (
	"bytes"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
)

// Defines a "source" of data, all supported sources are laid out in the Sources array.
// We opt for this approach over oop for clarity and extensibility. We're going to have 1
// version of this program running for all the validators, so defining all the sources in a clear table
// seems more appropriate.
type Source struct {
	Name     string
	JoinFunc func(ctx context.Context, source Source, symbol string) (chan []byte, error)
	ApiURL   string
	Symbols  []string
	// Note that the Request field has {{symbol}} replaced with the symbol.
	Request string
}

// The master table with all our sources.
var Sources = [...]Source{
	// Exchanges:
	{"binance", wsCEXJoin, "wss://stream.binance.com:9443/ws", []string{"usdt.usdc", "btc.eth", "eth.usdt"}, "{\"method\": \"SUBSCRIBE\", \"params\": [ \"{{symbol}}@aggTrade\" ], \"id\": 1}"},
	{"dydx", wsCEXJoin, "wss://api.dydx.exchange/v3/ws", []string{}, "{\"type\": \"subscribe\", \"id\": \"{{symbol}}\", \"channel\": \"v3_trades\"}"},
	{"coingecko", wsCEXJoin, "", []string{}, ""},
	// Blockchains:
	// {"ethereum", "", nil, []string{}},
}

// Subscribe will join the chosen source and .
func Subscribe(ctx context.Context, source Source, symbol string, bufferSizeMax int) (*bytes.Buffer, error) {
	msgChannel, err := source.JoinFunc(ctx, source, symbol)
	if err != nil {
		return nil, err
	}

	// TODO: Move allocation elsewhere.
	writeMessageBuffer := bytes.NewBuffer(make([]byte, bufferSizeMax))
	writeMessageBuffer.Reset()

	go func() {
		for {
			select {
			case msg := <-msgChannel:
				// TODO: Add optional callback to re-encode the data for better size efficiency here.

				writeMessageBuffer.Write(msg)
				break
			case <-ctx.Done():
				break
			}
		}
	}()

	return writeMessageBuffer, nil
}

func wsCEXJoin(ctx context.Context, source Source, symbol string) (chan []byte, error) {
	url := "wss://stream.binance.com:9443/ws"
	ws, err := websocket.Dial(url, "", url)
	if err != nil {
		panic(err)
	}

	// Convert to binance symbol format.
	// NOTE(Tom): This feels like a waste, might be better to just use their format.
	split := strings.Split(symbol, ".")
	exchangeSymbol := split[0] + split[1]

	request := strings.Replace(source.Request, "{{symbol}}", exchangeSymbol, 1)

	ws.Write([]byte(request))

	// HACK: This feels very ugly, there might be a better approach.
	msgChannel := make(chan []byte)
	go func() {
		buf := make([]byte, 512)
		for {
			n, err := ws.Read(buf)
			if err != nil {
				// Connection was severed, so quit.
				return
			} else {
				msgChannel <- buf[:n]
			}
		}
	}()

	go func() {
		<-ctx.Done()
		ws.Close()
	}()

	return msgChannel, nil
}

// func dydxJoin(ctx context.Context, symbol string) (chan []byte, error) {
// 	url := "wss://api.dydx.exchange/v3/ws"
// 	ws, err := websocket.Dial(url, "", url)
// 	if err != nil {
// 		panic(err)
// 	}

// 	// Convert to binance symbol format.
// 	// NOTE(Tom): This feels like a waste, might be better to just use their format.
// 	split := strings.Split(symbol, ".")
// 	dydxSymbol := split[0] + split[1]

// 	// TODO: Which kind of trade do we want to store? Need to talk with team.
// 	ws.Write([]byte("{\"type\": \"subscribe\", \"id\": \"" + dydxSymbol + "\", \"channel\": \"v3_trades\"}"))

// 	// HACK: This feels very ugly, there might be a better approach.
// 	msgChannel := make(chan []byte)
// 	go func() {
// 		buf := make([]byte, 512)
// 		for {
// 			n, err := ws.Read(buf)
// 			if err != nil {
// 				// Connection was severed, so quit.
// 				return
// 			} else {
// 				msgChannel <- buf[:n]
// 			}
// 		}
// 	}()

// 	go func() {
// 		<-ctx.Done()
// 		ws.Close()
// 	}()

// 	return msgChannel, nil
// }

func coinspotJoin(val string) chan []byte {
	return nil
}

func ethRead(val string) chan []byte {
	return nil
}

// Need something that converts symbols to our format (or do we?).
// Need to connect to a source and listen to the data, if the data exceeds a buffer then it's given to the validator.
// How is our data structured? Is it sources then symbols or just sources?
// What are the "lanes" in our system?

// What is the common case? The common case is we're subscribed to a source and plan to switch relatively quickly.

// type DataSource interface {
// 	Name() string
// 	Symbols() []string
// 	JoinFunc(string) chan []byte
// 	ReadFunc(string) chan []byte
// }

// var dataSources = [...]DataSource{
// 	Binance{},
// 	// Coinbase{},
// }

// type Binance struct {
// }

// func (b Binance) Name() string {
// 	return "binance"
// }
// func (b Binance) Symbols() []string {
// 	return []string{"usdt-usdc", "btc-eth", "eth-usdt"}
// }
// func (b Binance) JoinFunc(a string) chan []byte {
// 	var bruh chan []byte
// 	return bruh
// }
// func (b Binance) ReadFunc(a string) chan []byte {
// 	var bruh chan []byte
// 	return bruh
// }
