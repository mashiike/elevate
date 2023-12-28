# elevate
AWS Lambda Websocket API Proxy integration event bridge to Go net/http.

## Usage

elevate is a bridge to convert API Gateway Websocket API with AWS Lambda Proxy integration event request/response and net/http.Request and net/http.ResponseWriter.

```go
package main

import (
	"encoding/json"
	"net/http"

	"github.com/mashiike/elevate"
)

func handler(w http.ResponseWriter, req *http.Request) {
	connectionID := elevate.ConnectionID(req)
	switch elevate.RouteKey(req) {
	case "$connect":
		// do something on connect websocket, for example, store connection id to database.
		w.WriteHeader(http.StatusOK) // return 200 OK, success to connect.
	case "$disconnect":
		// do something on disconnect websocket, for example, delete connection id from database.
		w.WriteHeader(http.StatusOK) // return 200 OK, success to disconnect.
	case "$default":
		// do something on default route
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(connectionID))
	case "notify":
		// default RouteKeySelector emulates $request.body.action
		var data struct {
			Action  string   `json:"action"`
			Targets []string `json:"targets"`
			Message string   `json:"message"`
		}
		if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(http.StatusText(http.StatusBadRequest)))
			return
		}
		for _, target := range data.Targets {
			if err := elevate.PostToConnection(req.Context(), target, []byte(data.Message)); err != nil {
				if elevate.ConnectionIsGone(err) {
					continue
				}
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(http.StatusText(http.StatusInternalServerError)))
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}

func main() {
	elevate.Run(http.HandlerFunc(handler))
}
```

## `elevate.Run(http.Handler)` and `elevate.RunWithOptions(http.Handler, ...elevate.Options)`

`elevate.Run(http.Handler)` and `elevate.RunWithOptions(http.Handler, ...elevate.Options)` works as below.

- if running on AWS Lambda(defined `AWS_LAMBDA_FUNCTION_NAME` environment variable), run as AWS Lambda handler.
    - Call `lambda.StartWithOptions()`
- otherwise, run as net/http server with websocket handler.
    - default address `ws://localhost:8080/` , you can change `elevate.WithListenAddress(address)` option.
    - default route key selector emulates `$request.body.action` , you can change `elevate.WithRouteKeySelector(selector)` option.

## License

MIT
