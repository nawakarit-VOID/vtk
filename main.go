// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type appState struct {
	lib         *Library
	win         fyne.Window
	seriesList  *widget.List
	episodeList *widget.List
	selectedIdx int
	rootPath    string
}

func main() {
	a := app.New()
	w := a.NewWindow("Video Tracker")
	w.Resize(fyne.NewSize(900, 600))

	lib, err := LoadLibrary()
	if err != nil {
		lib = &Library{}
	}
	sortSeriesForDisplay(lib.SeriesList)

	state := &appState{lib: lib, win: w, selectedIdx: -1}

	scanBtn := widget.NewButtonWithIcon("สแกนโฟลเดอร์", theme.FolderOpenIcon(), func() {
		state.chooseAndScan()
	})
	organizeBtn := widget.NewButton("จัดกลุ่มไฟล์ชื่อคล้ายกัน", func() {
		state.organizeSimilar()
	})
	deleteSeriesBtn := widget.NewButtonWithIcon("ลบซีรีส์นี้", theme.DeleteIcon(), func() {
		state.confirmDeleteSeries()
	})
	deleteSeriesBtn.Importance = widget.DangerImportance
	toolbar := container.NewHBox(scanBtn, organizeBtn, deleteSeriesBtn)

	state.seriesList = widget.NewList(
		func() int { return len(state.lib.SeriesList) },
		func() fyne.CanvasObject {
			return widget.NewLabel("series")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			s := state.lib.SeriesList[i]
			label := o.(*widget.Label)
			label.SetText(fmt.Sprintf("%s\nดูล่าสุด: ตอน %d  (ดูแล้ว %d/%d ตอน)",
				s.Name, s.LastWatchedEpisode(), s.WatchedCount(), s.TotalCount()))
		},
	)

	state.episodeList = widget.NewList(
		func() int {
			if state.selectedIdx < 0 || state.selectedIdx >= len(state.lib.SeriesList) {
				return 0
			}
			return len(state.lib.SeriesList[state.selectedIdx].Episodes)
		},
		func() fyne.CanvasObject {
			check := widget.NewCheck("", nil)
			label := widget.NewLabel("episode")
			status := widget.NewLabel("")
			delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)
			return container.NewHBox(check, label, status, delBtn)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			if state.selectedIdx < 0 {
				return
			}
			series := state.lib.SeriesList[state.selectedIdx]
			ep := series.Episodes[i]

			row := o.(*fyne.Container)
			check := row.Objects[0].(*widget.Check)
			label := row.Objects[1].(*widget.Label)
			status := row.Objects[2].(*widget.Label)
			delBtn := row.Objects[3].(*widget.Button)

			label.SetText(fmt.Sprintf("ตอน %d - %s", ep.EpisodeNumber, ep.FileName))
			check.OnChanged = nil
			check.SetChecked(ep.Watched)
			check.OnChanged = func(v bool) {
				ep.Watched = v
				state.seriesList.Refresh()
				_ = SaveLibrary(state.lib)
			}
			delBtn.OnTapped = func() {
				state.confirmDeleteEpisode(series, ep)
			}

			if ep.Exists {
				status.SetText("")
			} else {
				status.Importance = widget.DangerImportance
				status.SetText("ไฟล์ถูกลบแล้ว")
			}
			status.Refresh()
		},
	)

	state.seriesList.OnSelected = func(id widget.ListItemID) {
		state.selectedIdx = id
		state.episodeList.Refresh()
	}

	split := container.NewHSplit(state.seriesList, state.episodeList)
	split.Offset = 0.32

	content := container.NewBorder(toolbar, nil, nil, nil, split)
	w.SetContent(content)
	w.ShowAndRun()
}

