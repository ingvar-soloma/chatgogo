# -------------------------------
# ⚙️ ChatGoGo Development Makefile
# -------------------------------

PROJECT_NAME := chatgogo
COMPOSE_FILE := docker-compose.dev.yml

# ==============================
# 🧱 Основні цілі
# ==============================

# 🚀 Запуск дев оточення з Air (live reload)
dev:
	@echo "🚀 Starting development environment..."
	docker compose -f $(COMPOSE_FILE) -p $(PROJECT_NAME) up --build

# 🧹 Зупинка та видалення контейнерів
down:
	@echo "🧹 Stopping containers..."
	docker compose -f $(COMPOSE_FILE) -p $(PROJECT_NAME) down

# 🔁 Перезбірка без кешу
rebuild:
	@echo "♻️ Rebuilding containers without cache..."
	docker compose -f $(COMPOSE_FILE) -p $(PROJECT_NAME) build --no-cache

# 📜 Логи (live tail)
logs:
	@echo "📜 Tailing logs..."
	docker compose -f $(COMPOSE_FILE) -p $(PROJECT_NAME) logs -f backend

# 🐚 Увійти в контейнер backend
sh:
	@echo "🐚 Entering backend container shell..."
	docker exec -it $(PROJECT_NAME)-backend-dev /bin/sh

# ==============================
# 🧰 Додаткові утиліти
# ==============================

# Видалити все (контейнери, volume-и)
reset:
	@echo "🧨 Removing all containers and volumes..."
	docker compose -f $(COMPOSE_FILE) -p $(PROJECT_NAME) down -v

# Очистити кеши Docker
clean:
	@echo "🧼 Cleaning Docker system cache..."
	docker system prune -af --volumes
