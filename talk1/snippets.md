# Final Before refactoring
https://github.com/davidfowl/signalr-ports/blob/86496e708ff860fce0283d75480c0a188f6c4fc0/signalr-go-server/pkg/signalr/signalr.go

# go hello world

```go
package main

import (
	"fmt"
)

func main() {
	fmt.Println("Hello World")
}
```

# Hello HTTP

```go
package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(writer http.ResponseWriter, req *http.Request) {
		writer.Write([]byte("Hello NDC"))
	})

	http.ListenAndServe("localhost:8085", nil)
}
```

# Setup static file server with SignalR harness

```go
package main

import (
	"fmt"
	"net/http"
)

func main() {
    http.Handle("/", http.FileServer(http.Dir("public")))

	http.ListenAndServe("localhost:8085", nil)
}
```

# mapHub stub

```go
package main

import (
	"fmt"
	"net/http"
)

func main() {
    http.Handle("/", http.FileServer(http.Dir("public")))

	mapHub("/chat")

	http.ListenAndServe("localhost:8085", nil)
}

func mapHub(path string) {
    
}

```

# Transport protocol

https://github.com/dotnet/aspnetcore/blob/master/src/SignalR/docs/specs/TransportProtocols.md

# Negotiate

```go
func mapHub(path string) {
	http.HandleFunc(fmt.Sprintf("%s/negotiate", path), negotiateHandler)
}

type availableTransport struct {
	Transport       string   `json:"transport"`
	TransferFormats []string `json:"transferFormats"`
}

type negotiateResponse struct {
	ConnectionID        string               `json:"connectionId"`
	AvailableTransports []availableTransport `json:"availableTransports"`
}

func negotiateHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		w.WriteHeader(400)
		return
	}

	connectionID := getConnectionID()

	response := negotiateResponse{
		ConnectionID: connectionID,
		AvailableTransports: []availableTransport{
			{
				Transport:       "WebSockets",
				TransferFormats: []string{"Text", "Binary"},
			},
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		fmt.Println(err)
	}
}

func getConnectionID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		fmt.Println(err)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}
```

# WebSocket Connect

```go
func mapHub(path string) {
	http.HandleFunc(fmt.Sprintf("%s/negotiate", path), negotiateHandler)

	http.Handle(path, websocket.Handler(func(ws *websocket.Conn) {
		connectionID := ws.Request().URL.Query().Get("id")
		if len(connectionID) == 0 {
			// Support websocket connection without negotiate
			connectionID = getConnectionID()
		}

		hubConnectionHandler(connectionID, ws)
	}))
}

func hubConnectionHandler(connectionID string, ws *websocket.Conn) {
 
}
```

# HubProtocol
https://github.com/dotnet/aspnetcore/blob/master/src/SignalR/docs/specs/HubProtocol.md

# Process handshake

```go
type handshakeRequest struct {
	Protocol string `json:"protocol"`
	Version  int    `json:"version"`
}

func processHandshake(ws *websocket.Conn) error {
	var data []byte
	const handshakeResponse = "{}\u001e"
	const errorHandshakeResponse = "{\"error\":\"%s\"}\u001e"

	if err := websocket.Message.Receive(ws, &data); err != nil {
		return err
	}

	fmt.Println("Received " + string(data))

	request := handshakeRequest{}
	index := bytes.IndexByte(data, 30)
	err := json.Unmarshal(data[0:index], &request)

	if err != nil {
		return err
	}

	if request.Protocol != "json" {
		// Protocol not supported
		err = fmt.Errorf("Protocol \"%s\" not supported", request.Protocol)
		websocket.Message.Send(ws, fmt.Sprintf(errorHandshakeResponse, err.Error()))
		return err
	}

	websocket.Message.Send(ws, handshakeResponse)
	return nil
}

func hubConnectionHandler(connectionID string, ws *websocket.Conn) {
	var data []byte

	err := processHandshake(ws)

	if err != nil {
		return
	}

	for {
		if err := websocket.Message.Receive(ws, &data); err != nil {
			break
		}

		fmt.Println("Received " + string(data))
	}
}
```

# Send Server -> Client Pings

