rm -r ./cbft-home/data ./cbft-home/config
go run github.com/cometbft/cometbft/cmd/cometbft@v0.38.6 init --home ./cbft-home
go build .
