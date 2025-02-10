Read the [oracle documentation](https://github.com/skip-mev/connect/blob/main/oracle/README.md) first, as this is just an implementation of that.

## Connect sidecar

The sidecar we use has a unary interceptor in order to create a report and send it to the client. This report is created by using the [ego](https://github.com/edgelesssys/ego) library and added to the returning gRPC trailers.

### How to build

To build the sidecar [we use Ego](https://github.com/edgelesssys/ego), so make sure you have it installed before building.

Ego is a tool that allows you to build Go applications with Intel SGX support, and for this binary given that it generates a report, the machine running it also needs to have an Intel SGX enabled CPU.

```bash
make all
```

#### Building for a testnet/mainnet

Because we are signing this binary and then verifying its outputs, **this binary can't be built by everyone**. This is meant to be built by a single trusted party, and then distributed to everyone else along with the signer ID.

For local tests, it's fine to just build the binary locally.

### How to run

```bash
ego run connectd
```

### Get signerid

```bash
ego signerid ./public.pem
```