// organizeSimilar วิเคราะห์ episode ในซีรีส์ที่เลือกอยู่ หาไฟล์ที่ชื่อคล้ายกัน
// แล้วเสนอให้ผู้ใช้ยืนยันก่อนย้ายไฟล์จริงเข้าโฟลเดอร์ใหม่
func (s *appState) organizeSimilar() {
	if s.selectedIdx < 0 || s.selectedIdx >= len(s.lib.SeriesList) {
		dialog.ShowInformation("จัดกลุ่มไฟล์", "กรุณาเลือกซีรีส์ทางซ้ายก่อน", s.win)
		return
	}
	series := s.lib.SeriesList[s.selectedIdx]

	proposals := BuildProposals(series.Episodes)
	if len(proposals) == 0 {
		dialog.ShowInformation("จัดกลุ่มไฟล์", "ไม่พบไฟล์ที่ชื่อคล้ายกันมากพอที่จะจัดกลุ่มในซีรีส์นี้", s.win)
		return
	}

	var b strings.Builder
	b.WriteString("จะสร้างโฟลเดอร์และย้ายไฟล์ดังนี้:\n\n")
	for _, p := range proposals {
		fmt.Fprintf(&b, "📁 %s  (%d ไฟล์)\n", p.FolderName, len(p.Episodes))
		for _, e := range p.Episodes {
			fmt.Fprintf(&b, "    - %s\n", e.FileName)
		}
		b.WriteString("\n")
	}

	preview := widget.NewLabel(b.String())
	preview.Wrapping = fyne.TextWrapWord
	scroll := container.NewVScroll(preview)
	scroll.SetMinSize(fyne.NewSize(520, 420))

	confirmDialog := dialog.NewCustomConfirm(
		"จัดกลุ่มไฟล์ที่ชื่อคล้ายกัน", "ย้ายไฟล์", "ยกเลิก",
		scroll,
		func(ok bool) {
			if !ok {
				return
			}
			if err := s.applyGrouping(series, proposals); err != nil {
				dialog.ShowError(err, s.win)
			}
			sortSeriesForDisplay(s.lib.SeriesList)
			s.selectedIdx = -1 // โครงสร้างซีรีส์เปลี่ยนไปแล้ว (แยก/ย้ายไฟล์) ตำแหน่งเดิมใช้ไม่ได้อีกต่อไป
			s.seriesList.UnselectAll()
			s.seriesList.Refresh()
			s.episodeList.Refresh()
		},
		s.win,
	)
	confirmDialog.Resize(fyne.NewSize(560, 480))
	confirmDialog.Show()
}

// applyGrouping ย้ายไฟล์จริงตาม proposals แต่ละกลุ่ม สร้างโฟลเดอร์ย่อยใต้ series.RootPath
// อัปเดต path ของ episode และปรับ Library ให้ตรงกับโครงสร้างไฟล์ใหม่ (สถานะดูแล้วจะติดไปกับไฟล์)
func (s *appState) applyGrouping(orig *Series, proposals []GroupProposal) error {
	movedSet := map[*Episode]bool{}

	for _, p := range proposals {
		folderName := sanitizeFolderName(p.FolderName)
		newDir := filepath.Join(orig.RootPath, folderName)
		if err := os.MkdirAll(newDir, 0755); err != nil {
			return err
		}

		var target *Series
		for _, sr := range s.lib.SeriesList {
			if sr.RootPath == newDir {
				target = sr
				break
			}
		}
		if target == nil {
			target = &Series{Name: folderName, RootPath: newDir}
			s.lib.SeriesList = append(s.lib.SeriesList, target)
		}

		for _, ep := range p.Episodes {
			newPath := filepath.Join(newDir, ep.FileName)
			if ep.FilePath != newPath {
				if err := os.Rename(ep.FilePath, newPath); err != nil {
					return fmt.Errorf("ย้ายไฟล์ %s ไม่สำเร็จ: %w", ep.FileName, err)
				}
				ep.FilePath = newPath
			}
			target.Episodes = append(target.Episodes, ep)
			movedSet[ep] = true
		}
	}

	var remaining []*Episode
	for _, ep := range orig.Episodes {
		if !movedSet[ep] {
			remaining = append(remaining, ep)
		}
	}
	orig.Episodes = remaining

	if len(orig.Episodes) == 0 {
		var newList []*Series
		for _, sr := range s.lib.SeriesList {
			if sr != orig {
				newList = append(newList, sr)
			}
		}
		s.lib.SeriesList = newList
	}

	return SaveLibrary(s.lib)
}

func (s *appState) chooseAndScan() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, s.win)
			return
		}
		if uri == nil {
			return // ผู้ใช้กดยกเลิก
		}
		path := uri.Path()
		s.rootPath = path

		scanned, err := ScanFolder(path)
		if err != nil {
			dialog.ShowError(err, s.win)
			return
		}
		MergeScan(s.lib, scanned, path)
		sortSeriesForDisplay(s.lib.SeriesList)
		s.selectedIdx = -1 // ลำดับซีรีส์เปลี่ยนไปแล้ว ตำแหน่งที่เคยเลือกไว้ไม่ตรงของเดิมอีกต่อไป
		s.seriesList.UnselectAll()
		if err := SaveLibrary(s.lib); err != nil {
			dialog.ShowError(err, s.win)
		}
		s.seriesList.Refresh()
		s.episodeList.Refresh()
	}, s.win)
}

