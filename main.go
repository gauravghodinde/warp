package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Configuration constants
const (
	DefaultBufferSize = 1024 * 64 // Increased buffer size for better performance
	DefaultBasePort   = 27001
)

// Config holds all configuration for the application
type Config struct {
	ServerAddress string
	FileName      string
	BasePort      int
	BufferSize    int
}

// Global configuration
var config Config

func main() {
	// Configure flags
	flag.StringVar(&config.ServerAddress, "server", "", "(run as client) server address to connect to")
	flag.StringVar(&config.FileName, "file", "", "(run as server) file to serve")
	flag.IntVar(&config.BasePort, "baseport", DefaultBasePort, "base port number")
	flag.IntVar(&config.BufferSize, "buffer", DefaultBufferSize, "buffer size in bytes")
	flag.Parse()

	// Display banner

	// Determine mode and run
	if config.FileName != "" {
		fmt.Printf("Running as server, serving file: %s\n", config.FileName)
		runServer()
	} else if config.ServerAddress != "" {
		fmt.Printf("Running as client, connecting to: %s\n", config.ServerAddress)
		runClient()
	} else {
		fmt.Println("You must specify either -file (for running as a server) or -server (for running as a client)")
		flag.Usage()
	}
}

// padString ensures a string is padded to the specified length
func padString(s string, length int) string {
	if len(s) >= length {
		return s[:length]
	}
	return s + strings.Repeat(":", length-len(s))
}

// Server Implementation
func runServer() {
	// Validate file exists
	fileInfo, err := os.Stat(config.FileName)
	if err != nil {
		log.Fatalf("Error accessing file %s: %v", config.FileName, err)
	}

	fmt.Printf("Starting server\n")
	fmt.Printf("Serving file: %s (%.2f MB)\n", config.FileName, float64(fileInfo.Size())/1024/1024)

	address := fmt.Sprintf(":%d", config.BasePort)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Error listening on port %d: %v", config.BasePort, err)
	}
	defer listener.Close()

	fmt.Printf("Listening on port %d\n", config.BasePort)
	fmt.Println("Waiting for client connection...")

	// Accept a single connection
	conn, err := listener.Accept()
	if err != nil {
		log.Fatalf("Error accepting connection: %v", err)
	}
	defer conn.Close()

	sendFile(conn)
}

func sendFile(conn net.Conn) {
	defer conn.Close()

	// Open the file
	file, err := os.Open(config.FileName)
	if err != nil {
		log.Printf("Error opening file: %v", err)
		return
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("Error getting file info: %v", err)
		return
	}

	// Format header information
	fileSizeHeader := padString(strconv.FormatInt(fileInfo.Size(), 10), 10)
	fileNameHeader := padString(filepath.Base(fileInfo.Name()), 64)
	log.Printf("Sending file: %s (%s bytes)", fileNameHeader, fileSizeHeader)

	// Send headers
	conn.Write([]byte(fileSizeHeader))
	conn.Write([]byte(fileNameHeader))

	// Create buffer for sending
	buffer := make([]byte, config.BufferSize)

	// Send the data
	bytesWritten, err := io.CopyBuffer(conn, file, buffer)
	if err != nil {
		log.Printf("Error sending file: %v", err)
	}

	log.Printf("Sent %d bytes", bytesWritten)
}

// Client Implementation
func runClient() {
	fmt.Printf("Starting client\n")

	address := net.JoinHostPort(config.ServerAddress, fmt.Sprintf("%d", config.BasePort))

	// Connect to server
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Fatalf("Error connecting to %s: %v", address, err)
	}
	defer conn.Close()

	// Read headers
	bufferFileSize := make([]byte, 10)
	_, err = conn.Read(bufferFileSize)
	if err != nil {
		log.Fatalf("Error reading file size: %v", err)
	}

	fileSize, err := strconv.ParseInt(strings.Trim(string(bufferFileSize), ":"), 10, 64)
	if err != nil {
		log.Fatalf("Error parsing file size: %v", err)
	}

	bufferFileName := make([]byte, 64)
	_, err = conn.Read(bufferFileName)
	if err != nil {
		log.Fatalf("Error reading file name: %v", err)
	}

	fileName := strings.Trim(string(bufferFileName), ":")

	fmt.Printf("Downloading file: %s\n", fileName)

	// Create output file
	outputFile, err := os.Create(fileName)
	if err != nil {
		log.Fatalf("Error creating output file: %v", err)
	}
	defer outputFile.Close()

	// Copy data
	bytesWritten, err := io.Copy(outputFile, io.LimitReader(conn, fileSize))
	if err != nil {
		log.Fatalf("Error receiving data: %v", err)
	}

	log.Printf("Received %d bytes", bytesWritten)
	fmt.Printf("\nDownload complete: %s\n", fileName)
}
