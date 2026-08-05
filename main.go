// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
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
	tabs        map[string]*tabState // 1 แท็บ ต่อ 1 โฟลเดอร์แม่ที่เคยสแกน (key = root path)
	tabsWidget  *container.AppTabs
	selectedIdx int
	rootPath    string
}

// โหลด icon
func loadIcon(size int) fyne.Resource {
	var file string

	switch {
	case size >= 512:
		file = "assets/icons/icon-512.png" ///ที่อยู่
	case size >= 256:
		file = "assets/icons/icon-256.png"
	case size >= 128:
		file = "assets/icons/icon-128.png"
	default:
		file = "assets/icons/icon-64.png"
	}

	data, err := iconFS.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot load icon %s: %v\n", file, err)
		return fyne.NewStaticResource("missing-icon", nil)
	}
	if len(data) == 0 {
		fmt.Fprintf(os.Stderr, "warning: icon %s is empty\n", file)
		return fyne.NewStaticResource("empty-icon", nil)
	}
	return fyne.NewStaticResource(file, data)
}

//go:embed assets/icons/*
var iconFS embed.FS

//go:embed assets/font/Itim-Regular.ttf
var fontItim []byte
var myFont = fyne.NewStaticResource("Itim-Regular.ttf", fontItim)

// = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
// # Main #
// = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func main() {
	a := app.NewWithID("com.nawakarit.vtk")
	a.Settings().SetTheme(&MyTheme{})
	icon := loadIcon(64)
	a.SetIcon(icon)

	w := a.NewWindow("Video Tracker")
	w.Resize(fyne.NewSize(900, 600))
	w.SetIcon(icon)

	lib, err := LoadLibrary()
	if err != nil {
		lib = &Library{}
	}
	sortSeriesForDisplay(lib.SeriesList)

	state := &appState{
		lib:         lib,
		win:         w,
		tabs:        map[string]*tabState{},
		tabsWidget:  container.NewAppTabs(),
		selectedIdx: -1,
	}

	scanBtn := widget.NewButtonWithIcon("สแกนโฟลเดอร์", theme.FolderOpenIcon(), func() {
		state.chooseAndScan()
	})
	refreshAllBtn := widget.NewButtonWithIcon("รีเฟรชโฟลเดอร์แม่ทั้งหมด", theme.ViewRefreshIcon(), func() {
		state.refreshAllRootFolders()
	})
	organizeBtn := widget.NewButton("จัดกลุ่มไฟล์ชื่อคล้ายกัน", func() {
		state.organizeSimilar()
	})
	deleteSeriesBtn := widget.NewButtonWithIcon("ลบซีรีส์นี้", theme.DeleteIcon(), func() {
		state.confirmDeleteSeries()
	})
	deleteSeriesBtn.Importance = widget.DangerImportance
	playSeriesBtn := widget.NewButtonWithIcon("เล่นซีรีส์นี้", theme.MediaPlayIcon(), func() {
		state.playSelectedSeries()
	})
	renameSeriesBtn := widget.NewButtonWithIcon("แก้ชื่อ", theme.DocumentCreateIcon(), func() {
		state.renameSelectedSeries()
	})
	statsBtn := widget.NewButtonWithIcon("สถิติ", theme.InfoIcon(), func() {
		state.showStatistics()
	})
	continueBtn := widget.NewButtonWithIcon("ดูต่อ", theme.HistoryIcon(), func() {
		state.showContinueWatching()
	})
	toolbar := container.NewHBox(scanBtn, refreshAllBtn, organizeBtn, playSeriesBtn, renameSeriesBtn, deleteSeriesBtn, statsBtn, continueBtn)

	// สร้างแท็บให้ครบทุกโฟลเดอร์แม่ที่เคยสแกนไว้จากเซสชันก่อนหน้า (ถ้ามี) เรียงตามชื่อให้ลำดับคงที่ทุกครั้งที่เปิดแอป
	rootSet := map[string]bool{}
	for _, sr := range lib.SeriesList {
		rootSet[seriesRootKey(sr)] = true
	}
	var existingRoots []string
	for p := range rootSet {
		existingRoots = append(existingRoots, p)
	}
	sort.Strings(existingRoots)
	for _, p := range existingRoots {
		state.ensureTab(p)
	}

	state.refreshSeriesRows()
	state.refreshEpisodeRows()

	content := container.NewBorder(toolbar, nil, nil, nil, state.tabsWidget)
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

	var siblings []*Series
	existingRootPaths := map[string]bool{}
	for _, sr := range s.lib.SeriesList {
		if sr == series || sr.IsRoot {
			continue
		}
		if filepath.Dir(sr.RootPath) == series.RootPath {
			siblings = append(siblings, sr)
			existingRootPaths[sr.RootPath] = true
		}
	}

	// เพิ่มโฟลเดอร์ย่อยที่มีอยู่จริงในดิสก์แต่ยังไม่มีไฟล์วิดีโอข้างใน (เลยไม่เคยถูก track ไว้ในลิสต์มาก่อน)
	// ให้เป็นตัวเลือกปลายทางได้ด้วย เผื่อมีไฟล์หลวม ๆ ชื่อคล้ายโฟลเดอร์เปล่านั้นอยู่
	if entries, err := os.ReadDir(series.RootPath); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			fullPath := filepath.Join(series.RootPath, entry.Name())
			if existingRootPaths[fullPath] {
				continue // มีอยู่แล้วในลิสต์ ไม่ต้องเพิ่มซ้ำ
			}
			siblings = append(siblings, &Series{Name: entry.Name(), RootPath: fullPath})
		}
	}

	proposals := BuildProposals(series.Episodes, siblings)
	if len(proposals) == 0 {
		dialog.ShowInformation("จัดกลุ่มไฟล์", "ไม่พบไฟล์ที่ชื่อคล้ายกันมากพอที่จะจัดกลุ่มในซีรีส์นี้", s.win)
		return
	}

	var b strings.Builder
	b.WriteString("จะย้ายไฟล์ดังนี้:\n\n")
	for _, p := range proposals {
		if p.ExistingSeries != nil {
			fmt.Fprintf(&b, "📁 %s  (%d ไฟล์ → ย้ายเข้าโฟลเดอร์เดิมที่มีอยู่แล้ว)\n", p.FolderName, len(p.Episodes))
		} else {
			fmt.Fprintf(&b, "📁 %s  (%d ไฟล์ → สร้างโฟลเดอร์ใหม่)\n", p.FolderName, len(p.Episodes))
		}
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
			s.refreshSeriesRows()
			s.refreshEpisodeRows()
		},
		s.win,
	)
	confirmDialog.Resize(fyne.NewSize(560, 480))
	confirmDialog.Show()
}

