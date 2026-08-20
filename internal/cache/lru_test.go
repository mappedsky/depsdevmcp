package cache

import (
	"sync"
	"testing"
)

func TestLRUEvictsLeastRecentlyUsed(t *testing.T) {
	c := NewLRU[string, int](2)
	c.Add("a", 1)
	c.Add("b", 2)

	if _, ok := c.Get("a"); !ok {
		t.Fatal("expected a cache hit for a")
	}
	c.Add("c", 3)

	if _, ok := c.Get("b"); ok {
		t.Fatal("expected b to be evicted")
	}
	if got, ok := c.Get("a"); !ok || got != 1 {
		t.Fatalf("Get(a) = %v, %v; want 1, true", got, ok)
	}
	if got, ok := c.Get("c"); !ok || got != 3 {
		t.Fatalf("Get(c) = %v, %v; want 3, true", got, ok)
	}
}

func TestLRUUpdateDoesNotEvict(t *testing.T) {
	c := NewLRU[string, int](2)
	c.Add("a", 1)
	c.Add("b", 2)
	c.Add("a", 3)

	if got := c.Len(); got != 2 {
		t.Fatalf("Len() = %d; want 2", got)
	}
	if got, _ := c.Get("a"); got != 3 {
		t.Fatalf("Get(a) = %d; want 3", got)
	}
}

func TestLRUConcurrentAccess(t *testing.T) {
	c := NewLRU[int, int](16)
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Add(i, i)
			c.Get(i)
		}()
	}
	wg.Wait()

	if got := c.Len(); got > c.Capacity() {
		t.Fatalf("Len() = %d; exceeds capacity %d", got, c.Capacity())
	}
}
