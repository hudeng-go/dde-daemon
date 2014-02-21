/**
 * Copyright (c) 2011 ~ 2013 Deepin, Inc.
 *               2011 ~ 2013 jouyouyun
 *
 * Author:      jouyouyun <jouyouwen717@gmail.com>
 * Maintainer:  jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, see <http://www.gnu.org/licenses/>.
 **/

package main

import (
        "fmt"
        "github.com/howeyc/fsnotify"
)

func listenFileChanged(filename string) {
        watcher, err := fsnotify.NewWatcher()
        if err != nil {
                fmt.Println("New Watcher Failed:", err)
                panic(err)
        }

        err = watcher.Watch(filename)
        if err != nil {
                fmt.Printf("Watch '%s' failed: %s\n", filename, err)
                panic(err)
        }

        go func() {
                defer watcher.Close()
                for {
                        select {
                        case ev := <-watcher.Event:

                                if ev.IsDelete() {
                                        watcher.Watch(filename)
                                } else {
                                        updateUserList()
                                }
                        case err := <-watcher.Error:
                                fmt.Println("Watch Error:", err)
                        }
                }
        }()
}
