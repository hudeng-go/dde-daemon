/**
 * Copyright (C) 2014 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package dock

import (
	"github.com/BurntSushi/xgb/xproto"
	"pkg.deepin.io/lib/dbus"
	"sort"
	"sync"
)

const (
	entryDBusObjPathPrefix = dockManagerDBusObjPath + "/entries/"
	entryDBusInterface     = dockManagerDBusInterface + ".Entry"

	FieldTitle   = "title"
	FieldIcon    = "icon"
	FieldMenu    = "menu"
	FieldAppXids = "app-xids"

	FieldStatus   = "app-status"
	ActiveStatus  = "active"
	NormalStatus  = "normal"
	InvalidStatus = "invalid"
)

type XidInfo struct {
	Xid   uint32
	Title string
}

type AppEntry struct {
	dockManager *DockManager

	Id      string
	innerId string

	IsActive bool
	Name     string
	Icon     string
	Menu     string

	WindowTitles windowTitlesType
	windows      map[xproto.Window]*WindowInfo

	current       *WindowInfo
	CurrentWindow xproto.Window
	windowMutex   sync.Mutex

	coreMenu  *Menu
	appInfo   *AppInfo
	IsDocked  bool
	dockMutex sync.Mutex
}

func newAppEntry(dockManager *DockManager, id string, appInfo *AppInfo) *AppEntry {
	entry := &AppEntry{
		dockManager:  dockManager,
		Id:           dockManager.allocEntryId(),
		innerId:      id,
		WindowTitles: newWindowTitles(),
		windows:      make(map[xproto.Window]*WindowInfo),
		appInfo:      appInfo,
	}
	return entry
}

func (entry *AppEntry) setAppInfo(newAppInfo *AppInfo) {
	if entry.appInfo == newAppInfo {
		logger.Debug("setAppInfo failed: old == new")
		return
	}
	if entry.appInfo != nil {
		entry.appInfo.Destroy()
	}
	entry.appInfo = newAppInfo
}

func (entry *AppEntry) hasWindow() bool {
	return len(entry.windows) != 0
}

func (entry *AppEntry) getExec(oneLine bool) string {
	if entry.current == nil {
		return ""
	}
	winProcess := entry.current.process
	if winProcess != nil {
		if oneLine {
			return winProcess.GetOneCommandLine()
		} else {
			return winProcess.GetShellScriptLines()
		}
	}
	return ""
}

func (entry *AppEntry) getDisplayName() string {
	if entry.appInfo != nil {
		return entry.appInfo.GetDisplayName()
	}
	if entry.current != nil {
		return entry.current.getDisplayName()
	}
	return ""
}

func (e *AppEntry) setCurrentWindow(win xproto.Window) {
	if e.CurrentWindow != win {
		e.CurrentWindow = win
		logger.Debug("setCurrentWindow", win)
		dbus.NotifyChange(e, "CurrentWindow")
	}
}

func (entry *AppEntry) setCurrentWindowInfo(winInfo *WindowInfo) {
	entry.current = winInfo
	if winInfo == nil {
		entry.setCurrentWindow(0)
	} else {
		entry.setCurrentWindow(winInfo.window)
	}
}

func (entry *AppEntry) findNextLeader() xproto.Window {
	winSlice := make(windowSlice, 0, len(entry.windows))
	for win, _ := range entry.windows {
		winSlice = append(winSlice, win)
	}
	sort.Sort(winSlice)
	currentWin := entry.current.window
	logger.Debug("sorted window slice:", winSlice)
	logger.Debug("current window:", currentWin)
	currentIndex := -1
	for i, win := range winSlice {
		if win == currentWin {
			currentIndex = i
		}
	}
	if currentIndex == -1 {
		logger.Warning("findNextLeader unexpect, return 0")
		return 0
	}
	// if current window is max, return min: winSlice[0]
	// else return winSlice[currentIndex+1]
	nextIndex := 0
	if currentIndex < len(winSlice)-1 {
		nextIndex = currentIndex + 1
	}
	logger.Debug("next window:", winSlice[nextIndex])
	return winSlice[nextIndex]
}

func (entry *AppEntry) attachWindow(winInfo *WindowInfo) {
	win := winInfo.window
	logger.Debugf("attach win %v to entry", win)

	winInfo.entry = entry
	if _, ok := entry.windows[win]; ok {
		logger.Debugf("win %v is already attach to entry", win)
		return
	}

	entry.windows[win] = winInfo
	entry.updateWindowTitles()
	entry.updateIsActive()

	if (entry.dockManager != nil && win == entry.dockManager.activeWindow) ||
		entry.current == nil {
		entry.setCurrentWindowInfo(winInfo)
		winInfo.updateWmName()
		winInfo.updateIcon()
	}
}

// return is detached
func (entry *AppEntry) detachWindow(winInfo *WindowInfo) bool {
	win := winInfo.window
	logger.Debug("detach window ", win)
	if _, ok := entry.windows[win]; ok {
		delete(entry.windows, win)
		if len(entry.windows) == 0 {
			return true
		}
		for _, winInfo := range entry.windows {
			// select first
			entry.setCurrentWindowInfo(winInfo)
			break
		}
		return true
	}
	logger.Debug("detachWindow failed: window not attach with entry")
	return false
}

func (entry *AppEntry) destroy() {
	entry.dockManager = nil
	if entry.appInfo != nil {
		entry.appInfo.Destroy()
		entry.appInfo = nil
	}
}

func (e *AppEntry) setName(name string) {
	if e.Name != name {
		e.Name = name
		dbus.NotifyChange(e, "Name")
	}
}

func (e *AppEntry) setIcon(icon string) {
	if e.Icon != icon {
		e.Icon = icon
		dbus.NotifyChange(e, "Icon")
	}
}

func (e *AppEntry) setIsActive(isActive bool) {
	if e.IsActive != isActive {
		e.IsActive = isActive
		dbus.NotifyChange(e, "IsActive")
	}
}

func (e *AppEntry) setIsDocked(isDocked bool) {
	if e.IsDocked != isDocked {
		e.IsDocked = isDocked
		dbus.NotifyChange(e, "IsDocked")
	}
}

func (entry *AppEntry) updateName() {
	var name string
	if entry.appInfo != nil {
		name = entry.appInfo.GetDisplayName()
	} else if entry.current != nil {
		name = entry.current.getDisplayName()
	} else {
		logger.Debug("updateName failed")
		return
	}
	entry.setName(name)
}

func (entry *AppEntry) updateIcon() {
	var icon string
	if entry.hasWindow() {
		icon = entry.current.getIcon()
	} else {
		icon = entry.appInfo.GetIcon()
	}
	entry.setIcon(icon)
}

func (e *AppEntry) updateWindowTitles() {
	windowTitles := newWindowTitles()
	for win, winInfo := range e.windows {
		windowTitles[win] = winInfo.Title
	}
	if !e.WindowTitles.Equal(windowTitles) {
		e.WindowTitles = windowTitles
		dbus.NotifyChange(e, "WindowTitles")
	}
}

func (e *AppEntry) updateIsActive() {
	if e.dockManager == nil {
		return
	}
	_, ok := e.windows[e.dockManager.activeWindow]
	e.setIsActive(ok)
}
