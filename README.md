<div align="center">

# 🚀 NexusDeploy

**Platform-as-a-Service (PaaS) with Automated CI/CD Pipeline**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.0+-3178C6?logo=typescript)](https://www.typescriptlang.org/)
[![Next.js](https://img.shields.io/badge/Next.js-14+-000000?logo=next.js)](https://nextjs.org/)

*Deploy your applications from GitHub to production in minutes*

[Overview](#-overview) • [Features](#-key-features) • [Architecture](#-architecture) • [Tech Stack](#-technology-stack) • [Status](#-project-status)

</div>

---

## 📖 Overview

**NexusDeploy** is a modern Platform-as-a-Service solution designed to simplify web application deployment for developers. The platform automates the entire CI/CD pipeline from source code to production hosting, eliminating the complexity of setting up build environments, deployment infrastructure, and domain management.

### What NexusDeploy Does

✅ **Connects** your GitHub repositories to a fully managed hosting environment  
✅ **Monitors** your code changes and automatically triggers builds and deployments  
✅ **Builds** your applications in isolated Docker environments  
✅ **Tests** your code automatically before deployment  
✅ **Deploys** successful builds to production with zero manual configuration  
✅ **Provides** public subdomains with SSL certificates automatically  

---

## ✨ Key Features

### 🔄 Automated CI/CD Pipeline

The platform monitors your connected GitHub repositories and automatically triggers builds and deployments whenever you push new code. Each project runs through a complete pipeline that includes building your application in an isolated Docker environment, running tests, and deploying successful builds to production.

**Key Benefits:**
- Zero-configuration setup
- Isolated build environments
- Automatic deployment on successful builds
- Real-time build logs and status updates

---

### 🔐 GitHub Integration

Authentication and repository management are handled seamlessly through GitHub OAuth. Users can securely connect their GitHub account and select which repositories they want to deploy. The platform manages webhooks and repository access automatically.

**Features:**
- Secure OAuth authentication
- Repository selection and management
- Automatic webhook configuration
- Support for both public and private repositories

---

### 🤖 Intelligent Error Analysis

When builds or tests fail, NexusDeploy offers an AI-powered error analysis feature that examines failure logs and provides actionable suggestions for fixing issues. This helps developers quickly understand and resolve problems without spending time debugging.

**Capabilities:**
- Automatic log analysis
- Actionable error suggestions
- Context-aware recommendations
- Quick problem resolution

---

### 🌐 Automatic Hosting and Domain Management

Every successfully deployed application receives a public subdomain with valid SSL certificates. The platform handles all aspects of domain routing, SSL certificate provisioning, and container orchestration, so your applications are accessible via HTTPS immediately after deployment.

**Included:**
- Automatic subdomain assignment
- SSL certificate provisioning
- HTTPS enabled by default
- Custom domain support (coming soon)

---

### 🔒 Environment Variables and Secrets Management

The platform provides a secure way to manage environment variables and secrets for your applications. Sensitive information like API keys and database credentials are encrypted and stored securely, accessible only to your deployed containers.

**Security Features:**
- Encrypted storage
- Secure injection into containers
- Granular access control
- Audit logging

---

### 📊 Flexible Resource Management

NexusDeploy offers different subscription plans with varying resource limits. Users can manage their projects within the constraints of their plan, with options to upgrade for higher limits on concurrent builds, maximum projects, and resource allocation.

**Plan Features:**
- Free tier with basic limits
- Standard plan for growing teams
- Premium plan for production workloads
- Easy plan upgrades and downgrades

---

### 📈 Real-time Monitoring

Build logs and deployment status are streamed in real-time through web interfaces. Users can monitor the progress of builds, view detailed logs, and track the status of their deployments from a centralized dashboard.

**Dashboard Features:**
- Real-time build logs
- Deployment status tracking
- Project overview and statistics
- Activity timeline

---

## 🏗️ Architecture

NexusDeploy is built as a **microservices architecture** using Go, with each service handling a specific domain of functionality. Services communicate through gRPC for efficient inter-service communication. The frontend is built with Next.js, providing a modern and responsive user interface.

### System Components

| Component | Description |
|-----------|-------------|
| **API Gateway** | Single entry point for all client requests, handles routing and authentication |
| **Auth Service** | Manages user authentication, authorization, and plan management |
| **Project Service** | Handles project CRUD operations and GitHub repository integration |
| **Build Service** | Manages build lifecycle, status tracking, and build queuing |
| **Deployment Service** | Handles application deployment, container orchestration, and lifecycle management |
| **Runner Service** | Executes builds in isolated Docker environments |
| **Notification Service** | Manages real-time notifications via WebSocket connections |
| **AI Service** | Provides intelligent error analysis and suggestions |

### Infrastructure

The platform leverages **Docker** for containerization, ensuring that each build and deployment runs in an isolated environment. A **reverse proxy** handles routing, SSL termination, and load balancing automatically.

---

## 🛠️ Technology Stack

### Backend
- **Language**: Go 1.21+
- **Communication**: gRPC for inter-service communication
- **Database**: PostgreSQL
- **Message Queue**: Redis with Asynq
- **Containerization**: Docker

### Frontend
- **Framework**: Next.js 14+ with App Router
- **Language**: TypeScript
- **Styling**: TailwindCSS
- **State Management**: Zustand

### Infrastructure
- **Reverse Proxy**: Traefik
- **Container Runtime**: Docker
- **Authentication**: GitHub OAuth 2.0

---

## 📊 Project Status

### ✅ Completed Features

- User authentication via GitHub OAuth
- Project management and GitHub repository integration
- Automated CI/CD pipeline (build, test, deploy)
- Real-time build logs and deployment status
- Environment variables and secrets management
- Resource limits based on subscription plans
- Plan upgrade and downgrade functionality

### 🚧 In Progress

- AI-powered error analysis enhancements
- Custom domain configuration
- Advanced monitoring and analytics
- Performance optimizations

### 📋 Planned

- Database-as-a-Service integration
- Multi-region deployment support
- Advanced caching strategies
- Comprehensive documentation

---

## 📄 License

This project is licensed under the **MIT License** - see the [LICENSE](LICENSE) file for details.

---

<div align="center">

**Built with ❤️ for developers who want to focus on code, not infrastructure**

[Report Bug](https://github.com/Khoi1909/NexusDeploy/issues) • [Request Feature](https://github.com/Khoi1909/NexusDeploy/issues) • [View Documentation](./docs/)

</div>
