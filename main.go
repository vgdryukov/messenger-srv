package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"telegraf/server"
)

func getConfig() (string, string, string) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		// В production используем тот же порт для HTTP
		httpPort = port
	}

	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "production" // По умолчанию production на Render
	}

	return port, httpPort, environment
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

	log.Printf("✅ Server configured - Host: %s, TCP Port: %s, HTTP Port: %s", host, port, httpPort)

	// Создаем контекст для graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := messengerServer.Start(ctx, httpPort); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
