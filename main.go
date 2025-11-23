package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"telegraf/config"
	"telegraf/server"
)

func main() {
	// Инициализация конфигурации
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	// Инициализация логгера
	logger := server.NewLogger(cfg.Environment)

	logger.Info("🚀 Starting P2P Messenger Server...")
	logger.Info("📍 Environment: %s", cfg.Environment)
	logger.Info("🔌 Host: %s, Port: %s", cfg.Server.Host, cfg.Server.Port)
	logger.Info("📊 Max connections: %d", cfg.Server.MaxConnections)

	// Создание сервера
	messengerServer := server.NewMessengerServer(cfg, logger)

	// Graceful shutdown
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	// Обработка сигналов
	go func() {
		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)
		<-sigchan
		logger.Info("Received shutdown signal, shutting down gracefully...")
		stop()
	}()

	// Запуск сервера
	if err := messengerServer.Start(ctx); err != nil {
		logger.Error("Failed to start server: %v", err)
		os.Exit(1)
	}
}
