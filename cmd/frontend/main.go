package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var (
	//go:embed static/*
	staticFiles embed.FS
	mu          sync.Mutex
	conns       []*websocket.Conn
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

type Point struct {
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Color string `json:"color"`
}

type Userinfo struct {
	Name string `json:"name"`
}

type PlaceRequest struct {
	Point    `json:"point"`
	Userinfo `json:"userinfo"`
}

type Pixel struct {
	Point    `json:"point"`
	Userinfo `json:"userinfo"`
}

type Operation struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

func main() {
	client, err := dapr.NewClient()
	if err != nil {
		panic(fmt.Errorf("error creating connection to catalyst: %w", err))
	}
	defer client.Close()

	r := mux.NewRouter()
	r.HandleFunc("/", serveIndex)
	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(w, r, client)
	})
	log.Println("Server started at :8080")

	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	// Read the contents of the index.html file from the embedded FS
	indexHTML, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println("Error reading index.html:", err)
		return
	}

	// Serve the index.html content
	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write(indexHTML); err != nil {
		log.Println("Error serving index.html:", err)
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request, client dapr.Client) {
	ctx := r.Context()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	mu.Lock()
	conns = append(conns, conn)
	mu.Unlock()
	log.Println("Client connected")

	var op Operation
	// First message from the client is their info
	if err := conn.ReadJSON(&op); err != nil {
		log.Println("Error reading JSON message:", err)
		return
	}
	log.Printf("Received %v: %+v", op.Type, op.Data)
	var userinfo Userinfo
	if err := json.Unmarshal([]byte(op.Data), &userinfo); err != nil {
		log.Println("Error unmarshalling JSON:", err)
		return
	}

	// shoot the canvas to the client

	for {
		var op Operation
		// Read the message from the client
		if err := conn.ReadJSON(&op); err != nil {
			log.Println("Error reading JSON message:", err)
			return
		}
		log.Printf("Received op from client: %+v", op)

		switch op.Type {
		case "place":
			var p PlaceRequest
			if err := json.Unmarshal([]byte(op.Data), &p); err != nil {
				log.Println("Error unmarshalling JSON:", err)
				break
			}
			log.Printf("Received place pixel request: %+v", p)

			if err := save(ctx, client, p); err != nil {
				log.Println("Error painting:", err)
				break
			}
		case "pixelinfo":
			var p Point
			if err := json.Unmarshal([]byte(op.Data), &p); err != nil {
				log.Println("Error unmarshalling JSON:", err)
				break
			}
			log.Printf("Received pixel info request: %+v", p)
			if err := getPixelInfo(ctx, client, conn, p); err != nil {
				log.Println("Error getting pixel info:", err)
				break
			}
		case "canvas":
			// get the canvas
			if err := getCanvas(ctx, conn, client); err != nil {
				log.Println("Error getting canvas:", err)
				break
			}
			log.Printf("Received canvas request")
		}
	}
}

func getPixelInfo(ctx context.Context, client dapr.Client, conn *websocket.Conn, p Point) error {
	key := fmt.Sprintf("c%d_%d", p.X, p.Y)
	item, err := client.GetState(ctx, "kvstore", key, nil)
	if err != nil {
		return err
	}
	if item.Value == nil {
		return nil
	}

	jsonData, err := json.Marshal(Operation{
		Type: "pixelinfo",
		Data: string(item.Value),
	})
	if err != nil {
		return err
	}

	if err := conn.WriteMessage(websocket.TextMessage, jsonData); err != nil {
		return err
	}

	return nil
}

func getCanvas(ctx context.Context, conn *websocket.Conn, client dapr.Client) error {
	image := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQAAAAA3bvkkAAAACklEQVR4AWNgAAAAAgABc3UBGAAAAABJRU5ErkJggg=="
	jsonData, err := json.Marshal(Operation{
		Type: "canvas",
		Data: image,
	})
	if err != nil {
		return err
	}

	if err := conn.WriteMessage(websocket.TextMessage, jsonData); err != nil {
		return err
	}

	return nil
}

func save(ctx context.Context, client dapr.Client, r PlaceRequest) error {
	// build the place event
	var p Pixel
	p.Point = r.Point
	p.Userinfo = r.Userinfo

	jsonPixel, err := json.Marshal(p)
	if err != nil {
		return err
	}

	jsonData, err := json.Marshal(Operation{
		Type: "place",
		Data: string(jsonPixel),
	})
	if err != nil {
		return err
	}

	key := fmt.Sprintf("c%d_%d", p.X, p.Y)

	if err := client.SaveState(ctx, "kvstore", key, jsonPixel, nil); err != nil {
		panic(fmt.Errorf("error saving state: %w", err))
	}

	for _, c := range conns {
		if err := c.WriteMessage(websocket.TextMessage, jsonData); err != nil {
			log.Println("Error writing message to client:", err)
		}
	}

	return nil
}
