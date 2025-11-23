package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"sync"
	"telegraf/config"
	"telegraf/shared"
	"time"
)

type MessengerServer struct {
	storage     Storage
	config      *config.Config
	logger      *Logger
	validator   *Validator
	rateLimiter *RateLimiter

	onlineUsers map[string]net.Conn
	mutex       sync.RWMutex

	messageBatch *MessageBatch
}

type ServerConfig struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

func NewMessengerServer(cfg *config.Config, logger *Logger) *MessengerServer {
	storage := NewDataStorage(cfg.Storage)
	validator := NewValidator()
	rateLimiter := NewRateLimiter(
		cfg.RateLimit.MaxAttempts,
		cfg.RateLimit.WindowSize,
		cfg.RateLimit.MessageLimit,
		cfg.RateLimit.MessageWindow,
	)

	return &MessengerServer{
		storage:     storage,
		config:      cfg,
		logger:      logger,
		validator:   validator,
		rateLimiter: rateLimiter,
		onlineUsers: make(map[string]net.Conn),
	}
}

func (ms *MessengerServer) Start(ctx context.Context) error {
	// Загрузка данных
	if err := ms.storage.LoadAll(); err != nil {
		return fmt.Errorf("failed to load data: %v", err)
	}

	// Запуск фоновых задач
	go ms.cleanupWorker(ctx)

	address := fmt.Sprintf("%s:%s", ms.config.Server.Host, ms.config.Server.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to start listener on %s: %v", address, err)
	}
	defer listener.Close()

	ms.logger.Info("🚀 P2P Messenger Server started on %s", address)
	ms.logger.Info("📍 Host: %s, Port: %s", ms.config.Server.Host, ms.config.Server.Port)

	// Обработка входящих соединений
	go ms.acceptConnections(ctx, listener)

	// Ожидание сигнала завершения
	<-ctx.Done()
	ms.logger.Info("Shutting down server...")

	// Graceful shutdown - даем время на завершение операций
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ms.performCleanup(shutdownCtx)

	return nil
}

func (ms *MessengerServer) acceptConnections(ctx context.Context, listener net.Listener) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn, err := listener.Accept()
			if err != nil {
				// Проверяем, не был ли listener закрыт
				if ctx.Err() != nil {
					return
				}
				ms.logger.Error("Accept error: %v", err)
				continue
			}
			go ms.handleConnection(conn)
		}
	}
}

func (ms *MessengerServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	ms.logger.Info("🔗 New connection from %s", remoteAddr)

	// Установка таймаутов
	conn.SetReadDeadline(time.Now().Add(ms.config.Server.ReadTimeout))
	conn.SetWriteDeadline(time.Now().Add(ms.config.Server.WriteTimeout))

	decoder := json.NewDecoder(conn)

	for {
		var request shared.Request
		if err := decoder.Decode(&request); err != nil {
			if err != io.EOF {
				ms.logger.Error("JSON decode error from %s: %v", remoteAddr, err)
			} else {
				ms.logger.Info("📤 Connection closed by %s", remoteAddr)
			}

			// Удаляем пользователя из онлайн списка при отключении
			ms.mutex.Lock()
			for username, userConn := range ms.onlineUsers {
				if userConn == conn {
					delete(ms.onlineUsers, username)
					ms.logger.Info("👤 User %s went offline", username)
					break
				}
			}
			ms.mutex.Unlock()
			return
		}

		// Обновляем таймаут
		conn.SetReadDeadline(time.Now().Add(ms.config.Server.ReadTimeout))
		conn.SetWriteDeadline(time.Now().Add(ms.config.Server.WriteTimeout))

		ms.logger.Debug("📨 Request from %s: %s (user: %s)", remoteAddr, request.Action, request.Username)

		response := ms.handleRequest(request, conn)
		responseData, _ := json.Marshal(response)

		if _, err := conn.Write(responseData); err != nil {
			ms.logger.Error("Write error to %s: %v", remoteAddr, err)
			return
		}

		ms.logger.Debug("📤 Response sent to %s: %s", remoteAddr, response.Status)
	}
}

