#!/bin/sh

rm -rf cbft-home/data/*.db
rm -rf cbft-home/data/cs.wal
echo '{}' > cbft-home/data/priv_validator_state.json
