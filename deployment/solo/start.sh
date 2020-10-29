#!/usr/bin/env bash
./build/bin/geth \
--datadir data \
--port 20301 \
--mine \
--unlock 0xadbf6f637deaa832bbb2613c5394272c130dcac6 \
--password password.txt \
--miner.threads=1 \
--allow-insecure-unlock \
--rpc \
--rpcapi debug,admin,eth,miner,net,personal,txpool,web3 \
--wsapi debug,admin,eth,miner,net,personal,txpool,web3
