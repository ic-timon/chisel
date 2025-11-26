package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// PerformanceMetrics represents performance test results
type PerformanceMetrics struct {
	TestName      string        `json:"test_name"`
	Version       string        `json:"version"` // "original" or "enhanced"
	PacketSize    int           `json:"packet_size"`
	TransferTime  time.Duration `json:"transfer_time"`
	Throughput    float64       `json:"throughput_mbps"`
	Latency       time.Duration `json:"latency"`
	ConnSetupTime time.Duration `json:"connection_setup_time"`
	CPUUsage      float64       `json:"cpu_usage_percent"`
	MemoryUsage   int64         `json:"memory_usage_bytes"`
}

// PerformanceReport contains all test results
type PerformanceReport struct {
	Timestamp   time.Time           `json:"timestamp"`
	Tests       []PerformanceMetrics `json:"tests"`
	Summary     SummaryMetrics      `json:"summary"`
}

// SummaryMetrics contains summary statistics
type SummaryMetrics struct {
	OriginalAvgThroughput  float64 `json:"original_avg_throughput_mbps"`
	EnhancedAvgThroughput  float64 `json:"enhanced_avg_throughput_mbps"`
	ThroughputOverhead     float64 `json:"throughput_overhead_percent"`
	OriginalAvgLatency     float64 `json:"original_avg_latency_ms"`
	EnhancedAvgLatency     float64 `json:"enhanced_avg_latency_ms"`
	LatencyOverhead        float64 `json:"latency_overhead_percent"`
	OriginalAvgConnSetup   float64 `json:"original_avg_conn_setup_ms"`
	EnhancedAvgConnSetup   float64 `json:"enhanced_avg_conn_setup_ms"`
	ConnSetupOverhead      float64 `json:"conn_setup_overhead_percent"`
}

// testThroughput tests throughput for a given packet size
func testThroughput(port string, size int, version string) PerformanceMetrics {
	t0 := time.Now()
	resp, err := requestFile(port, size)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		log.Fatalf("Unexpected status code: %d", resp.StatusCode)
	}

	n, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		log.Fatalf("Copy failed: %v", err)
	}
	t1 := time.Now()
	
	transferTime := t1.Sub(t0)
	throughput := float64(n) / transferTime.Seconds() / MB // MB/s
	
	if int(n) != size {
		log.Fatalf("%d bytes expected, got %d", size, n)
	}
	
	return PerformanceMetrics{
		TestName:     "throughput",
		Version:      version,
		PacketSize:   size,
		TransferTime: transferTime,
		Throughput:   throughput,
	}
}

// testLatency tests latency for small packets
func testLatency(port string, size int, version string) PerformanceMetrics {
	var totalLatency time.Duration
	iterations := 100
	
	for i := 0; i < iterations; i++ {
		t0 := time.Now()
		resp, err := requestFile(port, size)
		if err != nil {
			log.Fatalf("Request failed: %v", err)
		}
		io.Copy(io.Discard, resp.Body)
		t1 := time.Now()
		totalLatency += t1.Sub(t0)
	}
	
	avgLatency := totalLatency / time.Duration(iterations)
	
	return PerformanceMetrics{
		TestName:  "latency",
		Version:   version,
		PacketSize: size,
		Latency:   avgLatency,
	}
}

// testConnectionSetup tests connection establishment time
func testConnectionSetup(port string, version string) PerformanceMetrics {
	var totalTime time.Duration
	iterations := 10
	
	for i := 0; i < iterations; i++ {
		t0 := time.Now()
		resp, err := http.Get("http://127.0.0.1:" + port + "/health")
		if err != nil {
			log.Fatalf("Connection failed: %v", err)
		}
		resp.Body.Close()
		t1 := time.Now()
		totalTime += t1.Sub(t0)
		time.Sleep(100 * time.Millisecond) // Small delay between tests
	}
	
	avgTime := totalTime / time.Duration(iterations)
	
	return PerformanceMetrics{
		TestName:     "connection_setup",
		Version:      version,
		ConnSetupTime: avgTime,
	}
}