// showDeleteChoiceDialog แสดง dialog ที่มี 2 ปุ่มให้เลือก: "ลบไฟล์จริง" กับ "ลบแค่ลิสต์"
// พร้อมปุ่มยกเลิก ใช้ร่วมกันทั้งกรณีลบไฟล์เดี่ยวและลบทั้งซีรีส์/โฟลเดอร์แม่
func showDeleteChoiceDialog(win fyne.Window, title, message string, onDeleteReal func(), onListOnly func()) {
	var d *dialog.CustomDialog

	msgLabel := widget.NewLabel(message)
	msgLabel.Wrapping = fyne.TextWrapWord

	realBtn := widget.NewButtonWithIcon("ลบไฟล์จริง", theme.DeleteIcon(), func() {
		d.Hide()
		onDeleteReal()
	})
	realBtn.Importance = widget.DangerImportance

	listOnlyBtn := widget.NewButton("ลบแค่ลิสต์ (เก็บไฟล์ไว้)", func() {
		d.Hide()
		onListOnly()
	})

	content := container.NewVBox(
		msgLabel,
		widget.NewSeparator(),
		container.NewHBox(listOnlyBtn, realBtn),
	)

	d = dialog.NewCustom(title, "ยกเลิก", content, win)
	d.Resize(fyne.NewSize(480, 220))
	d.Show()
}

// confirmDeleteEpisode ถามว่าจะลบไฟล์วิดีโอ 1 ไฟล์แบบไหน: ลบจริงในดิสก์ หรือเอาออกจากลิสต์อย่างเดียว
func (s *appState) confirmDeleteEpisode(series *Series, ep *Episode) {
	msg := fmt.Sprintf(
		"\"%s\"\n\n• ลบไฟล์จริง = ลบไฟล์นี้ออกจากดิสก์จริง ย้อนกลับไม่ได้\n"+
			"• ลบแค่ลิสต์ = เอาออกจากรายการติดตาม ไฟล์บนดิสก์ยังอยู่เหมือนเดิม",
		ep.FileName,
	)
	showDeleteChoiceDialog(s.win, "ลบตอนนี้", msg,
		func() {
			if ep.Exists {
				if err := os.Remove(ep.FilePath); err != nil && !os.IsNotExist(err) {
					dialog.ShowError(fmt.Errorf("ลบไฟล์ %s ไม่สำเร็จ: %w", ep.FileName, err), s.win)
					return
				}
			}
			s.removeEpisodeFromLibrary(series, ep)
		},
		func() {
			s.removeEpisodeFromLibrary(series, ep)
		},
	)
}

// removeEpisodeFromLibrary เอา episode ออกจาก library เท่านั้น (ไม่แตะไฟล์บนดิสก์)
// ถ้าเป็นตอนสุดท้ายของซีรีส์ จะเอาซีรีส์นั้นออกจากลิสต์ไปด้วย (ไม่เหลือตอนให้แสดง)
func (s *appState) removeEpisodeFromLibrary(series *Series, ep *Episode) {
	var remaining []*Episode
	for _, e := range series.Episodes {
		if e != ep {
			remaining = append(remaining, e)
		}
	}
	series.Episodes = remaining

	if len(series.Episodes) == 0 {
		var newList []*Series
		for _, sr := range s.lib.SeriesList {
			if sr != series {
				newList = append(newList, sr)
			}
		}
		s.lib.SeriesList = newList
		s.selectedIdx = -1
		s.seriesList.UnselectAll()
	}

	if err := SaveLibrary(s.lib); err != nil {
		dialog.ShowError(err, s.win)
	}
	sortSeriesForDisplay(s.lib.SeriesList)
	s.seriesList.Refresh()
	s.episodeList.Refresh()
}

