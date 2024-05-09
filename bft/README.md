# BFT Consensus library

## Notes on Execution provider (Polygon)
* The proposer of each block will add a transaction indicating the current height of polygon for the purpose of validating redeem events. 
    * This height is verified in FinalizeBlock by nodes using a spread calculated by the cometbft configured ~10 + blockTime / 2 (2 second polygon time, add 10 as 'slack')
    