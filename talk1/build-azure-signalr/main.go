package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/vmihailenco/msgpack"
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

func main() {
	c := make(chan *websocket.Conn, 1)
	var clients sync.Map

	http.Handle("/server/", websocket.Server{
		Handler: websocket.Handler(func(ws *websocket.Conn) {
			connectionID := ws.Request().URL.Query().Get("id")
			if len(connectionID) == 0 {
				// Support websocket connection without negotiate
				connectionID = getConnectionID()
			}

			severConnectionHandler(&clients, connectionID, ws, c)
		}),
	})

	http.HandleFunc("/client/negotiate", negotiateHandler)
	http.Handle("/client/", websocket.Handler(func(ws *websocket.Conn) {
		connectionID := ws.Request().URL.Query().Get("id")
		if len(connectionID) == 0 {
			// Support websocket connection without negotiate
			connectionID = getConnectionID()
		}

		clientConnectionHandler(&clients, connectionID, ws, c)
	}))

	http.ListenAndServe("localhost:8087", nil)
}

func processHandshake(ws *websocket.Conn) error {
	var data []byte

	// https://github.com/Azure/azure-signalr/blob/dev/specs/ServiceProtocol.md#handshakeresponse-message
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

func severConnectionHandler(clients *sync.Map, connectionID string, ws *websocket.Conn, c chan *websocket.Conn) {
	var data []byte
	// https://github.com/Azure/azure-signalr/blob/dev/specs/ServiceProtocol.md#ping-message
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

	c <- ws

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

		fmt.Printf("Server Received: MessageType = %d\n", messageType)

		switch messageType {
		case 10:
			// https://github.com/Azure/azure-signalr/blob/dev/specs/ServiceProtocol.md#broadcastdata-message
			excludeListLen, err := decoder.DecodeArrayLen()

			if err != nil {
				fmt.Println(err)
				break
			}

			// Skip the exclude list for now
			for index := 0; index < excludeListLen; index++ {
				decoder.DecodeString()
			}

			mapLen, err := decoder.DecodeMapLen()

			if err != nil {
				fmt.Println(err)
				break
			}

			// Assume it's a single protocol for now
			fmt.Printf("Map has %d elements.\n", mapLen)

			protocol, err := decoder.DecodeString()

			if err != nil {
				fmt.Println(err)
				break
			}

			fmt.Printf("Broadcasting %s message\n", protocol)

			payload, err := decoder.DecodeBytes()

			if err != nil {
				fmt.Println(err)
				break
			}

			fmt.Printf("Broadcasting payload size: %db\n", len(payload))

			clients.Range(func(key, value interface{}) bool {
				websocket.Message.Send(value.(*websocket.Conn), string(payload))
				return true
			})

			break
		case 5:
			// https://github.com/Azure/azure-signalr/blob/dev/specs/ServiceProtocol.md#closeconnection-message
			break
		case 6:
			// https://github.com/Azure/azure-signalr/blob/dev/specs/ServiceProtocol.md#connectiondata-message
			clientConnectionID, err := decoder.DecodeString()

			if err != nil {
				fmt.Println(err)
				break
			}

			payload, err := decoder.DecodeBytes()

			if err != nil {
				fmt.Println(err)
				break
			}

			clientWs, ok := clients.Load(clientConnectionID)
			if !ok {
				break
			}

			// Assume text for now
			websocket.Message.Send(clientWs.(*websocket.Conn), string(payload))
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

func clientConnectionHandler(clients *sync.Map, connectionID string, ws *websocket.Conn, c chan *websocket.Conn) {
	var data []byte

	// Wait for a server connection
	target := <-c

	// Write the target connection back to the channel immediately
	c <- target

	clients.Store(connectionID, ws)

	writeOpenConnectionMessage(connectionID, target)

	for {
		if err := websocket.Message.Receive(ws, &data); err != nil {
			break
		}

		fmt.Printf("Client Received %d bytes\n", len(data))

		writeConnectionMessage(connectionID, target, data)
	}

	writeCloseConnectionMessage(connectionID, target)

	clients.Delete(connectionID)
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

func writeConnectionMessage(connectionID string, ws *websocket.Conn, data []byte) {

	var buf bytes.Buffer
	encoder := msgpack.NewEncoder(&buf)
	encoder.EncodeArrayLen(3)
	encoder.EncodeInt(6)
	encoder.EncodeString(connectionID)
	encoder.EncodeBytes(data)

	var message bytes.Buffer
	lengthPrefix(&message, buf.Len())
	buf.WriteTo(&message)

	websocket.Message.Send(ws, message.Bytes())
}

func negotiateHandler(w http.ResponseWriter, req *http.Request) {
	// CORS
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

func getConnectionID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		fmt.Println(err)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}
