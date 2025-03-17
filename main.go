package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	Port       = 27001
	BufferSize = 1024 * 64 // 64 KB buffer size
)

var (
	clients    = make(map[net.Conn]bool)
	clientsMux sync.Mutex
)

func main() {
	serverMode := flag.Bool("server", false, "Run as server")
	flag.Parse()

	if *serverMode {
		startServer()
	} else {
		startClient()
	}
}

// ========================== SERVER CODE ==========================
func startServer() {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", Port))
	if err != nil {
		log.Fatal("Error starting server:", err)
	}
	defer ln.Close()

	log.Printf("Server started on port %d\n", Port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Connection error:", err)
			continue
		}
		clientsMux.Lock()
		clients[conn] = true
		clientsMux.Unlock()
		go handleClient(conn)
	}
}

func handleClient(conn net.Conn) {
	defer func() {
		clientsMux.Lock()
		delete(clients, conn)
		clientsMux.Unlock()
		conn.Close()
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		message := scanner.Text()
		if strings.HasPrefix(message, "file:") {
			handleFileTransfer(conn, message[5:])
		} else {
			broadcastMessage(fmt.Sprintf("%s: %s", conn.RemoteAddr(), message), conn)
		}
	}
}

func broadcastMessage(msg string, sender net.Conn) {
	log.Println("Broadcasting message:", msg)
	clientsMux.Lock()
	for client := range clients {
		if client != sender {
			fmt.Fprintln(client, msg)
		}
	}
	clientsMux.Unlock()
}

func handleFileTransfer(conn net.Conn, fileName string) {
	log.Printf("Receiving file: %s from %s\n", fileName, conn.RemoteAddr())

	file, err := os.Create(filepath.Base(fileName))
	if err != nil {
		log.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	io.Copy(file, conn)
	log.Printf("File %s received successfully\n", fileName)
	broadcastMessage(fmt.Sprintf("[FILE] %s shared a file: %s", conn.RemoteAddr(), fileName), conn)
}

// ========================== CLIENT CODE ==========================
func startClient() {
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", Port))
	if err != nil {
		log.Fatal("Error connecting to server:", err)
	}
	defer conn.Close()

	go listenForMessages(conn)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		message := scanner.Text()
		if strings.HasPrefix(message, "file:") {
			sendFile(conn, strings.TrimPrefix(message, "file:"))
		} else {
			fmt.Fprintln(conn, message)
		}
	}
}

func listenForMessages(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			log.Println("Disconnected from server")
			return
		}
		fmt.Print(msg)
	}
}

func sendFile(conn net.Conn, fileName string) {
	file, err := os.Open(fileName)
	if err != nil {
		log.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	fmt.Fprintf(conn, "file:%s\n", fileName)
	io.Copy(conn, file)
	log.Printf("File %s sent successfully\n", fileName)
}
