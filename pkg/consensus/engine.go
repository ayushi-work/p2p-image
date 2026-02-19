package consensus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"p2p-image/pkg/protocol"
)

var (
	ErrConsensusTimeout  = errors.New("consensus timeout")
	ErrInsufficientPeers = errors.New("insufficient peers for consensus")
	ErrInconsistentState = errors.New("inconsistent state across peers")
	ErrAllPeersMatch     = errors.New("all peers already have this hash")
)

// ConsensusResult represents the outcome of a consensus round
type ConsensusResult struct {
	Accepted  bool
	AllMatch  bool // true if all peers already have the hash
	Responses []protocol.ConsensusResponse
}

// Engine orchestrates the consensus protocol
type Engine struct {
	store            *HashStore
	minPeers         int
	consensusTimeout time.Duration
}

// NewEngine creates a new consensus engine
func NewEngine(minPeers int, timeout time.Duration) *Engine {
	return &Engine{
		store:            NewHashStore(),
		minPeers:         minPeers,
		consensusTimeout: timeout,
	}
}

// GetStore returns the underlying hash store
func (e *Engine) GetStore() *HashStore {
	return e.store
}

// ComputeHash computes SHA-256 hash of data
func ComputeHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// RequestConsensus initiates a consensus round
// Returns true if file should be accepted (all peers agree)
func (e *Engine) RequestConsensus(
	ctx context.Context,
	hash string,
	operation string,
	sendFunc func(protocol.ConsensusRequest) ([]protocol.ConsensusResponse, error),
) (*ConsensusResult, error) {

	// Create consensus request
	req := protocol.ConsensusRequest{
		Hash:      hash,
		Operation: operation,
		Timestamp: time.Now(),
	}

	// Set timeout
	ctx, cancel := context.WithTimeout(ctx, e.consensusTimeout)
	defer cancel()

	// Send request and collect responses
	responseChan := make(chan []protocol.ConsensusResponse, 1)
	errChan := make(chan error, 1)

	go func() {
		responses, err := sendFunc(req)
		if err != nil {
			errChan <- err
			return
		}
		responseChan <- responses
	}()

	// Wait for responses or timeout
	var responses []protocol.ConsensusResponse
	select {
	case <-ctx.Done():
		return nil, ErrConsensusTimeout
	case err := <-errChan:
		return nil, err
	case responses = <-responseChan:
		// Continue processing
	}

	// Validate we have enough peers
	if len(responses) < e.minPeers {
		return nil, fmt.Errorf("%w: got %d, need %d", ErrInsufficientPeers, len(responses), e.minPeers)
	}

	// Analyze responses
	result := e.analyzeResponses(responses)

	return result, nil
}

// analyzeResponses determines consensus outcome
func (e *Engine) analyzeResponses(responses []protocol.ConsensusResponse) *ConsensusResult {
	if len(responses) == 0 {
		return &ConsensusResult{Accepted: false, AllMatch: false, Responses: responses}
	}

	matchCount := 0
	noMatchCount := 0

	for _, resp := range responses {
		if resp.Match {
			matchCount++
		} else {
			noMatchCount++
		}
	}

	// All peers say NO_MATCH → new file, accept
	if noMatchCount == len(responses) {
		return &ConsensusResult{
			Accepted:  true,
			AllMatch:  false,
			Responses: responses,
		}
	}

	// All peers say MATCH → duplicate file, accept but don't reprocess
	if matchCount == len(responses) {
		return &ConsensusResult{
			Accepted:  true,
			AllMatch:  true,
			Responses: responses,
		}
	}

	// Mixed responses → inconsistent state, reject
	return &ConsensusResult{
		Accepted:  false,
		AllMatch:  false,
		Responses: responses,
	}
}

// StoreHash stores a hash locally after consensus
func (e *Engine) StoreHash(hash string) {
	e.store.Store(hash)
}

// CheckHash checks if hash exists locally (for peer nodes)
func (e *Engine) CheckHash(hash string) bool {
	return e.store.Has(hash)
}

// PeerConsensusHandler handles consensus requests on peer nodes
type PeerConsensusHandler struct {
	store  *HashStore
	peerID string
	mu     sync.RWMutex
}

// NewPeerConsensusHandler creates a handler for peer nodes
func NewPeerConsensusHandler(peerID string) *PeerConsensusHandler {
	return &PeerConsensusHandler{
		store:  NewHashStore(),
		peerID: peerID,
	}
}

// HandleRequest processes a consensus request and returns response
func (h *PeerConsensusHandler) HandleRequest(req protocol.ConsensusRequest) protocol.ConsensusResponse {
	h.mu.RLock()
	defer h.mu.RUnlock()

	match := h.store.Has(req.Hash)

	return protocol.ConsensusResponse{
		PeerID: h.peerID,
		Hash:   req.Hash,
		Match:  match,
	}
}

// StoreHash stores a hash after consensus
func (h *PeerConsensusHandler) StoreHash(hash string, timestamp time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.store.StoreWithTime(hash, timestamp)
}

// GetStore returns the hash store
func (h *PeerConsensusHandler) GetStore() *HashStore {
	return h.store
}