```go
ticker := time.NewTicker(10 * time.Second)
done := make(chan bool)
go func() {
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			websocket.Message.Send(ws, pingMessage)
		}
	}
}()

for {
	if err := websocket.Message.Receive(ws, &data); err != nil {
		break
	}

	fmt.Println("Received " + string(data))
}

ticker.Stop()
done <- true
```

# Handle invocations

```go
type hubMessage struct {
	Type int `json:"type"`
}

type hubInvocationMessage struct {
	Type         int               `json:"type"`
	Target       string            `json:"target"`
	InvocationID string            `json:"invocationId,omitempty"`
	Arguments    []json.RawMessage `json:"arguments"`
}
```

```go
for {
	if err := websocket.Message.Receive(ws, &data); err != nil {
		break
	}

	fmt.Println("Received " + string(data))
	index := bytes.IndexByte(data, 30)
	rawHubMessage := data[0:index]

	var message hubMessage
	json.Unmarshal(rawHubMessage, &message)

	switch message.Type {
	case 1:
		var invocation hubInvocationMessage
		json.Unmarshal(rawHubMessage, &invocation)

		result, err := onInvocation(ws, invocation.InvocationID, invocation.Target, invocation.Arguments)

		break
	}
}

func onInvocation(invocationID string, target string, args []json.RawMessage) (interface{}, error) {
	return nil, nil
}
```

# Invocation Ack

```go
type completionMessage struct {
	Type         int         `json:"type"`
	InvocationID string      `json:"invocationId"`
	Result       interface{} `json:"result,omitempty"`
	Error        string      `json:"error,omitempty"`
}
```

```go
switch message.Type {
case 1:
	var invocation hubInvocationMessage
	json.Unmarshal(rawHubMessage, &invocation)

	result, err := onInvocation(ws, invocation.InvocationID, invocation.Target, invocation.Arguments)

	errorValue := ""

	if err != nil {
		errorValue = err.Error()
	}

	completion := completionMessage{
		Type:         3,
		InvocationID: invocation.InvocationID,
		Result:       result,
		Error:        errorValue,
	}

	completionData, err := json.Marshal(completion)

	if err != nil {
		break
	}

	ws.Write(append(completionData, 30))
	break
}
```

# Reply to one connection

```go
func onInvocation(ws *websocket.Conn, invocationID string, target string, args []json.RawMessage) (interface{}, error) {
	target = strings.ToLower(target);

	if target == "send" {
		reply := hubInvocationMessage{
			Type:      1,
			Target:    "send",
			Arguments: args,
		}

		data, err := json.Marshal(reply)

		if err != nil {
			return nil, err
		}

		data = append(data, 30)

		ws.Write(data)
	}

	return nil, nil
}
```

# Broadcast to all connections

```go
func mapHub(path string) {
	var clients sync.Map

	http.HandleFunc(fmt.Sprintf("%s/negotiate", path), negotiateHandler)

	http.Handle(path, websocket.Handler(func(ws *websocket.Conn) {
		connectionID := ws.Request().URL.Query().Get("id")
		if len(connectionID) == 0 {
			// Support websocket connection without negotiate
			connectionID = getConnectionID()
		}

		hubConnectionHandler(&clients, connectionID, ws)
	}))
}
```

```go
func hubConnectionHandler(clients *sync.Map, connectionID string, ws *websocket.Conn) {
	...

	clients.Store(connectionID, ws)

	for {
		...

		switch message.Type {
		case 1:
			...

			result, err := onInvocation(clients, ws, invocation.InvocationID, invocation.Target, invocation.Arguments)

			...
			break
		}
	}

	...

	clients.Delete(connectionID)
}
```

```go
func onInvocation(clients *sync.Map, ws *websocket.Conn, invocationID string, target string, args []json.RawMessage) (interface{}, error) {
	target = strings.ToLower(target)

	if target == "send" {
		reply := hubInvocationMessage{
			Type:      1,
			Target:    "send",
			Arguments: args,
		}

		data, err := json.Marshal(reply)

		if err != nil {
			return nil, err
		}

		data = append(data, 30)

		clients.Range(func(key, value interface{}) bool {
			value.(*websocket.Conn).Write(data)
			return true
		})
	}

	return nil, nil
}
```

