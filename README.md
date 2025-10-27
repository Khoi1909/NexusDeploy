Tên dự án: Nền tảng PaaS mini tích hợp AI Phân tích Lỗi
Mục đích:
Xây dựng một nền tảng "Platform-as-a-Service" hoàn chỉnh. Khi người dùng git push, hệ thống sẽ tự động:

CI (Tích hợp): Chạy test trong môi trường Docker. Nếu lỗi, dùng AI (LLM) để phân tích log.

CD (Triển khai): Nếu test thành công, tự động build code thành Docker image, đẩy lên registry.

Host (Lưu trữ): Tự động triển khai container mới từ image đó và trỏ tên miền (với SSL) vào ứng dụng đang chạy.

Tech Stack:
Frontend: React

Backend & Runner: Python (FastAPI, Docker SDK)

Queue & Real-time: Redis, WebSocket

Reverse Proxy: Traefik

Registry: Docker Hub

AI: API của LLM
