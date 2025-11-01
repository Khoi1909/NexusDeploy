# NexusDeploy

> Nền tảng PaaS mini tích hợp AI Phân tích Lỗi

## 📋 Giới thiệu

NexusDeploy là một Platform-as-a-Service (PaaS) hoàn chỉnh với kiến trúc microservices. Khi người dùng `git push`, hệ thống sẽ tự động:

- **CI (Continuous Integration)**: Chạy tests trong môi trường Docker. Nếu có lỗi, dùng AI (LLM) để phân tích logs và đưa ra gợi ý sửa lỗi.
- **CD (Continuous Deployment)**: Nếu tests thành công, tự động build code thành Docker image và đẩy lên registry.
- **Hosting**: Tự động triển khai container mới từ image và cấu hình domain (với SSL) cho ứng dụng.

## 🛠️ Tech Stack

### Backend (Go Microservices)
- **Language**: Go 1.21+
- **Architecture**: Microservices (8 services)
- **API Gateway**: Gin/Fiber (HTTP Router)
- **Communication**: 
  - gRPC (Internal service-to-service)
  - REST API (External clients)
  - WebSocket (Real-time logs)
- **Database ORM**: GORM
- **Queue**: Redis + Asynq (Job queue)
- **Docker**: Docker SDK for Go
- **Git Operations**: go-git

### Frontend
- **Framework**: React 18
- **Build Tool**: Vite
- **Styling**: TailwindCSS
- **State Management**: Zustand/Redux
- **API Client**: Axios
- **WebSocket**: Native WebSocket API
- **Routing**: React Router

### Infrastructure
- **Database**: PostgreSQL 15+
- **Cache/Queue**: Redis 7+
- **Reverse Proxy**: Traefik 2.x
- **Container Registry**: Docker Hub
- **Containerization**: Docker, Docker Compose

### AI/LLM
- **Provider**: OpenAI / Anthropic
- **Use Case**: Error log analysis & suggestions

### DevOps
- **Development**: Docker Compose, Air (Go live reload)
- **Migrations**: sql-migrate
- **CI/CD**: GitHub Actions (planned)

## 🏗️ Kiến trúc Microservices

### Backend Services (8 services)

1. **API Gateway** (`:8000`) - Entry point, routing, authentication
2. **Auth Service** (`:8001`) - GitHub OAuth, JWT, user management
3. **Project Service** (`:8002`) - Repository management, webhooks
4. **Build Service** (`:8003`) - CI/CD orchestration
5. **Runner Service** (`:8004`) - Job execution workers (scalable)
6. **AI Service** (`:8005`) - LLM error analysis
7. **Deployment Service** (`:8006`) - Container & Traefik management
8. **Notification Service** (`:8007`) - WebSocket & Pub/Sub

### Databases

- **auth_db** - Users, tokens, permissions
- **project_db** - Projects, repositories, webhooks
- **build_db** - Builds, deployments, logs

## 📁 Cấu trúc Project

```
NexusDeploy/
├── backend/              # Go microservices
│   ├── pkg/             # Shared packages
│   ├── services/        # 8 microservices
│   └── migrations/      # Database migrations
├── frontend/            # React application
├── deployments/         # Docker & Traefik configs
├── scripts/             # Helper scripts
├── docs/                # Documentation
├── docker-compose.yml   # Development setup
└── Makefile            # Common commands
```

## 🚀 Prerequisites

- **Go**: 1.21 or higher
- **Node.js**: 18+ và npm/yarn
- **Docker**: 24+ và Docker Compose
- **Git**: 2.30+
- **Make**: GNU Make (optional, for Makefile commands)

## ⚙️ Setup & Development

### 1. Clone repository

```bash
git clone https://github.com/khoi1909/nexusdeploy.git
cd nexusdeploy
```

### 2. Environment setup

```bash
# Copy environment template
cp .env.example .env

# Edit .env và điền thông tin:
# - GitHub OAuth credentials
# - LLM API key
# - Database passwords
# - JWT secret
```

### 3. Start development

```bash
# Start all services
make dev

# Or using Docker Compose directly
docker-compose up -d
```

### 4. Access services

- **Frontend**: http://localhost:3000
- **API Gateway**: http://localhost:8000
- **Traefik Dashboard**: http://localhost:8080

## 📝 Development Commands

```bash
make dev        # Start all services
make down       # Stop all services
make build      # Build Docker images
make logs       # View logs
make proto      # Generate protobuf code
make migrate    # Run database migrations
make test       # Run tests
```

## 📖 Documentation

Chi tiết implementation và API documentation xem trong folder `/docs`.

## 🔐 GitHub OAuth Setup

1. Truy cập: https://github.com/settings/developers
2. Tạo New OAuth App
3. Điền thông tin:
   - Homepage URL: `http://localhost:3000`
   - Callback URL: `http://localhost:8000/auth/callback`
4. Copy Client ID và Client Secret vào `.env`

## 🤝 Contributing

1. Fork repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

## 📄 License

MIT License - xem file LICENSE để biết thêm chi tiết.

## 👥 Authors

- Tên của bạn - [@yourhandle](https://github.com/yourhandle)

## 🙏 Acknowledgments

- Inspired by Heroku, Vercel, và Railway
- Powered by Go, React, và open-source community
