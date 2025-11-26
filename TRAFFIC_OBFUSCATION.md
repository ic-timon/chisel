# Traffic Obfuscation Features

This document describes the traffic obfuscation features implemented to make Chisel traffic patterns more realistic and harder to detect.

## Overview

All obfuscation features are **enabled by default at the highest level** to maximize traffic realism.

## Implemented Features

### 1. Protocol Masking

**WebSocket Protocol Header**
- **Original**: `chisel-v3` (easily identifiable)
- **Masked**: `chat` (common WebSocket subprotocol)
- **Location**: `share/version.go`, `client/client_connect.go`, `server/server_handler.go`
- Protocol verification now happens via SSH custom request after handshake

**SSH Version Strings**
- **Original**: `SSH-chisel-v3-server/client` (easily identifiable)
- **Masked**: `SSH-2.0-OpenSSH_8.0` (standard SSH version)
- **Location**: `client/client.go`, `server/server.go`

### 2. Realistic HTTP Headers

Automatically adds common HTTP headers to WebSocket connections:
- **User-Agent**: Randomly selected from common browsers (Chrome, Firefox, Safari, Edge)
- **Accept**: `*/*`
- **Accept-Language**: Randomly selected from common languages
- **Accept-Encoding**: `gzip, deflate, br`
- **Origin**: Randomly selected from common domains (Google, GitHub, etc.)
- **Cache-Control**: `no-cache`
- **Pragma**: `no-cache`

**Location**: `share/traffic/headers.go`, `client/client.go`
**Default**: Enabled, all headers added

### 3. Keepalive Interval Randomization

- **Jitter**: ±30% of base keepalive interval
- **Default**: Enabled
- **Location**: `share/tunnel/tunnel.go`

Makes keepalive ping intervals unpredictable, avoiding fixed timing patterns.

### 4. Packet Timing Randomization

- **Delay Range**: 0-100ms (highest level)
- **Behavior**: Delay is proportional to packet size
- **Default**: Enabled
- **Location**: `share/cnet/conn_ws.go`

Adds randomized delays to packet transmission to simulate real network behavior.

### 5. Packet Size Randomization

- **Chunk Size**: 1KB - 32KB (randomized, normal distribution approximation)
- **Inter-chunk Delay**: 0-20ms
- **Default**: Enabled
- **Location**: `share/cio/pipe.go`

Splits large data transfers into randomized chunks to avoid uniform packet size patterns.

### 6. Traffic Pattern Simulator

Supports multiple traffic simulation patterns:
- **HTTP-like**: Bursty traffic with request/response patterns
- **SSH-like**: Steady low-volume traffic with occasional bursts
- **Random**: Completely random intervals

**Location**: `share/traffic/simulator.go`
**Default**: Enabled at highest intensity (100%)

## Performance Impact

All features are designed to have minimal performance impact while maximizing obfuscation:

- **Throughput**: Expected 5-15% decrease due to chunking and delays
- **Latency**: Expected 10-50ms increase due to randomized delays
- **Connection Setup**: Minimal impact (<10ms)

These overheads are acceptable trade-offs for improved traffic obfuscation.

## Testing

Performance comparison tests are available in `test/bench/`:

```bash
# Run performance comparison
go run test/bench/main.go compare <original_port> <enhanced_port>
```

Results are saved to `performance_report_<timestamp>.json` with detailed metrics.

## Configuration

All features are enabled by default. No configuration is required for basic usage.

For advanced users, the traffic simulator can be customized:
- Pattern selection (HTTP-like, SSH-like, Random)
- Intensity level (0-100%)
- Enable/disable individual features

## Backward Compatibility

- Original protocol (`chisel-v3`) is still accepted for backward compatibility during transition
- All features work transparently with existing Chisel clients and servers
- No breaking changes to the API

## Security Considerations

These features are designed to:
1. Hide protocol fingerprints from deep packet inspection
2. Make traffic patterns look like normal web traffic
3. Avoid detection by traffic analysis tools
4. Blend in with legitimate network traffic

**Note**: These features improve obfuscation but do not guarantee anonymity. Always use additional security measures as appropriate for your threat model.



