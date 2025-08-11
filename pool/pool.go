package objectPool

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrPoolClosed = errors.New("pool is closed")
	ErrPoolFull   = errors.New("pool is full")
)

// PoolConfig holds configuration for the object pool
type PoolConfig struct {
	MaxSize      int           // Maximum number of objects in pool
	InitialSize  int           // Initial number of objects to create
	IdleTimeout  time.Duration // How long idle objects stay in pool before being discarded
	WaitTimeout  time.Duration // Timeout for waiting on available objects
	TestOnBorrow bool          // Whether to test object health when borrowed
}

// ObjectFactory defines how to create and destroy objects
type ObjectFactory[T any] interface {
	Create() (T, error)
	Destroy(T) error
	Validate(T) bool
}

// ObjectPool manages a pool of reusable objects
type ObjectPool[T any] struct {
	config    PoolConfig
	factory   ObjectFactory[T]
	objects   chan T
	mu        sync.Mutex
	closed    bool
	created   int
	available int
}

// NewObjectPool creates a new object pool
func NewObjectPool[T any](config PoolConfig, factory ObjectFactory[T]) (*ObjectPool[T], error) {
	if config.MaxSize <= 0 {
		return nil, errors.New("max size must be positive")
	}
	if config.InitialSize > config.MaxSize {
		return nil, errors.New("initial size cannot exceed max size")
	}
	if config.WaitTimeout == 0 {
		config.WaitTimeout = 5 * time.Second
	}

	pool := &ObjectPool[T]{
		config:    config,
		factory:   factory,
		objects:   make(chan T, config.MaxSize),
		available: 0,
	}

	for i := 0; i < config.InitialSize; i++ {
		obj, err := factory.Create()
		if err != nil {
			return nil, err
		}
		pool.objects <- obj
		pool.created++
		pool.available++
	}

	if config.IdleTimeout > 0 {
		go pool.cleanupIdleObjects()
	}

	return pool, nil
}

// Get borrows an object from the pool with context
func (p *ObjectPool[T]) Get(ctx context.Context) (T, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		var zero T
		return zero, ErrPoolClosed
	}
	p.mu.Unlock()

	for {
		select {
		case obj := <-p.objects:
			p.mu.Lock()
			p.available--
			p.mu.Unlock()

			if p.config.TestOnBorrow && !p.factory.Validate(obj) {
				_ = p.factory.Destroy(obj)
				p.mu.Lock()
				p.created--
				p.mu.Unlock()
				continue
			}
			return obj, nil

		default:
			p.mu.Lock()
			if p.created < p.config.MaxSize {
				obj, err := p.factory.Create()
				if err != nil {
					p.mu.Unlock()
					var zero T
					return zero, err
				}
				p.created++
				p.available++
				p.mu.Unlock()
				return obj, nil
			}
			p.mu.Unlock()

			select {
			case obj := <-p.objects:
				p.mu.Lock()
				p.available--
				p.mu.Unlock()
				return obj, nil
			case <-ctx.Done():
				var zero T
				return zero, ctx.Err()
			case <-time.After(p.config.WaitTimeout):
				var zero T
				return zero, errors.New("timeout waiting for object")
			}
		}
	}
}

// Put returns an object to the pool
func (p *ObjectPool[T]) Put(obj T) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return p.factory.Destroy(obj)
	}

	select {
	case p.objects <- obj:
		p.available++
		return nil
	default:
		p.created--
		return p.factory.Destroy(obj)
	}
}

// Close releases all resources in the pool
func (p *ObjectPool[T]) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	close(p.objects)

	var errs []error
	for obj := range p.objects {
		if err := p.factory.Destroy(obj); err != nil {
			errs = append(errs, err)
		}
		p.mu.Lock()
		p.created--
		p.available--
		p.mu.Unlock()
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// Stats returns pool statistics
func (p *ObjectPool[T]) Stats() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.available, p.created
}

// cleanupIdleObjects periodically removes idle objects
func (p *ObjectPool[T]) cleanupIdleObjects() {
	ticker := time.NewTicker(p.config.IdleTimeout)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return
		}
		p.mu.Unlock()

		select {
		case obj := <-p.objects:
			if !p.factory.Validate(obj) {
				_ = p.factory.Destroy(obj)
				p.mu.Lock()
				p.created--
				p.available--
				p.mu.Unlock()
			} else {
				select {
				case p.objects <- obj:
				default:
					_ = p.factory.Destroy(obj)
					p.mu.Lock()
					p.created--
					p.available--
					p.mu.Unlock()
				}
			}
		default:
		}
	}
}
