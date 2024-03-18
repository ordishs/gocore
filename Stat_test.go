package gocore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// func TestNewStat(t *testing.T) {
// 	now := time.Now().UTC()
// 	_ = now

// 	s1 := NewStat("level1 - ignore children", true)
// 	_ = s1
// 	// s1.processTime(now, 1*time.Second.Nanoseconds())

// 	s2 := NewStat("level1 - include children", false)
// 	_ = s2
// 	// s2.processTime(now, 1*time.Second.Nanoseconds())

// 	s1.NewStat("c1").processTime(now, 1*time.Second)
// 	s2.NewStat("c2").processTime(now, 1*time.Second)

// 	print(t, RootStat)

// 	// t.Logf("%#v", RootStat)

// 	// for _, v := range RootStat.children {
// 	// 	fmt.Println(v)
// 	// }

// 	StartStatsServer("localhost:9006")

// 	ch := make(chan bool)
// 	<-ch

// }

// func print(t *testing.T, s *Stat) {
// 	t.Log(s)

// 	for _, v := range s.children {
// 		print(t, v)
// 	}
// }

func TestThousands(t *testing.T) {
	var num int64 = 123

	s := addThousandsOperator(num)

	t.Log(s)
}

func TestRanges(t *testing.T) {
	s := NewStat("test")
	s.AddRanges(100, 1_000, 10_000, 0)
	st := CurrentTime()

	_ = s.AddTimeForRange(st, 5)
	_ = s.AddTimeForRange(st, 500)
	_ = s.AddTimeForRange(st, 999)
	_ = s.AddTimeForRange(st, 1_000)
	_ = s.AddTimeForRange(st, 5_000_000)

	s.childMap.Range(func(key, value interface{}) bool {
		item := value.(*Stat)
		t.Logf("%-14s: %s", key, addThousandsOperator(item.count))
		return true
	})
	t.Logf("%-14s: %s", s.key, addThousandsOperator(s.count))

	c, _ := s.childMap.Load("0 - 100")
	assert.Equal(t, int64(1), c.(*Stat).count)

	c, _ = s.childMap.Load("100 - 1,000")
	assert.Equal(t, int64(2), c.(*Stat).count)

	c, _ = s.childMap.Load("1,000 - 10,000")
	assert.Equal(t, int64(1), c.(*Stat).count)

	c, _ = s.childMap.Load("10,000 -")
	assert.Equal(t, int64(1), c.(*Stat).count)

	assert.Equal(t, int64(5), s.count)
}
