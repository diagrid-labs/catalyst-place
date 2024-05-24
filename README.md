**Place** is a simple catalyst based application that tries to replicate some functionality from the famous r/place event.
Still very rough but a minimally usable version is being hosted [here](https://place.88288338.xyz/).

It is a Go application that:
* Keeps it's state in state store
* Uses a websocket connection to broadcast changes to the clients
* Uses pubsub to broadcast changes across the replicas