// applyGrouping ย้ายไฟล์จริงตาม proposals แต่ละกลุ่ม:
//   - ถ้ามี ExistingSeries (เจอโฟลเดอร์ที่ชื่อคล้ายกันอยู่แล้ว) -> ย้ายเข้าโฟลเดอร์นั้นเลย ไม่สร้างใหม่
//   - ถ้าไม่มี -> สร้างโฟลเดอร์ย่อยใหม่ใต้ series.RootPath
//
// ถ้าปลายทางมีไฟล์ชื่อซ้ำอยู่แล้ว จะเติม " (1)", " (2)" ฯลฯ ต่อท้ายให้อัตโนมัติ (ดู uniqueDestPath)
// อัปเดต path ของ episode และปรับ Library ให้ตรงกับโครงสร้างไฟล์ใหม่ (สถานะดูแล้วจะติดไปกับไฟล์)
func (s *appState) applyGrouping(orig *Series, proposals []GroupProposal) error {
	movedSet := map[*Episode]bool{}

	for _, p := range proposals {
		var target *Series
		var newDir string

		if p.ExistingSeries != nil {
			target = p.ExistingSeries
			newDir = target.RootPath
			// กรณีโฟลเดอร์เปล่าที่ยังไม่เคยถูก track มาก่อน (ไม่มีไฟล์วิดีโอ เลยไม่เคยอยู่ใน lib.SeriesList)
			// ต้องเพิ่มเข้าลิสต์ตอนนี้เอง เพราะจะมีไฟล์ย้ายเข้ามาแล้ว
			tracked := false
			for _, sr := range s.lib.SeriesList {
				if sr == target {
					tracked = true
					break
				}
			}
			if !tracked {
				s.lib.SeriesList = append(s.lib.SeriesList, target)
			}
		} else {
			folderName := sanitizeFolderName(p.FolderName)
			newDir = filepath.Join(orig.RootPath, folderName)
			if err := os.MkdirAll(newDir, 0755); err != nil {
				return err
			}
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
		}

		for _, ep := range p.Episodes {
			destPath := uniqueDestPath(newDir, ep.FileName)
			if ep.FilePath != destPath {
				if err := os.Rename(ep.FilePath, destPath); err != nil {
					return fmt.Errorf("ย้ายไฟล์ %s ไม่สำเร็จ: %w", ep.FileName, err)
				}
				ep.FilePath = destPath
				ep.FileName = filepath.Base(destPath)
			}
			target.Episodes = append(target.Episodes, ep)
			movedSet[ep] = true
		}

		sort.Slice(target.Episodes, func(i, j int) bool {
			return naturalLess(target.Episodes[i].FileName, target.Episodes[j].FileName)
		})
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

// showStatistics คำนวณและแสดงสรุปสถิติ: จำนวนตอนทั้งหมด/ดูแล้ว/เหลือ และเวลาที่ดูไปรวม (ประมาณ)
// นับเฉพาะไฟล์ที่ Exists อยู่จริง เวลาที่ดูไปรวมจะแม่นยำเฉพาะไฟล์ที่เคยเล่นผ่าน mpv ในแอปนี้
// อย่างน้อย 1 ครั้ง (ถึงจะรู้ DurationSeconds) ไฟล์ที่ติ๊กว่าดูแล้วเองโดยไม่เคยเล่นผ่านแอปจะไม่ถูกนับเวลา
func (s *appState) showStatistics() {
	var totalEpisodes, watchedCount, remainingCount, unknownDurationWatched int
	var totalWatchedSeconds float64

	for _, series := range s.lib.SeriesList {
		for _, ep := range series.Episodes {
			if !ep.Exists {
				continue
			}
			totalEpisodes++
			if ep.Watched {
				watchedCount++
				if ep.DurationSeconds > 0 {
					totalWatchedSeconds += ep.DurationSeconds
				} else {
					unknownDurationWatched++
				}
			} else {
				remainingCount++
				if ep.ResumeSeconds > 0 {
					totalWatchedSeconds += ep.ResumeSeconds // นับความคืบหน้าที่ดูไปแล้วบางส่วนด้วย
				}
			}
		}
	}

	msg := fmt.Sprintf(
		"ตอนทั้งหมด: %d ตอน\nดูแล้ว: %d ตอน\nเหลืออีก: %d ตอน\n\nเวลาที่ดูไปรวม (ประมาณ): %s",
		totalEpisodes, watchedCount, remainingCount, formatHoursMinutes(totalWatchedSeconds),
	)
	if unknownDurationWatched > 0 {
		msg += fmt.Sprintf(
			"\n\n(มี %d ตอนที่ดูแล้วแต่ยังไม่ทราบความยาว เพราะไม่เคยเปิดเล่นผ่าน mpv ในแอปนี้ เลยไม่ถูกนับรวมเวลาด้านบน)",
			unknownDurationWatched,
		)
	}

	dialog.ShowInformation("สรุปสถิติ", msg, s.win)
}

// showSeriesContextMenu แสดงเมนูคลิกขวา (เล่น / แก้ชื่อ / ลบ) สำหรับซีรีส์ที่ตำแหน่ง idx ใน lib.SeriesList
// ทำงานเดียวกันกับปุ่มบนแถบเครื่องมือทุกประการ แค่เข้าถึงผ่านคลิกขวาแทน
func (s *appState) showSeriesContextMenu(idx int, ev *fyne.PointEvent) {
	selectThis := func() {
		s.selectedIdx = idx
		s.updateSeriesSelectionHighlight()
		s.refreshEpisodeRows()
		if idx >= 0 && idx < len(s.lib.SeriesList) {
			key := seriesRootKey(s.lib.SeriesList[idx])
			if t, ok := s.tabs[key]; ok && t.episodeScroll != nil {
				t.episodeScroll.ScrollToTop()
			}
		}
	}

	playItem := fyne.NewMenuItem("▶ เล่น", func() {
		selectThis()
		s.playSelectedSeries()
	})
	renameItem := fyne.NewMenuItem("แก้ชื่อ", func() {
		selectThis()
		s.renameSelectedSeries()
	})
	deleteItem := fyne.NewMenuItem("ลบ", func() {
		selectThis()
		s.confirmDeleteSeries()
	})
	deleteItem.Icon = theme.DeleteIcon()

	menu := fyne.NewMenu("", playItem, renameItem, deleteItem)
	widget.ShowPopUpMenuAtPosition(menu, s.win.Canvas(), ev.AbsolutePosition)
}

// showEpisodeContextMenu แสดงเมนูคลิกขวา (เล่น / แก้ชื่อ / ลบ) สำหรับไฟล์ตอนนี้
// ทำงานเดียวกันกับปุ่มต่อแถวในหน้าตอนทุกประการ แค่เข้าถึงผ่านคลิกขวาแทน
func (s *appState) showEpisodeContextMenu(series *Series, ep *Episode, ev *fyne.PointEvent) {
	playItem := fyne.NewMenuItem("▶ เล่น", func() {
		s.playEpisode(ep)
	})
	playItem.Disabled = !ep.Exists

	renameItem := fyne.NewMenuItem("แก้ชื่อ", func() {
		s.renameEpisode(ep)
	})
	renameItem.Disabled = !ep.Exists

	deleteItem := fyne.NewMenuItem("ลบ", func() {
		s.confirmDeleteEpisode(series, ep)
	})
	deleteItem.Icon = theme.DeleteIcon()

	menu := fyne.NewMenu("", playItem, renameItem, deleteItem)
	widget.ShowPopUpMenuAtPosition(menu, s.win.Canvas(), ev.AbsolutePosition)
}

// showContinueWatching รวมทุกตอน (จากทุกซีรีส์) ที่มีจุดค้างไว้ (ResumeSeconds > 0 และไฟล์ยังอยู่จริง)
// เรียงจากที่เพิ่งเล่นค้างล่าสุดไปเก่าสุด (LastPlayedAt) แต่ละแถวกดเล่นต่อได้ทันที
func (s *appState) showContinueWatching() {
	type cwItem struct {
		series *Series
		ep     *Episode
	}
	var items []cwItem
	for _, series := range s.lib.SeriesList {
		for _, ep := range series.Episodes {
			if ep.Exists && ep.ResumeSeconds > 1 {
				items = append(items, cwItem{series: series, ep: ep})
			}
		}
	}

	if len(items) == 0 {
		dialog.ShowInformation("ดูต่อ", "ยังไม่มีตอนที่ดูค้างไว้เลย", s.win)
		return
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].ep.LastPlayedAt > items[j].ep.LastPlayedAt // เพิ่งเล่นล่าสุดขึ้นก่อน
	})

	list := container.NewVBox()
	for i, it := range items {
		it := it // capture ไว้ในลูป ป้องกันปัญหาตัวแปรซ้ำใน closure

		label := widget.NewLabel(fmt.Sprintf("%s\n%s — ค้างไว้ที่ %s",
			it.series.Name, it.ep.FileName, formatDuration(it.ep.ResumeSeconds)))
		label.Wrapping = fyne.TextWrapWord

		playBtn := widget.NewButtonWithIcon("", theme.MediaPlayIcon(), func() {
			s.playEpisode(it.ep)
		})

		row := container.NewBorder(nil, nil, nil, playBtn, label)
		list.Add(row)

		if i < len(items)-1 {
			list.Add(widget.NewSeparator())
		}
	}

	scroll := container.NewVScroll(list)
	scroll.SetMinSize(fyne.NewSize(520, 420))

	d := dialog.NewCustom("ดูต่อ", "ปิด", scroll, s.win)
	d.Resize(fyne.NewSize(560, 480))
	d.Show()
}

