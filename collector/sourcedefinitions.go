package collector

var SourcesCEX = []Source{
	// Centralised Exchanges:
	// Note that the topics are incomplete as they are undecided.
	{"binance", defaultJoinCEX, panicStubFunctionSeeBodyForExplanation, "wss://stream.binance.com:9443/ws", []string{"btcusdt", "ethusdt", "solusdt"}, "{ \"method\": \"SUBSCRIBE\", \"params\": [ \"{{topic}}@aggTrade\" ], \"id\": 1 }"},
	{"coinbase", defaultJoinCEX, panicStubFunctionSeeBodyForExplanation, "wss://ws-feed.pro.coinbase.com", []string{"BTC-USD", "ETH-USD", "BTC-ETH"}, "{\"type\": \"subscribe\", \"product_ids\": [ \"{{topic}}\" ], \"channels\": [ \"ticker\" ]}"},

	{
		"dydx",
		defaultJoinCEX,
		panicStubFunctionSeeBodyForExplanation,
		"wss://api.dydx.exchange/v3/ws",
		[]string{"MATIC-USD", "LINK-USD", "SOL-USD", "ETH-USD", "BTC-USD"},
		"{\"type\": \"subscribe\", \"id\": \"{{topic}}\", \"channel\": \"v3_trades\"}",
	},
	{
		"bybit",
		defaultJoinCEX,
		panicStubFunctionSeeBodyForExplanation,
		"wss://stream.bybit.com/v5/public/spot",
		[]string{"orderbook.50.BTCUSDT", "publicTrade.BTCUSDT", "tickers.BTCUSDT", "kline.M.BTCUSDT"},
		`{"op": "subscribe","args": ["{{topic}}"]}`,
	},

	// OKX
	// https://www.okx.com/docs-v5/en/#spread-trading-websocket-public-channel
	{
		"okx",
		defaultJoinCEX,
		panicStubFunctionSeeBodyForExplanation,
		"wss://ws.okx.com:8443/ws/v5/business",
		[]string{"sprd-bbo-tbt", "sprd-books5", "sprd-public-trades", "sprd-tickers"},
		`{"op": "subscribe","args": [{"channel": "{{topic}}","sprdId": "BTC-USDT_BTC-USDT-SWAP"}]}`,
	},
}

var SourcesBlockchainRPC = []Source{
	{"ethereum-ankr-rpc", joinEthereumRPC, nil, "https://rpc.ankr.com/eth", []string{""}, ""},
	{"polygon-ankr-rpc", joinEthereumRPC, nil, "https://rpc.ankr.com/polygon", []string{""}, ""},
}

var SourcesNFTExchange = []Source{
	// XXX: Disabled for now since it requires an API key. We can't guarantee nodes in the actual network will have this key.
	// Centralised NFT Exchange:
	// Opensea Request structure: {topic: \ event: \ payload:{} \ ref: }
	// {"opensea", defaultJoinNFTCEX, "wss://stream.openseabeta.com/socket", []string{"item_listed", "item_cancelled", "item_sold", "item_transferred", "item_received_offer", "item_received_bid"}, "collections:*"},
}

// The global table with all our sources.
var Sources = append(SourcesCEX, SourcesBlockchainRPC...)

func init() {
	// Note(Tom): Have to do this here otherwise we'll get a initialization cycle.
	for i := range SourcesCEX {
		SourcesCEX[i].ParseFunc = parseCEX
	}
}

func panicStubFunctionSeeBodyForExplanation(source Source, data []byte) []byte {
	panic("This function should never run, it's here to avoid an initialization cycle for the parseCEX function.\n The actual function used by CEXs is set on the init function in the sourcedefinitions.go file.")
}
