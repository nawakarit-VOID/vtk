// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2/dialog"
)

// resumeThresholdSeconds คือระยะเวลาขั้นต่ำที่ถือว่า "มีความคืบหน้าควรค่าแก่การบันทึกจุดค้าง"
// ถ้าเล่นแล้วหยุดเร็วกว่านี้ (เช่นเปิดผิดไฟล์แล้วปิดทันที) จะไม่บันทึกจุดค้างไว้ ถือว่าเริ่มใหม่ตั้งแต่ต้น
const resumeThresholdSeconds = 15.0

// mpvIPCMessage คือโครงสร้างข้อความที่ mpv ส่งกลับผ่าน IPC socket (ครอบคลุมทั้ง event ที่เราสนใจ)
type mpvIPCMessage struct {
	Event  string      `json:"event,omitempty"`
	Reason string      `json:"reason,omitempty"`
	Name   string      `json:"name,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}

// mpvAvailable เช็คว่ามี mpv ติดตั้งอยู่ในเครื่องไหม
func mpvAvailable() bool {
	_, err := exec.LookPath("mpv")
	return err == nil
}

// formatDuration แปลงวินาทีเป็นรูปแบบ H:MM:SS หรือ MM:SS ให้อ่านง่าย
func formatDuration(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	total := int(seconds)
	h := total / 3600
	m := (total % 3600) / 60
	sec := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%d:%02d", m, sec)
}

// playEpisodeTracked เล่นไฟล์ด้วย mpv พร้อมติดตามความคืบหน้าผ่าน IPC socket:
//   - เล่นจบจริง (mpv รายงาน reason "eof") -> ติ๊กว่าดูแล้วอัตโนมัติ เคลียร์จุดค้าง
//   - ปิดกลางคัน -> บันทึกวินาทีล่าสุดที่เล่นถึงไว้เป็นจุดค้าง (ถ้าเกิน resumeThresholdSeconds)
//   - ถ้าเคยมีจุดค้างไว้ก่อนหน้า จะสั่งให้ mpv เริ่มเล่นจากจุดนั้นให้อัตโนมัติผ่าน --start
//
// ทำงานเป็น goroutine เบื้องหลัง ไม่บล็อก UI thread
func (s *appState) playEpisodeTracked(ep *Episode) {
	sockPath := filepath.Join(os.TempDir(), fmt.Sprintf("videotracker-mpv-%d.sock", time.Now().UnixNano()))

	args := []string{"--input-ipc-server=" + sockPath, "--force-window=yes"}
	if ep.ResumeSeconds > 1 {
		args = append(args, fmt.Sprintf("--start=%.0f", ep.ResumeSeconds))
	}
	args = append(args, ep.FilePath)

	cmd := exec.Command("mpv", args...)
	if err := cmd.Start(); err != nil {
		dialog.ShowError(fmt.Errorf("เปิด mpv ไม่สำเร็จ: %w", err), s.win)
		return
	}

	go func() {
		defer os.Remove(sockPath)

		conn := dialMPVSocket(sockPath)
		if conn == nil {
			// เชื่อมต่อ IPC ไม่ได้ (เช่น mpv ปิดเร็วมากจนต่อไม่ทัน) ก็ปล่อยให้เล่นต่อไปเฉย ๆ ไม่ track
			_ = cmd.Wait()
			return
		}
		defer conn.Close()

		// สั่งให้ mpv ส่งค่า time-pos กลับมาทุกครั้งที่ตำแหน่งเล่นเปลี่ยน
		observeCmd, err := json.Marshal(map[string]interface{}{
			"command": []interface{}{"observe_property", 1, "time-pos"},
		})
		if err == nil {
			_, _ = conn.Write(append(observeCmd, '\n'))
		}

		var lastPos float64
		var reason string

		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			var msg mpvIPCMessage
			if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
				continue
			}
			switch msg.Event {
			case "property-change":
				if msg.Name == "time-pos" {
					if f, ok := msg.Data.(float64); ok {
						lastPos = f
					}
				}
			case "end-file":
				reason = msg.Reason
			}
		}
		// scanner.Scan() คืน false ทั้งตอน socket ปิดปกติ (mpv ออกจากโปรแกรม) และตอนอ่านผิดพลาดจริง ๆ
		// ทั้งสองกรณีเราจะเดินหน้าไปสรุปผลจาก lastPos/reason ที่เก็บได้อยู่ดี แต่ log error ไว้เผื่อ debug
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "videotracker: mpv IPC read error: %v\n", err)
		}

		_ = cmd.Wait()

		if reason == "eof" {
			ep.Watched = true
			ep.ResumeSeconds = 0
		} else if lastPos > resumeThresholdSeconds {
			ep.ResumeSeconds = lastPos
		} else {
			ep.ResumeSeconds = 0
		}

		if err := SaveLibrary(s.lib); err != nil {
			dialog.ShowError(err, s.win)
		}
		s.seriesList.Refresh()
		s.episodeList.Refresh()
	}()
}

// dialMPVSocket พยายามเชื่อมต่อ IPC socket ของ mpv โดย retry จนกว่า socket จะพร้อม (สูงสุด ~5 วินาที)
func dialMPVSocket(sockPath string) net.Conn {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("unix", sockPath)
		if err == nil {
			return conn
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// playEpisode เล่นไฟล์นี้: ใช้ mpv แบบ track ความคืบหน้าถ้ามีในเครื่อง ไม่งั้น fallback ไปเปิดด้วยโปรแกรมเริ่มต้นของระบบ
func (s *appState) playEpisode(ep *Episode) {
	if mpvAvailable() {
		s.playEpisodeTracked(ep)
		return
	}
	playFile(s.win, ep.FilePath)
}

// playSeriesTracked เล่นทุกตอนที่ยังไม่ได้ดูของซีรีส์เป็น playlist เดียวผ่าน mpv เรียงตามลำดับปกติ
// (ข้ามเฉพาะตอนที่ติ๊กว่าดูแล้วไปเลย ไม่เอามาเล่นซ้ำ) พร้อม track ความคืบหน้าทีละไฟล์
// ถ้าไฟล์ไหนในลิสต์มีจุดค้างไว้อยู่ (ไม่ว่าจะอยู่ตำแหน่งไหน) พอ mpv เล่นมาถึงไฟล์นั้นจริง ๆ
// จะสั่ง seek ผ่าน IPC ไปตรงจุดค้างให้ทันที (ไม่ใช้ --playlist-start ข้ามไฟล์ก่อนหน้าไปเลย
// เพราะจะทำให้ตอนที่ยังไม่ได้ดูก่อนหน้าตอนที่ค้างไว้ถูกข้ามไปด้วยโดยไม่ตั้งใจ)
//
// observe "playlist-pos" คู่กับ "time-pos" ไว้ เพื่อรู้ว่า event "end-file" ที่มาถึงนั้น
// เป็นการจบของตอนไหนในลิสต์ และรู้ว่า mpv ขยับไปเล่นไฟล์ไหนแล้วเพื่อสั่ง seek ให้ถูกไฟล์
//
// ข้อควรรู้: อาศัยสมมติฐานว่า mpv ส่ง event "end-file" ก่อนที่ "playlist-pos" จะขยับไปตอนถัดไป
// (ซึ่งเป็นลำดับปกติของ mpv) ถ้า mpv บางเวอร์ชันส่งไม่ตรงลำดับนี้ อาจมีโอกาสจับผิดตอนได้
func (s *appState) playSeriesTracked(series *Series) {
	var eps []*Episode
	var paths []string
	for _, ep := range series.Episodes {
		if ep.Exists && !ep.Watched {
			eps = append(eps, ep)
			paths = append(paths, ep.FilePath)
		}
	}
	if len(paths) == 0 {
		dialog.ShowInformation("เล่นซีรีส์", "ไม่มีตอนที่ยังไม่ได้ดูให้เล่นแล้ว", s.win)
		return
	}

	sockPath := filepath.Join(os.TempDir(), fmt.Sprintf("videotracker-mpv-series-%d.sock", time.Now().UnixNano()))

	// เล่นเรียงตามลำดับปกติเสมอ ไม่ข้ามไฟล์ไหนไปก่อน (ไม่ใช้ --playlist-start/--start)
	// ถ้าไฟล์ไหนมีจุดค้าง จะสั่ง seek ผ่าน IPC ตอนที่ mpv เล่นมาถึงไฟล์นั้นจริง ๆ แทน (ดูด้านล่าง)
	args := []string{"--input-ipc-server=" + sockPath, "--force-window=yes"}
	args = append(args, paths...)

	cmd := exec.Command("mpv", args...)
	if err := cmd.Start(); err != nil {
		dialog.ShowError(fmt.Errorf("เปิด mpv ไม่สำเร็จ: %w", err), s.win)
		return
	}

	go func() {
		defer os.Remove(sockPath)

		conn := dialMPVSocket(sockPath)
		if conn == nil {
			_ = cmd.Wait()
			return
		}
		defer conn.Close()

		observePos, err := json.Marshal(map[string]interface{}{
			"command": []interface{}{"observe_property", 1, "time-pos"},
		})
		if err == nil {
			_, _ = conn.Write(append(observePos, '\n'))
		}
		observePlaylistPos, err := json.Marshal(map[string]interface{}{
			"command": []interface{}{"observe_property", 2, "playlist-pos"},
		})
		if err == nil {
			_, _ = conn.Write(append(observePlaylistPos, '\n'))
		}

		currentIdx := 0
		var lastPos float64
		seekedFor := -1 // index ที่สั่ง seek ไปแล้ว กันสั่งซ้ำ

		// finalizeEpisode บันทึกผลของตอนที่เพิ่งเล่นจบ (idx) เหมือน logic ของ playEpisodeTracked
		finalizeEpisode := func(idx int, reason string, pos float64) {
			if idx < 0 || idx >= len(eps) {
				return
			}
			ep := eps[idx]
			if reason == "eof" {
				ep.Watched = true
				ep.ResumeSeconds = 0
			} else if pos > resumeThresholdSeconds {
				ep.ResumeSeconds = pos
			} else {
				ep.ResumeSeconds = 0
			}
			if err := SaveLibrary(s.lib); err != nil {
				dialog.ShowError(err, s.win)
			}
			s.seriesList.Refresh()
			s.episodeList.Refresh()
		}

		// seekToResumeIfNeeded สั่ง mpv seek ไปจุดค้างของไฟล์ที่ idx (ถ้ามี และยังไม่เคยสั่งสำหรับ idx นี้)
		seekToResumeIfNeeded := func(conn net.Conn, idx int) {
			if idx < 0 || idx >= len(eps) || idx == seekedFor {
				return
			}
			seekedFor = idx
			if eps[idx].ResumeSeconds <= 1 {
				return
			}
			seekCmd, err := json.Marshal(map[string]interface{}{
				"command": []interface{}{"seek", eps[idx].ResumeSeconds, "absolute"},
			})
			if err == nil {
				_, _ = conn.Write(append(seekCmd, '\n'))
			}
		}

		// เผื่อไฟล์แรกสุด (index 0) มีจุดค้างด้วย เช็คทันทีตั้งแต่เริ่ม ไม่ต้องรอ event playlist-pos
		seekToResumeIfNeeded(conn, currentIdx)

		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			var msg mpvIPCMessage
			if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
				continue
			}
			switch msg.Event {
			case "property-change":
				switch msg.Name {
				case "time-pos":
					if f, ok := msg.Data.(float64); ok {
						lastPos = f
					}
				case "playlist-pos":
					if f, ok := msg.Data.(float64); ok {
						currentIdx = int(f)
						seekToResumeIfNeeded(conn, currentIdx)
					}
				}
			case "end-file":
				finalizeEpisode(currentIdx, msg.Reason, lastPos)
				lastPos = 0
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "videotracker: mpv IPC read error: %v\n", err)
		}

		_ = cmd.Wait()
	}()
}