// refreshAllRootFolders สแกนซ้ำทุกโฟลเดอร์แม่ที่เคย track ไว้ (ทุกแท็บที่มีอยู่)
// เพื่ออัปเดตโฟลเดอร์ย่อยใหม่/ไฟล์ใหม่ที่เพิ่มเข้ามาทีหลัง โดยไม่ต้องเปิด dialog เลือกทีละโฟลเดอร์
// ถ้าโฟลเดอร์แม่ไหนสแกนไม่สำเร็จ (เช่น USB ไม่ได้เสียบ) จะข้ามไปตัวถัดไป ไม่หยุดทั้งหมด แล้วสรุป error รวมท้ายสุด
func (s *appState) refreshAllRootFolders() {
	rootSet := map[string]bool{}
	for _, sr := range s.lib.SeriesList {
		rootSet[seriesRootKey(sr)] = true
	}
	var rootPaths []string
	for p := range rootSet {
		rootPaths = append(rootPaths, p)
	}
	if len(rootPaths) == 0 {
		dialog.ShowInformation("รีเฟรชโฟลเดอร์แม่", "ยังไม่เคยสแกนโฟลเดอร์แม่ไว้เลย ลองกด \"สแกนโฟลเดอร์\" ก่อน", s.win)
		return
	}

	var failed []string
	for _, path := range rootPaths {
		s.ensureTab(path)
		scanned, err := ScanFolder(path)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", path, err))
			continue
		}
		MergeScan(s.lib, scanned, path)
	}

	sortSeriesForDisplay(s.lib.SeriesList)
	s.selectedIdx = -1 // ลำดับซีรีส์อาจเปลี่ยนไปแล้ว ตำแหน่งที่เคยเลือกไว้ไม่ตรงของเดิมอีกต่อไป
	if err := SaveLibrary(s.lib); err != nil {
		dialog.ShowError(err, s.win)
	}
	s.refreshSeriesRows()
	s.refreshEpisodeRows()

	if len(failed) > 0 {
		dialog.ShowError(fmt.Errorf("บางโฟลเดอร์สแกนไม่สำเร็จ:\n%s", strings.Join(failed, "\n")), s.win)
	}
}

