package traffic

import (
	"context"
	"math/rand"
	"time"
)

// TrafficPattern represents different traffic simulation patterns
type TrafficPattern int

const (
	// PatternHTTPLike simulates HTTP-like traffic patterns
	PatternHTTPLike TrafficPattern = iota
	// PatternSSHLike simulates SSH-like traffic patterns
	PatternSSHLike
	// PatternRandom simulates random traffic patterns
	PatternRandom
)

// Simulator simulates realistic traffic patterns
type Simulator struct {
	pattern     TrafficPattern
	rng         *rand.Rand
	enabled     bool
	intensity   int // 0-100, higher = more intense
	ctx         context.Context
	cancel      context.CancelFunc
	// Enhanced pattern tracking
	packetCount int64
	lastUpdate  time.Time
	// Advanced pattern configuration
	burstMode   bool
	burstCount  int
	idlePeriod  time.Duration
}

// NewSimulator creates a new traffic simulator
// Default: enabled at highest intensity
func NewSimulator(pattern TrafficPattern) *Simulator {
	ctx, cancel := context.WithCancel(context.Background())
	return &Simulator{
		pattern:    pattern,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
		enabled:    true, // Default: enabled
		intensity:  100,  // Default: highest intensity
		ctx:        ctx,
		cancel:     cancel,
		lastUpdate: time.Now(),
		burstMode:  false,
		burstCount: 0,
		idlePeriod: 0,
	}
}

// Start starts the traffic simulator
func (s *Simulator) Start() {
	if !s.enabled {
		return
	}
	
	go s.run()
}

// Stop stops the traffic simulator
func (s *Simulator) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// run executes the traffic simulation pattern
func (s *Simulator) run() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			s.simulatePattern()
		}
	}
}

// simulatePattern simulates traffic based on the selected pattern
func (s *Simulator) simulatePattern() {
	var interval time.Duration
	
	switch s.pattern {
	case PatternHTTPLike:
		// HTTP-like: Bursty traffic with request/response patterns
		interval = s.simulateHTTPLike()
		
	case PatternSSHLike:
		// SSH-like: Steady low-volume traffic with occasional bursts
		interval = s.simulateSSHLike()
		
	case PatternRandom:
		// Random: Completely random intervals
		interval = s.simulateRandom()
	}
	
	// Scale by intensity (0-100)
	actualInterval := time.Duration(float64(interval) * (float64(s.intensity) / 100.0))
	if actualInterval < 50*time.Millisecond {
		actualInterval = 50 * time.Millisecond
	}
	
	time.Sleep(actualInterval)
}

// simulateHTTPLike simulates HTTP-like traffic patterns
func (s *Simulator) simulateHTTPLike() time.Duration {
	// HTTP traffic: bursty with request/response patterns
	if s.burstMode {
		s.burstCount--
		if s.burstCount <= 0 {
			s.burstMode = false
			s.idlePeriod = time.Duration(s.rng.Int63n(int64(3*time.Second))) + 2*time.Second
		}
		// Quick bursts: 50-200ms between requests
		return time.Duration(s.rng.Int63n(int64(150*time.Millisecond))) + 50*time.Millisecond
	}
	
	// Check if it's time for a burst
	if time.Since(s.lastUpdate) > s.idlePeriod {
		s.burstMode = true
		s.burstCount = s.rng.Intn(10) + 5 // 5-15 requests per burst
		s.lastUpdate = time.Now()
	}
	
	// Normal HTTP interval: 1-3 seconds
	return time.Duration(s.rng.Int63n(int64(2*time.Second))) + 1*time.Second
}

// simulateSSHLike simulates SSH-like traffic patterns
func (s *Simulator) simulateSSHLike() time.Duration {
	// SSH traffic: steady with occasional keystroke bursts
	if s.burstMode {
		s.burstCount--
		if s.burstCount <= 0 {
			s.burstMode = false
		}
		// Keystroke bursts: 100-500ms
		return time.Duration(s.rng.Int63n(int64(400*time.Millisecond))) + 100*time.Millisecond
	}
	
	// Chance to start a burst (5% chance per cycle)
	if s.rng.Intn(100) < 5 {
		s.burstMode = true
		s.burstCount = s.rng.Intn(8) + 3 // 3-10 keystrokes per burst
	}
	
	// Normal SSH interval: 500ms - 3s
	return time.Duration(s.rng.Int63n(int64(2500*time.Millisecond))) + 500*time.Millisecond
}

// simulateRandom simulates completely random traffic patterns
func (s *Simulator) simulateRandom() time.Duration {
	// Random intervals with some structure
	if s.burstMode {
		s.burstCount--
		if s.burstCount <= 0 {
			s.burstMode = false
		}
		// Burst intervals: very short
		return time.Duration(s.rng.Int63n(int64(100*time.Millisecond))) + 10*time.Millisecond
	}
	
	// Chance to start a random burst (10% chance)
	if s.rng.Intn(100) < 10 {
		s.burstMode = true
		s.burstCount = s.rng.Intn(20) + 5 // 5-25 packets per burst
	}
	
	// Base random interval
	return time.Duration(s.rng.Int63n(int64(5*time.Second))) + 100*time.Millisecond
}

// SetIntensity sets the simulation intensity (0-100)
func (s *Simulator) SetIntensity(intensity int) {
	if intensity < 0 {
		intensity = 0
	}
	if intensity > 100 {
		intensity = 100
	}
	s.intensity = intensity
}

// SetEnabled enables or disables the simulator
func (s *Simulator) SetEnabled(enabled bool) {
	s.enabled = enabled
	if !enabled && s.cancel != nil {
		s.cancel()
	}
}

// GetPattern returns the current traffic pattern
func (s *Simulator) GetPattern() TrafficPattern {
	return s.pattern
}

// SetPattern sets the traffic pattern
func (s *Simulator) SetPattern(pattern TrafficPattern) {
	s.pattern = pattern
}

