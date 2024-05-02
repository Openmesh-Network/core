#!/bin/sh

rm -rf cbft-home/data/*.db
echo '{}' > cbft-home/data/priv_validator_state.json
