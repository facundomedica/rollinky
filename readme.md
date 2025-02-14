# rollinky
**rollinky** is a blockchain built using Cosmos SDK and Rollkit and created with [Ignite CLI](https://ignite.com/cli).

## Sequencer

The sequencer is the centralized-sequencer with modifications to allow adding a Head and a Tail tx to the block. The sequencer is responsible of adding the prices to the block.

## Skip Connect

### Sidecar

The sidecar is meant to be ran on an Intel SGX machine in an TEE, it uses a gRPC unary interceptor in order to create a report and send it to the client.

### Oracle client

This is a library, which contains modified code from Skip's oracle client, in order to make it possible to receive gRPC trailers that the sidecar populates with the verifiable report.

## Running it all together

### Prerequisites

This has been tested on Linux. MacOS is not supported, and Windows hasn't been tested.

Install:

- Golang
- Make (sudo apt-get install build-essential)
- Rollkit
- Ego (https://github.com/edgelesssys/ego)


#### Configure Intel SGX (If this step is skipped, you might get "OE_QUOTE_PROVIDER_LOAD_ERROR"):

```bash
sudo apt install libsgx-dcap-default-qpl
```

Check if this file exists: `/etc/sgx_default_qcnl.conf`; if it doesn't, download the default one:

```bash
wget -qO- https://raw.githubusercontent.com/intel/SGXDataCenterAttestationPrimitives/master/QuoteGeneration/qcnl/linux/sgx_default_qcnl.conf | sudo tee /etc/sgx_default_qcnl.conf > /dev/null
```

Finally, add a pccs_url to `/etc/sgx_default_qcnl.conf` (required for attestations to work):

```json
"pccs_url": "https://global.acccache.azure.net/sgx/certification/v4/"
```

### Build all the parts

1. Build rollkinkyd (`CGO_CFLAGS=-I/opt/ego/include CGO_LDFLAGS=-L/opt/ego/lib rollkit rebuild` or `CGO_CFLAGS=-I/opt/ego/include CGO_LDFLAGS=-L/opt/ego/lib go build ./cmd/…`)

2. Build the sequencer

```bash 
cd ./sequencer
make build
```

3. Build the sidecar (this is the only part that needs to be built and run on an Intel SGX machine):

```bash
cd ./connect
make all # build and sign
```

This step will create a new key pair and an enclave.json (if it hasn't been previously created). Now modify the enclave.json file to include the CA certificates **and re-build**.

```json
 "files": [
    {
        "source": "/etc/ssl/certs/ca-certificates.crt",
        "target": "/etc/ssl/certs/ca-certificates.crt"
    }
]
```

### Configure genesis

We currently use ignite to generate the genesis file, but it's not mandatory and can be done manually.

```bash
ignite chain build && ignite rollkit init
```

Add some config to the oracle and marketmap modules (optional):

```json
    "oracle": {
      "currency_pair_genesis": [
        {
          "currency_pair": {
            "Base": "BTC",
            "Quote": "USD"
          },
          "nonce": 0,
          "id": 1
        },
        {
          "currency_pair": {
            "Base": "ETH",
            "Quote": "USD"
          },
          "nonce": 0,
          "id": 2
        },
        {
          "currency_pair": {
            "Base": "USDT",
            "Quote": "USD"
          },
          "nonce": 0,
          "id": 3
        }
      ],
      "next_id": "4"
    },
...
"markets":{"BTC/USD":{"ticker":{"currency_pair":{"Base":"BTC","Quote":"USD"},"decimals":8,"min_provider_count":3,"enabled":true},"provider_configs":[{"name":"coinbase_api","off_chain_ticker":"BTC-USD"},{"name":"coinbase_api","off_chain_ticker":"BTC-USDT","normalize_by_pair":{"Base":"USDT","Quote":"USD"}},{"name":"binance_api","off_chain_ticker":"BTCUSDT","normalize_by_pair":{"Base":"USDT","Quote":"USD"}}]},"ETH/USD":{"ticker":{"currency_pair":{"Base":"ETH","Quote":"USD"},"decimals":11,"min_provider_count":3,"enabled":true},"provider_configs":[{"name":"coinbase_api","off_chain_ticker":"ETH-USD"},{"name":"coinbase_api","off_chain_ticker":"ETH-USDT","normalize_by_pair":{"Base":"USDT","Quote":"USD"}},{"name":"binance_api","off_chain_ticker":"ETHUSDT","normalize_by_pair":{"Base":"USDT","Quote":"USD"}}]},"USDT/USD":{"ticker":{"currency_pair":{"Base":"USDT","Quote":"USD"},"decimals":6,"min_provider_count":2,"enabled":true},"provider_configs":[{"name":"coinbase_api","off_chain_ticker":"USDT-USD"},{"name":"coinbase_api","off_chain_ticker":"USDC-USDT","invert":true},{"name":"kucoin_ws","off_chain_ticker":"BTC-USDT","normalize_by_pair":{"Base":"BTC","Quote":"USD"},"invert":true}]}}
```


### Run it all together

Start the DA:

```bash
curl -sSL https://rollkit.dev/install-local-da.sh | bash -s v0.3.1
```

Get the signer id, necessary to run the sequencer and the node:

```bash
ego signerid ./connect/public.pem
```

Then the sequencer (pass the signer id as a flag), which requires a toml config file (see oracleconfig.toml for an example):

```bash 
./sequencer/build/sequencer -rollup-id rollinky -da_address http://0.0.0.0:7980 -signer-id 102e485ef291ba28712e3fde8beccfb667e6e55734433119303d9653aa6db661 -config ./oracleconfig.toml
```

Now we start the sidecar which will throw some warnings until we start the chain, because the marketmap is missing. This needs to be built and run with Ego on an Intel SGX machine.

```bash
ego run ./build/connect
```

Finally we start our Cosmos SDK app, again passing the signer-id:

```bash
rollkit start --rollkit.sequencer_rollup_id rollinky --rollkit.da_address http://localhost
:7980 --rollkit.sequencer_address 0.0.0.0:50051  --rollkit.aggregator --signer-id 102e485ef291ba28712e3fde8beccfb667e6e55734433119303d9653aa6db661
```

After all of this is running and some blocks have passed we can get some prices from the oracle:

```bash
./rollinkyd q oracle price USDT USD
```

## Learn more

- [Ignite CLI](https://ignite.com/cli)
- [Tutorials](https://docs.ignite.com/guide)
- [Ignite CLI docs](https://docs.ignite.com)
- [Cosmos SDK docs](https://docs.cosmos.network)
- [Developer Chat](https://discord.gg/ignite)
