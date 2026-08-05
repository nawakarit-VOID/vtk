// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// tabState คือ UI + สถานะทั้งหมดของ 1 แท็บ (1 แท็บ = 1 โฟลเดอร์แม่ที่เคยสแกน)
type tabState struct {
	rootPath      string
	tabItem       *container.TabItem
	seriesBox     *fyne.Container // VBox ของแถวซีรีส์ในแท็บนี้
	seriesRows    []*seriesRow
	searchQuery   string            // คำค้นหาของแท็บนี้ (แยกจากแท็บอื่น)
	episodeBox    *fyne.Container   // VBox ของแถวตอนในแท็บนี้
	episodeScroll *container.Scroll // ตัวครอบ episodeBox เก็บไว้เพื่อสั่ง Refresh/เลื่อนกลับบนสุดได้ตรง ๆ
}

// seriesRootKey คำนวณว่าซีรีส์นี้ "เป็นของโฟลเดอร์แม่ไหน" (ใช้จัดกลุ่มเข้าแท็บที่ถูกต้อง)
//   - ถ้าเป็นโฟลเดอร์แม่เอง (IsRoot) -> คือ RootPath ของตัวมันเอง
//   - ถ้าเป็นโฟลเดอร์ย่อย -> คือโฟลเดอร์ที่อยู่เหนือมันขึ้นไป 1 ชั้น (เพราะ ScanFolder สแกนลึกแค่ชั้นเดียว
//     โฟลเดอร์ย่อยทุกตัวจึงเป็นลูกโดยตรงของโฟลเดอร์แม่ที่ถูกสแกนเสมอ)
func seriesRootKey(sr *Series) string {
	if sr.IsRoot {
		return sr.RootPath
	}
	return filepath.Dir(sr.RootPath)
}

// ensureTab สร้างแท็บใหม่สำหรับโฟลเดอร์แม่ path นี้ ถ้ายังไม่เคยมีแท็บนี้มาก่อน
// (ไม่ได้เลือกโฟกัสแท็บให้อัตโนมัติ ผู้เรียกต้องสั่ง tabsWidget.Select เองถ้าต้องการ)
func (s *appState) ensureTab(path string) *tabState {
	if t, ok := s.tabs[path]; ok {
		return t
	}

	t := &tabState{rootPath: path}
	t.seriesBox = container.NewVBox()
	t.episodeBox = container.NewVBox()
	t.episodeScroll = container.NewScroll(t.episodeBox)
	t.episodeScroll.Direction = container.ScrollBoth

	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("ค้นหาชื่อซีรีส์...")
	searchEntry.OnChanged = func(text string) {
		t.searchQuery = text
		s.refreshSeriesRows()
	}
	seriesScroll := container.NewVScroll(t.seriesBox)
	seriesPanel := container.NewBorder(searchEntry, nil, nil, nil, seriesScroll)

	split := container.NewHSplit(seriesPanel, t.episodeScroll)
	split.Offset = 0.38

	t.tabItem = container.NewTabItem(filepath.Base(path), split)

	s.tabs[path] = t
	s.tabsWidget.Append(t.tabItem)
	return t
}
