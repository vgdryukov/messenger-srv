package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"telegraf/server"
)

func getConfig() (string, string, string) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = "8081" // Отдельный порт для HTTP health checks
	}

	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "production"
	}

	return port, httpPort, environment
}

// startHealthCheckServer запускает HTTP сервер для health checks от Render
func startHealthCheckServer(port string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" || r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	go func() {
		log.Printf("🌐 HTTP Health Check server starting on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("❌ HTTP server error: %v", err)
		}
	}()

	return server
}

func main() {
	port, httpPort, environment := getConfig()

	fmt.Printf("🚀 Starting P2P Messenger Server...\n")
	fmt.Printf("📍 Environment: %s\n", environment)
	fmt.Printf("🔌 TCP Port: %s\n", port)
	fmt.Printf("🌐 HTTP Port: %s\n", httpPort)

	host := "0.0.0.0" // Всегда слушаем все интерфейсы в production
	if environment == "development" {
		host = "localhost"
	}

	serverConfig := server.ServerConfig{
		Host: host,
		Port: port,
	}

	storageConfig := server.StorageConfig{
		UsersFile:    "users.dat",
		MessagesFile: "messages.dat",
		ContactsFile: "contacts.dat",
		GroupsFile:   "groups.dat",
	}

	messengerServer := server.NewMessengerServer(serverConfig, storageConfig)

	// Запускаем HTTP сервер для health checks
	healthServer := startHealthCheckServer(httpPort)

	log.Printf("✅ Server configured - Host: %s, TCP Port: %s, HTTP Port: %s", host, port, httpPort)

	// Создаем контекст для graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Запускаем основной TCP сервер
	go func() {
		if err := messengerServer.Start(ctx); err != nil {
			log.Printf("❌ TCP server error: %v", err)
		}
	}()

	// Ожидаем сигнал завершения
	<-ctx.Done()
	log.Println("🛑 Shutdown signal received")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Останавливаем HTTP сервер
	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("❌ HTTP server shutdown error: %v", err)
	} else {
		log.Println("✅ HTTP server stopped gracefully")
	}

	// Даем время на завершение TCP соединений
	time.Sleep(2 * time.Second)
	log.Println("👋 Server stopped")
}