// confirmDeleteSeries ถามว่าจะลบซีรีส์ที่เลือกอยู่แบบไหน: ลบจริงในดิสก์ (ไฟล์/ทั้งโฟลเดอร์) หรือเอาออกจากลิสต์อย่างเดียว
// ใช้ได้กับทั้งโฟลเดอร์ย่อยทั่วไปและโฟลเดอร์แม่
func (s *appState) confirmDeleteSeries() {
	if s.selectedIdx < 0 || s.selectedIdx >= len(s.lib.SeriesList) {
		dialog.ShowInformation("ลบซีรีส์", "กรุณาเลือกซีรีส์ทางซ้ายก่อน", s.win)
		return
	}
	series := s.lib.SeriesList[s.selectedIdx]

	var realDesc string
	if series.IsRoot {
		realDesc = fmt.Sprintf("ลบไฟล์วิดีโอทั้งหมด %d ไฟล์ในนี้ออกจากดิสก์จริง (ไฟล์อื่นที่ไม่ใช่วิดีโอในโฟลเดอร์เดียวกันจะไม่ถูกแตะ)", series.TotalCount())
	} else {
		realDesc = fmt.Sprintf("ลบทั้งโฟลเดอร์ \"%s\" ออกจากดิสก์จริง (รวมไฟล์ทั้งหมด %d ไฟล์ข้างใน)", series.Name, series.TotalCount())
	}
	msg := fmt.Sprintf(
		"\"%s\"\n\n• ลบไฟล์จริง = %s ย้อนกลับไม่ได้\n"+
			"• ลบแค่ลิสต์ = เอาออกจากรายการติดตาม ไฟล์/โฟลเดอร์บนดิสก์ยังอยู่เหมือนเดิม",
		series.Name, realDesc,
	)
	if series.IsRoot {
		msg += "\n\n(นี่คือโฟลเดอร์แม่ ซีรีส์ย่อยที่อยู่ในโฟลเดอร์นี้จะถูกเอาออกจากลิสต์ติดตามไปด้วย ไม่ว่าจะเลือกลบแบบไหนก็ตาม)"
	}

	showDeleteChoiceDialog(s.win, "ลบซีรีส์", msg,
		func() {
			if series.IsRoot {
				for _, ep := range series.Episodes {
					if !ep.Exists {
						continue
					}
					if err := os.Remove(ep.FilePath); err != nil && !os.IsNotExist(err) {
						dialog.ShowError(fmt.Errorf("ลบไฟล์ %s ไม่สำเร็จ: %w", ep.FileName, err), s.win)
						return
					}
				}
			} else {
				if err := os.RemoveAll(series.RootPath); err != nil {
					dialog.ShowError(fmt.Errorf("ลบโฟลเดอร์ %s ไม่สำเร็จ: %w", series.RootPath, err), s.win)
					return
				}
			}
			s.removeSeriesFromLibrary(series)
		},
		func() {
			s.removeSeriesFromLibrary(series)
		},
	)
}

// removeSeriesFromLibrary เอาซีรีส์ออกจาก library เท่านั้น (ไม่แตะไฟล์บนดิสก์)
// ถ้าซีรีส์นี้เป็นโฟลเดอร์แม่ (IsRoot) จะเอาซีรีส์ย่อยที่อยู่ใต้โฟลเดอร์แม่นี้ออกจากลิสต์ไปด้วย
// (เอาออกจากลิสต์เท่านั้น ไม่ลบไฟล์/โฟลเดอร์จริงของลูกแต่อย่างใด)
func (s *appState) removeSeriesFromLibrary(series *Series) {
	var newList []*Series
	for _, sr := range s.lib.SeriesList {
		if sr == series {
			continue
		}
		if series.IsRoot && isUnderRoot(sr.RootPath, series.RootPath) {
			continue
		}
		newList = append(newList, sr)
	}
	s.lib.SeriesList = newList
	s.selectedIdx = -1
	s.seriesList.UnselectAll()

	if err := SaveLibrary(s.lib); err != nil {
		dialog.ShowError(err, s.win)
	}
	s.seriesList.Refresh()
	s.episodeList.Refresh()
}

// isUnderRoot ตรวจว่า path อยู่ใต้ root หรือไม่ (เป็นโฟลเดอร์ย่อยจริง ๆ ไม่ใช่แค่ขึ้นต้นด้วยตัวอักษรคล้ายกัน)
func isUnderRoot(path, root string) bool {
	sep := string(os.PathSeparator)
	if !strings.HasSuffix(root, sep) {
		root += sep
	}
	return strings.HasPrefix(path, root)
}

// sortSeriesForDisplay จัดลำดับซีรีส์สำหรับแสดงผล: โฟลเดอร์แม่ (IsRoot) มาก่อนเสมอ
// ไม่ว่าจะเคยสแกนมากี่รอบก็ตาม ส่วนที่เหลือเรียงตามตัวอักษร
func sortSeriesForDisplay(list []*Series) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].IsRoot != list[j].IsRoot {
			return list[i].IsRoot
		}
		return list[i].Name < list[j].Name
	})
}
