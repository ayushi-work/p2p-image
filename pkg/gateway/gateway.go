package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"p2p-image/pkg/consensus"
	"p2p-image/pkg/imaging"
	"p2p-image/pkg/p2p"
	"p2p-image/pkg/protocol"

	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	MaxFileSize = 50 * 1024 * 1024 // 50MB
)

// Gateway handles client requests and orchestrates P2P network
type Gateway struct {
	node            *p2p.Node
	consensusEngine *consensus.Engine
	httpServer      *http.Server

	// Request tracking
	activeRequests sync.Map // requestID -> *ProcessingRequest
}

// ProcessingRequest tracks an in-flight request
type ProcessingRequest struct {
	ID        string
	Hash      string
	Operation string
	StartTime time.Time
	Status    string
}

// NewGateway creates a new gateway node
func NewGateway(ctx context.Context, httpPort, p2pPort int) (*Gateway, error) {
	// Create P2P node
	node, err := p2p.NewNode(ctx, p2p.NodeConfig{
		ListenPort: p2pPort,
		IsGateway:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create P2P node: %w", err)
	}

	// Create consensus engine (allow 0 peers for local testing)
	// When peer_count is 0, gateway will process images locally
	engine := consensus.NewEngine(0, 5*time.Second)

	gw := &Gateway{
		node:            node,
		consensusEngine: engine,
	}

	// Set up protocol handlers
	gw.setupProtocolHandlers()

	// Set up HTTP server
	gw.setupHTTPServer(httpPort)

	return gw, nil
}

// setupProtocolHandlers configures P2P protocol handlers
func (gw *Gateway) setupProtocolHandlers() {
	// Gateway doesn't handle incoming process requests
	// It only sends them to peers
}

// setupHTTPServer configures the HTTP API server
func (gw *Gateway) setupHTTPServer(port int) {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/process", gw.handleProcess)
	mux.HandleFunc("/api/status", gw.handleStatus)
	mux.HandleFunc("/health", gw.handleHealth)

	// Serve frontend
	mux.Handle("/", http.FileServer(http.Dir("./web")))

	gw.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: corsMiddleware(mux),
	}
}

// Start starts the gateway services
func (gw *Gateway) Start() error {
	fmt.Printf("Gateway started\n")
	fmt.Printf("  Peer ID: %s\n", gw.node.ID())
	fmt.Printf("  HTTP API: http://localhost%s\n", gw.httpServer.Addr)

	// Start HTTP server in goroutine
	go func() {
		if err := gw.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	return nil
}

// handleProcess handles image processing requests
func (gw *Gateway) handleProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(MaxFileSize); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	// Get image file
	file, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "No image provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read image data
	imageData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read image", http.StatusInternalServerError)
		return
	}

	// Get operation
	operation := r.FormValue("operation")
	if operation != "dither" && operation != "remove_bg" {
		http.Error(w, "Invalid operation", http.StatusBadRequest)
		return
	}

	// Process the request
	result, err := gw.processImage(r.Context(), imageData, operation)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return processed image
	w.Header().Set("Content-Type", "image/png")
	w.Write(result)
}

