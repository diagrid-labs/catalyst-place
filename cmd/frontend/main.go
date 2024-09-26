package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	daprsdk "github.com/dapr/go-sdk/client"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/spf13/viper"

	"github.com/lrascao/place/cmd/frontend/client"
	"github.com/lrascao/place/cmd/frontend/config"
	"github.com/lrascao/place/cmd/frontend/subscriber"
	"github.com/lrascao/place/pkg/pixel"
)

var (
	//go:embed static/*
	staticFiles embed.FS

	mu      sync.Mutex
	clients []client.Client
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var configVar string
	flag.StringVar(&configVar, "config", "config.yaml", "config")
	flag.Parse()
	log.Printf("config file: %s\n", configVar)

	// set the config without the extension
	viper.SetConfigName(strings.TrimSuffix(filepath.Base(configVar), filepath.Ext(configVar)))
	viper.SetConfigType(filepath.Ext(configVar)[1:])
	viper.AddConfigPath(filepath.Dir(configVar))

	viper.AutomaticEnv()
	viper.BindEnv("diagrid_api_key")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			log.Fatalf("config file not found: %s\n", configVar)
		} else {
			// Config file was found but another error was produced
			log.Fatalf("error reading config file: %v", err)
		}
	}

	var cfg config.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("error unmarshaling configuration: %v", err)
	}

	log.Printf("%+v\n", cfg)

	// create a connection to the Dapr runtime, it will be the same for all the requests
	dapr, err := daprsdk.NewClient()
	if err != nil {
		panic(fmt.Errorf("error creating connection to catalyst: %w", err))
	}
	defer dapr.Close()

	r := mux.NewRouter()
	// serve the index.html file that contains the whole FE
	r.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
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
		})

	// handle a websocket client
	r.HandleFunc("/ws",
		func(w http.ResponseWriter, r *http.Request) {
			// upgrade the HTTP connection to a WebSocket connection
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Println(err)
				return
			}
			defer conn.Close()

			// add this new client to our local list
			c := client.New(conn, dapr, cfg)
			mu.Lock()
			clients = append(clients, c)
			mu.Unlock()
			log.Printf("client connected from %s (total: %d)",
				conn.RemoteAddr(), len(clients))

			// handle all requests coming in from this client
			c.Handle(r.Context())
		})

	// start the subscriber that will handle external events
	if err := subscriber.Start(ctx, dapr, &cfg,
		func(p pixel.Pixel) error {
			data, err := p.Marshal()
			if err != nil {
				return fmt.Errorf("error marshaling pixel: %w", err)
			}

			// broadcast event to all websocket clients in this replica
			mu.Lock()
			defer mu.Unlock()
			log.Printf("broadcasting pixel %s to %d clients",
				p, len(clients))

			for _, c := range clients {
				if c.Send(client.Event{
					Type: client.EventTypePut,
					Data: string(data),
				}); err != nil {
					log.Printf("error sending pixel to client: %v", err)
				}
			}

			return nil
		}); err != nil {
		log.Fatalf("error starting grpc server: %v", err)
	}

	log.Printf("Starting server at http://localhost:%d\n", cfg.Port)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), r); err != nil {
		log.Fatal(err)
	}
}