// chooseAndScan ให้ผู้ใช้เลือกโฟลเดอร์ผ่าน dialog แล้วสแกน โฟลเดอร์แม่แต่ละอันจะได้แท็บของตัวเอง
// (ถ้าเป็นโฟลเดอร์แม่ใหม่ที่ยังไม่เคยสแกน จะสร้างแท็บใหม่ต่อท้ายและโฟกัสไปแท็บนั้นให้อัตโนมัติ)
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
		t := s.ensureTab(path)
		s.tabsWidget.Select(t.tabItem)

		scanned, err := ScanFolder(path)
		if err != nil {
			dialog.ShowError(err, s.win)
			return
		}
		MergeScan(s.lib, scanned, path)
		sortSeriesForDisplay(s.lib.SeriesList)
		s.selectedIdx = -1 // ลำดับซีรีส์เปลี่ยนไปแล้ว ตำแหน่งที่เคยเลือกไว้ไม่ตรงของเดิมอีกต่อไป
		if err := SaveLibrary(s.lib); err != nil {
			dialog.ShowError(err, s.win)
		}
		s.refreshSeriesRows()
		s.refreshEpisodeRows()
	}, s.win)
}

// moveToTrash ย้ายไฟล์หรือโฟลเดอร์ไปถังขยะของระบบ (ตาม freedesktop.org trash spec) แทนการลบถาวร
// ลองหาเครื่องมือที่มีอยู่ในเครื่องตามลำดับ: gio (GNOME/Cinnamon) -> trash-put (trash-cli) -> kioclient5 (KDE)
// ใช้ได้ทั้งไฟล์เดี่ยวและทั้งโฟลเดอร์ (เครื่องมือพวกนี้ย้ายทั้งโฟลเดอร์ให้อัตโนมัติ)
func moveToTrash(path string) error {
	candidates := [][]string{
		{"gio", "trash", path},
		{"trash-put", path},
		{"kioclient5", "move", path, "trash:/"},
	}
	for _, c := range candidates {
		bin, err := exec.LookPath(c[0])
		if err != nil {
			continue
		}
		cmd := exec.Command(bin, c[1:]...)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("ไม่พบเครื่องมือย้ายไฟล์ไปถังขยะในเครื่อง ลองติดตั้งด้วยคำสั่ง: sudo apt install trash-cli")
}

// showDeleteChoiceDialog แสดง dialog ที่มี 2 ปุ่มให้เลือก: "ลบไฟล์จริง" กับ "ลบแค่ลิสต์"
// พร้อมปุ่มยกเลิก ใช้ร่วมกันทั้งกรณีลบไฟล์เดี่ยวและลบทั้งซีรีส์/โฟลเดอร์แม่
func showDeleteChoiceDialog(win fyne.Window, title, message string, onDeleteReal func(), onListOnly func()) {
	var d *dialog.CustomDialog

	msgLabel := widget.NewLabel(message)
	msgLabel.Wrapping = fyne.TextWrapWord

	realBtn := widget.NewButtonWithIcon("ย้ายไปถังขยะ", theme.DeleteIcon(), func() {
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
		"\"%s\"\n\n• ย้ายไปถังขยะ = ย้ายไฟล์นี้ไปถังขยะของระบบ (กู้คืนได้ภายหลังถ้าจำเป็น)\n"+
			"• ลบแค่ลิสต์ = เอาออกจากรายการติดตาม ไฟล์บนดิสก์ยังอยู่เหมือนเดิม",
		ep.FileName,
	)
	showDeleteChoiceDialog(s.win, "ลบตอนนี้", msg,
		func() {
			if ep.Exists {
				if err := moveToTrash(ep.FilePath); err != nil {
					dialog.ShowError(err, s.win)
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
	}

	if err := SaveLibrary(s.lib); err != nil {
		dialog.ShowError(err, s.win)
	}
	sortSeriesForDisplay(s.lib.SeriesList)
	s.refreshSeriesRows()
	s.refreshEpisodeRows()
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
		realDesc = fmt.Sprintf("ย้ายไฟล์วิดีโอทั้งหมด %d ไฟล์ในนี้ไปถังขยะ (ไฟล์อื่นที่ไม่ใช่วิดีโอในโฟลเดอร์เดียวกันจะไม่ถูกแตะ)", series.TotalCount())
	} else {
		realDesc = fmt.Sprintf("ย้ายทั้งโฟลเดอร์ \"%s\" ไปถังขยะ (รวมไฟล์ทั้งหมด %d ไฟล์ข้างใน)", series.Name, series.TotalCount())
	}
	msg := fmt.Sprintf(
		"\"%s\"\n\n• ย้ายไปถังขยะ = %s (กู้คืนได้ภายหลังถ้าจำเป็น)\n"+
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
					if err := moveToTrash(ep.FilePath); err != nil {
						dialog.ShowError(err, s.win)
						return
					}
				}
			} else {
				if err := moveToTrash(series.RootPath); err != nil {
					dialog.ShowError(err, s.win)
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

	if err := SaveLibrary(s.lib); err != nil {
		dialog.ShowError(err, s.win)
	}
	s.refreshSeriesRows()
	s.refreshEpisodeRows()
}

// isUnderRoot ตรวจว่า path อยู่ใต้ root หรือไม่ (เป็นโฟลเดอร์ย่อยจริง ๆ ไม่ใช่แค่ขึ้นต้นด้วยตัวอักษรคล้ายกัน)
func isUnderRoot(path, root string) bool {
	sep := string(os.PathSeparator)
	if !strings.HasSuffix(root, sep) {
		root += sep
	}
	return strings.HasPrefix(path, root)
}

// knownPlayers คือรายชื่อโปรแกรมเล่นวิดีโอที่รองรับการรับหลายไฟล์เป็น playlist
// เรียงตามลำดับที่จะลองหา ถ้าเจอตัวไหนในเครื่องก่อนจะใช้ตัวนั้น
var knownPlayers = []string{"mpv", "vlc", "smplayer", "celluloid", "totem", "xplayer"}

// playFile เปิดไฟล์วิดีโอ 1 ไฟล์ด้วยโปรแกรมเริ่มต้นของระบบ (ผ่าน xdg-open)
func playFile(win fyne.Window, path string) {
	cmd := exec.Command("xdg-open", path)
	if err := cmd.Start(); err != nil {
		dialog.ShowError(fmt.Errorf("เปิดไฟล์ไม่สำเร็จ: %w", err), win)
	}
}

// playSelectedSeries เล่นทุกตอนของซีรีส์ที่เลือกอยู่ต่อกันเป็น playlist เดียว
// (เฉพาะไฟล์ที่ยังอยู่จริงในดิสก์) โดยลองหาโปรแกรมเล่นวิดีโอที่รองรับ playlist ในเครื่องก่อน
// ถ้าไม่เจอเลยจะ fallback ไปเปิดตอนแรกด้วยโปรแกรมเริ่มต้นของระบบแทน
func (s *appState) playSelectedSeries() {
	if s.selectedIdx < 0 || s.selectedIdx >= len(s.lib.SeriesList) {
		dialog.ShowInformation("เล่นซีรีส์", "กรุณาเลือกซีรีส์ทางซ้ายก่อน", s.win)
		return
	}
	series := s.lib.SeriesList[s.selectedIdx]

	if mpvAvailable() {
		s.playSeriesTracked(series)
		return
	}

	var paths []string
	for _, ep := range series.Episodes {
		if ep.Exists {
			paths = append(paths, ep.FilePath)
		}
	}
	if len(paths) == 0 {
		dialog.ShowInformation("เล่นซีรีส์", "ไม่มีไฟล์ที่ยังอยู่จริงในดิสก์ให้เล่น", s.win)
		return
	}

	for _, playerName := range knownPlayers {
		bin, err := exec.LookPath(playerName)
		if err != nil {
			continue
		}
		cmd := exec.Command(bin, paths...)
		if err := cmd.Start(); err == nil {
			return
		}
	}

	// ไม่เจอโปรแกรมเล่นวิดีโอที่รองรับ playlist ในเครื่องเลย เปิดตอนแรกด้วยโปรแกรมเริ่มต้นของระบบแทน
	playFile(s.win, paths[0])
}

// refreshSeriesRows สร้างแถวซีรีส์ใหม่ทั้งหมดในทุกแท็บ ให้ตรงกับ lib.SeriesList ปัจจุบัน
// (กรองตามคำค้นหาของแต่ละแท็บ ถ้ามีการพิมพ์ค้นหาไว้) เรียกทุกครั้งที่ลิสต์ซีรีส์เปลี่ยน
// (สแกน, ลบ, จัดกลุ่ม, แก้ชื่อ, พิมพ์ค้นหา ฯลฯ)
// แต่ละแถวจองความสูงตามเนื้อหาจริงของตัวเอง (ชื่อยาวแค่ไหนก็ไม่ล้น เพราะไม่ใช่ list แบบ virtualized)
func (s *appState) refreshSeriesRows() {
	byRoot := map[string][]int{}
	for i, series := range s.lib.SeriesList {
		key := seriesRootKey(series)
		byRoot[key] = append(byRoot[key], i)
	}

	for rootPath, t := range s.tabs {
		t := t // capture ไว้ในลูป ป้องกันปัญหาตัวแปรซ้ำใน closure

		t.seriesBox.Objects = nil
		t.seriesRows = nil

		query := strings.ToLower(strings.TrimSpace(t.searchQuery))

		var matchedIdx []int
		for _, i := range byRoot[rootPath] {
			if query == "" || strings.Contains(strings.ToLower(s.lib.SeriesList[i].Name), query) {
				matchedIdx = append(matchedIdx, i)
			}
		}

		if len(matchedIdx) == 0 {
			msg := "ยังไม่มีซีรีส์ในโฟลเดอร์นี้"
			if query != "" {
				msg = "ไม่พบซีรีส์ที่ตรงกับคำค้นหา"
			}
			t.seriesBox.Add(widget.NewLabel(msg))
			t.seriesBox.Refresh()
			continue
		}

		for pos, i := range matchedIdx {
			idx := i // capture ไว้ในลูป ป้องกันปัญหาตัวแปรซ้ำใน closure
			series := s.lib.SeriesList[idx]
			sr := series // capture ไว้ในลูป สำหรับ closure ของปุ่มดาว

			nameLine := series.Name
			if series.Starred {
				nameLine = "★ " + nameLine
			}
			text := fmt.Sprintf("%s\nดูล่าสุด: ตอน %d  (ดูแล้ว %d/%d ตอน)",
				nameLine, series.LastWatchedEpisode(), series.WatchedCount(), series.TotalCount())

			row := newSeriesRow(text, !series.IsRoot, series.Starred, func() {
				s.selectedIdx = idx
				s.updateSeriesSelectionHighlight()
				s.refreshEpisodeRows()
				if t.episodeScroll != nil {
					t.episodeScroll.ScrollToTop()
				}
			}, func() {
				s.selectedIdx = idx
				s.updateSeriesSelectionHighlight()
				s.refreshEpisodeRows()
				if t.episodeScroll != nil {
					t.episodeScroll.ScrollToTop()
				}
				s.playSelectedSeries()
			}, func() {
				sr.Starred = !sr.Starred
				sortSeriesForDisplay(s.lib.SeriesList)
				_ = SaveLibrary(s.lib)
				s.refreshSeriesRows()
			})
			row.SetSelected(idx == s.selectedIdx)
			row.libIndex = idx
			row.onSecondaryTapped = func(ev *fyne.PointEvent) {
				s.showSeriesContextMenu(idx, ev)
			}
			t.seriesRows = append(t.seriesRows, row)
			t.seriesBox.Add(row)

			if pos < len(matchedIdx)-1 {
				t.seriesBox.Add(widget.NewSeparator())
			}
		}

		t.seriesBox.Refresh()
	}
}

// updateSeriesSelectionHighlight ปรับให้เห็นว่าแถวไหนถูกเลือกอยู่ (ในทุกแท็บ) โดยไม่ต้องสร้างแถวใหม่ทั้งหมด
func (s *appState) updateSeriesSelectionHighlight() {
	for _, t := range s.tabs {
		for _, row := range t.seriesRows {
			row.SetSelected(row.libIndex == s.selectedIdx)
		}
	}
}

// refreshEpisodeRows สร้างแถวตอนใหม่ทั้งหมดในแท็บที่เป็นเจ้าของซีรีส์ที่เลือกอยู่ (selectedIdx) เท่านั้น
// แท็บอื่นจะโชว์รายการตอนว่างเปล่า (เพราะ selection ใช้ร่วมกันทั้งแอป ไม่ใช่แยกต่อแท็บ)
// เรียกทุกครั้งที่รายการตอนควรเปลี่ยน (เลือกซีรีส์ใหม่, ติ๊กดูแล้ว, ลบ, เล่นจบ ฯลฯ)
// แต่ละแถวไม่ตัดคำ (ไม่ wrap) เพื่อให้ชื่อไฟล์ยาว ๆ ดันความกว้างของแถวออกไปได้ แล้วเลื่อนดูได้ผ่าน
// scroll แนวนอนของ container ที่ครอบอยู่
func (s *appState) refreshEpisodeRows() {
	var series *Series
	var selectedRoot string
	if s.selectedIdx >= 0 && s.selectedIdx < len(s.lib.SeriesList) {
		series = s.lib.SeriesList[s.selectedIdx]
		selectedRoot = seriesRootKey(series)
	}

	for rootPath, t := range s.tabs {
		t := t // capture ไว้ในลูป ป้องกันปัญหาตัวแปรซ้ำใน closure
		t.episodeBox.Objects = nil

		if series == nil || rootPath != selectedRoot {
			t.episodeBox.Refresh()
			if t.episodeScroll != nil {
				t.episodeScroll.Refresh()
			}
			continue
		}

		for i, ep := range series.Episodes {
			ep := ep // capture ไว้ในลูป ป้องกันปัญหาตัวแปรซ้ำใน closure

			check := widget.NewCheck("", nil)
			resumeNote := ""
			if ep.ResumeSeconds > 1 {
				resumeNote = fmt.Sprintf(" (ค้างไว้ที่ %s)", formatDuration(ep.ResumeSeconds))
			}
			label := widget.NewLabel(fmt.Sprintf("%s%s", ep.FileName, resumeNote))
			status := widget.NewLabel("")
			playBtn := widget.NewButtonWithIcon("", theme.MediaPlayIcon(), nil)
			renameBtn := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), nil)
			delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)

			check.SetChecked(ep.Watched)
			check.OnChanged = func(v bool) {
				ep.Watched = v
				s.refreshSeriesRows()
				_ = SaveLibrary(s.lib)
			}
			playBtn.OnTapped = func() {
				s.playEpisode(ep)
			}
			renameBtn.OnTapped = func() {
				s.renameEpisode(ep)
			}
			delBtn.OnTapped = func() {
				s.confirmDeleteEpisode(series, ep)
			}

			if ep.Exists {
				playBtn.Enable()
				renameBtn.Enable()
			} else {
				status.Importance = widget.DangerImportance
				status.SetText("ไฟล์ถูกลบแล้ว")
				playBtn.Disable()
				renameBtn.Disable()
			}

			row := container.NewHBox(check, label, status, playBtn, renameBtn, delBtn)
			wrappedRow := newDoubleTapWrapper(row, func() {
				s.playEpisode(ep)
			})
			wrappedRow.onSecondaryTapped = func(ev *fyne.PointEvent) {
				s.showEpisodeContextMenu(series, ep, ev)
			}
			t.episodeBox.Add(wrappedRow)

			if i < len(series.Episodes)-1 {
				t.episodeBox.Add(widget.NewSeparator())
			}
		}

		t.episodeBox.Refresh()
		// Refresh ตัว Scroll ที่ครอบอยู่ด้วยตรง ๆ ไม่งั้นบางครั้ง Fyne ไม่คำนวณขนาดเนื้อหาใหม่ให้
		// (ต้องรอ event อื่นมากระตุ้น เช่นเอาเมาส์ไปจ่อ scrollbar ถึงจะ redraw ให้เห็น)
		if t.episodeScroll != nil {
			t.episodeScroll.Refresh()
		}
	}
}

// sortSeriesForDisplay จัดลำดับซีรีส์สำหรับแสดงผล:
//  1. โฟลเดอร์แม่ (IsRoot) มาก่อนเสมอ ไม่ว่าจะเคยสแกนมากี่รอบก็ตาม
//  2. โฟลเดอร์ลูกที่ติดดาวไว้ มาก่อนโฟลเดอร์ลูกทั่วไป (แต่ยังคงอยู่หลังโฟลเดอร์แม่ทั้งหมด)
//  3. ที่เหลือเรียงตามชื่อ (natural sort เข้าใจตัวเลข)
func sortSeriesForDisplay(list []*Series) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].IsRoot != list[j].IsRoot {
			return list[i].IsRoot
		}
		if list[i].Starred != list[j].Starred {
			return list[i].Starred
		}
		return naturalLess(list[i].Name, list[j].Name)
	})
}

