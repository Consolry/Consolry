//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"

	"fyne.io/systray"
	"golang.org/x/sys/windows/registry"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// setAutostart adds or removes Consolry from the programs Windows starts when you sign in.
// It only touches the current user's own setting, so no administrator rights are needed.
func setAutostart(on bool, home string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if !on {
		if err := key.DeleteValue("Consolry"); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return key.SetStringValue("Consolry", fmt.Sprintf(`"%s" -no-browser -dir "%s"`, exe, home))
}

func autostartOn() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	_, _, err = key.GetStringValue("Consolry")
	return err == nil
}

// waitForExit shows an icon in the notification area and returns when Consolry should quit.
func waitForExit(ctx context.Context, stop context.CancelFunc, address, home string) {
	systray.Run(func() {
		systray.SetIcon(trayIcon())
		systray.SetTitle("Consolry")
		systray.SetTooltip("Consolry is running your servers")

		open := systray.AddMenuItem("Open Consolry", "Show the panel in your browser")
		startup := systray.AddMenuItemCheckbox("Start when I sign in", "Run Consolry automatically", autostartOn())
		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit and stop servers", "Save and close every server, then exit")

		go func() {
			for {
				select {
				case <-open.ClickedCh:
					openBrowser(address)
				case <-startup.ClickedCh:
					if err := setAutostart(!startup.Checked(), home); err == nil {
						if startup.Checked() {
							startup.Uncheck()
						} else {
							startup.Check()
						}
					}
				case <-quit.ClickedCh:
					stop()
				case <-ctx.Done():
					systray.Quit()
					return
				}
			}
		}()
	}, func() {})
}

// trayIcon draws the Consolry mark, a prompt on a blue block, as a 32-pixel Windows icon.
func trayIcon() []byte {
	const size = 32
	block := [4]byte{0xc9, 0x68, 0x12, 0xff} // blue, stored blue-green-red-alpha
	mark := [4]byte{0xff, 0xff, 0xff, 0xff}
	marks := [][4]int{{8, 10, 4, 4}, {12, 14, 4, 4}, {8, 18, 4, 4}, {18, 18, 8, 4}}

	pixel := func(x, y int) [4]byte {
		corner := (x < 4 || x >= size-4) && (y < 4 || y >= size-4)
		if corner {
			return [4]byte{}
		}
		for _, m := range marks {
			if x >= m[0] && x < m[0]+m[2] && y >= m[1] && y < m[1]+m[3] {
				return mark
			}
		}
		return block
	}

	var image bytes.Buffer
	// Icon images are stored bottom row first.
	for y := size - 1; y >= 0; y-- {
		for x := 0; x < size; x++ {
			p := pixel(x, y)
			image.Write(p[:])
		}
	}
	image.Write(make([]byte, size*size/8)) // the old-style transparency mask; alpha above does the work

	var out bytes.Buffer
	write := func(values ...any) {
		for _, value := range values {
			_ = binary.Write(&out, binary.LittleEndian, value)
		}
	}
	write(uint16(0), uint16(1), uint16(1))                                          // one icon in the file
	write(uint8(size), uint8(size), uint8(0), uint8(0), uint16(1), uint16(32))      // its size and colour depth
	write(uint32(40+image.Len()), uint32(22))                                       // length and where it starts
	write(uint32(40), int32(size), int32(size*2), uint16(1), uint16(32), uint32(0)) // bitmap header
	write(uint32(image.Len()), int32(0), int32(0), uint32(0), uint32(0))
	out.Write(image.Bytes())
	return out.Bytes()
}