// processImage orchestrates the full processing pipeline
func (gw *Gateway) processImage(ctx context.Context, imageData []byte, operation string) ([]byte, error) {
	// Step 1: Compute hash
	hash := consensus.ComputeHash(imageData)
	fmt.Printf("Processing image with hash: %s\n", hash[:16]+"...")

	// Step 2: Check peer availability
	peers := gw.node.GetPeers()
	if len(peers) < 1 {
		// No peers available - process locally
		fmt.Println("No peers available, processing locally...")
		return gw.processImageLocally(ctx, imageData, operation)
	}

	// Step 3: Request consensus
	fmt.Println("Requesting consensus from peers...")
	result, err := gw.consensusEngine.RequestConsensus(
		ctx,
		hash,
		operation,
		func(req protocol.ConsensusRequest) ([]protocol.ConsensusResponse, error) {
			return gw.broadcastConsensusRequest(ctx, req)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("consensus failed: %w", err)
	}

	if !result.Accepted {
		return nil, fmt.Errorf("file rejected by consensus (inconsistent state)")
	}

	// Step 4: If all peers already have it, return cached response
	if result.AllMatch {
		return nil, fmt.Errorf("file already processed (duplicate)")
	}

	// Step 5: Broadcast hash store command
	fmt.Println("Broadcasting hash store command...")
	if err := gw.broadcastHashStore(ctx, hash); err != nil {
		fmt.Printf("Warning: failed to broadcast hash store: %v\n", err)
	}

	// Step 6: Select random peer for processing
	selectedPeer := peers[rand.Intn(len(peers))]
	fmt.Printf("Selected peer %s for processing\n", selectedPeer.ID)

	// Step 7: Send processing request
	processedData, err := gw.sendProcessRequest(ctx, selectedPeer.ID, imageData, operation)
	if err != nil {
		return nil, fmt.Errorf("processing failed: %w", err)
	}

	fmt.Println("Processing completed successfully")
	return processedData, nil
}

// broadcastConsensusRequest sends consensus request to all peers
func (gw *Gateway) broadcastConsensusRequest(ctx context.Context, req protocol.ConsensusRequest) ([]protocol.ConsensusResponse, error) {
	peers := gw.node.GetPeers()
	responses := make([]protocol.ConsensusResponse, 0, len(peers))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, peerInfo := range peers {
		wg.Add(1)
		go func(pid peer.ID) {
			defer wg.Done()

			resp, err := gw.sendConsensusRequest(ctx, pid, req)
			if err != nil {
				fmt.Printf("Consensus request to %s failed: %v\n", pid, err)
				return
			}

			mu.Lock()
			responses = append(responses, resp)
			mu.Unlock()
		}(peerInfo.ID)
	}

	wg.Wait()
	return responses, nil
}

// sendConsensusRequest sends consensus request to a single peer
func (gw *Gateway) sendConsensusRequest(ctx context.Context, peerID peer.ID, req protocol.ConsensusRequest) (protocol.ConsensusResponse, error) {
	s, err := gw.node.Host().NewStream(ctx, peerID, p2p.ProtocolConsensus)
	if err != nil {
		return protocol.ConsensusResponse{}, err
	}
	defer s.Close()

	stream := p2p.WrapStream(s)

	// Send request
	if err := stream.SendJSON(req); err != nil {
		return protocol.ConsensusResponse{}, err
	}

	// Receive response
	var resp protocol.ConsensusResponse
	if err := stream.ReceiveJSON(&resp); err != nil {
		return protocol.ConsensusResponse{}, err
	}

	return resp, nil
}

// broadcastHashStore tells all peers to store a hash
func (gw *Gateway) broadcastHashStore(ctx context.Context, hash string) error {
	peers := gw.node.GetPeers()
	var wg sync.WaitGroup

	cmd := protocol.HashStoreCommand{
		Hash:      hash,
		Timestamp: time.Now(),
	}

	for _, peerInfo := range peers {
		wg.Add(1)
		go func(pid peer.ID) {
			defer wg.Done()
			if err := gw.sendHashStore(ctx, pid, cmd); err != nil {
				fmt.Printf("Hash store to %s failed: %v\n", pid, err)
			}
		}(peerInfo.ID)
	}

	wg.Wait()
	return nil
}

// sendHashStore sends hash store command to a peer
func (gw *Gateway) sendHashStore(ctx context.Context, peerID peer.ID, cmd protocol.HashStoreCommand) error {
	s, err := gw.node.Host().NewStream(ctx, peerID, p2p.ProtocolHashStore)
	if err != nil {
		return err
	}
	defer s.Close()

	stream := p2p.WrapStream(s)
	return stream.SendJSON(cmd)
}

// sendProcessRequest sends image to peer for processing
func (gw *Gateway) sendProcessRequest(ctx context.Context, peerID peer.ID, imageData []byte, operation string) ([]byte, error) {
	s, err := gw.node.Host().NewStream(ctx, peerID, p2p.ProtocolProcess)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	stream := p2p.WrapStream(s)

	// Send request
	req := protocol.ProcessRequest{
		ImageData: imageData,
		Operation: operation,
		Params:    make(map[string]interface{}),
	}

	if err := stream.SendJSON(req); err != nil {
		return nil, err
	}

	// Receive response
	var resp protocol.ProcessResponse
	if err := stream.ReceiveJSON(&resp); err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("processing error: %s", resp.Error)
	}

	return resp.ImageData, nil
}

// handleStatus returns gateway status
func (gw *Gateway) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"peer_id":    gw.node.ID().String(),
		"peer_count": gw.node.GetPeerCount(),
		"hash_count": gw.consensusEngine.GetStore().Count(),
		"is_gateway": true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleHealth returns health check
func (gw *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// Close shuts down the gateway
func (gw *Gateway) Close() error {
	if gw.httpServer != nil {
		gw.httpServer.Shutdown(context.Background())
	}
	return gw.node.Close()
}

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// processImageLocally processes image on gateway when no peers available
func (gw *Gateway) processImageLocally(ctx context.Context, imageData []byte, operation string) ([]byte, error) {
	fmt.Println("Processing image locally on gateway...")

	// Import imaging package
	img, err := decodeImage(imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Process based on operation
	var result image.Image
	switch operation {
	case "dither":
		result, err = gw.processDither(img)
	case "remove_bg":
		result, err = gw.processBackgroundRemoval(img)
	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}

	if err != nil {
		return nil, fmt.Errorf("processing failed: %w", err)
	}

	// Encode result
	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&buf, result); err != nil {
		return nil, fmt.Errorf("failed to encode result: %w", err)
	}

	return buf.Bytes(), nil
}

// processDither performs dithering locally
func (gw *Gateway) processDither(img image.Image) (image.Image, error) {
	opts := imaging.DitherOptions{
		Algorithm: "floyd-steinberg",
		Colors:    2,
	}
	return imaging.Dither(img, opts)
}

// processBackgroundRemoval performs background removal locally
func (gw *Gateway) processBackgroundRemoval(img image.Image) (image.Image, error) {
	opts := imaging.BackgroundRemovalOptions{
		Algorithm:  "simple",
		Threshold:  0.5,
		Iterations: 5,
	}
	return imaging.RemoveBackground(img, opts)
}

// Helper to decode image
func decodeImage(data []byte) (image.Image, error) {
	return png.Decode(bytes.NewReader(data))
}
