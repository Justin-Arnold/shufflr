# Shufflr - Self-Hosted Random Image API Service

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

### Setup Using Docker Compose

1. Create and navigate to app directory
    ```bash
    mkdir shufflr && cd shufflr
    ```

2. Clone down the Docker Compose file
   ```bash
   curl -O https://raw.githubusercontent.com/Justin-Arnold/shufflr/master/docker-compose.yml
   ```

3. Generate your session secret and add it to you .env file
    ```bash
    echo "SHUFFLR_SESSION_SECRET=$(openssl rand -hex 32)" > .env
    ```

4. Start the app
    ```bash
    docker-compose up -d
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
| `SHUFFLR_PORT` | `8080` | Server port |
| `SHUFFLR_DATABASE_PATH` | `./shufflr.db` | SQLite database file path |
| `SHUFFLR_UPLOAD_DIR` | `./uploads` | Directory for uploaded images |
| `SHUFFLR_BASE_URL` | `http://localhost:8080` | Base URL for the service |
| `SHUFFLR_SESSION_SECRET` | Generated | Secret key for session encryption |

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

Note: Note created yet

## 🙏 Acknowledgments

- Built with [Go](https://golang.org/)
- UI powered by [Tailwind CSS](https://tailwindcss.com/) and [DaisyUI](https://daisyui.com/)
- Database: [SQLite](https://www.sqlite.org/)

## 📞 Support

- 🐛 **Bug Reports**: [GitHub Issues](https://github.com/Justin-Arnold/shufflr/issues)
- 💬 **Discussions**: [GitHub Discussions](https://github.com/Justin-Arnold/shufflr/discussions)
- 📖 **Documentation**: [Wiki](https://github.com/Justin-Arnold/shufflr/wiki)

---

**Made with ❤️ for the self-hosting community**