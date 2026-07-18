package main

// Episode คือไฟล์วิดีโอ 1 ตอน
type Episode struct {
	FilePath      string `json:"file_path"`
	FileName      string `json:"file_name"`
	EpisodeNumber int    `json:"episode_number"`
	Watched       bool   `json:"watched"`
	Exists        bool   `json:"exists"` // false = ไฟล์ถูกลบไปแล้วจากดิสก์
}

// Series คือกลุ่มของ Episode (1 โฟลเดอร์ = 1 ซีรีส์)
type Series struct {
	Name     string     `json:"name"`
	RootPath string     `json:"root_path"`
	Episodes []*Episode `json:"episodes"`
}

// LastWatchedEpisode คืนเลขตอนล่าสุดที่ดูแล้ว (ตอนที่มีเลขสูงสุดที่ Watched = true)
func (s *Series) LastWatchedEpisode() int {
	last := 0
	for _, e := range s.Episodes {
		if e.Watched && e.EpisodeNumber > last {
			last = e.EpisodeNumber
		}
	}
	return last
}

func (s *Series) TotalCount() int {
	return len(s.Episodes)
}

func (s *Series) WatchedCount() int {
	c := 0
	for _, e := range s.Episodes {
		if e.Watched {
			c++
		}
	}
	return c
}

// Library คือฐานข้อมูลทั้งหมดที่จะถูกเซฟลง JSON
type Library struct {
	SeriesList []*Series `json:"series_list"`
}
