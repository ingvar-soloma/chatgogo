# ChatGoGo - Anonymous Chat Backend

## About The Project

ChatGoGo is a backend service for an anonymous chat application, enabling users to connect and communicate without revealing their identities. Built with Go, it leverages a powerful stack including PostgreSQL for data persistence, Redis for caching and real-time messaging, and the Telegram Bot API for client communication. The architecture is designed to be scalable and maintainable, with a clear separation of concerns between different services.

## Go Version

This project is built using Go `1.21.3`.

## 🚀 Quick Start Guide

### Prerequisites

Before running ChatGoGo locally, ensure you have:

- **Go 1.21.3+** - [Download](https://go.dev/dl/)
- **Docker & Docker Compose** - [Install](https://docs.docker.com/get-docker/)
- **Telegram Bot Token** - Create a bot via [@BotFather](https://t.me/botfather)
- **Make** (optional) - For convenient commands

### Local Development Setup

#### 1️⃣ Clone the Repository

```bash
git clone https://github.com/ingvar-soloma/chatgogo.git
cd chatgogo
```

#### 2️⃣ Configure Environment Variables

Copy the example environment file and fill in your credentials:

```bash
cp .env.example .env
```

Edit `.env` with your configuration:

```env
# PostgreSQL
DB_HOST=localhost
DB_PORT=5432
DB_USER=chatgogo_user
DB_PASSWORD=your_secure_password
DB_NAME=chatgogodb

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# Telegram Bot
TELEGRAM_BOT_TOKEN=123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11
```

#### 3️⃣ Start Infrastructure Services

Use Docker Compose to spin up PostgreSQL and Redis:

```bash
docker-compose up -d postgres redis
```

Verify services are running:

```bash
docker-compose ps
```

Expected output:
```
NAME                COMMAND                  STATUS
chatgogo-postgres   "docker-entrypoint.s…"   Up
chatgogo-redis      "docker-entrypoint.s…"   Up
```

#### 4️⃣ Run Database Migrations

Apply schema migrations to create tables:

```bash
# Using psql
psql -h localhost -U chatgogo_user -d chatgogodb -f migrations/001_init.sql

# OR using a migration tool (if available)
make migrate
```

#### 5️⃣ Install Go Dependencies

```bash
go mod download
go mod verify
```

#### 6️⃣ Run the Application

**Option A: Direct Go Run**
```bash
go run cmd/main.go
```

**Option B: Using Make (if Makefile exists)**
```bash
make dev
```

**Option C: Build and Run Binary**
```bash
go build -o chatgogo cmd/main.go
./chatgogo
```

Expected console output:
```
✅ Authorized on account @YourBotName
Restoring active Telegram sessions...
Active Telegram sessions restored.
Chat Hub Manager started and listening to channels...
Matcher Service started.
[GIN-debug] Listening and serving HTTP on :8080
```

#### 7️⃣ Test the Bot

1. Open Telegram and search for your bot (`@YourBotName`)
2. Send `/start` to begin searching for a partner
3. Open a second Telegram account or use a friend's account
4. Send `/start` from the second account
5. Both users should receive "✅ Співрозмовника знайдено!" (Match found!)
6. Start chatting!

### 🧪 Running Tests

Execute the test suite:

```bash
# Run all tests
go test ./... -v

# Run tests with coverage
go test ./... -cover

# Run specific package tests
go test ./internal/chathub -v
go test ./internal/models -v

# Generate coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### 🛠️ Development Workflow

#### Project Structure

```
chatgogo/
├── cmd/
│   └── main.go              # Application entry point
├── internal/
│   ├── chathub/             # Core message routing
│   │   ├── manager.go       # ManagerService (Hub)
│   │   ├── matcher.go       # MatcherService (Matchmaking)
│   │   ├── pubsub.go        # Redis Pub/Sub integration
│   │   ├── client.go        # Client interface
│   │   ├── ws_client.go     # WebSocket client impl
│   │   ├── matcher_test.go  # Unit tests
│   │   └── mocks_test.go    # Test mocks
│   ├── telegram/            # Telegram Bot integration
│   │   ├── bot_service.go   # BotService
│   │   └── tg_client.go     # Telegram client impl
│   ├── models/              # Data models
│   │   ├── user.go
│   │   ├── room.go
│   │   ├── history.go
│   │   ├── complaint.go
│   │   └── user_test.go     # Model tests
│   ├── storage/             # Data access layer
│   │   └── storage.go
│   └── api/                 # HTTP API handlers
│       └── handler/
├── migrations/              # Database migrations
├── docs/                    # Documentation
│   ├── ARCHITECTURE.md      # Architecture documentation
│   └── LLM_CONTEXT_INDEX.yaml  # Codebase index for LLMs
├── .env.example             # Example environment config
├── docker-compose.yml       # Docker services config
├── Makefile                 # Build automation (optional)
└── README.md                # This file
```

#### Common Commands

```bash
# Format code
go fmt ./...

# Lint code (requires golangci-lint)
golangci-lint run

# Build for production
go build -ldflags="-s -w" -o chatgogo cmd/main.go

# Run with live reload (requires air)
air

# View logs
docker-compose logs -f

# Stop services
docker-compose down

# Clean up volumes
docker-compose down -v
```

### 🐛 Troubleshooting

#### Database Connection Issues

**Problem**: `pq: password authentication failed`  
**Solution**: Verify DB credentials in `.env` match `docker-compose.yml`

**Problem**: `dial tcp [::1]:5432: connect: connection refused`  
**Solution**: Ensure PostgreSQL container is running:
```bash
docker-compose up -d postgres
docker-compose ps  # Check status
```

#### Redis Connection Issues

**Problem**: `dial tcp [::1]:6379: connect: connection refused`  
**Solution**: Start Redis container:
```bash
docker-compose up -d redis
```

#### Telegram Bot Issues

**Problem**: Bot not responding to messages  
**Solution**: 
1. Verify `TELEGRAM_BOT_TOKEN` in `.env`
2. Check bot is not paused in BotFather
3. Ensure bot has "Privacy Mode" disabled (BotFather → /mybots → @YourBot → Bot Settings → Privacy Mode)

**Problem**: `invalid auth_token`  
**Solution**: Regenerate token via BotFather:
```
/mybots → @YourBot → API Token → Revoke & Regenerate
```

#### Application Crashes

**Problem**: `panic: runtime error: invalid memory address`  
**Solution**: Check logs for nil pointer issues, ensure all required services are running

**Problem**: `too many open files`  
**Solution**: Increase file descriptor limit:
```bash
ulimit -n 4096
```

### 📚 Additional Resources

- **Architecture Documentation**: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)
- **LLM Context Index**: [`docs/LLM_CONTEXT_INDEX.yaml`](docs/LLM_CONTEXT_INDEX.yaml)
- **API Documentation**: (TODO: Add Swagger/OpenAPI docs)

### 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

### 🙏 Acknowledgments

- [Telegram Bot API](https://core.telegram.org/bots/api)
- [GORM](https://gorm.io/) - ORM library
- [go-redis](https://github.com/redis/go-redis) - Redis client
- [Gin](https://gin-gonic.com/) - HTTP framework

---

**Happy Coding! 🎉**