func (ms *MessengerServer) handleRequest(request shared.Request, conn net.Conn) shared.Response {
	switch request.Action {
	case "register":
		return ms.handleRegister(request)
	case "login":
		return ms.handleLogin(request, conn)
	case "recover":
		return ms.handleRecover(request)
	case "add_contact":
		return ms.handleAddContact(request)
	case "create_group":
		return ms.handleCreateGroup(request)
	case "send_message":
		return ms.handleSendMessage(request)
	case "get_messages":
		return ms.handleGetMessages(request)
	case "get_contacts":
		return ms.handleGetContacts(request)
	default:
		ms.logger.Warn("Unknown action from %s: %s", request.Username, request.Action)
		return shared.Response{Status: "error", Message: "Unknown action: " + request.Action}
	}
}

func (ms *MessengerServer) handleRegister(request shared.Request) shared.Response {
	if request.Username == "" || request.Password == "" {
		return shared.Response{Status: "error", Message: "Username and password are required"}
	}

	// Валидация
	if !ms.validator.ValidateUsername(request.Username) {
		return shared.Response{Status: "error", Message: "Username must be 3-16 characters and contain only letters, numbers, underscores and hyphens"}
	}

	if !ms.validator.ValidatePassword(request.Password) {
		return shared.Response{Status: "error", Message: "Password must be 6-32 characters"}
	}

	if !ms.validator.ValidateEmail(request.Email) {
		return shared.Response{Status: "error", Message: "Invalid email format"}
	}

	// Проверка существования пользователя
	if _, exists := ms.storage.GetUser(request.Username); exists {
		return shared.Response{Status: "error", Message: "Username already exists"}
	}

	// Создание пользователя
	passwordHash := fmt.Sprintf("%x", sha256.Sum256([]byte(request.Password)))
	user := &shared.User{
		Username:  request.Username,
		Password:  passwordHash,
		Email:     request.Email,
		CreatedAt: time.Now(),
	}

	ms.storage.AddUser(user)

	if err := ms.storage.SaveUsers(); err != nil {
		ms.logger.Error("Failed to save users: %v", err)
		return shared.Response{Status: "error", Message: "Failed to save user data"}
	}

	ms.logger.Info("✅ New user registered: %s", request.Username)
	return shared.Response{Status: "success", Message: "Registration successful"}
}

func (ms *MessengerServer) handleLogin(request shared.Request, conn net.Conn) shared.Response {
	if request.Username == "" || request.Password == "" {
		return shared.Response{Status: "error", Message: "Username and password are required"}
	}

	// Rate limiting
	if !ms.rateLimiter.AllowLogin(request.Username) {
		ms.logger.Warn("Rate limit exceeded for user: %s", request.Username)
		return shared.Response{Status: "error", Message: "Too many login attempts. Please try again later."}
	}

	user, exists := ms.storage.GetUser(request.Username)
	if !exists {
		ms.logger.Warn("Login failed - user not found: %s", request.Username)
		return shared.Response{Status: "error", Message: "Invalid credentials"}
	}

	passwordHash := fmt.Sprintf("%x", sha256.Sum256([]byte(request.Password)))
	if user.Password != passwordHash {
		ms.logger.Warn("Login failed - invalid password for: %s", request.Username)
		return shared.Response{Status: "error", Message: "Invalid credentials"}
	}

	// Обновляем время последнего входа
	user.LastLoginAt = time.Now()

	ms.mutex.Lock()
	ms.onlineUsers[request.Username] = conn
	ms.mutex.Unlock()

	ms.logger.Info("✅ User logged in: %s from %s", request.Username, conn.RemoteAddr())
	return shared.Response{Status: "success", Message: "Login successful"}
}

