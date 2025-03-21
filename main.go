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
	"sync"
)

var wg sync.WaitGroup

// Configuration constants
const (
	DefaultBufferSize = 1024 * 64 // Increased buffer size for better performance
	DefaultBasePort   = 27001
)

// Config holds all configuration for the application
type Config struct {
	server     bool
	FileName   string
	BasePort   int
	BufferSize int
	Text       string
}

// Global configuration
var config Config

func main() {
	// Configure flags
	// flag.BoolVar(&config.server, "server", true, "run as a server")
	flag.StringVar(&config.FileName, "file", "", "(run as server) file to serve")
	flag.StringVar(&config.Text, "text", "", "(run as server) Text to serve")
	flag.IntVar(&config.BasePort, "baseport", DefaultBasePort, "base port number")
	flag.IntVar(&config.BufferSize, "buffer", DefaultBufferSize, "buffer size in bytes")
	flag.Parse()
	config.server = config.FileName != "" || config.Text != ""
	// Determine mode and run
	if config.server {
		fmt.Printf("Running as server, connecting to all active IPs\n")
		runServer()
	} else {
		fmt.Printf("Running as client, listening for connections\n")
		runClient()
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
	active := ipaddr()
	fmt.Println("Active devices: ", strings.Join(active, ", "))
	for _, ip := range active {
		wg.Add(1)
		go sendToClient(ip)
	}
	wg.Wait()
}

func sendToClient(ip string) {
	defer wg.Done()
	address := net.JoinHostPort(ip, fmt.Sprintf("%d", config.BasePort))

	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Printf("Error connecting to %s: %v\n", address, err)
		return
	}
	defer conn.Close()

	if config.FileName != "" {
		sendFile(conn)
	} else if config.Text != "" {
		sendText(conn)
	}
}

func sendText(conn net.Conn) {
	defer conn.Close()

	// Format header information
	fileSizeHeader := padString(strconv.Itoa(len(config.Text)), 10)
	fileNameHeader := padString("text", 64)
	log.Printf("Sending text: %s (%s bytes)", fileNameHeader, fileSizeHeader)

	// Send headers
	conn.Write([]byte(fileSizeHeader))
	conn.Write([]byte(fileNameHeader))

	// Create buffer for sending
	buffer := make([]byte, config.BufferSize)

	// Send the data
	bytesWritten, err := io.CopyBuffer(conn, strings.NewReader(config.Text), buffer)
	if err != nil {
		log.Printf("Error sending text: %v", err)
	}

	log.Printf("Sent %d bytes", bytesWritten)
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
	address := fmt.Sprintf("0.0.0.0:%d", config.BasePort) // Accepts external connections

	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Error listening on port %d: %v", config.BasePort, err)
	}
	defer listener.Close()

	fmt.Printf("Listening on port %d\n", config.BasePort)
	fmt.Println("Waiting for server connection...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}
		go receiveData(conn)
	}
}

func receiveData(conn net.Conn) {
	defer conn.Close()

	// Read headers
	bufferFileSize := make([]byte, 10)
	_, err := conn.Read(bufferFileSize)
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

	// If the received "file" is actually text, print it instead of saving
	if fileName == "text" {
		buffer := make([]byte, fileSize)
		_, err := io.ReadFull(conn, buffer)
		if err != nil {
			log.Fatalf("Error reading text: %v", err)
		}
		fmt.Printf("\nReceived text: %s\n", string(buffer))
		return
	}
	fmt.Printf("Downloading file: %s\n", fileName)

	// Create output file
	outputFile, err := os.Create(fileName)
	if err != nil {
		log.Fatalf("Error creating output file: %v", err)
	}
	defer outputFile.Close()

	// Copy data to file
	bytesWritten, err := io.Copy(outputFile, io.LimitReader(conn, fileSize))
	if err != nil {
		log.Fatalf("Error receiving data: %v", err)
	}

	log.Printf("Received %d bytes", bytesWritten)
	fmt.Printf("\nDownload complete: %s\n", fileName)
}
