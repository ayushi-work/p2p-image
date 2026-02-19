package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"p2p-image/pkg/gateway"
	"p2p-image/pkg/peernode"
)

func main() {
	// Define command-line flags
	mode := flag.String("mode", "gateway", "Node mode: gateway or peer")
	httpPort := flag.Int("http-port", 8080, "HTTP API port (gateway only)")
	p2pPort := flag.Int("p2p-port", 9000, "P2P network port")

	flag.Parse()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start node based on mode
	switch *mode {
	case "gateway":
		if err := runGateway(ctx, *httpPort, *p2pPort, sigChan); err != nil {
			fmt.Fprintf(os.Stderr, "Gateway error: %v\n", err)
			os.Exit(1)
		}
	case "peer":
		if err := runPeer(ctx, *p2pPort, sigChan); err != nil {
			fmt.Fprintf(os.Stderr, "Peer error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Invalid mode: %s (must be 'gateway' or 'peer')\n", *mode)
		os.Exit(1)
	}
}

func runGateway(ctx context.Context, httpPort, p2pPort int, sigChan chan os.Signal) error {
	fmt.Println("=== P2P Image Processing Gateway ===")

	// Create gateway
	gw, err := gateway.NewGateway(ctx, httpPort, p2pPort)
	if err != nil {
		return fmt.Errorf("failed to create gateway: %w", err)
	}
	defer gw.Close()

	// Start gateway
	if err := gw.Start(); err != nil {
		return fmt.Errorf("failed to start gateway: %w", err)
	}

	fmt.Println("\nGateway is running. Press Ctrl+C to stop.")
	fmt.Println("Waiting for peers to connect...")

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down gateway...")

	return nil
}

func runPeer(ctx context.Context, p2pPort int, sigChan chan os.Signal) error {
	fmt.Println("=== P2P Image Processing Peer ===")

	// Create peer
	peer, err := peernode.NewPeer(ctx, p2pPort)
	if err != nil {
		return fmt.Errorf("failed to create peer: %w", err)
	}
	defer peer.Close()

	// Start peer
	if err := peer.Start(); err != nil {
		return fmt.Errorf("failed to start peer: %w", err)
	}

	fmt.Println("\nPeer is running. Press Ctrl+C to stop.")
	fmt.Println("Discovering and connecting to gateway...")

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down peer...")

	return nil
}
