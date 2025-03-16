package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Get local network subnet
func getLocalSubnet() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				if ipNet.IP.To4() != nil {
					ones, _ := ipNet.Mask.Size()
					network := ipNet.IP.Mask(ipNet.Mask)
					return fmt.Sprintf("%s/%d", network, ones), nil
				}
			}
		}
	}

	return "", fmt.Errorf("no valid network interface found")
}

// Ping an IP using system command
func ping(ip string, wg *sync.WaitGroup, results chan<- string) {
	defer wg.Done()

	var cmd *exec.Cmd
	if os.Getenv("OS") == "Windows_NT" {
		cmd = exec.Command("ping", "-n", "1", "-w", "1000", ip) // Windows
	} else {
		cmd = exec.Command("ping", "-c", "1", "-W", "1", ip) // Linux/macOS
	}

	output, err := cmd.Output()
	if err == nil && strings.Contains(string(output), "ttl=") { // TTL check to confirm response
		results <- ip
	}
}

// Scan the network range
func scanNetwork(subnet string) []string {
	fmt.Println("🔍 Scanning network:", subnet)
	timeStart := time.Now()

	ipList := generateIPRange(subnet)
	fmt.Println("📡 Found", len(ipList), "devices")

	var wg sync.WaitGroup
	results := make(chan string, len(ipList))

	// Ping all IPs in parallel
	for _, ip := range ipList {
		wg.Add(1)
		go ping(ip, &wg, results)
	}

	// Wait for all goroutines to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect active devices
	activeDevices := []string{}
	for ip := range results {
		activeDevices = append(activeDevices, ip)
	}

	if len(activeDevices) == 0 {
		fmt.Println("⚠️ No active devices found!")
		return activeDevices
	}

	fmt.Printf("\n✅ Active Devices Found: %d\n", len(activeDevices))
	// for _, ip := range activeDevices {
	// 	// fmt.Printf("IP: %s\n", ip)
	// }

	duration := time.Since(timeStart)
	fmt.Printf("\n⏱️ Scan completed in %s\n", duration)
	return activeDevices
}

// Generate a list of all IPs in the subnet
func generateIPRange(subnet string) []string {
	_, ipv4Net, err := net.ParseCIDR(subnet)
	if err != nil {
		fmt.Println("❌ Invalid subnet:", err)
		os.Exit(1)
	}

	var ips []string
	for ip := ipv4Net.IP.Mask(ipv4Net.Mask); ipv4Net.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}

	// Remove network and broadcast addresses
	if len(ips) > 2 {
		return ips[1 : len(ips)-1]
	}
	log.Println("ips ", ips)
	return ips
}

// Increment an IP address
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func ipaddr() []string {
	// timeStart := time.Now()

	localSubnet, err := getLocalSubnet()
	if err != nil {
		log.Fatalf("❌ Error getting subnet: %v", err)
	}

	// totalDuration := time.Since(timeStart)
	// fmt.Printf("\n⏳ Total execution time: %s\n", totalDuration)
	return scanNetwork(localSubnet)
}
