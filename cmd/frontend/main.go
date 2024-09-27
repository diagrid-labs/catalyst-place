package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

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

	// Set up signal handling for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	// Wait for a termination signal in a separate goroutine
	go func() {
		<-sigs
		log.Println("Received shutdown signal, starting graceful shutdown...")
		cancel()
	}()

	var configVar string
	flag.StringVar(&configVar, "config", "config.yaml", "config")
	flag.Parse()
	slog.Debug("config file: %s\n", configVar)

	// set the config without the extension
	viper.SetConfigName(strings.TrimSuffix(filepath.Base(configVar), filepath.Ext(configVar)))
	viper.SetConfigType(filepath.Ext(configVar)[1:])
	viper.AddConfigPath(filepath.Dir(configVar))

	viper.AutomaticEnv()
	viper.BindEnv("diagrid_api_key")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			slog.Error("config file not found: ", configVar)
			return
		} else {
			// Config file was found but another error was produced
			slog.Error("error reading config file: ", err)
			return
		}
	}

	var cfg config.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		slog.Error("error unmarshaling configuration: ", err)
		return
	}

	slog.Info("config: ", cfg)

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
				slog.Error("Error reading index.html:", err)
				return
			}

			// Serve the index.html content
			w.Header().Set("Content-Type", "text/html")
			if _, err := w.Write(indexHTML); err != nil {
				slog.Error("Error serving index.html:", err)
			}
		})

	// handle a websocket client
	r.HandleFunc("/ws",
		func(w http.ResponseWriter, r *http.Request) {
			// upgrade the HTTP connection to a WebSocket connection
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				slog.Error("Error upgrading websocket: ", err)
				return
			}
			defer conn.Close()

			// add this new client to our local list
			c := client.New(conn, dapr, cfg)
			mu.Lock()
			clients = append(clients, c)
			mu.Unlock()
			slog.Info("client connected", "remoteaddr", conn.RemoteAddr(), "total", len(clients))

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
			slog.Info("broadcasting pixel to clients", "pixel", p, "#clients", len(clients))

			for _, c := range clients {
				if c.Send(client.Event{
					Type: client.EventTypePut,
					Data: string(data),
				}); err != nil {
					slog.Error("error sending pixel to client: ", err)
				}
			}

			return nil
		}); err != nil {
		slog.Error("error starting grpc server: ", err)
		return
	}

	slog.Info(fmt.Sprintf("Starting server at http://localhost:%d", cfg.Port))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r,
	}

	// Start the server in a separate goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("error listening http: ", err)
			return
		}
	}()

	// Listen for context cancellation to shut down the server
	<-ctx.Done()
	slog.Info("Received shutdown signal, shutting down HTTP server...")

	// Create a context with a timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown failed", err)
		return
	}

	slog.Info("Server gracefully stopped")
}
