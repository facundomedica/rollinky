## Connect sidecar

The sidecar we use has a unary interceptor in order to create a report and send it to the client.

### How to run

```bash
EDG_LOG_FORMAT=json sudo ego run connectd --market-map-endpoint="tcp://2.tcp.eu.ngrok.io:14563"
```

### Get signerid

```bash
ego signerid ./public.pem
```