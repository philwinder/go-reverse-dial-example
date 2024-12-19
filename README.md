# go-reverse-dial-example

Two simple examples that use a `controller<-worker` architecture.

In both cases the worker initially connects to the worker and the controller uses the same connection to send HTTP-like requests to the worker.

## Running

In separate terminals:

```sh
go run http/controller/main.go
```

```sh
go run http/runner/main.go
```

OR


```sh
go run websocket/controller/main.go
```

```sh
go run websocket/runner/main.go
```