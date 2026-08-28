package system

type ProcessInfo struct {
	PID         int32
	User        string
	MemPct      float32
	CPUPct      float64
	Command     string
	State       string  // Running, Paused (SIGSTOP)
	Risk        string  // CRIT, WARN, OK
	Reclaimable uint64  // Bytes limpáveis com segurança
}

type SystemMetrics struct {
	TotalRAM    uint64
	UsedRAM     uint64
	FreeRAM     uint64
	CacheRAM    uint64
	SwapTotal   uint64
	SwapUsed    uint64
	Reclaimable uint64 // Cache + Buffers + Memória de swap limpa
}
