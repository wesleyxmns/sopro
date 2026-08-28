package memory

// Snapshot represents a point-in-time view of system memory.
type Snapshot struct {
	Total       uint64
	Used        uint64
	Available   uint64
	Free        uint64
	Cache       uint64
	SwapTotal   uint64
	SwapUsed    uint64
	Reclaimable uint64
	Pressure    Pressure
}