// renameSelectedSeries ให้ผู้ใช้แก้ชื่อซีรีส์ที่เลือกอยู่:
//   - โฟลเดอร์ย่อยธรรมดา -> เปลี่ยนชื่อโฟลเดอร์จริงในดิสก์ให้เลย (os.Rename) แล้วอัปเดต path ของทุกตอนในนั้น
//   - โฟลเดอร์แม่ (IsRoot) -> แก้แค่ชื่อที่แสดงในแอปเท่านั้น ไม่แตะโฟลเดอร์ root จริง (เสี่ยงเกินไป อาจมีไฟล์อื่นปนอยู่)
func (s *appState) renameSelectedSeries() {
	if s.selectedIdx < 0 || s.selectedIdx >= len(s.lib.SeriesList) {
		dialog.ShowInformation("แก้ชื่อซีรีส์", "กรุณาเลือกซีรีส์ทางซ้ายก่อน", s.win)
		return
	}
	series := s.lib.SeriesList[s.selectedIdx]

	entry := widget.NewEntry()
	entry.SetText(series.Name)

	var noteText string
	if series.IsRoot {
		noteText = "นี่คือโฟลเดอร์แม่ จะแก้แค่ชื่อที่แสดงในแอปเท่านั้น ไม่เปลี่ยนชื่อโฟลเดอร์จริงในดิสก์"
	} else {
		noteText = "จะเปลี่ยนชื่อโฟลเดอร์จริงในดิสก์ด้วย (ไม่ใช่แค่ชื่อที่แสดงในแอป)"
	}

	content := container.NewVBox(
		widget.NewLabel(noteText),
		entry,
	)

	d := dialog.NewCustomConfirm("แก้ชื่อซีรีส์", "บันทึก", "ยกเลิก", content, func(ok bool) {
		if !ok {
			return
		}
		newName := strings.TrimSpace(entry.Text)
		if newName == "" {
			dialog.ShowInformation("แก้ชื่อซีรีส์", "ชื่อห้ามเว้นว่าง", s.win)
			return
		}

		if series.IsRoot {
			series.Name = newName
		} else {
			newFolderName := sanitizeFolderName(newName)
			newDir := filepath.Join(filepath.Dir(series.RootPath), newFolderName)
			if newDir != series.RootPath {
				if _, err := os.Stat(newDir); err == nil {
					dialog.ShowError(fmt.Errorf("มีโฟลเดอร์ชื่อ \"%s\" อยู่แล้ว กรุณาตั้งชื่ออื่น", newFolderName), s.win)
					return
				}
				if err := os.Rename(series.RootPath, newDir); err != nil {
					dialog.ShowError(fmt.Errorf("เปลี่ยนชื่อโฟลเดอร์ไม่สำเร็จ: %w", err), s.win)
					return
				}
				oldDir := series.RootPath
				series.RootPath = newDir
				for _, ep := range series.Episodes {
					if strings.HasPrefix(ep.FilePath, oldDir) {
						ep.FilePath = filepath.Join(newDir, ep.FileName)
					}
				}
			}
			series.Name = newFolderName
		}

		sortSeriesForDisplay(s.lib.SeriesList)
		if err := SaveLibrary(s.lib); err != nil {
			dialog.ShowError(err, s.win)
		}
		s.refreshSeriesRows()
		s.refreshEpisodeRows()
	}, s.win)
	d.Resize(fyne.NewSize(420, 160))
	d.Show()
}

