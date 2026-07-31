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
	lib           *Library
	win           fyne.Window
	seriesBox     *fyne.Container // VBox ของแถวซีรีส์ (แทน widget.List เดิม เพื่อให้แต่ละแถวสูงตามเนื้อหาได้)
	seriesRows    []*seriesRow
	episodeBox    *fyne.Container   // VBox ของแถวตอน (แทน widget.List เดิม เพื่อให้เลื่อนแนวนอนได้ตอนชื่อไฟล์ยาว)
	episodeScroll *container.Scroll // ตัวครอบ episodeBox เก็บไว้เพื่อสั่ง Refresh/เลื่อนกลับบนสุดได้ตรง ๆ
	selectedIdx   int
	rootPath      string
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
	playSeriesBtn := widget.NewButtonWithIcon("เล่นซีรีส์นี้", theme.MediaPlayIcon(), func() {
		state.playSelectedSeries()
	})
	renameSeriesBtn := widget.NewButtonWithIcon("แก้ชื่อ", theme.DocumentCreateIcon(), func() {
		state.renameSelectedSeries()
	})
	toolbar := container.NewHBox(scanBtn, organizeBtn, playSeriesBtn, renameSeriesBtn, deleteSeriesBtn)

	state.seriesBox = container.NewVBox()
	state.refreshSeriesRows()

	state.episodeBox = container.NewVBox()
	state.episodeScroll = container.NewScroll(state.episodeBox)
	state.episodeScroll.Direction = container.ScrollBoth
	state.refreshEpisodeRows()

	seriesScroll := container.NewVScroll(state.seriesBox)

	split := container.NewHSplit(seriesScroll, state.episodeScroll)
	split.Offset = 0.38

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

// refreshSeriesRows สร้างแถวซีรีส์ใหม่ทั้งหมดใน seriesBox ให้ตรงกับ lib.SeriesList ปัจจุบัน
// เรียกทุกครั้งที่ลิสต์ซีรีส์เปลี่ยน (สแกน, ลบ, จัดกลุ่ม, แก้ชื่อ ฯลฯ) แทนที่ widget.List.Refresh() เดิม
// แต่ละแถวจองความสูงตามเนื้อหาจริงของตัวเอง (ชื่อยาวแค่ไหนก็ไม่ล้น เพราะไม่ใช่ list แบบ virtualized)
func (s *appState) refreshSeriesRows() {
	s.seriesBox.Objects = nil
	s.seriesRows = nil

	for i, series := range s.lib.SeriesList {
		idx := i     // capture ไว้ในลูป ป้องกันปัญหาตัวแปรซ้ำใน closure
		sr := series // capture ไว้ในลูป สำหรับ closure ของปุ่มดาว

		nameLine := series.Name
		if series.IsRoot {
			nameLine = "🏠 " + nameLine + " (โฟลเดอร์สแกน)"
		} else if series.Starred {
			nameLine = "★ " + nameLine
		}
		text := fmt.Sprintf("%s\nดูล่าสุด: ตอน %d  (ดูแล้ว %d/%d ตอน)",
			nameLine, series.LastWatchedEpisode(), series.WatchedCount(), series.TotalCount())

		row := newSeriesRow(text, !series.IsRoot, series.Starred, func() {
			s.selectedIdx = idx
			s.updateSeriesSelectionHighlight()
			s.refreshEpisodeRows()
			if s.episodeScroll != nil {
				s.episodeScroll.ScrollToTop()
			}
		}, func() {
			sr.Starred = !sr.Starred
			sortSeriesForDisplay(s.lib.SeriesList)
			_ = SaveLibrary(s.lib)
			s.refreshSeriesRows()
		})
		row.SetSelected(idx == s.selectedIdx)
		s.seriesRows = append(s.seriesRows, row)
		s.seriesBox.Add(row)

		if i < len(s.lib.SeriesList)-1 {
			s.seriesBox.Add(widget.NewSeparator())
		}
	}

	s.seriesBox.Refresh()
}

// updateSeriesSelectionHighlight ปรับให้เห็นว่าแถวไหนถูกเลือกอยู่ โดยไม่ต้องสร้างแถวใหม่ทั้งหมด
func (s *appState) updateSeriesSelectionHighlight() {
	for i, row := range s.seriesRows {
		row.SetSelected(i == s.selectedIdx)
	}
}

// refreshEpisodeRows สร้างแถวตอนใหม่ทั้งหมดใน episodeBox ให้ตรงกับซีรีส์ที่เลือกอยู่ (selectedIdx)
// เรียกทุกครั้งที่รายการตอนควรเปลี่ยน (เลือกซีรีส์ใหม่, ติ๊กดูแล้ว, ลบ, เล่นจบ ฯลฯ) แทน widget.List.Refresh() เดิม
// แต่ละแถวไม่ตัดคำ (ไม่ wrap) เพื่อให้ชื่อไฟล์ยาว ๆ ดันความกว้างของแถวออกไปได้ แล้วเลื่อนดูได้ผ่าน
// scroll แนวนอนของ container ที่ครอบอยู่ (ดูตอนสร้าง UI ใน main())
func (s *appState) refreshEpisodeRows() {
	s.episodeBox.Objects = nil

	if s.selectedIdx < 0 || s.selectedIdx >= len(s.lib.SeriesList) {
		s.episodeBox.Refresh()
		if s.episodeScroll != nil {
			s.episodeScroll.Refresh()
		}
		return
	}
	series := s.lib.SeriesList[s.selectedIdx]

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
		delBtn.OnTapped = func() {
			s.confirmDeleteEpisode(series, ep)
		}

		if ep.Exists {
			playBtn.Enable()
		} else {
			status.Importance = widget.DangerImportance
			status.SetText("ไฟล์ถูกลบแล้ว")
			playBtn.Disable()
		}

		row := container.NewHBox(check, label, status, playBtn, delBtn)
		s.episodeBox.Add(row)

		if i < len(series.Episodes)-1 {
			s.episodeBox.Add(widget.NewSeparator())
		}
	}

	s.episodeBox.Refresh()
	// Refresh ตัว Scroll ที่ครอบอยู่ด้วยตรง ๆ ไม่งั้นบางครั้ง Fyne ไม่คำนวณขนาดเนื้อหาใหม่ให้
	// (ต้องรอ event อื่นมากระตุ้น เช่นเอาเมาส์ไปจ่อ scrollbar ถึงจะ redraw ให้เห็น)
	if s.episodeScroll != nil {
		s.episodeScroll.Refresh()
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
