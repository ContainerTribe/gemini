package main

import (
	"io/ioutil"
	"math/rand"
	"os"
	"syscall"
	"time"
)

func isDirExists(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return os.IsExist(err)
	} else {
		return fi.IsDir()
	}
}

func mountBasicFilesystem() error {
	os.Mkdir("/tmp", 0755)
	sources := []string{"sysfs", "proc", "devtmpfs", "none"}
	targets := []string{"/sys", "/proc", "/dev", "/sys/fs/cgroup"}
	fstypes := []string{"sysfs", "proc", "devtmpfs", "cgroup"}

	// mount
	for index, source := range sources {
		if !isDirExists(targets[index]) {
			if err := os.Mkdir(targets[index], 0755); err != nil {
				return err
			}
		}
		if err := syscall.Mount(source, targets[index], fstypes[index], 0, ""); err != nil {
			return err
		}
	}
	return nil
}

func randomString(strlen int) string {
	rand.Seed(time.Now().UTC().UnixNano())
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, strlen)
	for i := 0; i < strlen; i++ {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

func openSerialPort(name string) (*os.File, error) {
	f, err := os.OpenFile(name, syscall.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func listDir(dirPth string, suffix string) (files []string, err error) {
	files = make([]string, 0, 10)
	dir, err := ioutil.ReadDir(dirPth)
	if err != nil {
		return nil, err
	}
	for _, fi := range dir {
		files = append(files, fi.Name())
	}
	return files, nil
}
