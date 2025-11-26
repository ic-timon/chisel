# Performance Testing Guide

## Overview

This directory contains performance testing tools to compare the original Chisel implementation with the enhanced version that includes traffic obfuscation features.

## Features Tested

1. **Throughput**: Data transfer speed for various packet sizes
2. **Latency**: Round-trip time for small packets
3. **Connection Setup**: Time to establish WebSocket and SSH connections

## Running Tests

### Basic Benchmark

```bash
go run main.go bench
```

### Performance Comparison

To compare original vs enhanced versions, you need to run two instances:

1. Start original version on port 2001
2. Start enhanced version on port 2003
3. Run comparison:

```bash
go run main.go compare 2001 2003
```

## Test Results

Results are saved to `performance_report_<timestamp>.json` with detailed metrics including:

- Throughput (MB/s)
- Latency (ms)
- Connection setup time (ms)
- Overhead percentage compared to original

## Metrics Explained

- **Throughput Overhead**: Percentage increase/decrease in data transfer speed
- **Latency Overhead**: Additional delay introduced by obfuscation features
- **Connection Setup Overhead**: Additional time to establish connections

## Expected Results

With all obfuscation features enabled at highest level:

- **Throughput**: Slight decrease (5-15%) due to packet chunking and delays
- **Latency**: Small increase (10-50ms) due to randomized delays
- **Connection Setup**: Minimal impact (<10ms)

These overheads are acceptable trade-offs for improved traffic obfuscation.

