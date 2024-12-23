# Place

![Place web frontend](place.png)

**Place** is a basic drawing application powered by Diagrid Catalyst that tries to replicate some functionality from the famous [r/place](https://reddit.com/r/place) event.
Still very rough but a minimally usable version is being hosted [here](https://place.88288338.xyz/).

It is a Go application that:
* Keeps it's state in a [state store](https://docs.dapr.io/developing-applications/building-blocks/state-management/state-management-overview/)
* Keeps a cooldown timer on a different [state store](https://docs.dapr.io/developing-applications/building-blocks/state-management/state-management-overview/)
* Uses a websocket connection to broadcast changes to the clients
* Uses [pubsub](https://docs.dapr.io/developing-applications/building-blocks/pubsub/pubsub-overview/) to broadcast changes across the replicas

## Setup

* Sign up for a free [Catalyst](https://catalyst.diagrid.io) account.

* Authenticate using the [Diagrid CLI](https://docs.diagrid.io/catalyst/references/cli-reference/intro)
```bash
diagrid login 
```

* List the organization you're onboarded to.
```bash
diagrid orgs list
```

* Create a catalyst project with managed State store and PubSub
```bash
diagrid project create place --deploy-managed-kv --deploy-managed-pubsub
```

* Create a catalyst appid
```bash
diagrid appid create frontend
```

* After a few seconds, you should see both your project and appid in the Ready state.
```bash
diagrid project list
diagrid appid list
```

## Creating the `canvas` component

* Log into a free managed PostgreSQL provider like [Neon](https://neon.tech/) or [ElephantSQL](https://www.elephantsql.com/) and create a database.
  Obtain the connection string and use it to create a component.

```bash
diagrid component create canvas --type state.postgresql --metadata connectionString=<connection string>
```

## Running the application

* Build It
```bash
make build
```

* Authenticate using the [Diagrid CLI](https://github.com/diagridio/cli)
```bash
diagrid login 
```

* Generate diagrid configuration 
```bash
diagrid dev scaffold
```

* Update the generated `dev-place.yaml` with the following:

```yaml
  workDir: .
  command: ["./bin/frontend"]
```

* Run it
```bash
diagrid dev start
```

* Open the browser and navigate to `http://localhost:8080`

* Enter your name and start drawing on the canvas pixel by pixel.

![Place web frontend](place.png)

* Use the Catalyst web dashboard to monitor the application.

![Catalyst Call Graph](catalyst-app-graph.png)

![Catalyst API Logs](catalyst-requests.png)

## More information

Want to know more about the capabilities that Catalyst offers? Read it on our [Diagrid](https://diagrid.io/catalyst) website.