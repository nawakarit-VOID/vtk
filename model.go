// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

// Episode คือไฟล์วิดีโอ 1 ตอน
type Episode struct {
	FilePath        string  `json:"file_path"`
	FileName        string  `json:"file_name"`
	EpisodeNumber   int     `json:"episode_number"`
	Watched         bool    `json:"watched"`
	Exists          bool    `json:"exists"`           // false = ไฟล์ถูกลบไปแล้วจากดิสก์
	ResumeSeconds   float64 `json:"resume_seconds"`   // จุดที่เล่นค้างไว้ (วินาที) มาจากการ track ผ่าน mpv IPC, 0 = ไม่มีจุดค้าง
	DurationSeconds float64 `json:"duration_seconds"` // ความยาวไฟล์ (วินาที) รู้ได้ก็ต่อเมื่อเคยเล่นผ่าน mpv ในแอปนี้อย่างน้อย 1 ครั้ง, 0 = ยังไม่ทราบ
	LastPlayedAt    int64   `json:"last_played_at"`   // unix timestamp ล่าสุดที่มีการเล่นค้างไว้ (ResumeSeconds > 0), ใช้เรียงหน้า "ดูต่อ"
}

// Series คือกลุ่มของ Episode (1 โฟลเดอร์ = 1 ซีรีส์)
type Series struct {
	Name     string     `json:"name"`
	RootPath string     `json:"root_path"`
	IsRoot   bool       `json:"is_root"` // true = ไฟล์หลวม ๆ ที่อยู่ตรงโฟลเดอร์แม่ (ตัวที่กดสแกน) โดยตรง ไม่ใช่โฟลเดอร์ย่อย
	Starred  bool       `json:"starred"` // true = ติดดาวไว้ (เฉพาะโฟลเดอร์ลูก) จะถูกเลื่อนขึ้นมาอยู่บนสุดของกลุ่มโฟลเดอร์ลูก
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
