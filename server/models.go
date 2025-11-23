package server

// ServerConfig - конфигурация сервера
type ServerConfig struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

// StorageConfig - конфигурация хранилища
type StorageConfig struct {
	UsersFile    string `json:"users_file"`
	MessagesFile string `json:"messages_file"`
	ContactsFile string `json:"contacts_file"`
	GroupsFile   string `json:"groups_file"`
}
