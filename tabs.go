// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
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

// tabTitleFor คืนชื่อที่ควรใช้แสดงเป็นชื่อแท็บของ root path นี้ (ที่ตั้งเองไว้ ถ้ามี ไม่งั้นใช้ชื่อโฟลเดอร์)
func (s *appState) tabTitleFor(path string) string {
	if s.lib.TabTitles != nil {
		if t, ok := s.lib.TabTitles[path]; ok && t != "" {
			return t
		}
	}
	return filepath.Base(path)
}

// ensureTab สร้างแท็บใหม่สำหรับโฟลเดอร์แม่ path นี้ ถ้ายังไม่เคยมีแท็บนี้มาก่อน
// (ไม่ได้เลือกโฟกัสแท็บให้อัตโนมัติ ผู้เรียกต้องสั่ง tabsWidget.Select เองถ้าต้องการ)
// แต่ละแท็บมีปุ่ม "แก้ชื่อ"/"ลบแท็บนี้" ของตัวเองอยู่ข้าง ๆ ช่องค้นหา (แทนคลิกขวาที่หัวแท็บ
// เพราะ Fyne ไม่เปิดให้ดักคลิกขวาที่หัวแท็บของ AppTabs ได้โดยตรง)
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

	renameTabBtn := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() {
		s.renameTab(t)
	})
	deleteTabBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		s.confirmDeleteTab(t)
	})
	deleteTabBtn.Importance = widget.DangerImportance

	topRow := container.NewBorder(nil, nil, nil, container.NewHBox(renameTabBtn, deleteTabBtn), searchEntry)
	seriesScroll := container.NewVScroll(t.seriesBox)
	seriesPanel := container.NewBorder(topRow, nil, nil, nil, seriesScroll)

	split := container.NewHSplit(seriesPanel, t.episodeScroll)
	split.Offset = 0.38

	t.tabItem = container.NewTabItem(s.tabTitleFor(path), split)

	s.tabs[path] = t
	s.tabsWidget.Append(t.tabItem)
	return t
}

// renameTab ให้ผู้ใช้ตั้งชื่อแท็บเอง (แค่ชื่อที่แสดง ไม่เปลี่ยนชื่อโฟลเดอร์จริงในดิสก์) เซฟไว้ถาวร
func (s *appState) renameTab(t *tabState) {
	entry := widget.NewEntry()
	entry.SetText(t.tabItem.Text)

	content := container.NewVBox(
		widget.NewLabel("ชื่อนี้จะใช้แสดงเป็นชื่อแท็บเท่านั้น ไม่เปลี่ยนชื่อโฟลเดอร์จริงในดิสก์"),
		entry,
	)

	d := dialog.NewCustomConfirm("แก้ชื่อแท็บ", "บันทึก", "ยกเลิก", content, func(ok bool) {
		if !ok {
			return
		}
		newName := strings.TrimSpace(entry.Text)
		if newName == "" {
			dialog.ShowInformation("แก้ชื่อแท็บ", "ชื่อห้ามเว้นว่าง", s.win)
			return
		}
		if s.lib.TabTitles == nil {
			s.lib.TabTitles = map[string]string{}
		}
		s.lib.TabTitles[t.rootPath] = newName
		t.tabItem.Text = newName
		s.tabsWidget.Refresh()
		if err := SaveLibrary(s.lib); err != nil {
			dialog.ShowError(err, s.win)
		}
	}, s.win)
	d.Resize(fyne.NewSize(420, 160))
	d.Show()
}

// confirmDeleteTab ถามว่าจะลบทั้งแท็บนี้แบบไหน: ลบไฟล์จริงทั้งหมดในแท็บ (ย้ายไปถังขยะ) หรือเอาออกจากลิสต์อย่างเดียว
func (s *appState) confirmDeleteTab(t *tabState) {
	var seriesInTab []*Series
	for _, sr := range s.lib.SeriesList {
		if seriesRootKey(sr) == t.rootPath {
			seriesInTab = append(seriesInTab, sr)
		}
	}

	msg := fmt.Sprintf(
		"แท็บ \"%s\" มีทั้งหมด %d ซีรีส์\n\n"+
			"• ลบไฟล์จริง = ย้ายไฟล์/โฟลเดอร์ทั้งหมดในแท็บนี้ไปถังขยะ (กู้คืนได้ภายหลังถ้าจำเป็น)\n"+
			"• ลบแค่ลิสต์ = เอาแท็บนี้ออกจากรายการติดตาม ไฟล์/โฟลเดอร์บนดิสก์ยังอยู่เหมือนเดิม",
		t.tabItem.Text, len(seriesInTab),
	)

	showDeleteChoiceDialog(s.win, "ลบแท็บนี้", msg,
		func() {
			for _, sr := range seriesInTab {
				if sr.IsRoot {
					for _, ep := range sr.Episodes {
						if !ep.Exists {
							continue
						}
						if err := moveToTrash(ep.FilePath); err != nil {
							dialog.ShowError(err, s.win)
						}
					}
				} else {
					if err := moveToTrash(sr.RootPath); err != nil {
						dialog.ShowError(err, s.win)
					}
				}
			}
			s.removeTab(t)
		},
		func() {
			s.removeTab(t)
		},
	)
}

// removeTab เอาแท็บนี้และซีรีส์ทั้งหมดที่อยู่ในแท็บนี้ออกจากลิสต์/หน้าจอ (ไม่แตะไฟล์บนดิสก์ - ถ้าจะลบไฟล์จริง
// ต้องทำก่อนเรียกฟังก์ชันนี้ ดู confirmDeleteTab)
func (s *appState) removeTab(t *tabState) {
	var newList []*Series
	for _, sr := range s.lib.SeriesList {
		if seriesRootKey(sr) != t.rootPath {
			newList = append(newList, sr)
		}
	}
	s.lib.SeriesList = newList
	s.selectedIdx = -1

	if s.lib.TabTitles != nil {
		delete(s.lib.TabTitles, t.rootPath)
	}

	s.tabsWidget.Remove(t.tabItem)
	delete(s.tabs, t.rootPath)

	if err := SaveLibrary(s.lib); err != nil {
		dialog.ShowError(err, s.win)
	}
	s.refreshSeriesRows()
	s.refreshEpisodeRows()
}