# Server -> Client Streaming

```go
switch message.Type {
case 1, 4:
	var invocation hubInvocationMessage
	json.Unmarshal(rawHubMessage, &invocation)

	result, err := onInvocation(clients, ws, invocation.InvocationID, invocation.Target, invocation.Arguments)

	if message.Type == 4 {
		break
	}

	errorValue := ""

	if err != nil {
		errorValue = err.Error()
	}

	completion := completionMessage{
		Type:         3,
		InvocationID: invocation.InvocationID,
		Result:       result,
		Error:        errorValue,
	}

	completionData, err := json.Marshal(completion)

	if err != nil {
		break
	}

	ws.Write(append(completionData, 30))
	break
}
```

```go
if target == "stream" {
	go func() {
		for index := 0; index < 10; index++ {
			streamItemMessage := streamItemMessage{
				Type:         2,
				InvocationID: invocationID,
				Item:         time.Now(),
			}

			data, _ := json.Marshal(streamItemMessage)

			ws.Write(append(data, 30))

			time.Sleep(2 * time.Second)
		}

		completion := completionMessage{
			Type:         3,
			InvocationID: invocationID,
		}
		data, _ := json.Marshal(completion)
		ws.Write(append(data, 30))
	}()
}
```

# Client -> Server Streaming

???

# Azure SignalR

https://github.com/Azure/azure-signalr/blob/dev/specs/ServiceProtocol.md

# Setup ASP.NET Core with Azure SignalR

```json
{
  "Azure": {
    "SignalR": {
      "ConnectionString": "Endpoint=http://localhost;Port=8087;AccessKey=ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789;"
    }
  }
}
```

# Allow Server Connections

```go
http.Handle("/server/", websocket.Server{
	Handler: websocket.Handler(func(ws *websocket.Conn) {
		connectionID := ws.Request().URL.Query().Get("id")
		if len(connectionID) == 0 {
			// Support websocket connection without negotiate
			connectionID = getConnectionID()
		}

		severConnectionHandler(connectionID, ws)
	}),
})

func severConnectionHandler(connectionID string, ws *websocket.Conn) {
	var data []byte
	for {
		if err := websocket.Message.Receive(ws, &data); err != nil {
			break
		}

		fmt.Println("Server Received " + string(data))
	}
}

func getConnectionID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		fmt.Println(err)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}
```

## Allow Client connections

```go
http.HandleFunc("/client/negotiate", negotiateHandler)
http.Handle("/client/", websocket.Handler(func(ws *websocket.Conn) {
	connectionID := ws.Request().URL.Query().Get("id")
	if len(connectionID) == 0 {
		// Support websocket connection without negotiate
		connectionID = getConnectionID()
	}

	clientConnectionHandler(connectionID, ws)
}))
```

```go
func negotiateHandler(w http.ResponseWriter, req *http.Request) {
	// CORS..
	if req.Method == "OPTIONS" {
		// Pre-flight
		w.Header().Add("Access-Control-Allow-Origin", req.Header.Get("origin"))
		w.Header().Add("Access-Control-Allow-Method", req.Header.Get("Access-Control-Request-Method"))
		w.Header().Add("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Access-Control-Allow-Headers", "x-requested-with, authorization")
		w.WriteHeader(204)
		return
	}

	w.Header().Add("Access-Control-Allow-Origin", req.Header.Get("origin"))
	w.Header().Add("Access-Control-Allow-Method", req.Method)
	w.Header().Add("Access-Control-Allow-Credentials", "true")

	if req.Method != "POST" {
		w.WriteHeader(400)
		return
	}

	connectionID := getConnectionID()

	response := negotiateResponse{
		ConnectionID: connectionID,
		AvailableTransports: []availableTransport{
			{
				Transport:       "WebSockets",
				TransferFormats: []string{"Text", "Binary"},
			},
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		fmt.Println(err)
	}
}
```

# Application -> Service Handshake

