package main

import (
	"fmt"
	"os"
	"path/filepath"
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

	state := &appState{lib: lib, win: w, selectedIdx: -1}

	scanBtn := widget.NewButtonWithIcon("สแกนโฟลเดอร์", theme.FolderOpenIcon(), func() {
		state.chooseAndScan()
	})
	organizeBtn := widget.NewButton("จัดกลุ่มไฟล์ชื่อคล้ายกัน", func() {
		state.organizeSimilar()
	})
	toolbar := container.NewHBox(scanBtn, organizeBtn)

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
			return container.NewHBox(check, label, status)
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

			label.SetText(fmt.Sprintf("ตอน %d - %s", ep.EpisodeNumber, ep.FileName))
			check.OnChanged = nil
			check.SetChecked(ep.Watched)
			check.OnChanged = func(v bool) {
				ep.Watched = v
				state.seriesList.Refresh()
				_ = SaveLibrary(state.lib)
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
		if err := SaveLibrary(s.lib); err != nil {
			dialog.ShowError(err, s.win)
		}
		s.seriesList.Refresh()
		s.episodeList.Refresh()
	}, s.win)
}
