**Place** is a simple catalyst based application that tries to replicate some functionality from the famous [r/place](https://reddit.com/r/place) event.
Still very rough but a minimally usable version is being hosted [here](https://place.88288338.xyz/).

It is a Go application that:
* Keeps it's state in state store
* Uses a websocket connection to broadcast changes to the clients
* Uses pubsub to broadcast changes across the replicas

## Running the application

* Build It
```bash
make build
```

* Authenticate using the [Diagrid CLI](https://github.com/diagridio/cli)
```bash
diagrid login 
```

* Generate diagrid dev configuration 
```bash
diagrid dev scaffold
```

* Update the generated `diagrid.dev.yaml` with the following variables
```yaml
  appPort: 8080
```

```yaml
  workDir: ./bin
  command: ["./frontend"]
```

* Run it
```bash
diagrid dev start
```

* Open the browser and navigate to `http://localhost:8080`

