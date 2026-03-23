package cloud

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// newStubConn creates a real (but unconnected) gRPC client connection suitable
// for pool bookkeeping tests. The connection targets a dummy address and uses
// insecure credentials so no network I/O is performed.
func newStubConn(t *testing.T) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient("passthrough:///localhost:0", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	return conn
}

func newTestPool(spawn func(url, authToken, providerVersion, terraformVersion string) (*grpc.ClientConn, error)) *ConnPool {
	p := NewConnPool("token", "v1", "tf1")
	p.spawnFunc = spawn
	return p
}

func TestGetConnection_Reuse(t *testing.T) {
	conn := newStubConn(t)
	var calls int
	pool := newTestPool(func(_, _, _, _ string) (*grpc.ClientConn, error) {
		calls++
		return conn, nil
	})

	c1, err := pool.GetConnection("https://cluster-a:443")
	require.NoError(t, err)
	c2, err := pool.GetConnection("https://cluster-a:443")
	require.NoError(t, err)

	assert.Same(t, c1, c2, "same URL should return same connection")
	assert.Equal(t, 1, calls, "spawnFunc should only be called once")
}

func TestGetConnection_DifferentURLs(t *testing.T) {
	pool := newTestPool(func(_, _, _, _ string) (*grpc.ClientConn, error) {
		return newStubConn(t), nil
	})

	c1, err := pool.GetConnection("https://cluster-a:443")
	require.NoError(t, err)
	c2, err := pool.GetConnection("https://cluster-b:443")
	require.NoError(t, err)

	assert.NotSame(t, c1, c2, "different URLs should return different connections")
}

func TestGetConnection_EvictsShutdown(t *testing.T) {
	first := newStubConn(t)
	second := newStubConn(t)
	var calls int
	pool := newTestPool(func(_, _, _, _ string) (*grpc.ClientConn, error) {
		calls++
		if calls == 1 {
			return first, nil
		}
		return second, nil
	})

	c1, err := pool.GetConnection("https://cluster-a:443")
	require.NoError(t, err)
	assert.Same(t, first, c1)

	// Close puts the connection into Shutdown state.
	require.NoError(t, first.Close())

	c2, err := pool.GetConnection("https://cluster-a:443")
	require.NoError(t, err)
	assert.Same(t, second, c2, "should get a new connection after shutdown")
	assert.Equal(t, 2, calls)
}

func TestGetConnection_ConcurrentSameURL(t *testing.T) {
	var spawnCalls atomic.Int32
	stub := newStubConn(t)
	pool := newTestPool(func(_, _, _, _ string) (*grpc.ClientConn, error) {
		spawnCalls.Add(1)
		time.Sleep(200 * time.Millisecond)
		return stub, nil
	})

	const goroutines = 50
	conns := make([]*grpc.ClientConn, goroutines)
	errs := make([]error, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	start := time.Now()
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			conns[idx], errs[idx] = pool.GetConnection("https://cluster-a:443")
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	for i := range goroutines {
		require.NoError(t, errs[i], "goroutine %d should not error", i)
		assert.Same(t, stub, conns[i], "goroutine %d should get the same connection", i)
	}
	assert.Equal(t, int32(1), spawnCalls.Load(), "singleflight should deduplicate to exactly 1 SpawnConn call")
	assert.Less(t, elapsed, 1*time.Second, "all goroutines should complete in roughly one spawn duration")
}

func TestGetConnection_CrossURLNonBlocking(t *testing.T) {
	pool := newTestPool(func(url, _, _, _ string) (*grpc.ClientConn, error) {
		if url == "https://slow-cluster:443" {
			time.Sleep(2 * time.Second)
		}
		return newStubConn(t), nil
	})

	var fastDone atomic.Bool
	var fastElapsed time.Duration
	var wg sync.WaitGroup
	wg.Add(2)

	// Start slow URL first.
	go func() {
		defer wg.Done()
		_, _ = pool.GetConnection("https://slow-cluster:443")
	}()

	// Give the slow goroutine a moment to enter singleflight.
	time.Sleep(10 * time.Millisecond)

	// Fast URL should not be blocked by slow URL.
	go func() {
		defer wg.Done()
		start := time.Now()
		_, err := pool.GetConnection("https://fast-cluster:443")
		fastElapsed = time.Since(start)
		require.NoError(t, err)
		fastDone.Store(true)
	}()

	wg.Wait()
	assert.True(t, fastDone.Load())
	assert.Less(t, fastElapsed, 500*time.Millisecond, "fast URL should not be blocked by slow URL dial")
}

func TestGetConnection_ConcurrentMixedURLs(t *testing.T) {
	var spawnCalls atomic.Int32
	pool := newTestPool(func(_, _, _, _ string) (*grpc.ClientConn, error) {
		spawnCalls.Add(1)
		time.Sleep(100 * time.Millisecond)
		return newStubConn(t), nil
	})

	urls := []string{
		"https://cluster-1:443",
		"https://cluster-2:443",
		"https://cluster-3:443",
		"https://cluster-4:443",
		"https://cluster-5:443",
	}

	const perURL = 20
	total := len(urls) * perURL
	conns := make([]*grpc.ClientConn, total)
	errs := make([]error, total)
	var wg sync.WaitGroup
	wg.Add(total)

	start := time.Now()
	for i := range total {
		go func(idx int) {
			defer wg.Done()
			conns[idx], errs[idx] = pool.GetConnection(urls[idx%len(urls)])
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	for i := range total {
		require.NoError(t, errs[i])
	}

	// All goroutines for the same URL should get the same connection.
	urlConns := make(map[string]*grpc.ClientConn)
	for i := range total {
		url := urls[i%len(urls)]
		if existing, ok := urlConns[url]; ok {
			assert.Same(t, existing, conns[i], "all goroutines for %s should share one conn", url)
		} else {
			urlConns[url] = conns[i]
		}
	}

	assert.Equal(t, int32(5), spawnCalls.Load(), "should dial exactly once per unique URL")
	assert.Less(t, elapsed, 1*time.Second, "all 5 URLs should dial in parallel")
}
