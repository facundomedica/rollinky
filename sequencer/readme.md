## Centralized Sequencer

Read first [here](https://github.com/rollkit/centralized-sequencer)

This is an implementation of the centralized sequencer with a custom BatchExtender, which connects to a Skip Connect oracle sidecar. The BatchExtender's Head function gets the latest prices, verifies the signature using the signer ID and adds it to the batch.


### How to build

To build and run the binary it's necessary to have [ego](https://github.com/edgelesssys/ego) installed. Intel SGX support is not required, as the sequencer only performs verification.

```bash
make build
```

### How to run

```bash
./build/sequencer
```