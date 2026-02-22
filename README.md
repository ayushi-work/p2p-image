# Image Processing

A peer-to-peer image processing system for dithering and background removal, built with Go and libp2p.

## Features

- **Floyd-Steinberg & Atkinson Dithering** - High-quality error diffusion algorithms
- **Background Removal** - Color-based segmentation with edge preservation
- **P2P Architecture** - Distributed processing with libp2p (optional)
- **Local Mode** - Works standalone without peers
- **SHA-256 Consensus** - Hash-based file validation across network
- **PNG Output** - Lossless image quality preservation

## Quick Start

### Prerequisites

- Go 1.25.3 or higher
- Modern web browser

### Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/p2p-image.git
cd p2p-image

# Build
go build -o p2p-image

# Start the gateway
./start-local.sh
```

### Usage

1. Open http://localhost:8080 in your browser
2. Upload an image (PNG or JPG, up to 50MB)
3. Select operation:
   - **Dither**: Convert to black & white with error diffusion
   - **Remove Background**: Extract foreground with alpha channel
4. Click "Process"
5. Download the result

## API Usage

```bash
# Dither an image
curl -X POST http://localhost:8080/api/process \
  -F "image=@photo.jpg" \
  -F "operation=dither" \
  -o result.png

# Remove background
curl -X POST http://localhost:8080/api/process \
  -F "image=@photo.jpg" \
  -F "operation=remove_bg" \
  -o result.png

# Check status
curl http://localhost:8080/api/status
```

## Architecture

### Components

- **Gateway Node**: HTTP API server, consensus orchestration, request routing
- **Peer Nodes**: Image processing workers (optional, for distributed mode)
- **Consensus Engine**: SHA-256 hash-based file validation
- **Image Processor**: Floyd-Steinberg/Atkinson dithering, background removal

### Network Modes

**Local Mode** (default):
- Gateway processes images directly
- No peer nodes required
- Perfect for single-machine usage

**Distributed Mode**:
- Multiple peer nodes share processing load
- Automatic peer discovery via mDNS
- Consensus-based file validation
- Tile-based parallel processing for large images

## Project Structure

```
p2p-image/
├── main.go              # Entry point
├── start-local.sh       # Startup script
├── pkg/
│   ├── consensus/      # SHA-256 consensus engine
│   ├── gateway/        # Gateway node implementation
│   ├── imaging/        # Image processing algorithms
│   ├── p2p/           # libp2p networking layer
│   ├── peernode/      # Peer node implementation
│   └── protocol/      # P2P message definitions
└── web/               # Frontend (HTML/CSS/JS)
    ├── index.html
    ├── style.css
    └── app.js
```

## Configuration

### CLI Flags

```bash
# Gateway mode
./p2p-image -mode=gateway -http-port=8080 -p2p-port=9000

# Peer mode
./p2p-image -mode=peer -p2p-port=9001
```

### Environment

- `MAX_FILE_SIZE`: Maximum upload size (default: 50MB)
- `CONSENSUS_TIMEOUT`: Consensus timeout (default: 5s)

## Development

### Build

```bash
go build -o p2p-image
```

### Run Tests

```bash
go test ./...
```

### Run with Race Detector

```bash
go build -race -o p2p-image
```

## Algorithms

### Dithering

**Floyd-Steinberg** (default):
- Error distribution: 7/16, 3/16, 5/16, 1/16
- Best for photographs and complex images

**Atkinson**:
- Error distribution: 1/8 each (6/8 total)
- Lighter, more artistic look

### Background Removal

**Color-based Segmentation**:
- Samples corner pixels for background color
- Calculates color distance for each pixel
- Applies edge-preserving smoothing
- Outputs RGBA with alpha channel

## Performance

- **Consensus latency**: < 500ms (5 peers, local network)
- **Dithering (1MP)**: < 2 seconds
- **Background removal (1MP)**: < 5 seconds
- **Max file size**: 50MB

## Stopping the Server

```bash
pkill -f p2p-image
```

## Acknowledgments

- Built with [libp2p](https://libp2p.io/) for P2P networking
- Uses Go's standard `image` library for processing
- Frontend styled with [Instrument Serif](https://fonts.google.com/specimen/Instrument+Serif) font
