package chshare

//ProtocolVersion of chisel. When backwards
//incompatible changes are made, this will
//be incremented to signify a protocol
//mismatch.
var ProtocolVersion = "chisel-v3"

//MaskedWebSocketProtocol is the masked WebSocket subprotocol used to hide chisel identity
//Using common WebSocket subprotocols to blend in with normal traffic
var MaskedWebSocketProtocol = "chat"

//MaskedSSHServerVersion is the masked SSH server version string
var MaskedSSHServerVersion = "SSH-2.0-OpenSSH_8.0"

//MaskedSSHClientVersion is the masked SSH client version string
var MaskedSSHClientVersion = "SSH-2.0-OpenSSH_8.0"

var BuildVersion = "0.0.0-src"
