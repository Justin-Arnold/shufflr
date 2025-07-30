# Docker Deployment Guide

## Quick Start

1. **Clone the repository and navigate to it**
2. **Copy the environment file:**
   ```bash
   cp .env.example .env
   ```
3. **Edit `.env` to configure your setup** (especially `SHUFFLR_SESSION_SECRET`)
4. **Start the application:**
   ```bash
   docker-compose up -d
   ```

## Data Persistence

The application uses **bind mounts** to store data on your host filesystem, making it easily accessible and persistent across container restarts.

### Default Setup

By default, all data is stored in `./shufflr-data/` relative to your docker-compose.yml:

```
./shufflr-data/
├── shufflr.db          # SQLite database
└── uploads/            # Uploaded images
    ├── image1.jpg
    ├── image2.png
    └── ...
```

### Directory Structure

- **Database**: `./shufflr-data/shufflr.db` - Contains all application data (users, API keys, image metadata, settings)
- **Uploads**: `./shufflr-data/uploads/` - Contains all uploaded image files

### Custom Data Directory

You can customize where data is stored by setting `SHUFFLR_DATA_DIR` in your `.env` file:

```env
# Store data in a custom location
SHUFFLR_DATA_DIR=/path/to/your/shufflr-data

# Or use an absolute path
SHUFFLR_DATA_DIR=/home/user/Documents/shufflr-data
```

### Separate Mount Points (Advanced)

For more granular control, you can uncomment the alternative volume mounts in `docker-compose.yml`:

```yaml
volumes:
  # Comment out the single mount
  # - ${SHUFFLR_DATA_DIR:-./shufflr-data}:/app/data
  
  # Use separate mounts instead
  - ${SHUFFLR_DATABASE_DIR:-./shufflr-data}:/app/data
  - ${SHUFFLR_UPLOAD_DIR_HOST:-./shufflr-data/uploads}:/app/data/uploads
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SHUFFLR_PORT` | `8080` | Port to run the server on |
| `SHUFFLR_BASE_URL` | `http://localhost:8080` | Base URL for the application |
| `SHUFFLR_DATA_DIR` | `./shufflr-data` | Host directory for all data |
| `SHUFFLR_SESSION_SECRET` | *(required)* | Session encryption key (generate with `openssl rand -hex 32`) |

## Backup and Migration

Since data is stored on your host filesystem:

- **Backup**: Simply copy the `shufflr-data` directory
- **Migration**: Move the `shufflr-data` directory to a new location and update your `.env`
- **Restore**: Replace the `shufflr-data` directory with your backup

## File Permissions

The container runs as user `shufflr` (UID: 1001, GID: 1001). If you experience permission issues:

```bash
# Fix ownership of the data directory
sudo chown -R 1001:1001 ./shufflr-data
```

## Accessing Your Images

Once running, your uploaded images are accessible:

1. **Via the web interface**: Upload and manage images through the admin panel
2. **Direct file access**: Access files directly in `./shufflr-data/uploads/`
3. **API access**: Use the API endpoints to serve images

## Example Commands

```bash
# Start in background
docker-compose up -d

# View logs
docker-compose logs -f

# Stop the application
docker-compose down

# Update to latest version
docker-compose pull && docker-compose up -d

# Restart the application
docker-compose restart
```