```go
func processHandshake(ws *websocket.Conn) error {
	var data []byte

	if err := websocket.Message.Receive(ws, &data); err != nil {
		return err
	}

	length, numBytes := decodeMessageLen(data)

	fmt.Printf("Server Received: Length = %d\n", length)

	decoder := msgpack.NewDecoder(bytes.NewReader(data[numBytes:]))
	length, err := decoder.DecodeArrayLen()
	if err != nil {
		return err
	}

	messageType, err := decoder.DecodeInt32()
	switch messageType {
	case 1:
		// Handshake
		version, _ := decoder.DecodeInt32()

		fmt.Printf("Protocol version: %d\n", version)

		var buf bytes.Buffer
		encoder := msgpack.NewEncoder(&buf)
		encoder.EncodeArrayLen(2)
		encoder.EncodeInt(2)
		encoder.EncodeString("")

		var message bytes.Buffer
		lengthPrefix(&message, buf.Len())
		buf.WriteTo(&message)

		websocket.Message.Send(ws, message.Bytes())
		break
	}

	return nil
}

func severConnectionHandler(connectionID string, ws *websocket.Conn) {
	var data []byte

	err := processHandshake(ws)

	if err != nil {
		fmt.Println(err)
		return
	}

	for {
		if err := websocket.Message.Receive(ws, &data); err != nil {
			break
		}
	}
}

func decodeMessageLen(buffer []byte) (int, int) {
	length := 0
	numBytes := 0

	var header = buffer[0:min(5, len(buffer))]
	var byteRead byte

	for {
		byteRead = header[numBytes]
		length = int(uint(length) | uint(byteRead&0x7f)<<(numBytes*7))
		numBytes++

		if numBytes == len(header) || (byteRead&0x80) == 0 {
			break
		}
	}
	return length, numBytes
}

func lengthPrefix(buf *bytes.Buffer, length int) {
	for length > 0 {
		var current = (byte)(length & 0x7f)
		buf.WriteByte(current)
		length >>= 7
		if length > 0 {
			current |= 0x80
		}
	}
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
```

# Service -> Application pings

```go
func severConnectionHandler(connectionID string, ws *websocket.Conn) {
	var data []byte
	pingMessage := []byte{2, 145, 3}

	err := processHandshake(ws)
	if err != nil {
		fmt.Println(err)
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				websocket.Message.Send(ws, pingMessage)
			}
		}
	}()

	for {
		if err := websocket.Message.Receive(ws, &data); err != nil {
			break
		}

		length, numBytes := decodeMessageLen(data)

		fmt.Printf("Server Received: Length = %d\n", length)

		decoder := msgpack.NewDecoder(bytes.NewReader(data[numBytes:]))
		length, err := decoder.DecodeArrayLen()
		if err != nil {
			fmt.Println(err)
			break
		}

		messageType, err := decoder.DecodeInt32()
		switch messageType {
		case 10: // Broadcast
			break
		}
	}
}
```

# Client connected/disconnect

```go
func clientConnectionHandler(connectionID string, ws *websocket.Conn) {
	var data []byte
	for {
		if err := websocket.Message.Receive(ws, &data); err != nil {
			break
		}

		fmt.Println("Client Received " + string(data))
	}
}

func writeOpenConnectionMessage(connectionID string, ws *websocket.Conn) {
	var buf bytes.Buffer
	encoder := msgpack.NewEncoder(&buf)
	encoder.EncodeArrayLen(3)
	encoder.EncodeInt(4)
	encoder.EncodeString(connectionID)
	encoder.EncodeMapLen(0)

	var message bytes.Buffer
	lengthPrefix(&message, buf.Len())
	buf.WriteTo(&message)

	websocket.Message.Send(ws, message.Bytes())
}

func writeCloseConnectionMessage(connectionID string, ws *websocket.Conn) {
	var buf bytes.Buffer
	encoder := msgpack.NewEncoder(&buf)
	encoder.EncodeArrayLen(3)
	encoder.EncodeInt(5)
	encoder.EncodeString(connectionID)
	encoder.EncodeString("")

	var message bytes.Buffer
	lengthPrefix(&message, buf.Len())
	buf.WriteTo(&message)

	websocket.Message.Send(ws, message.Bytes())
}
```

# Client message