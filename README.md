# Shufflr - Self-Hosted Random Image API Service

[![Build and Publish Docker Image](https://github.com/YOUR_USERNAME/shufflr/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/YOUR_USERNAME/shufflr/actions/workflows/docker-publish.yml)
[![Docker Image](https://img.shields.io/badge/docker-ghcr.io%2FYOUR_USERNAME%2Fshufflr-blue)](https://github.com/YOUR_USERNAME/shufflr/pkgs/container/shufflr)

Shufflr is a lightweight, self-hosted random image API service built in Go. It provides a simple REST API for retrieving random images from your collection, along with a clean web interface for managing images and API keys.

## ✨ Features

- **Random Image API**: RESTful API that returns random images from your collection
- **Admin Web Interface**: Modern, responsive web UI built with Tailwind CSS and DaisyUI
- **Image Management**: Upload, rename, and delete images through the web interface
- **API Key Management**: Generate, disable, regenerate, and delete API keys
- **Authentication**: Secure session-based admin authentication
- **Usage Tracking**: Monitor API usage with request counts and metrics
- **Docker Ready**: Production-ready Docker image with multi-architecture support
- **Lightweight**: Built with Go's standard library, minimal dependencies
- **SQLite Database**: Simple, file-based database with no external dependencies

## 🚀 Quick Start

### Using Docker (Recommended)

1. **Run with Docker:**
   ```bash
   docker run -d \
     --name shufflr \
     -p 8080:8080 \
     -v shufflr_data:/app/data \
     ghcr.io/YOUR_USERNAME/shufflr:latest
   ```

2. **Or use Docker Compose:**
   ```bash
   curl -O https://raw.githubusercontent.com/YOUR_USERNAME/shufflr/main/docker-compose.yml
   docker-compose up -d
   ```

3. **Access the admin interface:**
   Open http://localhost:8080 in your browser

4. **Complete initial setup:**
   - Create your admin account
   - Upload some images
   - Generate an API key

### Using Pre-built Binary

1. **Download the latest release:**
   ```bash
   curl -L https://github.com/YOUR_USERNAME/shufflr/releases/latest/download/shufflr-linux-amd64 -o shufflr
   chmod +x shufflr
   ```

2. **Run the server:**
   ```bash
   ./shufflr
   ```

### Building from Source

1. **Prerequisites:**
   - Go 1.21 or later
   - CGO enabled (for SQLite)

2. **Clone and build:**
   ```bash
   git clone https://github.com/YOUR_USERNAME/shufflr.git
   cd shufflr
   go mod download
   go build -o shufflr ./cmd/server
   ./shufflr
   ```

## 📖 API Documentation

### Authentication

All API endpoints require authentication using an API key. Include your API key in requests using either:

- **Header:** `X-API-Key: your_api_key_here`
- **Bearer Token:** `Authorization: Bearer your_api_key_here`

### Get Random Images

**Endpoint:** `GET /api/images`

**Parameters:**
- `count` (optional): Number of images to return (default: 20)

**Example Request:**
```bash
curl -H "X-API-Key: your_api_key_here" \
     "http://localhost:8080/api/images?count=5"
```

**Example Response:**
```json
{
  "images": [
    {
      "url": "/api/images/photo1.jpg",
      "filename": "photo1.jpg"
    },
    {
      "url": "/api/images/photo2.png",
      "filename": "photo2.png"
    }
  ],
  "count": 2
}
```

### Serve Images

**Endpoint:** `GET /api/images/{filename}`

Images are served directly and can be accessed without authentication once you have the URL.

**Example:**
```bash
curl "http://localhost:8080/api/images/photo1.jpg" -o downloaded_image.jpg
```

### Health Check

**Endpoint:** `GET /health`

Returns server health status and basic metrics.

```bash
curl "http://localhost:8080/health"
```

## 🔧 Configuration

Shufflr is configured using environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `DATABASE_PATH` | `./shufflr.db` | SQLite database file path |
| `UPLOAD_DIR` | `./uploads` | Directory for uploaded images |
| `BASE_URL` | `http://localhost:8080` | Base URL for the service |
| `SESSION_SECRET` | Generated | Secret key for session encryption |

### Example Configuration

Create a `.env` file:
```bash
PORT=8080
DATABASE_PATH=/data/shufflr.db
UPLOAD_DIR=/data/uploads
BASE_URL=https://images.example.com
SESSION_SECRET=your-secret-key-here
```

## 🐳 Docker Deployment

### Basic Deployment

```bash
docker run -d \
  --name shufflr \
  -p 8080:8080 \
  -v shufflr_data:/app/data \
  -e BASE_URL=https://your-domain.com \
  ghcr.io/YOUR_USERNAME/shufflr:latest
```

### Production Deployment with Docker Compose

```yaml
version: '3.8'

services:
  shufflr:
    image: ghcr.io/YOUR_USERNAME/shufflr:latest
    container_name: shufflr
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - BASE_URL=https://your-domain.com
      - SESSION_SECRET=your-secure-session-secret
    volumes:
      - shufflr_data:/app/data
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  # Optional: Nginx reverse proxy
  nginx:
    image: nginx:alpine
    container_name: shufflr_nginx
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/nginx/ssl:ro
    depends_on:
      - shufflr

volumes:
  shufflr_data:
```

## 🔐 Security

### Best Practices

1. **Use HTTPS in production**: Always serve Shufflr behind a reverse proxy with SSL/TLS
2. **Secure session secrets**: Use a strong, randomly generated `SESSION_SECRET`
3. **Regular backups**: Backup your database and uploaded images regularly
4. **API key rotation**: Regularly rotate API keys for better security
5. **Access control**: Restrict admin interface access using firewall rules or VPN

### Rate Limiting

Consider implementing rate limiting at the reverse proxy level:

```nginx
# Example Nginx rate limiting
location /api/ {
    limit_req zone=api burst=10 nodelay;
    proxy_pass http://shufflr:8080;
}
```

## 📱 Admin Interface

The admin web interface provides:

- **Dashboard**: Overview of images, API keys, and usage statistics
- **Image Management**: Upload, view, rename, and delete images
- **API Key Management**: Create, disable, regenerate, and delete API keys
- **Usage Metrics**: Track API requests and key usage

Access the admin interface at `http://your-domain/admin`

## 🛠️ Development

### Prerequisites

- Go 1.21+
- SQLite3 development libraries
- Node.js (for UI development, optional)

### Setup

```bash
git clone https://github.com/YOUR_USERNAME/shufflr.git
cd shufflr
go mod download
```

### Running Locally

```bash
go run ./cmd/server
```

### Running Tests

```bash
go test -v ./...
```

### Building

```bash
# Local build
go build -o shufflr ./cmd/server

# Cross-compilation
GOOS=linux GOARCH=amd64 go build -o shufflr-linux-amd64 ./cmd/server
GOOS=windows GOARCH=amd64 go build -o shufflr-windows-amd64.exe ./cmd/server
GOOS=darwin GOARCH=amd64 go build -o shufflr-darwin-amd64 ./cmd/server
```

## 🔧 Troubleshooting

### Common Issues

**Database locked error:**
```bash
# Stop the service and check for orphaned processes
pkill shufflr
# Restart the service
```

**Permission denied errors:**
```bash
# Check file permissions
ls -la shufflr.db uploads/
# Fix permissions
chmod 644 shufflr.db
chmod 755 uploads/
```

**Images not loading:**
- Verify the `UPLOAD_DIR` path is correct
- Check file permissions on the uploads directory
- Ensure images exist in the database and filesystem

**API key authentication failing:**
- Verify the API key is correct and enabled
- Check the request headers format
- Review server logs for authentication errors

### Logs

**Docker logs:**
```bash
docker logs shufflr
```

**Service logs:**
The application logs to stdout. In production, consider using a log aggregation service.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with [Go](https://golang.org/)
- UI powered by [Tailwind CSS](https://tailwindcss.com/) and [DaisyUI](https://daisyui.com/)
- Database: [SQLite](https://www.sqlite.org/)

## 📞 Support

- 🐛 **Bug Reports**: [GitHub Issues](https://github.com/YOUR_USERNAME/shufflr/issues)
- 💬 **Discussions**: [GitHub Discussions](https://github.com/YOUR_USERNAME/shufflr/discussions)
- 📖 **Documentation**: [Wiki](https://github.com/YOUR_USERNAME/shufflr/wiki)

---

**Made with ❤️ for the self-hosting community**