func (ms *MessengerServer) handleRecover(request shared.Request) shared.Response {
	if request.Username == "" || request.Email == "" {
		return shared.Response{Status: "error", Message: "Username and email are required"}
	}

	user, exists := ms.storage.GetUser(request.Username)
	if !exists {
		// Не раскрываем информацию о существовании пользователя
		ms.logger.Info("Password recovery attempted for non-existent user: %s", request.Username)
		return shared.Response{Status: "success", Message: "If the user exists, a recovery email has been sent"}
	}

	if user.Email != request.Email {
		ms.logger.Warn("Password recovery email mismatch for user: %s", request.Username)
		return shared.Response{Status: "success", Message: "If the user exists, a recovery email has been sent"}
	}

	// Генерация временного пароля
	tempPassword := generateTempPassword()
	user.Password = fmt.Sprintf("%x", sha256.Sum256([]byte(tempPassword)))

	if err := ms.storage.SaveUsers(); err != nil {
		ms.logger.Error("Failed to save temporary password: %v", err)
		return shared.Response{Status: "error", Message: "Failed to process recovery request"}
	}

	// В реальном приложении здесь была бы отправка email
	ms.logger.Info("🔐 Password recovery for user: %s, temp password: %s", request.Username, tempPassword)

	return shared.Response{
		Status:  "success",
		Message: "Temporary password has been sent to your email",
	}
}

func (ms *MessengerServer) handleAddContact(request shared.Request) shared.Response {
	if request.Username == "" || request.Contact == "" {
		return shared.Response{Status: "error", Message: "Username and contact are required"}
	}

	if request.Username == request.Contact {
		return shared.Response{Status: "error", Message: "Cannot add yourself as contact"}
	}

	if !ms.validator.ValidateUsername(request.Contact) {
		return shared.Response{Status: "error", Message: "Invalid contact username"}
	}

	if _, exists := ms.storage.GetUser(request.Contact); !exists {
		return shared.Response{Status: "error", Message: "Contact user not found"}
	}

	if err := ms.storage.AddContact(request.Username, request.Contact); err != nil {
		return shared.Response{Status: "error", Message: err.Error()}
	}

	if err := ms.storage.SaveContacts(); err != nil {
		ms.logger.Error("Failed to save contacts: %v", err)
		return shared.Response{Status: "error", Message: "Failed to save contacts"}
	}

	ms.logger.Info("✅ Contact added: %s -> %s", request.Username, request.Contact)
	return shared.Response{Status: "success", Message: "Contact added successfully"}
}

func (ms *MessengerServer) handleCreateGroup(request shared.Request) shared.Response {
	if request.Username == "" || request.Name == "" {
		return shared.Response{Status: "error", Message: "Group name and owner are required"}
	}

	if !ms.validator.ValidateGroupName(request.Name) {
		return shared.Response{Status: "error", Message: "Group name must be 1-32 characters"}
	}

	groupID := fmt.Sprintf("group_%d", time.Now().UnixNano())
	group := &shared.Group{
		ID:        groupID,
		Name:      request.Name,
		Owner:     request.Username,
		Members:   append([]string{request.Username}, request.Members...),
		CreatedAt: time.Now(),
	}

	// Проверяем существование всех участников
	for _, member := range request.Members {
		if !ms.validator.ValidateUsername(member) {
			return shared.Response{Status: "error", Message: "Invalid username: " + member}
		}
		if _, exists := ms.storage.GetUser(member); !exists {
			return shared.Response{Status: "error", Message: "User " + member + " not found"}
		}
	}

	ms.storage.AddGroup(group)

	if err := ms.storage.SaveGroups(); err != nil {
		ms.logger.Error("Failed to save groups: %v", err)
		return shared.Response{Status: "error", Message: "Failed to save group"}
	}

	ms.logger.Info("✅ Group created: %s (ID: %s) by %s", request.Name, groupID, request.Username)
	return shared.Response{
		Status:  "success",
		Message: "Group created successfully",
		Data:    map[string]string{"group_id": groupID},
	}
}

