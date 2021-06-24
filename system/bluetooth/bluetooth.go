/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     weizhixiang <weizhixiang@uniontech.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package bluetooth

import (
	"os/exec"
	"pkg.deepin.io/dde/daemon/loader"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/dbusutil/gsprop"
	"pkg.deepin.io/lib/log"
	gio "pkg.deepin.io/gir/gio-2.0"
	"strconv"
)

const (
	dbusServiceName = "com.deepin.daemon.Bluetooth"
	dbusPath        = "/com/deepin/daemon/Bluetooth"
	dbusInterface   = "com.deepin.daemon.Bluetooth"
	gsSchemaBluetooth = "com.deepin.dde.bluetooth"
	bluetoothSwitch = "bluetooth-switch"

	bluetoothInitScript = "/usr/share/dde-daemon/bluetooth/bluetooth_init.sh"
)

type Bluetooth struct {
	service 		*dbusutil.Service
	settings     	*gio.Settings
	BluetoothSwitch	gsprop.Bool `prop:"access:r"`
}

func newBluetooth(service *dbusutil.Service) (b *Bluetooth) {
	b = &Bluetooth{
		service: service,
		settings: gio.NewSettings(gsSchemaBluetooth),
	}
	b.BluetoothSwitch.Bind(b.settings, bluetoothSwitch)

	return
}

func (b *Bluetooth) bluetoothInit() {
	initArg := 0
	if b.BluetoothSwitch.Get() {
		initArg = 1
	}

	err := exec.Command("/bin/sh",  bluetoothInitScript, strconv.Itoa(initArg)).Run()
	if err != nil {
		logger.Error("bluetoothInit err: ", err)
	}
}

func (b *Bluetooth) destory() {
	b.settings.Unref()
}

func (d *Bluetooth) GetInterfaceName() string {
	return dbusInterface
}

type Daemon struct {
	*loader.ModuleBase
}

var (
	_m     *Bluetooth
	logger = log.NewLogger(dbusServiceName)
)

func init() {
	loader.Register(NewDaemon())
}

func NewDaemon() *Daemon {
	daemon := new(Daemon)
	daemon.ModuleBase = loader.NewModuleBase("bluetooth", daemon, logger)
	return daemon
}

func (d *Daemon) GetDependencies() []string {
	return []string{}
}

func (d *Daemon) Start() error {
	logger.BeginTracing()
	logger.Info("start bluetooth daemon")
	service := loader.GetService()
	_m = newBluetooth(service)
	err := service.Export(dbusPath, _m)
	if err != nil {
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}

	go _m.bluetoothInit()

	/*
	err = d.Stop()
	if err != nil {
		return err
	}
	 */
	return nil
}

func (d *Daemon) Stop() error {
	logger.Info("stop bluetooth daemon")
	if _m == nil {
		return nil
	}

	service := loader.GetService()
	err := service.StopExport(_m)
	if err != nil {
		return err
	}

	_m.destory()
	_m = nil
	return nil
}