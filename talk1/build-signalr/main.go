package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

type availableTransport struct {
	Transport       string   `json:"transport"`
	TransferFormats []string `json:"transferFormats"`
}

type negotiateResponse struct {
	ConnectionID        string               `json:"connectionId"`
	AvailableTransports []availableTransport `json:"availableTransports"`
}

type handshakeRequest struct {
	Protocol string `json:"protocol"`
	Version  int    `json:"version"`
}

type hubMessage struct {
	Type int `json:"type"`
}

type hubInvocationMessage struct {
	Type         int               `json:"type"`
	Target       string            `json:"target"`
	InvocationID string            `json:"invocationId,omitempty"`
	Arguments    []json.RawMessage `json:"arguments"`
}

type completionMessage struct {
	Type         int         `json:"type"`
	InvocationID string      `json:"invocationId"`
	Result       interface{} `json:"result,omitempty"`
	Error        string      `json:"error,omitempty"`
}

type streamItemMessage struct {
	Type         int         `json:"type"`
	InvocationID string      `json:"invocationId"`
	Item         interface{} `json:"item"`
}

func main() {
	http.Handle("/", http.FileServer(http.Dir("public")))

	mapHub("/chat")

	http.ListenAndServe("localhost:8085", nil)
}

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

func hubConnectionHandler(clients *sync.Map, connectionID string, ws *websocket.Conn) {
	var data []byte
	const pingMessage = "{\"type\":6}\u001e"

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

	clients.Store(connectionID, ws)

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

			// Handle the message

			ws.Write(append(completionData, 30))
			break
		}
	}

	ticker.Stop()
	done <- true

	clients.Delete(connectionID)
}

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
	} else if target == "stream" {
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

	return nil, nil
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
			availableTransport{
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