func (ms *MessengerServer) handleSendMessage(request shared.Request) shared.Response {
	if request.Username == "" || request.Content == "" {
		return shared.Response{Status: "error", Message: "Username and content are required"}
	}

	// Rate limiting для сообщений
	if !ms.rateLimiter.AllowMessage(request.Username) {
		return shared.Response{Status: "error", Message: "Message rate limit exceeded. Please slow down."}
	}

	if !ms.validator.ValidateMessage(request.Content) {
		return shared.Response{Status: "error", Message: "Message must be 1-1000 characters"}
	}

	baseTime := time.Now().UnixNano()

	if request.IsGroup {
		if request.GroupID == "" {
			return shared.Response{Status: "error", Message: "Group ID is required for group messages"}
		}

		group, exists := ms.storage.GetGroup(request.GroupID)
		if !exists {
			return shared.Response{Status: "error", Message: "Group not found"}
		}

		// Проверяем что пользователь состоит в группе
		isMember := false
		for _, member := range group.Members {
			if member == request.Username {
				isMember = true
				break
			}
		}
		if !isMember {
			return shared.Response{Status: "error", Message: "You are not a member of this group"}
		}

		// Создаем отдельное сообщение для каждого участника с уникальным ID
		for i, member := range group.Members {
			if member != request.Username {
				msg := shared.Message{
					ID:        baseTime + int64(i), // Уникальный ID для каждого сообщения
					From:      request.Username,
					To:        member,
					Content:   request.Content,
					SentAt:    time.Now(),
					IsGroup:   true,
					GroupID:   request.GroupID,
					Delivered: false,
				}
				ms.storage.AddMessage(msg)
			}
		}
	} else {
		if request.To == "" {
			return shared.Response{Status: "error", Message: "Recipient is required for private messages"}
		}

		if !ms.validator.ValidateUsername(request.To) {
			return shared.Response{Status: "error", Message: "Invalid recipient username"}
		}

		msg := shared.Message{
			ID:        baseTime,
			From:      request.Username,
			To:        request.To,
			Content:   request.Content,
			SentAt:    time.Now(),
			IsGroup:   false,
			Delivered: false,
		}
		ms.storage.AddMessage(msg)
	}

	ms.logger.Info("✅ Message sent: %s -> %s (group: %v)", request.Username, request.To, request.IsGroup)
	return shared.Response{Status: "success", Message: "Message sent successfully"}
}

func (ms *MessengerServer) handleGetMessages(request shared.Request) shared.Response {
	if request.Username == "" {
		return shared.Response{Status: "error", Message: "Username is required"}
	}

	messages := ms.storage.GetMessages(request.Username)

	if len(messages) > 0 {
		go ms.storage.SaveMessages()
	}

	ms.logger.Debug("📨 Retrieved %d messages for user: %s", len(messages), request.Username)
	return shared.Response{
		Status: "success",
		Data:   messages,
	}
}

func (ms *MessengerServer) handleGetContacts(request shared.Request) shared.Response {
	if request.Username == "" {
		return shared.Response{Status: "error", Message: "Username is required"}
	}

	contacts := ms.storage.GetContacts(request.Username)
	ms.logger.Debug("👥 Retrieved %d contacts for user: %s", len(contacts), request.Username)
	return shared.Response{
		Status: "success",
		Data:   contacts,
	}
}

// Вспомогательные методы

func generateTempPassword() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, 8)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

func (ms *MessengerServer) cleanupWorker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ms.rateLimiter.Cleanup()
			ms.logger.Debug("Rate limiter cleanup completed")
		}
	}
}

func (ms *MessengerServer) performCleanup(ctx context.Context) {
	// Сохраняем все данные перед завершением
	ms.logger.Info("Performing final cleanup...")

	if err := ms.storage.SaveUsers(); err != nil {
		ms.logger.Error("Error saving users during shutdown: %v", err)
	}

	if err := ms.storage.SaveMessages(); err != nil {
		ms.logger.Error("Error saving messages during shutdown: %v", err)
	}

	if err := ms.storage.SaveContacts(); err != nil {
		ms.logger.Error("Error saving contacts during shutdown: %v", err)
	}

	if err := ms.storage.SaveGroups(); err != nil {
		ms.logger.Error("Error saving groups during shutdown: %v", err)
	}

	ms.logger.Info("Cleanup completed")
}
