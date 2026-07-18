package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// dataFilePath คืน path ของไฟล์ข้อมูล เช่น ~/.config/videotracker/data.json
func dataFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(configDir, "videotracker")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "data.json"), nil
}

// LoadLibrary โหลดข้อมูลที่เคยเซฟไว้ ถ้ายังไม่มีไฟล์จะคืน Library เปล่า
func LoadLibrary() (*Library, error) {
	path, err := dataFilePath()
	if err != nil {
		return nil, err
	}
	lib := &Library{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return lib, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, lib); err != nil {
		return nil, err
	}
	return lib, nil
}

// SaveLibrary เซฟข้อมูลปัจจุบันลงไฟล์
func SaveLibrary(lib *Library) error {
	path, err := dataFilePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(lib, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
