package pm

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func TestPatternCacheBounded(t *testing.T) {
	var c Cache
	for i := 0; i < maxCachedPatterns*3; i++ {
		pat := fmt.Sprintf("x%d", i)
		ms, err := c.Find(pat, []byte("a "+pat+" b"), 0, -1)
		if err != nil || len(ms) != 1 {
			t.Fatalf("pattern %q: matches=%d err=%v", pat, len(ms), err)
		}
	}
	if n := atomic.LoadInt32(&c.n); n > maxCachedPatterns {
		t.Fatalf("cache holds %d patterns, max is %d", n, maxCachedPatterns)
	}
}
