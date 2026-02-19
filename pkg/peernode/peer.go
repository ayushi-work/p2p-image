package peernode

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"

	"p2p-image/pkg/consensus"
	"p2p-image/pkg/imaging"
	"p2p-image/pkg/p2p"
	"p2p-image/pkg/protocol"
)

// Peer represents a worker peer node
type Peer struct {
	node             *p2p.Node
	consensusHandler *consensus.PeerConsensusHandler
}

// NewPeer creates a new peer node
func NewPeer(ctx context.Context, p2pPort int) (*Peer, error) {
	// Create P2P node
	node, err := p2p.NewNode(ctx, p2p.NodeConfig{
		ListenPort: p2pPort,
		IsGateway:  false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create P2P node: %w", err)
	}

	peer := &Peer{
		node:             node,
		consensusHandler: consensus.NewPeerConsensusHandler(node.ID().String()),
	}

	// Set up protocol handlers
	peer.setupProtocolHandlers()

	return peer, nil
}

// setupProtocolHandlers configures P2P protocol handlers
func (p *Peer) setupProtocolHandlers() {
	// Handle consensus requests
	p.node.SetStreamHandler(p2p.ProtocolConsensus, p.handleConsensusRequest)

	// Handle hash store commands
	p.node.SetStreamHandler(p2p.ProtocolHashStore, p.handleHashStore)

	// Handle processing requests
	p.node.SetStreamHandler(p2p.ProtocolProcess, p.handleProcessRequest)
}

// handleConsensusRequest handles incoming consensus requests
func (p *Peer) handleConsensusRequest(stream p2p.Stream) {
	defer stream.Close()

	// Receive request
	var req protocol.ConsensusRequest
	if err := stream.ReceiveJSON(&req); err != nil {
		fmt.Printf("Failed to receive consensus request: %v\n", err)
		return
	}

	// Process request
	resp := p.consensusHandler.HandleRequest(req)

	// Send response
	if err := stream.SendJSON(resp); err != nil {
		fmt.Printf("Failed to send consensus response: %v\n", err)
		return
	}

	fmt.Printf("Consensus request for hash %s: match=%v\n", req.Hash[:16]+"...", resp.Match)
}

// handleHashStore handles hash store commands
func (p *Peer) handleHashStore(stream p2p.Stream) {
	defer stream.Close()

	// Receive command
	var cmd protocol.HashStoreCommand
	if err := stream.ReceiveJSON(&cmd); err != nil {
		fmt.Printf("Failed to receive hash store command: %v\n", err)
		return
	}

	// Store hash
	p.consensusHandler.StoreHash(cmd.Hash, cmd.Timestamp)
	fmt.Printf("Stored hash: %s\n", cmd.Hash[:16]+"...")
}

// handleProcessRequest handles image processing requests
func (p *Peer) handleProcessRequest(stream p2p.Stream) {
	defer stream.Close()

	fmt.Printf("Received processing request from %s\n", stream.Peer())

	// Receive request
	var req protocol.ProcessRequest
	if err := stream.ReceiveJSON(&req); err != nil {
		fmt.Printf("Failed to receive process request: %v\n", err)
		p.sendErrorResponse(stream, "Failed to receive request")
		return
	}

	// Decode image
	img, err := decodeImage(req.ImageData)
	if err != nil {
		fmt.Printf("Failed to decode image: %v\n", err)
		p.sendErrorResponse(stream, "Invalid image format")
		return
	}

	// Process image based on operation
	var result image.Image
	switch req.Operation {
	case "dither":
		result, err = p.processDither(img, req.Params)
	case "remove_bg":
		result, err = p.processBackgroundRemoval(img, req.Params)
	default:
		p.sendErrorResponse(stream, "Unknown operation")
		return
	}

	if err != nil {
		fmt.Printf("Processing failed: %v\n", err)
		p.sendErrorResponse(stream, fmt.Sprintf("Processing failed: %v", err))
		return
	}

	// Encode result
	var buf bytes.Buffer
	if err := imaging.EncodePNG(&buf, result); err != nil {
		fmt.Printf("Failed to encode result: %v\n", err)
		p.sendErrorResponse(stream, "Failed to encode result")
		return
	}

	// Send response
	resp := protocol.ProcessResponse{
		Success:   true,
		ImageData: buf.Bytes(),
	}

	if err := stream.SendJSON(resp); err != nil {
		fmt.Printf("Failed to send response: %v\n", err)
		return
	}

	fmt.Printf("Processing completed: %s\n", req.Operation)
}

// processDither performs image dithering
func (p *Peer) processDither(img image.Image, params map[string]interface{}) (image.Image, error) {
	opts := imaging.DitherOptions{
		Algorithm: "floyd-steinberg",
		Colors:    2,
	}

	// Parse parameters
	if algo, ok := params["algorithm"].(string); ok {
		opts.Algorithm = algo
	}
	if colors, ok := params["colors"].(float64); ok {
		opts.Colors = int(colors)
	}

	return imaging.Dither(img, opts)
}

// processBackgroundRemoval performs background removal
func (p *Peer) processBackgroundRemoval(img image.Image, params map[string]interface{}) (image.Image, error) {
	opts := imaging.BackgroundRemovalOptions{
		Algorithm:  "simple",
		Threshold:  0.5,
		Iterations: 5,
	}

	// Parse parameters
	if algo, ok := params["algorithm"].(string); ok {
		opts.Algorithm = algo
	}
	if threshold, ok := params["threshold"].(float64); ok {
		opts.Threshold = threshold
	}
	if iterations, ok := params["iterations"].(float64); ok {
		opts.Iterations = int(iterations)
	}

	return imaging.RemoveBackground(img, opts)
}

// sendErrorResponse sends an error response
func (p *Peer) sendErrorResponse(stream p2p.Stream, errMsg string) {
	resp := protocol.ProcessResponse{
		Success: false,
		Error:   errMsg,
	}
	stream.SendJSON(resp)
}

// Start starts the peer node
func (p *Peer) Start() error {
	fmt.Printf("Peer started\n")
	fmt.Printf("  Peer ID: %s\n", p.node.ID())
	return nil
}

// Close shuts down the peer
func (p *Peer) Close() error {
	return p.node.Close()
}

// GetStatus returns peer status
func (p *Peer) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"peer_id":    p.node.ID().String(),
		"peer_count": p.node.GetPeerCount(),
		"hash_count": p.consensusHandler.GetStore().Count(),
		"is_gateway": false,
	}
}

// Helper to decode image
func decodeImage(data []byte) (image.Image, error) {
	return png.Decode(bytes.NewReader(data))
}
