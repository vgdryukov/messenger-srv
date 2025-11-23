package server

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"telegraf/shared"
)

// Storage интерфейс
type Storage interface {
	GetUser(username string) (*shared.User, bool)
	AddUser(user *shared.User)
	SaveUsers() error
	LoadAll() error
	GetMessages(username string) []shared.Message
	AddMessage(msg shared.Message)
	SaveMessages() error
	GetContacts(username string) []string
	AddContact(username, contact string) error
	SaveContacts() error
	GetGroup(groupID string) (*shared.Group, bool)
	AddGroup(group *shared.Group)
	SaveGroups() error
}

type DataStorage struct {
	config   StorageConfig
	mutex    sync.RWMutex
	users    map[string]*shared.User
	contacts map[string][]string
	groups   map[string]*shared.Group
	messages []shared.Message
}

func NewDataStorage(cfg StorageConfig) *DataStorage {
	return &DataStorage{
		config:   cfg,
		users:    make(map[string]*shared.User),
		contacts: make(map[string][]string),
		groups:   make(map[string]*shared.Group),
		messages: make([]shared.Message, 0),
	}
}

func (ds *DataStorage) LoadAll() error {
	if err := ds.loadUsers(); err != nil {
		return fmt.Errorf("failed to load users: %v", err)
	}
	if err := ds.loadMessages(); err != nil {
		return fmt.Errorf("failed to load messages: %v", err)
	}
	if err := ds.loadContacts(); err != nil {
		return fmt.Errorf("failed to load contacts: %v", err)
	}
	if err := ds.loadGroups(); err != nil {
		return fmt.Errorf("failed to load groups: %v", err)
	}
	return nil
}

func (ds *DataStorage) GetUser(username string) (*shared.User, bool) {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()
	user, exists := ds.users[username]
	return user, exists
}

func (ds *DataStorage) AddUser(user *shared.User) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	ds.users[user.Username] = user
}

func (ds *DataStorage) SaveUsers() error {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	return ds.saveUsersDirect()
}

func (ds *DataStorage) saveUsersDirect() error {
	file, err := os.Create(ds.config.UsersFile)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	return encoder.Encode(ds.users)
}

func (ds *DataStorage) loadUsers() error {
	file, err := os.Open(ds.config.UsersFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	return decoder.Decode(&ds.users)
}

func (ds *DataStorage) GetMessages(username string) []shared.Message {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()
	var userMessages []shared.Message
	for _, msg := range ds.messages {
		if msg.To == username || msg.From == username {
			userMessages = append(userMessages, msg)
		}
	}
	return userMessages
}

func (ds *DataStorage) AddMessage(msg shared.Message) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	ds.messages = append(ds.messages, msg)
}

func (ds *DataStorage) SaveMessages() error {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	return ds.saveMessagesDirect()
}

func (ds *DataStorage) saveMessagesDirect() error {
	file, err := os.Create(ds.config.MessagesFile)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	return encoder.Encode(ds.messages)
}

func (ds *DataStorage) loadMessages() error {
	file, err := os.Open(ds.config.MessagesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	return decoder.Decode(&ds.messages)
}

func (ds *DataStorage) GetContacts(username string) []string {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()
	return ds.contacts[username]
}

func (ds *DataStorage) AddContact(username, contact string) error {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	for _, existing := range ds.contacts[username] {
		if existing == contact {
			return fmt.Errorf("contact already exists")
		}
	}
	ds.contacts[username] = append(ds.contacts[username], contact)
	return nil
}

func (ds *DataStorage) SaveContacts() error {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	return ds.saveContactsDirect()
}

func (ds *DataStorage) saveContactsDirect() error {
	file, err := os.Create(ds.config.ContactsFile)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	return encoder.Encode(ds.contacts)
}

func (ds *DataStorage) loadContacts() error {
	file, err := os.Open(ds.config.ContactsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	return decoder.Decode(&ds.contacts)
}

func (ds *DataStorage) GetGroup(groupID string) (*shared.Group, bool) {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()
	group, exists := ds.groups[groupID]
	return group, exists
}

func (ds *DataStorage) AddGroup(group *shared.Group) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	ds.groups[group.ID] = group
}

func (ds *DataStorage) SaveGroups() error {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	return ds.saveGroupsDirect()
}

func (ds *DataStorage) saveGroupsDirect() error {
	file, err := os.Create(ds.config.GroupsFile)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	return encoder.Encode(ds.groups)
}

func (ds *DataStorage) loadGroups() error {
	file, err := os.Open(ds.config.GroupsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	return decoder.Decode(&ds.groups)
}