// renameEpisode ให้ผู้ใช้แก้ชื่อไฟล์วิดีโอ 1 ไฟล์ เปลี่ยนชื่อไฟล์จริงในดิสก์ให้เลย (os.Rename)
// แก้ได้ทั้งชื่อและนามสกุล (พิมพ์ทับได้เต็มที่) เช็คชื่อชนกับไฟล์อื่นในโฟลเดอร์เดียวกันก่อนด้วย
func (s *appState) renameEpisode(ep *Episode) {
	entry := widget.NewEntry()
	entry.SetText(ep.FileName)

	content := container.NewVBox(
		widget.NewLabel("จะเปลี่ยนชื่อไฟล์จริงในดิสก์ (แก้นามสกุลไฟล์ได้ด้วยถ้าต้องการ)"),
		entry,
	)

	d := dialog.NewCustomConfirm("แก้ชื่อไฟล์", "บันทึก", "ยกเลิก", content, func(ok bool) {
		if !ok {
			return
		}
		newName := strings.TrimSpace(entry.Text)
		if newName == "" {
			dialog.ShowInformation("แก้ชื่อไฟล์", "ชื่อห้ามเว้นว่าง", s.win)
			return
		}
		newName = sanitizeFolderName(newName) // กันอักขระที่ใช้ในชื่อไฟล์ไม่ได้ เช่น / \ : * ? " < > |

		dir := filepath.Dir(ep.FilePath)
		newPath := filepath.Join(dir, newName)
		if newPath == ep.FilePath {
			return // ชื่อเดิม ไม่ต้องทำอะไร
		}
		if _, err := os.Stat(newPath); err == nil {
			dialog.ShowError(fmt.Errorf("มีไฟล์ชื่อ \"%s\" อยู่แล้ว กรุณาตั้งชื่ออื่น", newName), s.win)
			return
		}
		if err := os.Rename(ep.FilePath, newPath); err != nil {
			dialog.ShowError(fmt.Errorf("เปลี่ยนชื่อไฟล์ไม่สำเร็จ: %w", err), s.win)
			return
		}

		ep.FilePath = newPath
		ep.FileName = newName
		ep.EpisodeNumber = ExtractEpisodeNumber(newName)

		if err := SaveLibrary(s.lib); err != nil {
			dialog.ShowError(err, s.win)
		}
		s.refreshSeriesRows()
		s.refreshEpisodeRows()
	}, s.win)
	d.Resize(fyne.NewSize(420, 160))
	d.Show()
}
