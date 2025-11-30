package chathub_test

import (
	"chatgogo/backend/internal/chathub"
	"chatgogo/backend/internal/models"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestManager_Run(t *testing.T) {
	storageMock := new(MockStorage)
	hub := chathub.NewManagerService(storageMock)
	storageMock.On("GetActiveRoomIDs").Return([]string{}, nil)
	storageMock.On("SubscribeToAllRooms").Return(&redis.PubSub{})

	clientA := newMockClient("user_A")

	go hub.Run()

	hub.RegisterCh <- clientA
	time.Sleep(100 * time.Millisecond)
	assert.Contains(t, hub.Clients, "user_A")

	hub.UnregisterCh <- clientA
	time.Sleep(100 * time.Millisecond)
	assert.NotContains(t, hub.Clients, "user_A")
}

func TestManager_handleIncomingMessage(t *testing.T) {
	storageMock := new(MockStorage)
	hub := chathub.NewManagerService(storageMock)
	storageMock.On("GetActiveRoomIDs").Return([]string{}, nil)
	storageMock.On("SubscribeToAllRooms").Return(&redis.PubSub{})

	storageMock.On("SaveMessage", mock.AnythingOfType("*models.ChatMessage")).Return(nil)
	storageMock.On("PublishMessage", mock.AnythingOfType("string"), mock.AnythingOfType("models.ChatMessage")).Return(nil)

	go hub.Run()

	hub.IncomingCh <- models.ChatMessage{RoomID: "room1", SenderID: "user_A", Content: "hello"}
	time.Sleep(100 * time.Millisecond)

	storageMock.AssertCalled(t, "SaveMessage", mock.AnythingOfType("*models.ChatMessage"))
	storageMock.AssertCalled(t, "PublishMessage", "room1", mock.AnythingOfType("models.ChatMessage"))
}

func TestManager_handlePubSubMessage(t *testing.T) {
	storageMock := new(MockStorage)
	hub := chathub.NewManagerService(storageMock)
	storageMock.On("GetActiveRoomIDs").Return([]string{}, nil)
	storageMock.On("SubscribeToAllRooms").Return(&redis.PubSub{})

	clientB := newMockClient("user_B")
	hub.Clients["user_B"] = clientB

	room := &models.ChatRoom{RoomID: "room1", User1ID: "user_A", User2ID: "user_B"}
	storageMock.On("GetRoomByID", "room1").Return(room, nil)

	go hub.Run()

	hub.PubSubCh <- models.ChatMessage{RoomID: "room1", SenderID: "user_A", Content: "hello"}
	time.Sleep(100 * time.Millisecond)

	select {
	case msg := <-clientB.RecvChannel:
		assert.Equal(t, "hello", msg.Content)
	default:
		t.Error("clientB did not receive message")
	}
}

func TestManager_SetClientRestorer(t *testing.T) {
	hub := chathub.NewManagerService(nil)
	restorer := func(userID string) (chathub.Client, error) {
		return newMockClient(userID), nil
	}
	hub.SetClientRestorer(restorer)
	assert.NotNil(t, hub.ClientRestorer)
}