// runPerformanceComparison runs performance tests comparing original vs enhanced version
func runPerformanceComparison(originalPort, enhancedPort string) {
	report := PerformanceReport{
		Timestamp: time.Now(),
		Tests:     []PerformanceMetrics{},
	}
	
	// Test packet sizes
	sizes := []int{1 * B, 1 * KB, 10 * KB, 100 * KB, 1 * MB, 10 * MB, 100 * MB}
	
	fmt.Println("Running throughput tests...")
	for _, size := range sizes {
		fmt.Printf("  Testing %d bytes (original)...\n", size)
		original := testThroughput(originalPort, size, "original")
		report.Tests = append(report.Tests, original)
		
		fmt.Printf("  Testing %d bytes (enhanced)...\n", size)
		enhanced := testThroughput(enhancedPort, size, "enhanced")
		report.Tests = append(report.Tests, enhanced)
	}
	
	fmt.Println("Running latency tests...")
	latencySizes := []int{1 * B, 100 * B, 1 * KB}
	for _, size := range latencySizes {
		fmt.Printf("  Testing %d bytes (original)...\n", size)
		original := testLatency(originalPort, size, "original")
		report.Tests = append(report.Tests, original)
		
		fmt.Printf("  Testing %d bytes (enhanced)...\n", size)
		enhanced := testLatency(enhancedPort, size, "enhanced")
		report.Tests = append(report.Tests, enhanced)
	}
	
	fmt.Println("Running connection setup tests...")
	fmt.Println("  Testing original...")
	originalConn := testConnectionSetup(originalPort, "original")
	report.Tests = append(report.Tests, originalConn)
	
	fmt.Println("  Testing enhanced...")
	enhancedConn := testConnectionSetup(enhancedPort, "enhanced")
	report.Tests = append(report.Tests, enhancedConn)
	
	// Calculate summary
	report.Summary = calculateSummary(report.Tests)
	
	// Save report
	saveReport(report)
	
	// Print summary
	printSummary(report.Summary)
}

// calculateSummary calculates summary statistics
func calculateSummary(tests []PerformanceMetrics) SummaryMetrics {
	var originalThroughput, enhancedThroughput []float64
	var originalLatency, enhancedLatency []float64
	var originalConnSetup, enhancedConnSetup []float64
	
	for _, test := range tests {
		switch test.TestName {
		case "throughput":
			if test.Version == "original" {
				originalThroughput = append(originalThroughput, test.Throughput)
			} else {
				enhancedThroughput = append(enhancedThroughput, test.Throughput)
			}
		case "latency":
			if test.Version == "original" {
				originalLatency = append(originalLatency, float64(test.Latency.Milliseconds()))
			} else {
				enhancedLatency = append(enhancedLatency, float64(test.Latency.Milliseconds()))
			}
		case "connection_setup":
			if test.Version == "original" {
				originalConnSetup = append(originalConnSetup, float64(test.ConnSetupTime.Milliseconds()))
			} else {
				enhancedConnSetup = append(enhancedConnSetup, float64(test.ConnSetupTime.Milliseconds()))
			}
		}
	}
	
	summary := SummaryMetrics{}
	
	// Calculate averages
	summary.OriginalAvgThroughput = average(originalThroughput)
	summary.EnhancedAvgThroughput = average(enhancedThroughput)
	summary.ThroughputOverhead = ((summary.EnhancedAvgThroughput - summary.OriginalAvgThroughput) / summary.OriginalAvgThroughput) * 100
	
	summary.OriginalAvgLatency = average(originalLatency)
	summary.EnhancedAvgLatency = average(enhancedLatency)
	summary.LatencyOverhead = ((summary.EnhancedAvgLatency - summary.OriginalAvgLatency) / summary.OriginalAvgLatency) * 100
	
	summary.OriginalAvgConnSetup = average(originalConnSetup)
	summary.EnhancedAvgConnSetup = average(enhancedConnSetup)
	summary.ConnSetupOverhead = ((summary.EnhancedAvgConnSetup - summary.OriginalAvgConnSetup) / summary.OriginalAvgConnSetup) * 100
	
	return summary
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func saveReport(report PerformanceReport) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal report: %v", err)
	}
	
	filename := fmt.Sprintf("performance_report_%s.json", time.Now().Format("20060102_150405"))
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		log.Fatalf("Failed to write report: %v", err)
	}
	
	fmt.Printf("\nPerformance report saved to: %s\n", filename)
}

func printSummary(summary SummaryMetrics) {
	separator := "============================================================"
	fmt.Println("\n" + separator)
	fmt.Println("PERFORMANCE COMPARISON SUMMARY")
	fmt.Println(separator)
	
	fmt.Printf("\nThroughput:\n")
	fmt.Printf("  Original:  %.2f MB/s\n", summary.OriginalAvgThroughput)
	fmt.Printf("  Enhanced:  %.2f MB/s\n", summary.EnhancedAvgThroughput)
	fmt.Printf("  Overhead:  %.2f%%\n", summary.ThroughputOverhead)
	
	fmt.Printf("\nLatency:\n")
	fmt.Printf("  Original:  %.2f ms\n", summary.OriginalAvgLatency)
	fmt.Printf("  Enhanced:  %.2f ms\n", summary.EnhancedAvgLatency)
	fmt.Printf("  Overhead:  %.2f%%\n", summary.LatencyOverhead)
	
	fmt.Printf("\nConnection Setup:\n")
	fmt.Printf("  Original:  %.2f ms\n", summary.OriginalAvgConnSetup)
	fmt.Printf("  Enhanced:  %.2f ms\n", summary.EnhancedAvgConnSetup)
	fmt.Printf("  Overhead:  %.2f%%\n", summary.ConnSetupOverhead)
	
	fmt.Println(separator + "\n")
}

