package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"dlib/dbus"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
)

func quoteString(str string) string {
	return strconv.Quote(str)
}

func unquoteString(str string) string {
	if strings.HasPrefix(str, `"`) && strings.HasSuffix(str, `"`) {
		s, _ := strconv.Unquote(str)
		return s
	} else if strings.HasPrefix(str, `'`) && strings.HasSuffix(str, `'`) {
		return str[1 : len(str)-1]
	}
	return str
}

// TODO move to dde-api/os
func execAndWait(timeout int, name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Start()
	if err != nil {
		logError(err.Error())
		return err
	}

	// wait for process finish
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(time.Duration(timeout) * time.Second):
		if err := cmd.Process.Kill(); err != nil {
			logError(err.Error())
			return err
		}
		<-done
		logInfo("time out and process was killed")
	case err := <-done:
		logInfo("process output: %s", stdout.String())
		if err != nil {
			logError("process error output: %s", stderr.String())
			logError("process done with error = %v", err)
			return err
		}
	}
	return nil
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// TODO move dde-api/file
func unTarGz(archiveFile string, destDir string, prefix string) error {
	destDir = path.Clean(destDir) + string(os.PathSeparator)

	// open the archive file
	fr, err := os.Open(archiveFile)
	if err != nil {
		return err
	}
	defer fr.Close()

	// create a gzip reader
	gr, err := gzip.NewReader(fr)
	if err != nil {
		return err
	}
	defer gr.Close()

	// create a tar reader
	tr := tar.NewReader(gr)

	// loop files
	for hdr, err := tr.Next(); err != io.EOF; hdr, err = tr.Next() {
		if err != nil {
			return err
		}

		if !strings.HasPrefix(hdr.Name, prefix) {
			continue
		}

		fi := hdr.FileInfo()
		destFullPath := destDir + hdr.Name
		logInfo("UnTarGzing file: " + hdr.Name)

		if hdr.Typeflag == tar.TypeDir {
			// create dir
			os.MkdirAll(destFullPath, fi.Mode().Perm())
			os.Chmod(destFullPath, fi.Mode().Perm())
		} else {
			// create the parent dir for file
			os.MkdirAll(path.Dir(destFullPath), fi.Mode().Perm())

			// write data to file
			fw, err := os.Create(destFullPath)
			if err != nil {
				return err
			}
			_, err = io.Copy(fw, tr)
			if err != nil {
				return err
			}
			fw.Close()

			os.Chmod(destFullPath, fi.Mode().Perm())
		}
	}
	return nil
}

// find if a file in archive and return its path
func findFileInTarGz(archiveFile string, targetFile string) (string, error) {
	// open the archive file
	fr, err := os.Open(archiveFile)
	if err != nil {
		return "", err
	}
	defer fr.Close()

	// create a gzip reader
	gr, err := gzip.NewReader(fr)
	if err != nil {
		return "", err
	}
	defer gr.Close()

	// create a tar reader
	tr := tar.NewReader(gr)

	// loop files
	targetPath := ""
	for hdr, err := tr.Next(); err != io.EOF; hdr, err = tr.Next() {
		if err != nil {
			return "", err
		}

		if hdr.Typeflag != tar.TypeDir && strings.HasSuffix(hdr.Name, targetFile) {
			targetPath = hdr.Name
			break
		}
	}
	return targetPath, nil
}

func isFileExists(file string) bool {
	if _, err := os.Stat(file); err == nil {
		return true
	} else {
		return false
	}
}

func copyFile(src, dest string) (written int64, err error) {
	if dest == src {
		return -1, newError("source and destination are same file")
	}

	sf, err := os.Open(src)
	if err != nil {
		return
	}
	defer sf.Close()
	df, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return
	}
	defer df.Close()
	return io.Copy(df, sf)
}

func getPathLevel(p string) int {
	p = path.Clean(p)
	if len(p) == 0 {
		return 0
	}
	lv := len(strings.Split(p, string(os.PathSeparator)))
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, ".") {
		lv--
	}
	return lv
}

func newError(format string, v ...interface{}) error {
	return errors.New(fmt.Sprintf(format, v...))
}

func dbusGetSessionObject(dest, path string) (obj *dbus.Object, err error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		logError(err.Error())
		return
	}
	obj = conn.Object(dest, dbus.ObjectPath(path))
	var v string
	obj.Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&v)
	if strings.Index(v, dest) == -1 {
		return nil, errors.New(fmt.Sprintf("'%s' hasn't interface '%s'.", path, dest))
	}
	return
}

func dbusCallMethod(dest, path, method string, args ...interface{}) (call *dbus.Call, err error) {
	obj, err := dbusGetSessionObject(dest, path)
	if err != nil {
		return
	}
	logInfo("call dbus method: %s, %s, %s", dest, path, method, args)
	call = obj.Call(method, 0, args...)
	return
}

func dbusGetProperty(dest, path, property string) (value interface{}, err error) {
	obj, err := dbusGetSessionObject(dest, path)
	if err != nil {
		return
	}
	var v dbus.Variant
	err = obj.Call("org.freedesktop.DBus.Properties.Get", 0, dest, property).Store(&v)
	if err != nil {
		logError(err.Error())
		return
	}
	value = v.Value()
	logInfo("get property success: %s", v.String())
	return
}

func getPrimaryScreenBestResolution() (w int32, h int32) {
	w, h = 1024, 768 // default value

	// get primary output
	destDisplay := "com.deepin.daemon.Display"
	pathDisplay := "/com/deepin/daemon/Display"
	propertyDisplay := "PrimaryOutput"
	primaryOutput, err := dbusGetProperty(destDisplay, pathDisplay, propertyDisplay)
	if err != nil {
		logError("get primary output failed, use default value 1024x768") // TODO
		return
	}

	// get support modes
	destOutput := "com.deepin.daemon.Display"
	pathOutput := string(primaryOutput.(dbus.ObjectPath))
	methodOutput := "com.deepin.daemon.Display.Output.ListModes"
	call, err := dbusCallMethod(destOutput, pathOutput, methodOutput)
	if err != nil {
		logError("get output's modes failed, use default value 1024x768") // TODO
		return
	}

	type Mode struct {
		ID     uint32
		Width  uint16
		Height uint16
		Rate   float64
	}
	modes := make([]Mode, 0)
	err = call.Store(&modes)
	if err != nil {
		logError("get output's modes failed, use default value 1024x768") // TODO
		return
	}

	// get the best resolution
	w, h = int32(modes[0].Width), int32(modes[0].Height)

	logInfo("primary screen's best resolution is %dx%d", w, h)
	return
}

func getImgClipSizeByResolution(screenWidth, screenHeight, imgWidth, imgHeight int32) (w int32, h int32) {
	if imgWidth >= screenWidth && imgHeight >= screenHeight {
		w = screenWidth
		h = screenHeight
	} else {
		scale := float32(screenWidth) / float32(screenHeight)
		w = imgWidth
		h = int32(float32(w) / scale)
		if h > imgHeight {
			h = imgHeight
			w = int32(float32(h) * scale)
		}
	}
	return
}
