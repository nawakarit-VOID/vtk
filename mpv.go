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
