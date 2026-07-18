package main

import (
	"fmt"

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
	toolbar := container.NewHBox(scanBtn)

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
