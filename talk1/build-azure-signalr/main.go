package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

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

	var buf bytes.Buffer
	encoder := msgpack.NewEncoder(&buf)
	encoder.EncodeArrayLen(2)
	encoder.EncodeInt(2)
	encoder.EncodeString("")
	fmt.Printf("MessageLength = %d", buf.Len())

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

	http.HandleFunc("/client/negotiate", negotiateHandler)
	http.Handle("/client/", websocket.Handler(func(ws *websocket.Conn) {
		connectionID := ws.Request().URL.Query().Get("id")
		if len(connectionID) == 0 {
			// Support websocket connection without negotiate
			connectionID = getConnectionID()
		}

		clientConnectionHandler(connectionID, ws)
	}))

	http.ListenAndServe("localhost:8087", nil)
}

func severConnectionHandler(connectionID string, ws *websocket.Conn) {
	var data []byte

	for {
		if err := websocket.Message.Receive(ws, &data); err != nil {
			break
		}

		length, numBytes := decodeMessageLen(data)

		fmt.Printf("Server Received: Length = %d\n", length)

		// Handshake
		decoder := msgpack.NewDecoder(bytes.NewReader(data[numBytes:]))
		length, err := decoder.DecodeArrayLen()
		if err != nil {
			fmt.Println(err)
			break
		}

		messageType, err := decoder.DecodeInt32()
		switch messageType {
		case 1:
			// Handshake
			version, _ := decoder.DecodeInt32()

			fmt.Printf("Protocol version: %d\n", version)

			// handshakeResponse, _ := msgpack.Marshal([2]interface{}{2, nil})

			// r := msgpack.NewDecoder(bytes.NewReader(handshakeResponse))
			// hrl, _ := r.DecodeArrayLen()
			// hmt, _ := r.DecodeInt16()

			//fmt.Printf("AL=%d\n", hrl)
			// fmt.Printf("X=%d\n", hmt)

			// fmt.Printf("Len=%d, %d\n", len(handshakeResponse), len(lengthPrefix(len(handshakeResponse))))

			// x := lengthPrefix(len(handshakeResponse))
			// dl, dn := decodeMessageLen(x)

			// fmt.Printf("Len=%d, %d\n", dl, dn)

			var buf bytes.Buffer
			encoder := msgpack.NewEncoder(&buf)
			encoder.EncodeArrayLen(2)
			encoder.EncodeInt(2)
			encoder.EncodeString("")
			fmt.Printf("MessageLength = %d", buf.Len())

			var message bytes.Buffer
			message.Write(lengthPrefix(buf.Len()))
			buf.WriteTo(&message)

			x := message.Bytes()

			ws.PayloadType = 0

			// fmt.Printf("Payload Type %d\n", t)

			ws.Write(x)
			// x := len(handshakeResponse)

			// message := lengthPrefix(msgLength)

			// append(message, response)

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

func lengthPrefix(length int) []byte {
	var output [5]byte

	var lenNumBytes = 0
	for length > 0 {
		var current = (byte)(length & 0x7f)
		output[lenNumBytes] = current
		length >>= 7
		if length > 0 {
			current |= 0x80
		}
		lenNumBytes++
	}

	return output[0:lenNumBytes]
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func clientConnectionHandler(connectionID string, ws *websocket.Conn) {
	var data []byte
	for {
		if err := websocket.Message.Receive(ws, &data); err != nil {
			break
		}

		fmt.Println("Client Received " + string(data))
	}
}

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

func getConnectionID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		fmt.Println(err)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}
