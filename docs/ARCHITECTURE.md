# ChatGoGo - Architecture Documentation

## 📋 Table of Contents
1. [System Overview](#system-overview)
2. [Concurrency Model](#concurrency-model)
3. [Data Flow](#data-flow)
4. [Configuration](#configuration)
5. [Component Details](#component-details)

---

## 1. System Overview

ChatGoGo is a horizontally-scalable anonymous chat backend that connects users via Telegram. The architecture leverages **Redis Pub/Sub** for inter-instance communication and **PostgreSQL** for persistent storage.

### High-Level Architecture

```
┌─────────────────┐
│  Telegram User  │
└────────┬────────┘
         │ Messages/Commands
         ▼
┌─────────────────────────────────────────────────────────┐
│               Telegram Bot API                          │
└────────┬────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────┐
│           BotService (internal/telegram)                │
│  • Receives updates from Telegram                       │
│  • Translates to internal ChatMessage                   │
│  • Manages Client lifecycle                             │
└────────┬────────────────────────────────────────────────┘
         │
         │ ChatMessage
         ▼
┌─────────────────────────────────────────────────────────┐
│         ManagerService (Hub) - internal/chathub         │
│  • Central message router                               │
│  • Manages active Client connections                    │
│  • Coordinates with MatcherService                      │
│  • Publishes messages to Redis Pub/Sub                  │
└─┬───────┬───────────────────────────────────────────────┘
  │       │
  │       └──────► MatcherService (Queue & Matching Logic)
  │
  ├──────► Storage Service (DB + Redis Operations)
  │
  └──────► Redis Pub/Sub (Inter-instance Communication)
           │
           ▼
     ┌─────────────────────┐
     │  Other Go Instances │ (Horizontal Scaling)
     └─────────────────────┘
           │
           ▼
     ┌─────────────────────┐
     │  Partner's Client   │
     └─────────────────────┘
```

### Component Responsibilities

| Component | Responsibility |
|-----------|----------------|
| **BotService** | Telegram update handler, client lifecycle management |
| **ManagerService** | Central hub for routing messages between clients |
| **MatcherService** | Matchmaking queue management and partner pairing |
| **Storage Service** | Abstraction over PostgreSQL + Redis operations |
| **Client Interface** | Defines contract for Telegram/WebSocket connections |

---

## 2. Concurrency Model

The system uses **channel-based communication** to avoid shared mutable state and race conditions. Each service runs in its own goroutine with a single `select` loop.

### Channel Architecture

```go
ManagerService {
    IncomingCh     chan models.ChatMessage    // Producer: BotService, Consumer: ManagerService
    MatchRequestCh chan models.SearchRequest  // Producer: ManagerService, Consumer: MatcherService
    RegisterCh     chan Client                // Producer: BotService, Consumer: ManagerService
    UnregisterCh   chan Client                // Producer: BotService, Consumer: ManagerService
}
```

### Communication Flow

```
┌──────────────┐
│  BotService  │──┬──► IncomingCh ────► ManagerService.Run()
└──────────────┘  │                      │
                  │                      ├──► Storage.SaveMessage()
                  │                      ├──► Storage.PublishMessage(Redis)
                  │                      │
                  └──► RegisterCh ───────┤
                                         │
                                         └──► MatchRequestCh ──► MatcherService.Run()
                                                                  │
                                                                  ├──► findMatch()
                                                                  └──► Storage.SaveRoom()
```

### Thread Safety Guarantees

1. **No Shared Mutable State**: The `Clients` map in `ManagerService` is **only accessed** from the hub's main goroutine.
2. **Buffered Channels**: All channels have buffers (typically 10) to prevent blocking on send operations.
3. **Client Isolation**: Each `Client` has its own `Send` channel, written to by the hub and read by `writePump`.

## 2.1 User Preferences

The system supports user-specific preferences to customize the chat experience.

### Default Media Spoiler
Users can toggle a default "spoiler" flag for all media (photos, videos, animations) they send.
- **Command**: `/spoiler_on` enables the flag.
- **Command**: `/spoiler_off` disables the flag.
- **Storage**: Persisted in the `users` table as `default_media_spoiler`.
- **Behavior**: When enabled, the bot automatically sets `HasSpoiler: true` for any media message sent by the user to their partner.


---

## 3. Data Flow

### 3.1 Message Lifecycle (User A → User B)

```
1. User A sends message to Telegram Bot
   ↓
2. BotService.handleIncomingMessage()
   - Creates ChatMessage struct
   - Sends to Hub.IncomingCh
   ↓
3. ManagerService.Run() processes IncomingCh
   - Calls Storage.SaveMessage() → PostgreSQL (chat_histories table)
   - Calls Storage.PublishMessage() → Redis Pub/Sub (channel: roomID)
   ↓
4. Redis Pub/Sub broadcasts to ALL Go instances
   ↓
5. StartPubSubListener() on ALL instances receives message
   - Finds Client with matching roomID
   - Sends to Client.Send channel
   ↓
6. Client.writePump() (in tg_client.go)
   - Receives from Send channel
   - Calls BotAPI.Send() → Telegram Bot API
   ↓
7. User B receives message in Telegram
```

### 3.2 Matchmaking Flow

```
1. User sends /start command
   ↓
2. BotService identifies command → ChatMessage.Type = "command_start"
   ↓
3. ManagerService.Run() detects command
   - Creates SearchRequest{UserID: anonID}
   - Sends to MatchRequestCh
   ↓
4. MatcherService.Run() receives SearchRequest
   - Adds user to Queue map
   - Calls findMatch() for all queued users
   ↓
5. findMatch() identifies compatible partner
   - Generates roomID (UUID)
   - Calls Storage.SaveRoom() → PostgreSQL
   - Calls client1.SetRoomID(roomID), client2.SetRoomID(roomID)
   - Removes both users from Queue
   - Sends "system_match_found" message to both clients
   ↓
6. Users receive "Match found!" notification
```

### 3.3 Database Schema (Conceptual)

**PostgreSQL Tables:**
- `users` → User profiles (ID, TelegramID, Age, Gender, Interests)
- `chat_rooms` → Active/ended rooms (RoomID, User1ID, User2ID, IsActive, StartedAt, EndedAt)
- `chat_histories` → Message logs (ID, RoomID, SenderID, Content, Type, TgMessageIDSender, TgMessageIDReceiver)
- `complaints` → User reports (ID, RoomID, ReporterID, Reason, Status)

**Redis Data Structures:**
- **Pub/Sub Channels**: Named by `roomID` for message broadcasting
- **Sets**: `search_queue` for matchmaking queue
- **Keys**: `ban:{anonID}` for ban status checks

---

## 4. Configuration

### Environment Variables

Create a `.env` file based on `.env.example`:

| Variable | Description | Example |
|----------|-------------|---------|
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | Database username | `chatgogo_user` |
| `DB_PASSWORD` | Database password | `secure_password` |
| `DB_NAME` | Database name | `chatgogodb` |
| `REDIS_ADDR` | Redis address | `localhost:6379` |
| `REDIS_PASSWORD` | Redis password (optional) | `` |
| `REDIS_DB` | Redis database index | `0` |
| `TELEGRAM_BOT_TOKEN` | Token from @BotFather | `123456:ABC-DEF...` |

### Loading Configuration

Configuration is loaded in `cmd/main.go` using `godotenv`:

```go
err := godotenv.Load()
if err != nil {
    log.Fatal("Error loading .env file")
}

token := os.Getenv("TELEGRAM_BOT_TOKEN")
dbHost := os.Getenv("DB_HOST")
// ... etc
```

---

## 5. Component Details

### 5.1 BotService (`internal/telegram/bot_service.go`)

**Purpose**: Bridge between Telegram Bot API and internal message system.

**Key Methods**:
- `NewBotService()` – Initializes Telegram Bot API client
- `Run()` – Main loop receiving updates from Telegram
- `handleIncomingMessage()` – Processes new messages/media
- `handleEditedMessage()` – Handles message edits
- `handleTextMessage()` – Command parsing (/start, /stop, /next, etc.)
- `getOrCreateClient()` – Client lifecycle management
- `RestoreActiveSessions()` – Reconnects users to active rooms on restart

**Handled Message Types**:
- Text, Photo, Video, Sticker, Voice, Animation, VideoNote
- Commands: `/start`, `/stop`, `/next`, `/settings`, `/report`

### 5.2 ManagerService (`internal/chathub/manager.go` + `pubsub.go`)

**Purpose**: Central message router and client manager.

**Key Methods**:
- `Run()` – Main event loop processing channels
- `StartPubSubListener()` – Goroutine listening to Redis Pub/Sub
- `RecoverActiveRooms()` – Restores state from DB on startup

**Channel Handlers**:
```go
select {
case client := <-m.RegisterCh:        // New client connected
case client := <-m.UnregisterCh:      // Client disconnected
case req := <-m.MatchRequestCh:       // Matchmaking request
case msg := <-m.IncomingCh:           // New message from client
}
```

**Message Type Routing**:
- `command_start/search` → MatchRequestCh
- `command_stop` → CloseRoom + notify partner
- `command_next` → CloseRoom + new MatchRequest
- `text/photo/video...` → SaveMessage + PublishMessage
- `command_report` → SaveComplaint

### 5.3 MatcherService (`internal/chathub/matcher.go`)

**Purpose**: Matchmaking queue and partner pairing logic.

**Key Data Structures**:
```go
type MatcherService struct {
    Hub     *ManagerService
    Storage storage.Storage
    Queue   map[string]models.SearchRequest  // UserID → SearchRequest
}
```

**Algorithm** (simplified):
```
1. Receive SearchRequest from MatchRequestCh
2. Add to Queue map
3. For each queued user:
   - Find compatible partner in Queue
   - If found:
     a) Create ChatRoom (UUID)
     b) Storage.SaveRoom()
     c) SetRoomID on both clients
     d) Remove from Queue
     e) Send "match_found" system message
```

**Future Enhancement**: The current code has a placeholder for filtering logic (`if true /* умова збігу */`). Production implementation should check:
- Gender preferences
- Age range
- Interests matching
- Ban status (`Storage.IsUserBanned`)

### 5.4 Storage Service (`internal/storage/storage.go`)

**Purpose**: Unified interface for PostgreSQL and Redis operations.

**Interface Methods** (excerpt):
```go
type Storage interface {
    // User operations
    SaveUser(user *User) error
    SaveUserIfNotExists(telegramID string) error
    IsUserBanned(anonID string) (bool, error)
    
    // Room operations
    SaveRoom(room *ChatRoom) error
    CloseRoom(roomID string) error
    GetActiveRoomIDs() ([]string, error)
    GetActiveRoomIDForUser(userID string) (string, error)
    GetRoomByID(roomID string) (*ChatRoom, error)
    
    // Message operations
    SaveMessage(msg *ChatMessage) error
    PublishMessage(roomID string, msg ChatMessage) error
    SaveTgMessageID(historyID uint, anonID string, tgMsgID int) error
    
    // Queue operations
    AddUserToSearchQueue(userID string) error
    RemoveUserFromSearchQueue(userID string) error
    GetSearchingUsers() ([]string, error)
    
    // Complaint operations
    SaveComplaint(complaint *Complaint) error
}
```

**Implementation Notes**:
- **GORM** for PostgreSQL operations
- **go-redis/v9** for Redis operations
- Uses `context.Background()` for Redis calls
- Transaction support via GORM hooks

### 5.5 Client Interface (`internal/chathub/client.go`)

**Purpose**: Abstract connection type (Telegram or WebSocket).

```go
type Client interface {
    GetAnonID() string
    GetRoomID() string
    SetRoomID(string)
    GetSendChannel() chan<- models.ChatMessage
    Run()   // Start read/write pumps
    Close() // Cleanup
}
```

**Implementations**:
- `telegram.Client` (`internal/telegram/tg_client.go`)
- `chathub.WSClient` (`internal/chathub/ws_client.go`)

---

## 6. Deployment Considerations

### Horizontal Scaling

The architecture supports multiple Go instances via Redis Pub/Sub:

```
┌───────────────┐     ┌───────────────┐     ┌───────────────┐
│ Go Instance 1 │────▶│  Redis Pub/Sub │◀────│ Go Instance 2 │
│ (User A conn) │     │  (roomID chan) │     │ (User B conn) │
└───────────────┘     └───────────────┘     └───────────────┘
```

- Each instance runs `ManagerService.Run()` with its own `Clients` map
- Messages published to Redis reach ALL instances
- Instances filter messages by `client.GetRoomID() == msg.Channel`

### Database Migrations

Migrations are stored in `migrations/` directory:
- `add_ended_at_column.sql` – Adds `ended_at` timestamp to `chat_rooms`

Run migrations via:
```bash
psql -U $DB_USER -d $DB_NAME -f migrations/001_init.sql
```

### Monitoring Recommendations

**Key Metrics to Track**:
- `ManagerService.Clients` map size (active connections)
- `MatcherService.Queue` size (users waiting for match)
- Redis Pub/Sub latency
- PostgreSQL connection pool utilization
- Goroutine count (`runtime.NumGoroutine()`)

**Logging**:
- All services use Go's `log` package
- Structured logging recommended for production (e.g., `zap`, `logrus`)
- Current format: `log.Printf("INFO: message %s", var)`

---

## 7. Security Considerations

### Current Implementation

1. **Anonymous IDs**: Users identified by Telegram Chat ID (stringified)
2. **No Authentication**: Telegram Bot API handles auth
3. **Ban System**: Redis-based (`ban:{anonID}` keys)
4. **Complaint System**: PostgreSQL `complaints` table

### Recommended Enhancements

- **Rate Limiting**: Use Redis to track message frequency per user
- **Content Moderation**: Integrate ML-based filtering for harmful content
- **End-to-End Encryption**: Not applicable (Telegram encrypts transport)
- **GDPR Compliance**: Implement user data export/deletion endpoints

---

## 8. Testing Strategy

### Unit Tests

- **Service Layer**: Mock `Storage` interface using `testify/mock`
- **Matcher Logic**: Test queueing, matching, filtering
- **Model Hooks**: Test GORM `BeforeCreate` for UUID generation

### Integration Tests

- **Postgres**: Use Docker container with test fixtures
- **Redis**: Use `miniredis` for in-memory Redis mock
- **End-to-End**: Simulate Telegram updates with `httptest`

---

## 9. Future Enhancements

### Planned Features

1. **WebSocket Support**: Direct web client connections (partially implemented)
2. **Gender/Age Filtering**: Enhance `MatcherService.findMatch()` logic
3. **Voice/Video Chat**: Integration with WebRTC
4. **Admin Dashboard**: Web UI for moderation and analytics
5. **Multi-Language Support**: i18n for system messages
6. **User Profiles**: Extended user data (bio, photos)

### Technical Debt

- **Error Handling**: More granular error types (currently uses basic errors)
- **Context Propagation**: Use `context.Context` throughout (currently only in Redis)
- **Graceful Shutdown**: Implement signal handling to close rooms on shutdown
- **Metrics**: Add Prometheus metrics exporter

---

## 10. Glossary

| Term | Definition |
|------|------------|
| **AnonID** | Anonymous user identifier (Telegram chat ID as string) |
| **RoomID** | UUID identifying a chat session between two users |
| **Hub** | Synonym for `ManagerService` |
| **Matcher** | Synonym for `MatcherService` |
| **Client** | Interface representing a connection (Telegram or WebSocket) |
| **SearchRequest** | Matchmaking request sent when user initiates `/start` |
| **ChatMessage** | Internal message struct used throughout the system |
| **Pub/Sub** | Redis publish/subscribe pattern for inter-instance communication |

---

**Document Version**: 1.0  
**Last Updated**: 2025-11-30  
**Maintained By**: ChatGoGo Development Team
