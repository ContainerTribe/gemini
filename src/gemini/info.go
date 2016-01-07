// +build linux,cgo

package gemini  

type info struct {
	ID string
	driver *driver
}

func (i *info) IsRunning() bool {
	_, ok := i.driver.activeContainers[i.ID]
	return ok
}
