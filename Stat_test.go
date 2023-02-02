package gocore

import "testing"

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
