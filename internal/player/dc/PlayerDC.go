package dc

import (
	"context"
	"errors"
	"sync"
	"time"

	"ThreeKingdoms/internal/player/entity"
	"ThreeKingdoms/internal/player/service/port"
)

type PlayerID = entity.PlayerID

var (
	ErrClosed     = errors.New("player dc closed")
	ErrNoRepo     = errors.New("player repository is nil")
	ErrWriterDone = errors.New("player dc writer stopped")
)

type PlayerDC struct {
	repo       port.PlayerRepository
	entity     *entity.PlayerEntity
	flushEvery time.Duration

	mu sync.Mutex

	// coalescing：只保留最新快照
	pending *entity.PlayerEntitySnap

	// 版本控制
	version   uint64 // 已生成的最新版本
	persisted uint64 // 已成功落库的最新版本

	closed bool

	// 通知通道
	wake chan struct{} // 有 pending 可消费
	// persisted 推进广播：每次推进时 close(old)+new，一个推进可唤醒全部等待者
	persistNotify chan struct{}
	stop          chan struct{} // 请求停止 writer
	done          chan struct{} // writer 已退出
}

func NewPlayerDC(repo port.PlayerRepository) *PlayerDC {
	d := &PlayerDC{
		repo:          repo,
		flushEvery:    3 * time.Second,
		wake:          make(chan struct{}, 1),
		persistNotify: make(chan struct{}),
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
	}
	go d.writerLoop()
	return d
}

// ---------------- Public API ----------------

func (d *PlayerDC) Load(ctx context.Context, playerID PlayerID) (*entity.PlayerEntity, error) {
	if d.repo == nil {
		return nil, ErrNoRepo
	}
	player, err := d.repo.LoadPlayer(ctx, playerID)
	if err != nil {
		return nil, err
	}
	// 约定：Load 在 actor 串行上下文调用（Started/init）
	d.entity = player
	return player, nil
}

func (d *PlayerDC) Entity() *entity.PlayerEntity { return d.entity }
func (d *PlayerDC) FlushEvery() time.Duration    { return d.flushEvery }

func (d *PlayerDC) IsDirty() bool {
	if d.entity == nil {
		return false
	}
	return d.entity.Dirty()
}

// Tick：生成快照并异步落库（不等待落库完成）
// 返回：本次生成的版本（0 表示无脏/无快照）
func (d *PlayerDC) Tick() (uint64, error) {
	if !d.IsDirty() {
		return 0, nil
	}
	if d.repo == nil {
		return 0, ErrNoRepo
	}
	s, ok, err := d.buildNextSnapshot()
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	d.enqueueLatest(s)
	return s.Version, nil
}

// FlushSync：生成快照并阻塞等待“该版本（或更高版本）已成功落库”
func (d *PlayerDC) FlushSync(ctx context.Context) error {
	ctx = normalizeContext(ctx)

	target, err := d.Tick()
	if err != nil {
		return err
	}
	if target == 0 {
		return nil
	}
	return d.waitPersisted(ctx, target)
}

func (d *PlayerDC) Close(ctx context.Context) error {
	ctx = normalizeContext(ctx)

	// 尽最大努力同步 flush（不保证一定成功，取决于 ctx）
	_ = d.FlushSync(ctx)

	d.mu.Lock()
	if !d.closed {
		d.closed = true
		close(d.stop)
	}
	d.mu.Unlock()

	select {
	case <-d.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ---------------- Internals: snapshot & enqueue ----------------

func (d *PlayerDC) buildNextSnapshot() (*entity.PlayerEntitySnap, bool, error) {
	if d.entity == nil || !d.IsDirty() {
		return nil, false, nil
	}

	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil, false, ErrClosed
	}
	d.version++
	v := d.version
	d.mu.Unlock()

	s := entity.NewPlayerEntitySnap(v, d.entity)

	// 注意：生成快照即清脏（write-behind + coalescing 模式）
	d.entity.ClearDirty()
	return s, true, nil
}

func (d *PlayerDC) enqueueLatest(s *entity.PlayerEntitySnap) {
	if s == nil {
		return
	}

	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	if d.pending == nil || d.pending.Version < s.Version {
		d.pending = s
	}
	d.mu.Unlock()

	// 唤醒 writer（合并通知）
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

func (d *PlayerDC) popPending() *entity.PlayerEntitySnap {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.pending
	d.pending = nil
	return s
}

func (d *PlayerDC) requeueOnError(s *entity.PlayerEntitySnap) {
	if s == nil {
		return
	}

	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	// 已生成更高版本则当前快照可丢弃（更高版本会覆盖）
	if s.Version < d.version {
		d.mu.Unlock()
		return
	}
	if d.pending == nil || d.pending.Version < s.Version {
		d.pending = s
	}
	d.mu.Unlock()

	select {
	case d.wake <- struct{}{}:
	default:
	}
}

func (d *PlayerDC) hasNewerVersion(version uint64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.version > version
}

// ---------------- Internals: persistence waiting ----------------

// waitPersisted：等待 persisted >= targetVersion（或 ctx 超时/取消）
func (d *PlayerDC) waitPersisted(ctx context.Context, target uint64) error {
	for {
		d.mu.Lock()
		if d.persisted >= target {
			d.mu.Unlock()
			return nil
		}
		notify := d.persistNotify
		d.mu.Unlock()

		select {
		case <-notify:
			// persisted 有进展，继续循环检查条件
			continue
		case <-d.done:
			// writer 已退出，再检查一次 persisted
			d.mu.Lock()
			p2 := d.persisted
			d.mu.Unlock()
			if p2 >= target {
				return nil
			}
			return ErrWriterDone
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (d *PlayerDC) markPersisted(version uint64) {
	d.mu.Lock()
	if version <= d.persisted {
		d.mu.Unlock()
		return
	}
	d.persisted = version
	oldNotify := d.persistNotify
	d.persistNotify = make(chan struct{})
	d.mu.Unlock()

	close(oldNotify)
}

// ---------------- Writer loop ----------------

func (d *PlayerDC) writerLoop() {
	defer close(d.done)

	for {
		select {
		case <-d.wake:
			d.consumePending()
		case <-d.stop:
			// 尽可能消费最后一轮 pending（Close 已经先尝试 FlushSync）
			d.consumePending()
			return
		}
	}
}

func (d *PlayerDC) consumePending() {
	for {
		s := d.popPending()
		if s == nil {
			return
		}

		// 只落最新：如果已经有更新版本生成，则当前快照可跳过
		if d.hasNewerVersion(s.Version) {
			continue
		}

		// 写库失败重试：若期间出现更高版本，会自然被覆盖/跳过
		if err := d.repo.Save(context.TODO(), s); err != nil {
			d.requeueOnError(s)
			time.Sleep(200 * time.Millisecond)
			continue
		}

		// 成功：推进 persisted，唤醒 FlushSync
		d.markPersisted(s.Version)
	}
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
