#!/bin/bash


for i in {1..1024}
do
  PENULTIMATE=20
  FINAL=$i

  while [[ $FINAL -gt 255 ]]
  do
    PENULTIMATE=$(( PENULTIMATE + 1)) 
    FINAL=$(( FINAL - 256 ))
  done

  IP="192.168.$PENULTIMATE.$FINAL"
  echo $IP

  mkdir -p nodekeys/$IP
  cometbft gen-node-key --home nodekeys/$IP
  cometbft show-node-id --home nodekeys/$IP > nodekeys/$IP/id
done
