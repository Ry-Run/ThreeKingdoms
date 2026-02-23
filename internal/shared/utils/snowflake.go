package utils

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// 2024-01-01 00:00:00 UTC，单位毫秒
	snowflakeEpochMilli int64 = 1704067200000

	nodeBits uint8 = 10
	seqBits  uint8 = 12

	maxNodeID int64 = -1 ^ (-1 << nodeBits)
	maxSeq    int64 = -1 ^ (-1 << seqBits)

	nodeShift uint8 = seqBits
	timeShift uint8 = nodeBits + seqBits
)

type Snowflake struct {
	mu     sync.Mutex
	nodeID int64
	lastTS int64
	seq    int64
}

func NewSnowflake(nodeID int64) (*Snowflake, error) {
	if nodeID < 0 || nodeID > maxNodeID {
		return nil, fmt.Errorf("snowflake node id out of range: %d", nodeID)
	}
	return &Snowflake{nodeID: nodeID}, nil
}

func (s *Snowflake) NextID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	ts := time.Now().UnixMilli()
	if ts < s.lastTS {
		// 时钟回拨时不回退，保持单调递增。
		ts = s.lastTS
	}

	if ts == s.lastTS {
		s.seq = (s.seq + 1) & maxSeq
		if s.seq == 0 {
			ts = waitNextMillisecond(s.lastTS)
		}
	} else {
		s.seq = 0
	}

	s.lastTS = ts
	return ((ts - snowflakeEpochMilli) << timeShift) | (s.nodeID << nodeShift) | s.seq
}

func waitNextMillisecond(lastTS int64) int64 {
	ts := time.Now().UnixMilli()
	for ts <= lastTS {
		ts = time.Now().UnixMilli()
	}
	return ts
}

var (
	defaultSnowflakeOnce sync.Once
	defaultSnowflake     *Snowflake
	defaultSnowflakeErr  error
)

func DefaultSnowflake() (*Snowflake, error) {
	defaultSnowflakeOnce.Do(func() {
		nodeID := int64(1)
		raw := strings.TrimSpace(os.Getenv("SNOWFLAKE_NODE_ID"))
		if raw != "" {
			parsed, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				defaultSnowflakeErr = fmt.Errorf("invalid SNOWFLAKE_NODE_ID: %w", err)
				return
			}
			nodeID = parsed
		}
		defaultSnowflake, defaultSnowflakeErr = NewSnowflake(nodeID)
	})
	return defaultSnowflake, defaultSnowflakeErr
}

func NextSnowflakeID() (int64, error) {
	gen, err := DefaultSnowflake()
	if err != nil {
		return 0, err
	}
	if gen == nil {
		return 0, errors.New("snowflake generator is nil")
	}
	return gen.NextID(), nil
}
